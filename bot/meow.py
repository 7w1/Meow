import re

import aiohttp
from mautrix.types import EventType, MessageType, TextMessageEventContent
from mautrix.util.config import BaseProxyConfig, ConfigUpdateHelper
from maubot import Plugin, MessageEvent
from maubot.handlers import event

MAAS_OUTAGE_MSG = "MaaS is unavailable right now. Try again later."
MEOW_BALL_SUFFIX = re.compile(
    r"(?:"
    r"is\s*(?:this|that)\s*(?:true|correct|accurate|real|right|valid)"
    r"|fact[\s-]*check(?:\s*(?:this|that))?"
    r"|check\s*(?:the\s*)?facts?"
    r")\W*$",
    re.IGNORECASE,
)


class Config(BaseProxyConfig):
    def do_update(self, helper: ConfigUpdateHelper) -> None:
        helper.copy("maas_url")
        helper.copy("target_rooms")


class Meow(Plugin):
    @classmethod
    def get_config_class(cls) -> type[BaseProxyConfig]:
        return Config

    async def start(self) -> None:
        self.config.load_and_update()

    async def _maas_get_json(
        self, maas_url: str, path: str, params: dict[str, str] | None = None
    ) -> dict:
        async with self.http.get(f"{maas_url}{path}", params=params) as resp:
            resp.raise_for_status()
            return await resp.json()

    async def _is_meow(self, maas_url: str, text: str) -> bool:
        data = await self._maas_get_json(maas_url, "/ismeow", {"text": text})
        return bool(data.get("is_meow"))

    async def _detect_8ball(self, maas_url: str, ping_text: str) -> bool:
        stripped = ping_text.strip()
        match = MEOW_BALL_SUFFIX.search(stripped)
        if match:
            prefix = stripped[: match.start()].strip().rstrip("?").strip()
            if not prefix:
                return True
            return await self._is_meow(maas_url, prefix)

        if stripped.endswith("?"):
            candidate = stripped[:-1].strip()
            if candidate:
                return await self._is_meow(maas_url, candidate)
        return False

    async def _reply_maas_outage(
        self,
        evt: MessageEvent,
        orig_evt: MessageEvent | None,
        is_reply: bool,
    ) -> None:
        content = TextMessageEventContent(
            msgtype=MessageType.TEXT, body=MAAS_OUTAGE_MSG
        )
        content.set_reply(orig_evt if (is_reply and orig_evt) else evt)
        await self.client.send_message(evt.room_id, content)

    def _strip_bot_mention(self, body: str, localpart: str) -> str:
        text = body.replace(self.client.mxid, "").replace(f"@{localpart}", "")
        if localpart in text:
            text = text.replace(localpart, "")
        text = text.strip()
        if text.startswith(":"):
            text = text[1:].strip()
        return text

    @event.on(EventType.ROOM_MESSAGE)
    async def handle_message(self, evt: MessageEvent) -> None:
        if evt.sender == self.client.mxid or evt.content.msgtype != MessageType.TEXT:
            return

        if hasattr(evt.content, "trim_reply_fallback"):
            evt.content.trim_reply_fallback()

        body = evt.content.body.strip()
        maas_url = self.config["maas_url"]
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
                except Exception as e:
                    self.log.exception(f"Failed to fetch replied-to event: {e}")

            normalized_ping = re.sub(r"[\W_]+", "", ping_text).lower()

            if normalized_ping == "help":
                help_text = (
                    "Meow Help:\n"
                    "- Ping to generate a meow.\n"
                    "- Ping with text (or reply to a message) for a meownalysis.\n"
                    "- Ping with a reply and a meow and a question mark (e.g., mrrp?) to consult the Meow-Ball.\n"
                    "- Ping with a meow and a fact-check phrase (e.g., mrrp is this true?, mrrp fact check?, mrrp is that correct?) to consult the Meow-Ball.\n"
                )
                content = TextMessageEventContent(msgtype=MessageType.TEXT, body=help_text)
                content.set_reply(orig_evt if (is_reply and orig_evt) else evt)
                await self.client.send_message(evt.room_id, content)
                return

            try:
                if await self._detect_8ball(maas_url, ping_text):
                    target_text = parent_text if is_reply else ping_text
                    ask_payload = f"{target_text} [{evt.sender}]"

                    data = await self._maas_get_json(
                        maas_url, "/askmeow", {"text": ask_payload}
                    )
                    response_text = f"{data['answer']}"

                    content = TextMessageEventContent(
                        msgtype=MessageType.TEXT, body=response_text
                    )
                    content.set_reply(orig_evt if (is_reply and orig_evt) else evt)
                    await self.client.send_message(evt.room_id, content)
                    return

                if not ping_text and not is_reply:
                    data = await self._maas_get_json(maas_url, "/meow")
                    response_text = data["meow"]

                    content = TextMessageEventContent(
                        msgtype=MessageType.TEXT, body=response_text
                    )
                    content.set_reply(evt)
                    await self.client.send_message(evt.room_id, content)
                    return

                target_text = parent_text if is_reply else ping_text

                data_strict = await self._maas_get_json(
                    maas_url, "/ismeow", {"text": target_text}
                )
                data_fuzzy = await self._maas_get_json(
                    maas_url, "/meowlike", {"text": target_text}
                )

                response_text = f"Strict Confidence: {data_strict['meow_percentage']}\n"
                response_text += f"Fuzzy Confidence: {data_fuzzy['meow_percentage']}\n"

                if data_strict.get("is_meow"):
                    response_text += "Verdict: Meow."
                elif data_fuzzy.get("is_meow_like"):
                    response_text += "Verdict: Meow-like."
                else:
                    response_text += "Verdict: Not meow."

                content = TextMessageEventContent(
                    msgtype=MessageType.TEXT, body=response_text
                )
                content.set_reply(orig_evt if (is_reply and orig_evt) else evt)
                await self.client.send_message(evt.room_id, content)

            except (aiohttp.ClientError, KeyError, ValueError) as e:
                self.log.exception(f"HTTP request failed during ping handling: {e}")
                await self._reply_maas_outage(evt, orig_evt, is_reply)

            return

        target_rooms = self.config.get("target_rooms", [])
        if str(evt.room_id) in target_rooms:
            try:
                data_strict = await self._maas_get_json(
                    maas_url, "/ismeow", {"text": body}
                )
                data_fuzzy = await self._maas_get_json(
                    maas_url, "/meowlike", {"text": body}
                )

                if data_strict.get("is_meow") or data_fuzzy.get("is_meow_like"):
                    gen_data = await self._maas_get_json(maas_url, "/meow")
                    await evt.reply(gen_data["meow"])
            except (aiohttp.ClientError, KeyError, ValueError) as e:
                self.log.exception(f"HTTP request failed during passive reply: {e}")
