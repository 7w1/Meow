from mautrix.types import EventType, MessageType, TextMessageEventContent
from mautrix.util.config import BaseProxyConfig, ConfigUpdateHelper
from maubot import Plugin, MessageEvent
from maubot.handlers import event
import aiohttp
import re

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

    @event.on(EventType.ROOM_MESSAGE)
    async def handle_message(self, evt: MessageEvent) -> None:
        if evt.sender == self.client.mxid or evt.content.msgtype != MessageType.TEXT:
            return

        body = evt.content.body.strip()
        maas_url = self.config["maas_url"]
        localpart = self.client.mxid.split(":")[0]

        is_pinged = False
        mentions = getattr(evt.content, "mentions", None) or getattr(evt.content, "m_mentions", None)
        
        if mentions and hasattr(mentions, "user_ids") and mentions.user_ids and self.client.mxid in mentions.user_ids:
            is_pinged = True
        elif self.client.mxid in body or f"@{localpart}" in body or localpart in body:
            is_pinged = True

        if is_pinged:
            reply_to_event_id = evt.content.get_reply_to()
            clean_text = ""
            is_reply = False
            orig_evt = None
            
            if reply_to_event_id:
                try:
                    orig_evt = await self.client.get_event(evt.room_id, reply_to_event_id)
                    if orig_evt.content and orig_evt.content.body:
                        clean_text = orig_evt.content.body.strip()
                        is_reply = True
                except Exception as e:
                    self.log.exception(f"Failed to fetch replied-to event: {e}")
            
            if not is_reply:
                clean_text = body.replace(self.client.mxid, "").replace(f"@{localpart}", "").replace(localpart, "").strip()
                if clean_text.startswith(":"):
                    clean_text = clean_text[1:].strip()

            normalized_text = re.sub(r'[\W_]+', '', clean_text).lower()

            if normalized_text == "help":
                help_text = (
                    "Meow Help:\n"
                    "- Ping to generate a meow.\n"
                    "- Ping with text (or replying to a message) for meownalysis.\n"
                )
                content = TextMessageEventContent(msgtype=MessageType.TEXT, body=help_text)
                
                if is_reply and orig_evt:
                    content.set_reply(orig_evt)
                else:
                    content.set_reply(evt)
                    
                await self.client.send_message(evt.room_id, content)
                return

            try:
                async with aiohttp.ClientSession() as session:
                    if not clean_text:
                        async with session.get(f"{maas_url}/meow") as resp:
                            data = await resp.json()
                            response_text = data["meow"]
                    else:
                        async with session.get(f"{maas_url}/ismeow", params={"text": clean_text}) as resp:
                            data = await resp.json()
                            response_text = f"Confidence: {data['meow_percentage']}\n"
                            response_text += "Verdict: Meow." if data['is_meow'] else "Verdict: Not meow."
                    
                    content = TextMessageEventContent(msgtype=MessageType.TEXT, body=response_text)
                    
                    if is_reply and orig_evt:
                        content.set_reply(orig_evt)
                    else:
                        content.set_reply(evt)
                    
                    await self.client.send_message(evt.room_id, content)

            except Exception as e:
                self.log.exception(f"HTTP Request failed during ping handling: {e}")
                    
            return

        target_rooms = self.config.get("target_rooms", [])
        if str(evt.room_id) in target_rooms:
            try:
                async with aiohttp.ClientSession() as session:
                    async with session.get(f"{maas_url}/ismeow", params={"text": body}) as resp:
                        data = await resp.json()
                        
                        if data.get("is_meow"):
                            async with session.get(f"{maas_url}/meow") as gen_resp:
                                gen_data = await gen_resp.json()
                                await evt.reply(gen_data["meow"])
            except Exception as e:
                self.log.exception(f"HTTP Request failed during passive reply: {e}")