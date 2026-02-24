from __future__ import annotations

import base64
import hashlib
import logging
import struct
from typing import Optional
from xml.etree import ElementTree as ET

from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives.ciphers import Cipher, algorithms, modes
from flask import Flask, Response, request

from config import settings
from customer import CustomerStore
from handler import handle_text_message

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)
store = CustomerStore()


class WeComCrypto:
    def __init__(self, token: str, encoding_aes_key: str, receive_id: str):
        self.token = token
        self.receive_id = receive_id
        self.aes_key = base64.b64decode(encoding_aes_key + "=")
        self.iv = self.aes_key[:16]

    def _sha1(self, *parts: str) -> str:
        raw = "".join(sorted(parts)).encode("utf-8")
        return hashlib.sha1(raw).hexdigest()

    def verify_signature(self, signature: str, timestamp: str, nonce: str, encrypt: str) -> bool:
        return self._sha1(self.token, timestamp, nonce, encrypt) == signature

    def decrypt(self, encrypted_text: str) -> str:
        cipher = Cipher(algorithms.AES(self.aes_key), modes.CBC(self.iv), backend=default_backend())
        decryptor = cipher.decryptor()
        encrypted = base64.b64decode(encrypted_text)
        plain_padded = decryptor.update(encrypted) + decryptor.finalize()
        pad_len = plain_padded[-1]
        plain = plain_padded[:-pad_len]

        content = plain[16:]
        msg_len = struct.unpack("!I", content[:4])[0]
        msg = content[4 : 4 + msg_len]
        receive_id = content[4 + msg_len :].decode("utf-8")

        if receive_id != self.receive_id:
            raise ValueError("receive_id mismatch")
        return msg.decode("utf-8")


crypto = WeComCrypto(
    token=settings.wecom_token,
    encoding_aes_key=settings.wecom_encoding_aes_key,
    receive_id=settings.wecom_receive_id,
)


def extract_encrypt(xml_text: str) -> Optional[str]:
    root = ET.fromstring(xml_text)
    elem = root.find("Encrypt")
    return elem.text if elem is not None else None


@app.route("/wecom/callback", methods=["GET", "POST"])
def wecom_callback():
    signature = request.args.get("msg_signature", "")
    timestamp = request.args.get("timestamp", "")
    nonce = request.args.get("nonce", "")

    if request.method == "GET":
        echostr = request.args.get("echostr", "")
        if not crypto.verify_signature(signature, timestamp, nonce, echostr):
            return Response("invalid signature", status=401)
        try:
            plain = crypto.decrypt(echostr)
        except Exception as exc:
            logger.exception("failed to decrypt echostr: %s", exc)
            return Response("decrypt failed", status=400)
        return Response(plain, content_type="text/plain; charset=utf-8")

    body = request.data.decode("utf-8")
    encrypted = extract_encrypt(body)
    if not encrypted:
        return Response("missing Encrypt", status=400)
    if not crypto.verify_signature(signature, timestamp, nonce, encrypted):
        return Response("invalid signature", status=401)

    try:
        plain_xml = crypto.decrypt(encrypted)
        root = ET.fromstring(plain_xml)
        msg_type = (root.findtext("MsgType") or "").strip()
        if msg_type != "text":
            return Response("success", content_type="text/plain; charset=utf-8")

        user_id = (root.findtext("FromUserName") or "").strip()
        content = (root.findtext("Content") or "").strip()

        if not user_id:
            return Response("success", content_type="text/plain; charset=utf-8")

        tag, reply = handle_text_message(content)
        store.add_interaction(user_id, content, tag)

        logger.info("user=%s tag=%s reply=%s", user_id, tag, reply)
        return Response(reply, content_type="text/plain; charset=utf-8")
    except Exception as exc:
        logger.exception("callback handling failed: %s", exc)
        return Response("success", content_type="text/plain; charset=utf-8")


@app.get("/health")
def health():
    return {"status": "ok", "customers": store.count()}


if __name__ == "__main__":
    app.run(host=settings.host, port=settings.port, debug=settings.debug)

