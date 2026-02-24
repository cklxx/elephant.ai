from typing import Dict
from customer import CustomerStore


DEFAULT_REPLY = "收到您的消息，顾问将尽快联系您"

TAG_RULES: Dict[str, str] = {
    "价格": "price_intent",
    "报价": "price_intent",
    "方案": "solution_intent",
    "演示": "demo_intent",
    "合作": "biz_intent",
    "退款": "risk_signal",
    "投诉": "risk_signal",
}


class MessageHandler:
    def __init__(self, customer_store: CustomerStore):
        self.customer_store = customer_store

    def handle_text_message(self, external_userid: str, content: str) -> str:
        self.customer_store.update_by_message(
            external_userid=external_userid,
            content=content,
            tag_rules=TAG_RULES,
        )
        return DEFAULT_REPLY

