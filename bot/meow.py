import html
import math
import re

import aiohttp
from mautrix.types import EventType, Format, MessageType, TextMessageEventContent
from mautrix.util.config import BaseProxyConfig, ConfigUpdateHelper
from maubot import Plugin, MessageEvent
from maubot.handlers import event


SUPPORTED_LANGUAGES = frozenset(
    {
        "en", "fr", "ja", "tr", "ru", "uk", "zh-hans", "zh-hant",
        "es", "nl", "de", "hy", "eo", "ar", "hi",
    }
)
SUPPORTED_LANGUAGE_LABELS = "auto, ar, de, en, eo, es, fr, hi, hy, ja, nl, ru, tr, uk, zh-Hans, zh-Hant"
_VERDICT = r"(?:true|correct|accurate|real|right|valid)"
_POINTER = r"(?:this|that)"
MEOW_BALL_PHRASE = re.compile(
    r"(?:"
    rf"is\s*{_POINTER}\s*{_VERDICT}"
    r"|fact[\s-]*check(?:\s*(?:this|that))?"
    r"|check\s*(?:the\s*)?facts?"
    rf"|^is\s+{_POINTER}\b\s*[:.]?\s*"
    rf"|(?:^|\s)is\s+{_POINTER}\b\s*\??\s*$"
    rf"|^{_POINTER}\s+{_VERDICT}\b\s*[:.]?\s*"
    rf"|(?:^|\s){_POINTER}\s+{_VERDICT}\b\s*\??\s*$"
    rf"|^{_VERDICT}\b\s*[:.]?\s*"
    rf"|(?:^|\s){_VERDICT}\b\s*\??\s*$"
    r")",
    re.IGNORECASE,
)
LANGUAGE_OPTION = re.compile(r"^--(detect|reply)=([A-Za-z0-9-]+)$")
PASTEL_RED = "#D98792"
PASTEL_AMBER = "#E1B97D"
PASTEL_GREEN = "#7FC4A3"


MESSAGES = {
    "help": (
        "Meow Help:\n"
        "- Ping to generate a meow (English by default).\n"
        "- Ping with text (or reply to a message) for meownalysis.\n"
        "- Passive replies in target rooms detect the meow language automatically.\n"
        "- Detection defaults to auto. Passive replies use the detected language. Use leading --detect=<tag> and --reply=<tag> options for this ping only.\n"
        "- Ping with a fact-check phrase to consult the Meow-Ball; reply with a fact-check phrase or meow? to check the replied-to message.\n"
        "- Example: @meow --detect=fr --reply=ja miaou"
    ),
    "option_usage": f"Use each leading option at most once: --detect=<tag> and --reply=<tag>. Tags: {SUPPORTED_LANGUAGE_LABELS}. --reply selects generated meows only.",
    "bad_language": f"Unsupported language '{{language}}'. Supported languages: {SUPPORTED_LANGUAGE_LABELS}.",
    "api_language_error": "This MaaS server does not support language '{language}' yet. Update the API to a version with /languages support.",
    "outage": "MaaS is unavailable right now. Try again later.",
    "verdict_meow": "Verdict: Meow.",
    "verdict_like": "Verdict: Meow-like.",
    "verdict_no": "Verdict: Not meow.",
}


class UnsupportedLanguageError(Exception):
    def __init__(self, language: str):
        super().__init__(language)
        self.language = language


class Config(BaseProxyConfig):
    def do_update(self, helper: ConfigUpdateHelper) -> None:
        helper.copy("maas_url")
        helper.copy("target_rooms")
        helper.copy("detect_language")
        helper.copy("response_language")
        helper.copy("room_languages")
        helper.copy("strict_min_score")
        helper.copy("fuzzy_min_score")
        helper.copy("embedded_min_coverage")


class Meow(Plugin):
    @classmethod
    def get_config_class(cls) -> type[BaseProxyConfig]:
        return Config

    async def start(self) -> None:
        self.config.load_and_update()
        self._global_detect_language = self._valid_language(
            self.config.get("detect_language", "auto"), "detect"
        )
        self._global_response_language = self._valid_language(
            self.config.get("response_language", "auto"), "response"
        )
        self._strict_min_score = self._valid_threshold("strict_min_score", 75)
        self._fuzzy_min_score = self._valid_threshold("fuzzy_min_score", 55)
        self._embedded_min_coverage = self._valid_threshold("embedded_min_coverage", 25)
        self._room_language_cache: dict[str, tuple[str, str]] = {}
        room_languages = self.config.get("room_languages", {}) or {}
        if not isinstance(room_languages, dict):
            self.log.error("room_languages must be a mapping.")
            return
        for room_id, overrides in room_languages.items():
            detect = self._global_detect_language
            response = self._global_response_language
            if not isinstance(overrides, dict):
                self.log.error("Room language override for %s must be a mapping.", room_id)
                continue
            if "detect" in overrides:
                detect = self._valid_language(
                    overrides["detect"], f"detect for room {room_id}", detect
                )
            if "response" in overrides:
                response = self._valid_language(
                    overrides["response"], f"response for room {room_id}", response
                )
            self._room_language_cache[str(room_id)] = detect, response

    def _valid_language(self, value: object, setting: str, fallback: str = "auto") -> str:
        language = "auto" if value is None else str(value).strip().lower()
        if language != "auto" and language not in SUPPORTED_LANGUAGES:
            self.log.error(
                "Invalid %s language %r; supported languages are %s. Using %s.",
                setting,
                value,
                SUPPORTED_LANGUAGE_LABELS,
                fallback,
            )
            return fallback
        return language

    def _valid_threshold(self, setting: str, minimum: float) -> float:
        value = self.config.get(setting, minimum)
        try:
            threshold = float(value) if not isinstance(value, bool) else float("nan")
        except (TypeError, ValueError):
            threshold = float("nan")
        if not math.isfinite(threshold) or threshold > 100:
            self.log.error("Invalid %s %r; using %s.", setting, value, minimum)
            return minimum
        if threshold < minimum:
            self.log.warning(
                "%s cannot be below the API minimum %s; using %s.",
                setting, minimum, minimum,
            )
            return minimum
        return threshold

    @staticmethod
    def _percentage(value: object) -> float:
        return float(str(value).rstrip("%"))

    @staticmethod
    def _score_color(score: float) -> str:
        if not math.isfinite(score):
            return "#A99FB3"
        score = max(0.0, min(score, 100.0))
        low, high, fraction = (
            ((217, 135, 146), (225, 185, 125), score / 50)
            if score <= 50
            else ((225, 185, 125), (127, 196, 163), (score - 50) / 50)
        )
        return "#{:02X}{:02X}{:02X}".format(
            *(round(start + (end - start) * fraction) for start, end in zip(low, high))
        )

    @staticmethod
    def _colored(text: str, color: str) -> str:
        return f'<span data-mx-color="{color}">{html.escape(text)}</span>'

    def _matches_strict(self, data: dict) -> bool:
        return bool(data.get("is_meow")) and (
            self._percentage(data["meow_percentage"]) >= self._strict_min_score
        )

    def _matches_fuzzy(self, data: dict) -> bool:
        if not data.get("is_meow_like"):
            return False
        if data.get("match_type") == "embedded":
            return float(data["coverage_percentage"]) >= self._embedded_min_coverage
        return self._percentage(data["meow_percentage"]) >= self._fuzzy_min_score

    def _resolve_languages(self, room_id: str) -> tuple[str, str]:
        return self._room_language_cache.get(
            room_id, (self._global_detect_language, self._global_response_language)
        )

    async def _maas_get_json(
        self, maas_url: str, path: str, params: dict[str, str] | None = None
    ) -> dict:
        async with self.http.get(f"{maas_url}{path}", params=params) as resp:
            try:
                resp.raise_for_status()
            except aiohttp.ClientResponseError as error:
                if error.status == 400 and params and "lang" in params:
                    try:
                        error_data = await resp.json()
                    except (aiohttp.ContentTypeError, ValueError):
                        error_data = {}
                    if error_data.get("error") == "unsupported language":
                        raise UnsupportedLanguageError(params["lang"]) from error
                raise
            data = await resp.json()
        if params and "lang" in params:
            actual_language = str(data.get("language", "")).lower()
            expected_language = params["lang"].lower()
            if expected_language == "auto":
                valid_language = actual_language == "auto" or actual_language in SUPPORTED_LANGUAGES
            else:
                valid_language = actual_language == expected_language
            if not valid_language:
                raise UnsupportedLanguageError(expected_language)
        return data

    async def _is_meow(self, maas_url: str, text: str, language: str = "auto") -> bool:
        data = await self._maas_get_json(
            maas_url, "/ismeow", {"text": text, "lang": language}
        )
        return self._matches_strict(data)

    def _extract_phrase_subject(self, stripped: str) -> str | None:
        match = MEOW_BALL_PHRASE.search(stripped)
        if not match:
            return None

        before = stripped[: match.start()].strip().rstrip("?").strip()
        after = stripped[match.end() :].strip().lstrip(":?. ").strip().rstrip("?").strip()

        if before and after:
            return before if len(before) >= len(after) else after
        if before:
            return before
        if after:
            return after
        return ""

    async def _parse_meow_ball(
        self,
        maas_url: str,
        ping_text: str,
        is_reply: bool,
        parent_text: str,
        detect_language: str = "auto",
    ) -> str | None:
        stripped = ping_text.strip()

        phrase_subject = self._extract_phrase_subject(stripped)
        if phrase_subject is not None:
            if is_reply:
                if not parent_text:
                    return None
                return phrase_subject if phrase_subject else parent_text
            return phrase_subject if phrase_subject else None

        if stripped.endswith("?"):
            candidate = stripped[:-1].strip()
            if candidate and await self._is_meow(maas_url, candidate, detect_language):
                if is_reply:
                    return parent_text if parent_text else None
                return candidate
        return None

    async def _reply_text(
        self,
        evt: MessageEvent,
        orig_evt: MessageEvent | None,
        is_reply: bool,
        body: str,
        formatted_body: str | None = None,
    ) -> None:
        content = TextMessageEventContent(msgtype=MessageType.TEXT, body=body)
        if formatted_body:
            content.format = Format.HTML
            content.formatted_body = formatted_body
        content.set_reply(orig_evt if (is_reply and orig_evt) else evt)
        await self.client.send_message(evt.room_id, content)

    def _strip_bot_mention(self, body: str, localpart: str) -> str:
        text = body.replace(self.client.mxid, "")
        text = re.sub(
            rf"(?<![\w])@{re.escape(localpart)}(?=$|[\s:,.!?])", "", text, count=1
        )
        text = text.strip()
        if text.startswith(":"):
            text = text[1:].strip()
        return text

    @staticmethod
    def _parse_language_options(text: str) -> tuple[str, dict[str, str], str | None]:
        remaining = text.strip()
        options: dict[str, str] = {}
        while remaining:
            pieces = remaining.split(None, 1)
            token = pieces[0]
            is_language_option = (
                token == "--detect"
                or token == "--reply"
                or token.startswith("--detect=")
                or token.startswith("--reply=")
            )
            if not is_language_option:
                break
            match = LANGUAGE_OPTION.fullmatch(token)
            if not match:
                return remaining, options, token
            name, language = match.groups()
            name = "detect" if name == "detect" else "response"
            if name in options:
                return remaining, options, token
            language = language.lower()
            if language != "auto" and language not in SUPPORTED_LANGUAGES:
                return remaining, options, f"language:{language}"
            options[name] = language
            remaining = pieces[1].lstrip() if len(pieces) > 1 else ""
        return remaining, options, None

    def _format_analysis(self, strict: dict, fuzzy: dict) -> tuple[str, str]:
        strict_match = self._matches_strict(strict)
        fuzzy_match = self._matches_fuzzy(fuzzy)
        if strict_match:
            verdict = MESSAGES["verdict_meow"]
        elif fuzzy_match:
            verdict = MESSAGES["verdict_like"]
        else:
            verdict = MESSAGES["verdict_no"]

        if strict_match:
            selected = strict
        elif fuzzy_match:
            selected = fuzzy
        elif self._percentage(strict["meow_percentage"]) > self._percentage(fuzzy["meow_percentage"]):
            selected = strict
        else:
            selected = fuzzy

        language = selected.get("language", "auto")
        if language == "auto":
            closest = selected.get("closest_language")
            language_line = f"Language: undetermined (closest: {closest})" if closest else "Language: undetermined"
        else:
            language_line = f"Language: {language}"

        family = selected.get("family", "unknown")
        match_type = selected.get("match_type", "none")
        if family == "unknown":
            sound_line = "Sound: none"
        else:
            sound_label = "Sound" if strict_match or fuzzy_match else "Closest sound"
            sound_line = f"{sound_label}: {family} · {match_type}"

        strict_score = strict["meow_percentage"]
        fuzzy_score = fuzzy["meow_percentage"]
        if strict_score == fuzzy_score:
            score_line = f"Strict / fuzzy score: {strict_score}"
        else:
            score_line = f"Strict: {strict_score} · Fuzzy: {fuzzy_score}"

        lines = [verdict, language_line, sound_line, score_line]
        matched_html = None
        if selected.get("matched_text"):
            matched_line = f"Matched: {selected['matched_text']!r}"
            matched_html = html.escape(matched_line)
            if selected.get("coverage_percentage") is not None:
                coverage = float(selected["coverage_percentage"])
                coverage_text = f"{coverage:.1f}%"
                matched_line += f" · {coverage_text} of letters"
                matched_html += " · " + self._colored(
                    coverage_text, self._score_color(coverage)
                ) + " of letters"
            lines.append(matched_line)

        if selected.get("match_type") == "embedded":
            fuzzy_limit = f"coverage ≥{self._embedded_min_coverage:g}%"
        else:
            fuzzy_limit = f"fuzzy ≥{self._fuzzy_min_score:g}%"
        diagnostics = [f"Limits: strict ≥{self._strict_min_score:g}%, {fuzzy_limit}"]
        if selected.get("squeezed_form"):
            normalized = str(selected["squeezed_form"])
            diagnostics.append(f"Normalized: {normalized[:80]}{'…' if len(normalized) > 80 else ''}")
        timings = [
            f"{name} {data['detection_time']}"
            for name, data in (("strict", strict), ("fuzzy", fuzzy))
            if data.get("detection_time")
        ]
        if timings:
            diagnostics.append("API: " + ", ".join(timings))

        footer = " · ".join(diagnostics)
        body = "\n".join(lines + ([footer] if footer else []))
        verdict_color = PASTEL_GREEN if strict_match else PASTEL_AMBER if fuzzy_match else PASTEL_RED
        if strict_score == fuzzy_score:
            score_html = "Strict / fuzzy score: " + self._colored(
                strict_score, self._score_color(self._percentage(strict_score))
            )
        else:
            score_html = "Strict: " + self._colored(
                strict_score, self._score_color(self._percentage(strict_score))
            ) + " · Fuzzy: " + self._colored(
                fuzzy_score, self._score_color(self._percentage(fuzzy_score))
            )
        formatted_lines = [
            f"<strong>{self._colored(verdict, verdict_color)}</strong>",
            html.escape(language_line),
            html.escape(sound_line),
            score_html,
        ]
        if matched_html:
            formatted_lines.append(matched_html)
        formatted = "<br>".join(formatted_lines)
        if footer:
            formatted += f'<br><sub><span data-mx-color="#8994A8">{html.escape(footer)}</span></sub>'
        return body, formatted

    @event.on(EventType.ROOM_MESSAGE)
    async def handle_message(self, evt: MessageEvent) -> None:
        if evt.sender == self.client.mxid or evt.content.msgtype != MessageType.TEXT:
            return
        relations = evt.content.get("m.relates_to") or {}
        if isinstance(relations, dict) and relations.get("rel_type") == "m.replace":
            return
        if evt.content.get("m.url") or evt.content.get("com.beeper.linkpreviews"):
            return

        if hasattr(evt.content, "trim_reply_fallback"):
            evt.content.trim_reply_fallback()

        body = evt.content.body.strip()
        maas_url = self.config["maas_url"]
        room_id = str(evt.room_id)
        detect_language, response_language = self._resolve_languages(room_id)
        localpart, _ = self.client.parse_user_id(self.client.mxid)

        is_pinged = False
        if "m.mentions" in evt.content:
            mentions_data = evt.content.get("m.mentions") or {}
            user_ids = (
                mentions_data.get("user_ids")
                if isinstance(mentions_data, dict)
                else getattr(mentions_data, "user_ids", None)
            )
            if user_ids and self.client.mxid in user_ids:
                is_pinged = True
        elif self.client.mxid in body or f"@{localpart}" in body:
            is_pinged = True

        if is_pinged:
            reply_to_event_id = evt.content.get_reply_to()
            ping_text = self._strip_bot_mention(body, localpart)
            ping_text, options, option_error = self._parse_language_options(ping_text)
            detect_language = options.get("detect", detect_language)
            response_language = options.get("response", response_language)

            is_reply = False
            orig_evt = None
            parent_text = ""

            if reply_to_event_id:
                try:
                    orig_evt = await self.client.get_event(evt.room_id, reply_to_event_id)
                    if orig_evt and orig_evt.content and orig_evt.content.body:
                        if hasattr(orig_evt.content, "trim_reply_fallback"):
                            orig_evt.content.trim_reply_fallback()
                        parent_text = orig_evt.content.body.strip()
                        is_reply = True
                except Exception as error:
                    self.log.exception(f"Failed to fetch replied-to event: {error}")

            if option_error:
                if option_error.startswith("language:"):
                    language = option_error.partition(":")[2]
                    option_reply = MESSAGES["bad_language"].format(language=language)
                    option_reply += "\n" + MESSAGES["option_usage"]
                else:
                    option_reply = MESSAGES["option_usage"]
                await self._reply_text(
                    evt,
                    orig_evt,
                    is_reply,
                    option_reply,
                )
                return

            normalized_ping = re.sub(r"[\W_]+", "", ping_text).lower()

            if normalized_ping == "help":
                await self._reply_text(
                    evt, orig_evt, is_reply, MESSAGES["help"]
                )
                return

            try:
                target_text = await self._parse_meow_ball(
                    maas_url, ping_text, is_reply, parent_text, detect_language
                )
                if target_text is not None:
                    analyzed_sender = (
                        orig_evt.sender if (is_reply and orig_evt) else evt.sender
                    )
                    ask_payload = f"{target_text} [{analyzed_sender}]"
                    data = await self._maas_get_json(
                        maas_url,
                        "/askmeow",
                        {"text": ask_payload},
                    )
                    await self._reply_text(evt, orig_evt, is_reply, str(data["answer"]))
                    return

                if not ping_text and not is_reply:
                    meow_language = "en" if response_language == "auto" else response_language
                    data = await self._maas_get_json(maas_url, "/meow", {"lang": meow_language})
                    await self._reply_text(evt, evt, False, str(data["meow"]))
                    return

                target_text = parent_text if is_reply else ping_text
                data_strict = await self._maas_get_json(
                    maas_url,
                    "/ismeow",
                    {"text": target_text, "lang": detect_language},
                )
                data_fuzzy = await self._maas_get_json(
                    maas_url,
                    "/meowlike",
                    {"text": target_text, "lang": detect_language},
                )
                analysis_body, analysis_html = self._format_analysis(data_strict, data_fuzzy)
                await self._reply_text(
                    evt,
                    orig_evt,
                    is_reply,
                    analysis_body,
                    analysis_html,
                )

            except UnsupportedLanguageError as error:
                await self._reply_text(
                    evt,
                    orig_evt,
                    is_reply,
                    MESSAGES["api_language_error"].format(
                        language=error.language
                    ),
                )
            except (aiohttp.ClientError, KeyError, ValueError) as error:
                self.log.exception(f"HTTP request failed during ping handling: {error}")
                await self._reply_text(evt, orig_evt, is_reply, MESSAGES["outage"])
            return

        target_rooms = self.config.get("target_rooms", [])
        if room_id in target_rooms:
            try:
                data_fuzzy = await self._maas_get_json(
                    maas_url,
                    "/meowlike",
                    {"text": body, "lang": "auto"},
                )
                if self._matches_fuzzy(data_fuzzy):
                    detected_language = str(data_fuzzy.get("language", "")).lower()
                    if detected_language not in SUPPORTED_LANGUAGES:
                        raise UnsupportedLanguageError(detected_language or "auto")
                    gen_data = await self._maas_get_json(
                        maas_url,
                        "/meow",
                        {"lang": str(data_fuzzy["language"])},
                    )
                    await evt.reply(gen_data["meow"])
            except UnsupportedLanguageError as error:
                self.log.error("MaaS API rejected configured language %s", error.language)
            except (aiohttp.ClientError, KeyError, ValueError) as error:
                self.log.exception(f"HTTP request failed during passive reply: {error}")
