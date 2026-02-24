#!/usr/bin/env python3
"""
wecom_data_connector.py
企业微信客户数据接入脚本（MVP 框架）
- 支持客户数据拉取
- 支持标签同步（本地映射 + 远端更新占位）
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import Dict, List, Any

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger(__name__)


@dataclass
class WeComConfig:
    corp_id: str
    corp_secret: str
    agent_id: str
    api_base: str = "https://qyapi.weixin.qq.com"


class WeComDataConnector:
    def __init__(self, config: WeComConfig) -> None:
        self.config = config
        self.access_token: str | None = None

    def get_access_token(self) -> str:
        """获取企业微信 access_token（MVP：返回占位，生产接入真实 API）"""
        logger.info("Fetching access token...")
        self.access_token = "mock_access_token"
        return self.access_token

    def fetch_external_contacts(self) -> List[Dict[str, Any]]:
        """拉取外部联系人列表（MVP：模拟数据）"""
        logger.info("Fetching external contacts...")
        return [
            {"external_userid": "wmock_001", "name": "客户A", "tags": ["意向", "教育"]},
            {"external_userid": "wmock_002", "name": "客户B", "tags": ["沉默", "零售"]},
        ]

    def build_tag_mapping(self) -> Dict[str, str]:
        """本地业务标签到企微标签 ID 的映射（MVP 占位）"""
        return {
            "意向": "tag_high_intent",
            "沉默": "tag_silent",
            "续费": "tag_renewal",
            "教育": "tag_industry_edu",
            "零售": "tag_industry_retail",
        }

    def sync_tags(self, contacts: List[Dict[str, Any]]) -> None:
        """同步标签到企微（MVP：日志输出 + 调用占位）"""
        tag_mapping = self.build_tag_mapping()
        for c in contacts:
            external_userid = c.get("external_userid")
            mapped_tags = [tag_mapping[t] for t in c.get("tags", []) if t in tag_mapping]
            logger.info("Sync tags for %s -> %s", external_userid, mapped_tags)
            # TODO: 调用企业微信 API 同步标签

    def run(self) -> None:
        self.get_access_token()
        contacts = self.fetch_external_contacts()
        self.sync_tags(contacts)
        logger.info("Connector run complete. contacts=%s", json.dumps(contacts, ensure_ascii=False))


if __name__ == "__main__":
    cfg = WeComConfig(corp_id="${WECOM_CORP_ID}", corp_secret="${WECOM_CORP_SECRET}", agent_id="${WECOM_AGENT_ID}")
    WeComDataConnector(cfg).run()

