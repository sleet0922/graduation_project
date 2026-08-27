#!/usr/bin/env python3
"""Black-box end-to-end test for graduation_project.

Install once in a virtualenv:
    python3 -m venv .e2e-venv
    .e2e-venv/bin/pip install requests websocket-client livekit

Run against the deployed mini host:
    .e2e-venv/bin/python api/e2e_test.py

The test creates disposable users and removes them at the end.  Set
E2E_KEEP_DATA=1 while debugging to keep them for inspection.
"""

from __future__ import annotations

import argparse
import asyncio
import base64
import hashlib
import json
import os
import secrets
import sys
import time
import uuid
from dataclasses import dataclass
from typing import Any

import requests

try:
    import websocket
except ImportError as exc:  # pragma: no cover - exercised by the CLI
    raise SystemExit("missing websocket-client; install: pip install websocket-client") from exc


@dataclass
class User:
    email: str
    password: str
    id: int = 0
    account: str = ""
    token: str = ""
    refresh: str = ""


class E2E:
    def __init__(self, base: str, http_base: str | None = None, keep: bool = False):
        self.base = base.rstrip("/")
        derived_http = self.base.replace("https://", "http://", 1).replace(":444", ":81")
        self.http_base = (http_base or derived_http).rstrip("/")
        self.ws_base = self.base.replace("https://", "wss://").replace("http://", "ws://")
        self.http = requests.Session()
        self.http.verify = True
        self.keep = keep
        self.users: list[User] = []
        self.group_id = 0
        self.post_id = 0
        self.comment_id = 0
        self.call_id = ""
        self.ws: dict[str, websocket.WebSocket] = {}
        self.failures: list[str] = []

    def check(self, label: str, condition: bool, detail: str = "") -> bool:
        if condition:
            print(f"PASS {label}")
            return True
        msg = f"{label}: {detail}".strip()
        self.failures.append(msg)
        print(f"FAIL {msg}")
        return False

    def request(self, label: str, method: str, path: str, user: User | None = None,
                *, expected=(200,), params=None, body=None, files=None) -> dict[str, Any]:
        headers = {"Accept": "application/json"}
        if user and user.token:
            headers["Authorization"] = f"Bearer {user.token}"
        try:
            response = self.http.request(method, self.base + path, headers=headers,
                                         params=params, json=body, files=files, timeout=20)
            payload: dict[str, Any]
            try:
                payload = response.json()
            except ValueError:
                payload = {"_text": response.text[:300]}
            ok = response.status_code in expected
            if ok and "code" in payload and path != "/health":
                # Error responses use their HTTP status as `code` (for example
                # 428 while a group's first key is waiting to be published).
                # Successful API responses use 200.
                ok = payload.get("code") in (200, response.status_code)
            self.check(label, ok, f"HTTP {response.status_code}: {payload}")
            return payload
        except requests.RequestException as exc:
            self.check(label, False, str(exc))
            return {}

    @staticmethod
    def data(payload: dict[str, Any]) -> Any:
        return payload.get("data")

    def http_redirect_flow(self):
        """Verify the clear-text listener redirects to the configured TLS port."""
        try:
            response = self.http.get(self.http_base + "/health", allow_redirects=False, timeout=10)
            location = response.headers.get("Location", "")
            self.check("HTTP listener redirects to HTTPS", response.status_code in (301, 308) and
                       location.startswith(self.base + "/"),
                       f"HTTP {response.status_code}, Location={location!r}")
        except requests.RequestException as exc:
            self.check("HTTP listener redirects to HTTPS", False, str(exc))

    def create_user(self, index: int) -> User:
        suffix = f"{int(time.time())}{secrets.token_hex(3)}{index}"
        user = User(f"bocker-e2e-{suffix}@example.com", "E2ePassword!2026")
        payload = self.request(f"register user {index}", "POST", "/api/user/register",
                               body={"email": user.email, "password": user.password})
        data = self.data(payload) or {}
        user.id, user.account = int(data.get("id", 0)), str(data.get("account", ""))
        self.check(f"registered user {index} has id/account", user.id > 0 and bool(user.account), str(data))
        payload = self.request(f"login user {index}", "POST", "/api/user/login",
                               body={"account": user.email, "password": user.password})
        data = self.data(payload) or {}
        user.token, user.refresh = str(data.get("token", "")), str(data.get("refresh_token", ""))
        self.check(f"login user {index} tokens", bool(user.token and user.refresh), str(data))
        self.users.append(user)
        return user

    def user_flow(self, a: User, b: User):
        self.request("health", "GET", "/health")
        refreshed = self.request("refresh access/refresh tokens", "POST", "/api/user/refresh",
                                body={"refresh_token": a.refresh})
        fresh = self.data(refreshed) or {}
        if fresh.get("token"):
            a.token, a.refresh = fresh["token"], fresh["refresh_token"]
        self.request("user self", "POST", "/api/user/self", a)
        self.request("user search by email", "GET", "/api/user/search", a,
                     params={"keyword": b.email})
        self.request("name update", "POST", "/api/user/name_update", a, body={"name": "E2E Tester"})
        self.request("avatar update", "POST", "/api/user/avatar_update", a,
                     body={"avatar": "e2e-avatar.png"})
        self.request("profile update", "POST", "/api/user/profile_update", a,
                     body={"gender": 1, "birthday": "2000-01-02", "location": "Shanghai"})
        self.request("location update", "POST", "/api/user/location", a,
                     body={"latitude": 31.23, "longitude": 121.47, "province": "上海市",
                           "city": "上海市", "district": "黄浦区", "address": "E2E", "timestamp": int(time.time())})
        old = a.password
        a.password = "E2ePassword!2027"
        self.request("password update", "POST", "/api/user/password_update", a,
                     body={"password": old, "new_password": a.password})
        relogin = self.request("relogin after password update", "POST", "/api/user/login",
                               body={"account": a.account, "password": a.password})
        data = self.data(relogin) or {}
        if data.get("token"):
            a.token, a.refresh = data["token"], data["refresh_token"]

    def friend_flow(self, a: User, b: User):
        self.request("friend request", "POST", "/api/friend/request", a, body={"friend_id": b.id})
        requests_payload = self.request("friend requests", "GET", "/api/friend/requests", b)
        rows = self.data(requests_payload) or []
        request_id = next((int(x.get("ID", x.get("id", 0))) for x in rows
                           if int(x.get("sender_id", 0)) == a.id), 0)
        self.check("friend request id returned", request_id > 0, str(rows))
        self.request("accept friend request", "POST", "/api/friend/handle", b,
                     body={"request_id": request_id, "status": 1})
        self.request("friend list A", "GET", "/api/friend/list", a)
        self.request("friend list B", "GET", "/api/friend/list", b)
        self.request("friendship check", "POST", "/api/friend/check", a, body={"friend_id": b.id})
        self.request("friend remark update", "POST", "/api/friend/remark_update", a,
                     body={"friend_id": b.id, "remark": "E2E friend"})

    def e2ee_flow(self, users: list[User]):
        keys: dict[int, tuple[str, str]] = {}
        for user in users:
            raw = secrets.token_bytes(32)
            public = base64.b64encode(raw).decode()
            key_id = hashlib.sha256(raw).hexdigest()
            keys[user.id] = (public, key_id)
            self.request(f"publish identity key {user.id}", "POST", "/api/e2ee/keys/publish", user,
                         body={"key_type": "x25519", "public_key": public})
            self.request(f"read identity key {user.id}", "GET", "/api/e2ee/keys/public", user,
                         params={"user_id": user.id})
        return keys

    def group_flow(self, users: list[User], keys: dict[int, tuple[str, str]]):
        a, b = users[0], users[1]
        payload = self.request("create group", "POST", "/api/group/create", a,
                               body={"name": "Bocker E2E", "avatar": "", "member_ids": [b.id]})
        self.group_id = int((self.data(payload) or {}).get("id", 0))
        self.check("group id returned", self.group_id > 0, str(payload))
        self.request("group list", "GET", "/api/group/list", a)
        self.request("group members", "GET", "/api/group/members", a,
                     params={"group_id": self.group_id})
        current = self.request("group current key initial", "GET", "/api/e2ee/group/key/current", a,
                               params={"group_id": self.group_id}, expected=(200, 428))
        version = int((self.data(current) or {}).get("key_version", 1))
        boxes = [{"user_id": u.id, "wrapped_group_key": base64.b64encode(secrets.token_bytes(32)).decode(),
                  "wrap_nonce": base64.b64encode(secrets.token_bytes(12)).decode()} for u in users[:2]]
        self.request("publish group key boxes", "POST", "/api/e2ee/group/key/publish", a,
                     body={"group_id": self.group_id, "key_version": version,
                           "key_wrap_alg": "chacha20poly1305-v1", "boxes": boxes})
        self.request("read current group key", "GET", "/api/e2ee/group/key/current", b,
                     params={"group_id": self.group_id})
        self.request("read group key by version", "GET", "/api/e2ee/group/key/by-version", b,
                     params={"group_id": self.group_id, "key_version": version})
        rotated = self.request("rotate group key", "POST", "/api/e2ee/group/key/rotate", a,
                               body={"group_id": self.group_id, "expected_key_version": version})
        new_version = int((self.data(rotated) or {}).get("key_version", version))
        # Add a third disposable user, exercising member add and the new key version.
        c = self.users[2]
        self.request("add group member", "POST", "/api/group/member/add", a,
                     body={"group_id": self.group_id, "member_ids": [c.id]})
        members = self.request("group members after add", "GET", "/api/group/members", a,
                               params={"group_id": self.group_id})
        ids = [int(x.get("user_id", 0)) for x in (self.data(members) or [])]
        latest = int((self.data(self.request("group current key after add", "GET",
                                            "/api/e2ee/group/key/current", a,
                                            params={"group_id": self.group_id}, expected=(200, 428))) or {}).get("key_version", new_version))
        boxes = [{"user_id": uid, "wrapped_group_key": base64.b64encode(secrets.token_bytes(32)).decode(),
                  "wrap_nonce": base64.b64encode(secrets.token_bytes(12)).decode()} for uid in ids]
        self.request("publish rotated group boxes", "POST", "/api/e2ee/group/key/publish", a,
                     body={"group_id": self.group_id, "key_version": latest, "boxes": boxes})
        self.request("remove group member", "POST", "/api/group/member/remove", a,
                     body={"group_id": self.group_id, "member_id": c.id})
        self.request("member leaves group", "POST", "/api/group/leave", b,
                     body={"group_id": self.group_id})
        self.request("delete group", "POST", "/api/group/delete", a,
                     body={"group_id": self.group_id})
        self.group_id = 0
        return latest

    def ws_connect(self, name: str, user: User, endpoint: str) -> websocket.WebSocket:
        url = self.ws_base + endpoint
        sock = websocket.create_connection(url, timeout=10, header=[f"Authorization: Bearer {user.token}"],
                                            origin=self.base, http_proxy_host=None)
        first = json.loads(sock.recv())
        self.check(f"{name} websocket connected", first.get("type") == "connected", str(first))
        self.ws[name] = sock
        return sock

    @staticmethod
    def ws_roundtrip(label: str, sock: websocket.WebSocket, message: dict[str, Any], expected: str) -> dict[str, Any]:
        sock.send(json.dumps(message))
        while True:
            value = json.loads(sock.recv())
            if value.get("type") == expected or value.get("type") == "error":
                return value

    def ws_recv_until(self, sock: websocket.WebSocket, expected: str, label: str,
                      timeout: float = 15) -> dict[str, Any]:
        deadline = time.monotonic() + timeout
        old_timeout = sock.gettimeout()
        try:
            while time.monotonic() < deadline:
                sock.settimeout(max(0.2, deadline - time.monotonic()))
                try:
                    value = json.loads(sock.recv())
                except websocket.WebSocketTimeoutException:
                    break
                if value.get("type") == expected:
                    return value
                if value.get("type") == "error":
                    break
            self.check(label, False, f"timed out waiting for {expected}")
        finally:
            sock.settimeout(old_timeout)
        return {}

    def websocket_flow(self, a: User, b: User, keys: dict[int, tuple[str, str]]):
        wa = self.ws_connect("chat A", a, "/ws/chat?client=foreground")
        wb = self.ws_connect("chat B", b, "/ws/chat?client=foreground")
        self.check("chat ping/pong A", self.ws_roundtrip("ping", wa, {"type": "ping"}, "pong").get("type") == "pong")
        self.check("chat ping/pong B", self.ws_roundtrip("ping", wb, {"type": "ping"}, "pong").get("type") == "pong")
        content = {"e2ee": 1, "v": "x25519+chacha20poly1305:v1", "key_id": secrets.token_hex(8),
                   "sender_key_id": keys[a.id][1], "recipient_key_id": keys[b.id][1],
                   "nonce": base64.b64encode(secrets.token_bytes(12)).decode(),
                   "ct": base64.b64encode(secrets.token_bytes(32)).decode()}
        sent = self.ws_roundtrip("chat send", wa, {"type": "chat", "to_user_id": b.id,
                          "message_type": "text", "content": json.dumps(content),
                          "client_message_id": str(uuid.uuid4())}, "sent")
        self.check("chat send acknowledgement", sent.get("type") == "sent", str(sent))
        delivered = json.loads(wb.recv())
        self.check("chat message delivered", delivered.get("type") == "chat", str(delivered))
        wb.send(json.dumps({"type": "mark_read", "to_user_id": a.id}))
        read_ack = self.ws_recv_until(wa, "read_ack", "chat read acknowledgement")
        self.check("chat read acknowledgement", read_ack.get("type") == "read_ack", str(read_ack))
        online = self.ws_connect("online A", a, "/ws/online")
        pong = self.ws_roundtrip("online ping", online, {"type": "ping"}, "pong")
        self.check("online ping/pong", pong.get("type") == "pong", str(pong))
        status = self.ws_roundtrip("online status", online, {"type": "check_online", "user_ids": [a.id, b.id]}, "online_status")
        self.check("online status response", status.get("type") == "online_status" and len(status.get("statuses", [])) == 2, str(status))

    def rtc_flow(self, a: User, b: User):
        for action in ("cancel", "reject"):
            pending = self.request(f"RTC {action} invite", "POST", "/api/rtc/call/invite", a,
                                   body={"peer_id": b.id, "call_type": "video"})
            pending_id = str((self.data(pending) or {}).get("call_id", ""))
            self.check(f"RTC {action} call id", bool(pending_id), str(pending))
            self.ws_recv_until(self.ws["chat B"], "rtc_invite", f"RTC {action} invite event")
            if action == "cancel":
                self.request("RTC cancel", "POST", "/api/rtc/call/cancel", a,
                             body={"call_id": pending_id})
                self.ws_recv_until(self.ws["chat B"], "rtc_cancel", "RTC cancel event")
            else:
                self.request("RTC reject", "POST", "/api/rtc/call/reject", b,
                             body={"call_id": pending_id, "reason": "rejected"})
                self.ws_recv_until(self.ws["chat A"], "rtc_reject", "RTC reject event")

        payload = self.request("RTC video invite", "POST", "/api/rtc/call/invite", a,
                               body={"peer_id": b.id, "call_type": "video"})
        call = self.data(payload) or {}
        self.call_id = str(call.get("call_id", ""))
        self.check("RTC call id", bool(self.call_id), str(call))
        invite = self.ws_recv_until(self.ws["chat B"], "rtc_invite", "RTC invite websocket event")
        self.check("RTC invite websocket event", invite.get("type") == "rtc_invite", str(invite))
        self.request("RTC accept", "POST", "/api/rtc/call/accept", b, body={"call_id": self.call_id})
        token_a = self.request("RTC token A", "POST", "/api/rtc/token", a,
                               body={"call_id": self.call_id, "call_type": "video", "peer_id": b.id})
        token_b = self.request("RTC token B", "POST", "/api/rtc/token", b,
                               body={"call_id": self.call_id, "call_type": "video"})
        return token_a, token_b

    def feed_flow(self, a: User):
        payload = self.request("feed create", "POST", "/api/feed/create", a,
                               body={"content": "Bocker E2E post", "media": []})
        self.post_id = int((self.data(payload) or {}).get("id", 0))
        self.check("feed post id", self.post_id > 0, str(payload))
        self.request("feed list", "GET", "/api/feed/list", a, params={"page": 1, "page_size": 20})
        self.request("my feed list", "GET", "/api/feed/my_posts", a, params={"page": 1, "page_size": 20})
        self.request("feed detail", "GET", "/api/feed/detail", a, params={"post_id": self.post_id})
        self.request("feed like", "POST", "/api/feed/like", a, body={"post_id": self.post_id})
        self.request("feed is liked", "GET", "/api/feed/is_liked", a, params={"post_id": self.post_id})
        comment = self.request("feed comment", "POST", "/api/feed/comment", a,
                               body={"post_id": self.post_id, "content": "E2E comment", "reply_to_id": None})
        self.comment_id = int((self.data(comment) or {}).get("id", 0))
        self.request("feed comments", "GET", "/api/feed/comments", a,
                     params={"post_id": self.post_id, "page": 1, "page_size": 20})
        if self.comment_id:
            self.request("delete feed comment", "DELETE", "/api/feed/comment", a,
                         body={"comment_id": self.comment_id})
        if self.post_id:
            self.request("delete feed post", "DELETE", "/api/feed/delete", a,
                         body={"post_id": self.post_id})

    def oss_flow(self, a: User):
        self.request("OSS upload URL", "GET", "/api/oss/upload-url", a,
                     params={"key": f"e2e-{secrets.token_hex(8)}.txt", "type": "chat"})
        self.request("OSS download URL", "GET", "/api/oss/download-url",
                     params={"key": "chat/e2e-nonexistent.txt"})
        # Exercise validation without writing disposable objects to OSS.
        image = {"file": ("e2e-empty.png", b"", "image/png")}
        self.request("chat image upload validation", "POST", "/api/chat/upload/image", a,
                     files=image, expected=(400,))
        video = {"file": ("e2e-empty.mp4", b"", "video/mp4")}
        self.request("chat video upload validation", "POST", "/api/chat/upload/video", a,
                     files=video, expected=(400,))

    async def livekit_flow(self, token_a: dict[str, Any], token_b: dict[str, Any]) -> bool:
        try:
            from livekit import rtc
        except ImportError:
            self.check("LiveKit SDK installed", False, "pip install livekit")
            return False
        da, db = self.data(token_a) or {}, self.data(token_b) or {}
        if not da.get("url") or not db.get("token"):
            self.check("LiveKit token payload", False, f"A={da} B={db}")
            return False
        room_a, room_b = rtc.Room(), rtc.Room()
        try:
            await asyncio.gather(room_a.connect(da["url"], da["token"]), room_b.connect(db["url"], db["token"]))
            self.check("LiveKit rooms connected", room_a.connection_state == rtc.ConnectionState.CONN_CONNECTED and
                       room_b.connection_state == rtc.ConnectionState.CONN_CONNECTED,
                       f"{room_a.connection_state}/{room_b.connection_state}")
            source = rtc.VideoSource(320, 240)
            track = rtc.LocalVideoTrack.create_video_track("e2e-video", source)
            await room_a.local_participant.publish_track(track)
            frame = rtc.VideoFrame(320, 240, rtc.VideoBufferType.RGBA, bytes(320 * 240 * 4))
            source.capture_frame(frame)
            await asyncio.sleep(3)
            remote = list(room_b.remote_participants.values())
            subscribed = any(pub.track is not None for p in remote for pub in p.track_publications.values())
            self.check("LiveKit video published/subscribed", bool(remote) and subscribed,
                       f"remote_participants={len(remote)}")
            return bool(remote) and subscribed
        except Exception as exc:
            self.check("LiveKit media call", False, repr(exc))
            return False
        finally:
            await asyncio.gather(room_a.disconnect(), room_b.disconnect(), return_exceptions=True)

    def cleanup(self):
        for sock in self.ws.values():
            try:
                sock.close()
            except Exception:
                pass
        if self.keep:
            print("E2E_KEEP_DATA=1: retaining disposable users and data")
            return
        if self.group_id and self.users:
            self.request("cleanup delete group", "POST", "/api/group/delete", self.users[0],
                         body={"group_id": self.group_id})
        if len(self.users) >= 2:
            for peer in self.users[1:]:
                self.request(f"cleanup delete friendship {peer.id}", "POST", "/api/friend/delete", self.users[0],
                             body={"friend_id": peer.id})
        for i, user in enumerate(self.users):
            self.request(f"cleanup delete user {i + 1}", "POST", "/api/user/delete", user)

    def run(self) -> int:
        try:
            self.http_redirect_flow()
            a, b, c = self.create_user(1), self.create_user(2), self.create_user(3)
            self.user_flow(a, b)
            self.friend_flow(a, b)
            self.friend_flow(a, c)
            keys = self.e2ee_flow([a, b, c])
            self.group_flow([a, b, c], keys)
            self.oss_flow(a)
            self.websocket_flow(a, b, keys)
            token_a, token_b = self.rtc_flow(a, b)
            asyncio.run(self.livekit_flow(token_a, token_b))
            self.request("RTC hangup", "POST", "/api/rtc/call/hangup", a,
                         body={"call_id": self.call_id})
            self.ws_recv_until(self.ws["chat B"], "rtc_hangup", "RTC hangup event")
            self.feed_flow(a)
        except Exception as exc:
            self.check("unexpected test exception", False, repr(exc))
        finally:
            self.cleanup()
        print(f"\nE2E completed: {len(self.failures)} failure(s)")
        for failure in self.failures:
            print(f" - {failure}")
        return 1 if self.failures else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default=os.getenv("E2E_BASE_URL", "https://mini.gelsomino.cn:444"))
    parser.add_argument("--http-base", default=os.getenv("E2E_HTTP_BASE_URL"),
                        help="clear-text HTTP listener, default derives :81 from --base")
    parser.add_argument("--keep-data", action="store_true", default=os.getenv("E2E_KEEP_DATA") == "1")
    args = parser.parse_args()
    return E2E(args.base, args.http_base, args.keep_data).run()


if __name__ == "__main__":
    sys.exit(main())
