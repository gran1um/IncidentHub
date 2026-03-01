#!/usr/bin/env python3
import argparse
import json
import sys
import threading
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse


def now_iso(offset_seconds: int = 0) -> str:
    return (datetime.now(timezone.utc) + timedelta(seconds=offset_seconds)).isoformat().replace('+00:00', 'Z')


class MockState:
    def __init__(self) -> None:
        self.lock = threading.Lock()
        self.slack_counter = 0
        self.slack_threads: dict[str, list[dict]] = {}
        self.outlook_counter = 0
        self.outlook_messages: dict[str, list[dict]] = {}
        self.outlook_threads: dict[str, dict] = {}

    def next_slack_ts(self) -> str:
        with self.lock:
            self.slack_counter += 1
            return f"1741439188.{self.slack_counter:06d}"

    def next_outlook_id(self) -> str:
        with self.lock:
            self.outlook_counter += 1
            return f"outlook-msg-{self.outlook_counter}"


STATE = MockState()


class Handler(BaseHTTPRequestHandler):
    server_version = "IncidentHubCommunicationsMock/1.0"

    def log_message(self, fmt: str, *args) -> None:
        sys.stderr.write("%s - - [%s] %s\n" % (self.address_string(), self.log_date_time_string(), fmt % args))

    def _read_json(self) -> dict:
        length = int(self.headers.get("Content-Length", "0") or "0")
        raw = self.rfile.read(length) if length > 0 else b"{}"
        if not raw:
            return {}
        return json.loads(raw.decode("utf-8"))

    def _read_form(self) -> dict[str, str]:
        length = int(self.headers.get("Content-Length", "0") or "0")
        raw = self.rfile.read(length) if length > 0 else b""
        parsed = parse_qs(raw.decode("utf-8"), keep_blank_values=True)
        return {key: values[-1] if values else "" for key, values in parsed.items()}

    def _json(self, status: int, payload: dict | list) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path == "/healthz":
            self._json(200, {"ok": True})
            return
        if parsed.path == "/slack/conversations.replies":
            self._handle_slack_replies(parsed)
            return
        if parsed.path == "/slack/conversations.history":
            self._handle_slack_history(parsed)
            return
        if parsed.path.startswith("/outlook/v1.0/users/") and parsed.path.endswith("/messages"):
            self._handle_outlook_messages(parsed)
            return
        self._json(404, {"error": "not_found", "path": parsed.path})

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path == "/slack/chat.postMessage":
            self._handle_slack_send()
            return
        if parsed.path.endswith("/oauth2/v2.0/token") and parsed.path.startswith("/outlook-auth/"):
            self._handle_outlook_token()
            return
        if parsed.path.startswith("/outlook/v1.0/users/") and parsed.path.endswith("/sendMail"):
            self._handle_outlook_send(parsed)
            return
        self._json(404, {"error": "not_found", "path": parsed.path})

    def _handle_slack_send(self) -> None:
        payload = self._read_json()
        channel = str(payload.get("channel") or "").strip() or "C123456789"
        text = str(payload.get("text") or "").strip() or "Empty message"
        thread_ts = str(payload.get("thread_ts") or "").strip()
        if not thread_ts:
            thread_ts = STATE.next_slack_ts()
        message_ts = STATE.next_slack_ts()
        conversation_id = f"{channel}:{thread_ts}"
        reply = {
            "type": "message",
            "user": "U_REPLY",
            "username": "Slack Responder",
            "text": f"Slack reply: {text}",
            "ts": STATE.next_slack_ts(),
            "thread_ts": thread_ts,
        }
        follow_up = {
            "type": "message",
            "user": "U_REPLY",
            "username": "Slack Responder",
            "text": f"Slack follow-up: {text}",
            "ts": STATE.next_slack_ts(),
            "thread_ts": thread_ts,
        }
        with STATE.lock:
            thread_messages = STATE.slack_threads.setdefault(conversation_id, [])
            if not thread_messages:
                thread_messages.append(reply)
            else:
                thread_messages.append(follow_up)
        self._json(200, {
            "ok": True,
            "channel": channel,
            "ts": message_ts,
            "message": {
                "text": text,
                "thread_ts": thread_ts,
            },
        })

    def _handle_slack_replies(self, parsed) -> None:
        query = parse_qs(parsed.query)
        channel = (query.get("channel") or [""])[-1].strip()
        thread_ts = (query.get("ts") or [""])[-1].strip()
        conversation_id = f"{channel}:{thread_ts}"
        oldest = (query.get("oldest") or [""])[-1].strip()
        with STATE.lock:
            messages = list(STATE.slack_threads.get(conversation_id, []))
        if oldest:
            messages = [item for item in messages if str(item.get("ts") or "") > oldest]
        self._json(200, {"ok": True, "messages": messages, "has_more": False})

    def _handle_slack_history(self, parsed) -> None:
        query = parse_qs(parsed.query)
        channel = (query.get("channel") or [""])[-1].strip()
        oldest = (query.get("oldest") or [""])[-1].strip()
        with STATE.lock:
            messages = []
            prefix = f"{channel}:"
            for key, items in STATE.slack_threads.items():
                if key.startswith(prefix):
                    messages.extend(items)
        if oldest:
            messages = [item for item in messages if str(item.get("ts") or "") > oldest]
        self._json(200, {"ok": True, "messages": messages, "has_more": False})

    def _handle_outlook_token(self) -> None:
        _ = self._read_form()
        self._json(200, {
            "token_type": "Bearer",
            "expires_in": 3600,
            "access_token": "mock-outlook-token",
        })

    def _handle_outlook_send(self, parsed) -> None:
        payload = self._read_json()
        path_parts = parsed.path.strip("/").split("/")
        mailbox = path_parts[3] if len(path_parts) >= 5 else "soc@example.com"
        message = payload.get("message") or {}
        subject = str(message.get("subject") or "No subject").strip() or "No subject"
        to_recipients = message.get("toRecipients") or []
        cc_recipients = message.get("ccRecipients") or []
        body = message.get("body") or {}
        recipient = ""
        if to_recipients:
            recipient = str((((to_recipients[0] or {}).get("emailAddress") or {}).get("address") or "")).strip()
        conversation_id = f"outlook-conversation-{len(STATE.outlook_threads) + 1}"
        message_id = STATE.next_outlook_id()
        with STATE.lock:
            existing = STATE.outlook_threads.get(subject)
            if existing:
                conversation_id = existing["conversation_id"]
            else:
                STATE.outlook_threads[subject] = {
                    "conversation_id": conversation_id,
                    "mailbox": mailbox,
                }
            mailbox_messages = STATE.outlook_messages.setdefault(mailbox, [])
            mailbox_messages.append({
                "id": message_id,
                "subject": subject,
                "conversationId": conversation_id,
                "receivedDateTime": now_iso(1),
                "from": {
                    "emailAddress": {
                        "name": "External Responder",
                        "address": recipient or "external@example.com",
                    }
                },
                "bodyPreview": f"Outlook reply: {str(body.get('content') or '').strip() or subject}",
                "internetMessageId": f"<{conversation_id}@incidenthub.local>",
            })
        self.send_response(202)
        self.send_header("Content-Length", "0")
        self.end_headers()
        _ = cc_recipients

    def _handle_outlook_messages(self, parsed) -> None:
        path_parts = parsed.path.strip("/").split("/")
        mailbox = path_parts[3] if len(path_parts) >= 5 else "soc@example.com"
        with STATE.lock:
            messages = list(STATE.outlook_messages.get(mailbox, []))
        self._json(200, {"value": messages})


def main() -> None:
    parser = argparse.ArgumentParser(description="IncidentHub communications mock server")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18081)
    args = parser.parse_args()

    server = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"communications mock server listening on http://{args.host}:{args.port}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
