#!/usr/bin/env python3
"""
Enterprise WeCom Webhook Receiver
- Verifies URL challenge and message signature using token/timestamp/nonce/msg_signature
- Supports AES decrypt/encrypt for WeCom callbacks
- Minimal HTTP server for 7-day MVP delivery
"""

import base64
import hashlib
import os
import socket
import struct
import time
import xml.etree.ElementTree as ET
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse

from Crypto.Cipher import AES

TOKEN = os.getenv("WECOM_TOKEN", "replace-with-token")
ENCODING_AES_KEY = os.getenv("WECOM_ENCODING_AES_KEY", "replace-with-43-char-key")
CORP_ID = os.getenv("WECOM_CORP_ID", "ww_replace_with_corpid")
HOST = os.getenv("HOST", "0.0.0.0")
PORT = int(os.getenv("PORT", "8080"))


def sha1_sign(token: str, timestamp: str, nonce: str, encrypted: str) -> str:
    items = [token, timestamp, nonce, encrypted]
    items.sort()
    raw = "".join(items).encode("utf-8")
    return hashlib.sha1(raw).hexdigest()


def _pkcs7_unpad(data: bytes) -> bytes:
    pad = data[-1]
    if pad < 1 or pad > 32:
        raise ValueError("Invalid PKCS7 padding")
    return data[:-pad]


def _pkcs7_pad(data: bytes) -> bytes:
    block_size = 32
    pad_len = block_size - (len(data) % block_size)
    return data + bytes([pad_len]) * pad_len


def _aes_key_from_encoding_key(encoding_aes_key: str) -> bytes:
    return base64.b64decode(encoding_aes_key + "=")


def decrypt_wecom(encrypted_b64: str, encoding_aes_key: str, corp_id: str) -> str:
    aes_key = _aes_key_from_encoding_key(encoding_aes_key)
    cipher = AES.new(aes_key, AES.MODE_CBC, aes_key[:16])
    encrypted = base64.b64decode(encrypted_b64)
    plain_padded = cipher.decrypt(encrypted)
    plain = _pkcs7_unpad(plain_padded)

    content = plain[16:]
    xml_len = struct.unpack("!I", content[:4])[0]
    xml_bytes = content[4:4 + xml_len]
    recv_corp_id = content[4 + xml_len:].decode("utf-8")

    if recv_corp_id != corp_id:
        raise ValueError("CorpID mismatch")

    return xml_bytes.decode("utf-8")


def encrypt_wecom(reply_xml: str, encoding_aes_key: str, corp_id: str, nonce: str = None) -> dict:
    aes_key = _aes_key_from_encoding_key(encoding_aes_key)
    rand16 = os.urandom(16)
    xml_bytes = reply_xml.encode("utf-8")
    msg_len = struct.pack("!I", len(xml_bytes))
    raw = rand16 + msg_len + xml_bytes + corp_id.encode("utf-8")
    padded = _pkcs7_pad(raw)

    cipher = AES.new(aes_key, AES.MODE_CBC, aes_key[:16])
    encrypted = cipher.encrypt(padded)
    encrypted_b64 = base64.b64encode(encrypted).decode("utf-8")

    timestamp = str(int(time.time()))
    nonce = nonce or str(int(time.time() * 1000))
    signature = sha1_sign(TOKEN, timestamp, nonce, encrypted_b64)

    response_xml = f"""<xml>
<Encrypt><![CDATA[{encrypted_b64}]]></Encrypt>
<MsgSignature><![CDATA[{signature}]]></MsgSignature>
<TimeStamp>{timestamp}</TimeStamp>
<Nonce><![CDATA[{nonce}]]></Nonce>
</xml>"""

    return {
        "xml": response_xml,
        "encrypt": encrypted_b64,
        "msg_signature": signature,
        "timestamp": timestamp,
        "nonce": nonce,
    }


class WeComWebhookHandler(BaseHTTPRequestHandler):
    def _send(self, code: int, body: str, content_type: str = "text/plain; charset=utf-8"):
        body_bytes = body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body_bytes)))
        self.end_headers()
        self.wfile.write(body_bytes)

    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path != "/wecom/callback":
            self._send(404, "not found")
            return

        q = parse_qs(parsed.query)
        msg_signature = q.get("msg_signature", [""])[0]
        timestamp = q.get("timestamp", [""])[0]
        nonce = q.get("nonce", [""])[0]
        echostr = q.get("echostr", [""])[0]

        if not all([msg_signature, timestamp, nonce, echostr]):
            self._send(400, "missing query params")
            return

        expected = sha1_sign(TOKEN, timestamp, nonce, echostr)
        if expected != msg_signature:
            self._send(403, "signature verify failed")
            return

        try:
            plain = decrypt_wecom(echostr, ENCODING_AES_KEY, CORP_ID)
        except Exception as e:
            self._send(400, f"decrypt failed: {e}")
            return

        self._send(200, plain)

    def do_POST(self):
        parsed = urlparse(self.path)
        if parsed.path != "/wecom/callback":
            self._send(404, "not found")
            return

        q = parse_qs(parsed.query)
        msg_signature = q.get("msg_signature", [""])[0]
        timestamp = q.get("timestamp", [""])[0]
        nonce = q.get("nonce", [""])[0]

        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length).decode("utf-8")

        try:
            root = ET.fromstring(raw)
            encrypted = root.findtext("Encrypt", default="")
        except ET.ParseError:
            self._send(400, "invalid xml")
            return

        if not all([msg_signature, timestamp, nonce, encrypted]):
            self._send(400, "missing signature payload")
            return

        expected = sha1_sign(TOKEN, timestamp, nonce, encrypted)
        if expected != msg_signature:
            self._send(403, "signature verify failed")
            return

        try:
            plain_xml = decrypt_wecom(encrypted, ENCODING_AES_KEY, CORP_ID)
        except Exception as e:
            self._send(400, f"decrypt failed: {e}")
            return

        print("[WeCom Callback]", plain_xml)

        # Minimal passive reply
        reply = "success"
        encrypted_reply = encrypt_wecom(reply, ENCODING_AES_KEY, CORP_ID, nonce=nonce)
        self._send(200, encrypted_reply["xml"], content_type="application/xml; charset=utf-8")


def main():
    print(f"Starting WeCom webhook receiver on {HOST}:{PORT}")
    print("Required env: WECOM_TOKEN, WECOM_ENCODING_AES_KEY, WECOM_CORP_ID")
    server = HTTPServer((HOST, PORT), WeComWebhookHandler)
    server.serve_forever()


if __name__ == "__main__":
    main()

