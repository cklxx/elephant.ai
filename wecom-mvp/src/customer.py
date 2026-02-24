from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List


@dataclass
class InteractionRecord:
    timestamp: str
    content: str


@dataclass
class CustomerProfile:
    external_userid: str
    tags: List[str] = field(default_factory=list)
    interactions: List[InteractionRecord] = field(default_factory=list)

    def add_tag(self, tag: str):
        if tag not in self.tags:
            self.tags.append(tag)

    def add_interaction(self, content: str):
        self.interactions.append(
            InteractionRecord(timestamp=datetime.utcnow().isoformat(), content=content)
        )


class CustomerStore:
    """内存实现：MVP阶段先用内存，后续可替换为Redis/MySQL。"""

    def __init__(self):
        self._customers: Dict[str, CustomerProfile] = {}

    def get_or_create(self, external_userid: str) -> CustomerProfile:
        if external_userid not in self._customers:
            self._customers[external_userid] = CustomerProfile(external_userid=external_userid)
        return self._customers[external_userid]

    def update_by_message(self, external_userid: str, content: str, tag_rules: Dict[str, str]):
        customer = self.get_or_create(external_userid)
        customer.add_interaction(content)

        lowered = content.lower()
        for keyword, tag in tag_rules.items():
            if keyword in lowered:
                customer.add_tag(tag)

        return customer

