import unittest
from types import SimpleNamespace
from unittest.mock import AsyncMock, Mock, patch

from bot import meow as meow_module


class FakeContent(dict):
    def __init__(self, body, reply_to=None):
        super().__init__(body=body, msgtype=meow_module.MessageType.TEXT)
        self.body = body
        self.msgtype = meow_module.MessageType.TEXT
        self._reply_to = reply_to

    def get_reply_to(self):
        return self._reply_to

    def trim_reply_fallback(self):
        if "\n\n" in self.body:
            self.body = self.body.rsplit("\n\n", 1)[1]


class FakeOutbound:
    def __init__(self, msgtype, body):
        self.msgtype = msgtype
        self.body = body
        self.reply_to = None

    def set_reply(self, event):
        self.reply_to = event


class BotLanguageTests(unittest.TestCase):
    def setUp(self):
        self.bot = meow_module.Meow.__new__(meow_module.Meow)
        self.bot.config = {
            "maas_url": "http://maas",
            "target_rooms": ["!room:example.org"],
            "detect_language": "fr",
            "response_language": "en",
            "room_languages": {},
        }
        self.bot._global_detect_language = "fr"
        self.bot._global_response_language = "en"
        self.bot._strict_min_score = 75
        self.bot._fuzzy_min_score = 55
        self.bot._embedded_min_coverage = 25
        self.bot._room_language_cache = {
            "!room:example.org": ("fr", "ja"),
        }
        self.bot.log = Mock()
        self.bot.client = SimpleNamespace(
            mxid="@meow:example.org",
            parse_user_id=lambda mxid: ("meow", "example.org"),
            send_message=AsyncMock(),
            get_event=AsyncMock(),
        )
        self.calls = []

        async def fake_get_json(url, path, params=None):
            self.calls.append((path, params))
            language = (params or {}).get("lang", "en")
            if path == "/ismeow":
                text = (params or {}).get("text", "")
                is_meow = text in {"miaou", "にゃー", "meow"}
                return {
                    "language": language,
                    "closest_language": language,
                    "family": "meow",
                    "match_type": "exact" if is_meow else "none",
                    "is_meow": is_meow,
                    "meow_percentage": "100.0%" if is_meow else "0.0%",
                }
            if path == "/meowlike":
                text = (params or {}).get("text", "")
                embedded = text != "meow" and text.endswith("meow")
                if language == "auto":
                    language = {
                        "miaou": "fr",
                        "miyav": "tr",
                        "мяу": "ru",
                        "няв": "uk",
                        "喵喵": "zh-Hans",
                        "meow": "en",
                    }.get(text, "en" if embedded else "auto")
                return {
                    "language": language,
                    "closest_language": language,
                    "family": "meow",
                    "is_meow_like": text in {
                        "miaou", "にゃー", "miyav", "мяу", "няв", "喵喵", "meow"
                    } or embedded,
                    "meow_percentage": "30.8%" if embedded else "100.0%",
                    "match_type": "embedded" if embedded else "exact",
                    "matched_text": "meow" if embedded else None,
                    "coverage_percentage": 30.8 if embedded else 100,
                }
            if path == "/meow":
                return {"language": language, "meow": "にゃー" if language == "ja" else "miaou"}
            if path == "/askmeow":
                return {"answer": "Answer."}
            raise AssertionError(f"unexpected MaaS path: {path}")

        self.bot._maas_get_json = AsyncMock(side_effect=fake_get_json)
        self.patch_outbound = patch.object(
            meow_module, "TextMessageEventContent", FakeOutbound
        )
        self.patch_outbound.start()
        self.addCleanup(self.patch_outbound.stop)

    def event(self, body, room="!room:example.org", reply_to=None, sender="@alice:example.org"):
        return SimpleNamespace(
            sender=sender,
            room_id=room,
            content=FakeContent(body, reply_to=reply_to),
            reply=AsyncMock(),
        )

    def sent_body(self):
        content = self.bot.client.send_message.await_args.args[1]
        return content.body

    def test_options_are_leading_and_one_time(self):
        remaining, options, error = meow_module.Meow._parse_language_options(
            " --detect=FR --reply=ja You’re cute meow"
        )
        self.assertEqual(remaining, "You’re cute meow")
        self.assertEqual(options, {"detect": "fr", "response": "ja"})
        self.assertIsNone(error)

        remaining, options, error = meow_module.Meow._parse_language_options(
            "meow --reply=ja"
        )
        self.assertEqual(remaining, "meow --reply=ja")
        self.assertEqual(options, {})
        self.assertIsNone(error)

        _, _, duplicate = meow_module.Meow._parse_language_options(
            "--reply=fr --reply=ja meow"
        )
        self.assertIsNotNone(duplicate)
        _, _, invalid = meow_module.Meow._parse_language_options("--detect=xx meow")
        self.assertEqual(invalid, "language:xx")

        remaining, options, error = meow_module.Meow._parse_language_options(
            "--detect=ZH-Hant --reply=hy 喵喵"
        )
        self.assertEqual(remaining, "喵喵")
        self.assertEqual(options, {"detect": "zh-hant", "response": "hy"})
        self.assertIsNone(error)

        remaining, options, error = meow_module.Meow._parse_language_options(
            "--detect=auto --reply=auto meow"
        )
        self.assertEqual(
            (remaining, options, error),
            ("meow", {"detect": "auto", "response": "auto"}, None),
        )

    def test_default_language_settings_are_auto(self):
        class Config(dict):
            def load_and_update(self):
                pass

        self.bot.config = Config(maas_url="http://maas", target_rooms=["!room:example.org"])

        import asyncio

        asyncio.run(self.bot.start())

        self.assertEqual(self.bot._resolve_languages("!room:example.org"), ("auto", "auto"))

        asyncio.run(
            meow_module.Meow.handle_message(self.bot, self.event("@meow:example.org miyav"))
        )
        self.assertIn(("/ismeow", {"text": "miyav", "lang": "auto"}), self.calls)
        self.assertIn(("/meowlike", {"text": "miyav", "lang": "auto"}), self.calls)
        self.assertIn("Verdict:", self.sent_body())

        self.calls.clear()
        self.bot.client.send_message.reset_mock()
        asyncio.run(
            meow_module.Meow.handle_message(self.bot, self.event("@meow:example.org"))
        )
        self.assertEqual(self.calls, [("/meow", {"lang": "en"})])

    def test_help_command_is_english_in_every_room(self):
        import asyncio

        asyncio.run(
            meow_module.Meow.handle_message(
                self.bot, self.event("@meow:example.org help")
            )
        )
        self.assertIn("Meow Help:", self.sent_body())
        self.assertFalse(self.calls)

    def test_room_override_is_independent(self):
        self.assertEqual(
            self.bot._resolve_languages("!room:example.org"), ("fr", "ja")
        )
        self.assertEqual(
            self.bot._resolve_languages("!other:example.org"), ("fr", "en")
        )

    def test_embedded_analysis_uses_english_diagnostic(self):
        strict = {"meow_percentage": "0.0%", "is_meow": False, "language": "auto"}
        fuzzy = {
            "meow_percentage": "30.8%",
            "is_meow_like": True,
            "match_type": "embedded",
            "matched_text": "meow",
            "coverage_percentage": 30.8,
            "language": "en",
            "family": "meow",
        }
        text, formatted = self.bot._format_analysis(strict, fuzzy)
        self.assertIn("Verdict: Meow-like.", text)
        self.assertIn("Language: en", text)
        self.assertIn("Sound: meow · embedded", text)
        self.assertIn("Strict: 0.0% · Fuzzy: 30.8%", text)
        self.assertIn("Matched: 'meow' · 30.8% of letters", text)
        self.assertIn(
            '<strong><span data-mx-color="#E1B97D">Verdict: Meow-like.</span></strong>',
            formatted,
        )
        self.assertIn('<span data-mx-color="#D98792">0.0%</span>', formatted)
        self.assertIn('30.8%</span> of letters', formatted)

    def test_shared_score_and_metadata_are_shown_once(self):
        strict = {
            "is_meow": True, "meow_percentage": "100.0%", "language": "en",
            "family": "meow", "match_type": "exact",
        }
        fuzzy = {
            **strict, "is_meow_like": True,
        }
        text, formatted = self.bot._format_analysis(strict, fuzzy)
        self.assertEqual(
            text,
            "Verdict: Meow.\nLanguage: en\nSound: meow · exact\n"
            "Strict / fuzzy score: 100.0%\nLimits: strict ≥75%, fuzzy ≥55%",
        )
        self.assertIn(
            '<strong><span data-mx-color="#7FC4A3">Verdict: Meow.</span></strong>',
            formatted,
        )
        self.assertIn('<span data-mx-color="#7FC4A3">100.0%</span>', formatted)

        fuzzy["meow_percentage"] = "85.0%"
        text, formatted = self.bot._format_analysis(strict, fuzzy)
        self.assertIn("Strict: 100.0% · Fuzzy: 85.0%", text)
        self.assertIn('<span data-mx-color="#9CC198">85.0%</span>', formatted)

    def test_analysis_footer_is_escaped_html_with_plain_text_fallback(self):
        strict = {"is_meow": False, "meow_percentage": "0.0%", "language": "auto"}
        fuzzy = {
            "is_meow_like": True, "meow_percentage": "50.0%",
            "language": "en", "family": "meow", "match_type": "embedded",
            "matched_text": "<meow & wow>", "coverage_percentage": 50,
            "squeezed_form": "<meow & wow>", "detection_time": "12µs",
        }
        body, formatted = self.bot._format_analysis(strict, fuzzy)
        self.assertIn("Normalized: <meow & wow> · API: fuzzy 12µs", body)
        self.assertIn("Limits: strict ≥75%, coverage ≥25%", body)
        self.assertIn("<sub><span", formatted)
        self.assertIn('<span data-mx-color="#E1B97D">50.0%</span>', formatted)
        self.assertIn("&lt;meow &amp; wow&gt;", formatted)
        self.assertNotIn("<meow & wow>", formatted)

    def test_configurable_thresholds_control_verdicts_and_passive_replies(self):
        import asyncio

        strict = {
            "is_meow": True, "meow_percentage": "80.0%", "language": "auto",
            "closest_language": "en", "family": "meow", "match_type": "typo",
        }
        fuzzy = {
            "is_meow_like": True, "meow_percentage": "60.0%",
            "language": "auto", "closest_language": "en", "family": "meow",
            "match_type": "typo",
        }
        self.bot._strict_min_score = 85
        self.bot._fuzzy_min_score = 65
        text, formatted = self.bot._format_analysis(strict, fuzzy)
        self.assertIn("Language: undetermined (closest: en)", text)
        self.assertIn("Verdict: Not meow.", text)
        self.assertIn(
            '<strong><span data-mx-color="#D98792">Verdict: Not meow.</span></strong>',
            formatted,
        )

        self.bot._embedded_min_coverage = 35
        evt = self.event("You're cute meow")
        asyncio.run(meow_module.Meow.handle_message(self.bot, evt))
        evt.reply.assert_not_awaited()

        embedded = {
            "is_meow_like": True, "meow_percentage": "30.8%",
            "match_type": "embedded", "coverage_percentage": 30.8,
        }
        self.assertFalse(self.bot._matches_fuzzy(embedded))

    def test_threshold_config_uses_api_floors(self):
        self.bot.config.update(
            strict_min_score=10, fuzzy_min_score="90", embedded_min_coverage="no"
        )
        self.assertEqual(self.bot._valid_threshold("strict_min_score", 75), 75)
        self.assertEqual(self.bot._valid_threshold("fuzzy_min_score", 55), 90)
        self.assertEqual(self.bot._valid_threshold("embedded_min_coverage", 25), 25)

    def test_ping_detects_in_french_and_keeps_analysis_english(self):
        async def run():
            await meow_module.Meow.handle_message(
                self.bot, self.event("@meow:example.org --detect=fr --reply=ja miaou")
            )

        import asyncio

        asyncio.run(run())
        self.assertEqual(self.calls[0], ("/ismeow", {"text": "miaou", "lang": "fr"}))
        self.assertEqual(self.calls[1], ("/meowlike", {"text": "miaou", "lang": "fr"}))
        self.assertIn("Verdict: Meow.", self.sent_body())
        content = self.bot.client.send_message.await_args.args[1]
        self.assertEqual(content.format, meow_module.Format.HTML)
        self.assertIn(
            '<strong><span data-mx-color="#7FC4A3">Verdict: Meow.</span></strong>',
            content.formatted_body,
        )

    def test_bare_ping_generates_using_room_response_language(self):
        import asyncio

        asyncio.run(
            meow_module.Meow.handle_message(
                self.bot, self.event("@meow:example.org")
            )
        )
        self.assertEqual(self.calls, [("/meow", {"lang": "ja"})])
        self.assertEqual(self.sent_body(), "にゃー")

    def test_per_ping_response_can_select_new_language(self):
        import asyncio

        asyncio.run(
            meow_module.Meow.handle_message(
                self.bot, self.event("@meow:example.org --reply=hy")
            )
        )
        self.assertEqual(self.calls, [("/meow", {"lang": "hy"})])

    def test_fact_check_stays_english_with_reply_language(self):
        import asyncio

        asyncio.run(
            meow_module.Meow.handle_message(
                self.bot,
                self.event("@meow:example.org --reply=ja is this true: cats purr"),
            )
        )
        path, params = self.calls[-1]
        self.assertEqual(path, "/askmeow")
        self.assertEqual(params, {"text": "cats purr [@alice:example.org]"})
        self.assertEqual(self.sent_body(), "Answer.")

    def test_invalid_option_gets_english_usage_without_api_call(self):
        import asyncio

        asyncio.run(
            meow_module.Meow.handle_message(
                self.bot, self.event("@meow:example.org --detect=xx miaou")
            )
        )
        self.assertFalse(self.calls)
        self.assertIn("Unsupported language", self.sent_body())
        self.assertIn("--detect", self.sent_body())

    def test_passive_room_auto_detects_and_replies_in_the_detected_language(self):
        import asyncio

        async def run():
            for form, language in (
                ("miaou", "fr"),
                ("miyav", "tr"),
                ("мяу", "ru"),
                ("няв", "uk"),
                ("喵喵", "zh-Hans"),
            ):
                evt = self.event(form)
                await meow_module.Meow.handle_message(self.bot, evt)
                self.assertIn(("/meowlike", {"text": form, "lang": "auto"}), self.calls)
                self.assertIn(("/meow", {"lang": language}), self.calls)
                evt.reply.assert_awaited_once()

        asyncio.run(run())

    def test_reply_fallback_text_is_trimmed_before_detection(self):
        import asyncio

        parent = SimpleNamespace(
            sender="@bob:example.org",
            content=FakeContent("> quoted old text\n\nmiaou"),
        )
        self.bot.client.get_event.return_value = parent
        evt = self.event(
            "@meow:example.org --detect=fr --reply=ja miaou?", reply_to="$parent"
        )
        self.assertEqual(evt.content.get_reply_to(), "$parent")
        asyncio.run(
            meow_module.Meow.handle_message(self.bot, evt)
        )
        self.bot.client.get_event.assert_awaited_once_with(evt.room_id, "$parent")
        self.assertIn(("/ismeow", {"text": "miaou", "lang": "fr"}), self.calls)
        self.assertIn(
            ("/askmeow", {"text": "miaou [@bob:example.org]"}),
            self.calls,
        )

    def test_edit_events_are_ignored(self):
        import asyncio

        evt = self.event("miaou")
        evt.content["m.relates_to"] = {"rel_type": "m.replace"}
        asyncio.run(meow_module.Meow.handle_message(self.bot, evt))
        self.assertFalse(self.calls)

    def test_older_api_without_language_field_is_actionable(self):
        import asyncio

        class Response:
            async def __aenter__(self):
                return self

            async def __aexit__(self, exc_type, exc, traceback):
                return False

            def raise_for_status(self):
                return None

            async def json(self):
                return {"meow": "meow"}

        self.bot.http = SimpleNamespace(get=Mock(return_value=Response()))

        async def request():
            with self.assertRaises(meow_module.UnsupportedLanguageError) as raised:
                await meow_module.Meow._maas_get_json(
                    self.bot, "http://maas", "/meow", {"lang": "fr"}
                )
            self.assertEqual(raised.exception.language, "fr")

        asyncio.run(request())

    def test_auto_detection_accepts_the_detected_canonical_language(self):
        import asyncio

        class Response:
            async def __aenter__(self):
                return self

            async def __aexit__(self, exc_type, exc, traceback):
                return False

            def raise_for_status(self):
                return None

            async def json(self):
                return {"language": "zh-Hans", "is_meow_like": True}

        self.bot.http = SimpleNamespace(get=Mock(return_value=Response()))

        async def request():
            data = await meow_module.Meow._maas_get_json(
                self.bot,
                "http://maas",
                "/meowlike",
                {"text": "喵喵", "lang": "auto"},
            )
            self.assertEqual(data["language"], "zh-Hans")

        asyncio.run(request())


if __name__ == "__main__":
    unittest.main()
