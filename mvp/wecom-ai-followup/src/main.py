import os
import time
import yaml
import logging
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Dict, List, Any

import requests
from apscheduler.schedulers.blocking import BlockingScheduler

"""
企业微信 AI 跟进 MVP（售后回访场景）

目标：客户购买后 24h 自动发送关怀消息（框架版，可无真实密钥运行）

接入说明（框架预留）：
1) 会话/订单来源：
   - 真实环境可接企业微信会话存档 API 或内部订单系统，拉取客户购买时间与 external_userid。
2) 发送消息：
   - 通过企业微信客服/应用消息接口发送文本消息。
3) 当前实现：
   - 使用本地 mock 订单数据触发逻辑；若未配置真实 key，则走 dry-run。
"""

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")


@dataclass
class WeComConfig:
    corp_id: str
    corp_secret: str
    agent_id: str
    token_url: str
    send_url: str
    dry_run: bool


class WeComClient:
    """
    企业微信 API 客户端封装（最小化）
    - get_access_token
    - send_text_message

    真实接入时：
    1) 将 corp_id / corp_secret / agent_id 写入 config.yaml 或环境变量。
    2) 可扩展重试、熔断、限流、签名校验等。
    """

    def __init__(self, cfg: WeComConfig):
        self.cfg = cfg
        self._cached_token = None
        self._token_expire_at = 0

    def get_access_token(self) -> str:
        if self.cfg.dry_run:
            return "dry-run-token"

        now_ts = int(time.time())
        if self._cached_token and now_ts < self._token_expire_at:
            return self._cached_token

        resp = requests.get(
            self.cfg.token_url,
            params={"corpid": self.cfg.corp_id, "corpsecret": self.cfg.corp_secret},
            timeout=10,
        )
        resp.raise_for_status()
        data = resp.json()
        if data.get("errcode") != 0:
            raise RuntimeError(f"get_access_token failed: {data}")

        self._cached_token = data["access_token"]
        self._token_expire_at = now_ts + int(data.get("expires_in", 7200)) - 60
        return self._cached_token

    def send_text_message(self, external_userid: str, content: str) -> None:
        if self.cfg.dry_run:
            logging.info("[DRY-RUN] send message -> external_userid=%s content=%s", external_userid, content)
            return

        access_token = self.get_access_token()
        resp = requests.post(
            f"{self.cfg.send_url}?access_token={access_token}",
            json={
                "touser": external_userid,
                "msgtype": "text",
                "agentid": int(self.cfg.agent_id),
                "text": {"content": content},
                "safe": 0,
            },
            timeout=10,
        )
        resp.raise_for_status()
        data = resp.json()
        if data.get("errcode") != 0:
            raise RuntimeError(f"send_text_message failed: {data}")


class TemplateRenderer:
    """消息模板渲染：支持 {customer_name} 等变量"""

    @staticmethod
    def render(template: str, context: Dict[str, Any]) -> str:
        return template.format(**context)


class AfterSalesFollowupService:
    """
    售后回访业务逻辑：
    - 找到购买时间 >= 24h 且未触达的客户
    - 渲染模板并发送消息

    真实接入建议：
    - 订单数据从数据库/CRM读取
    - 发送记录写入数据库避免重复触达
    - 接入会话存档 API 做回复监控与二次跟进
    """

    def __init__(self, client: WeComClient, message_template: str):
        self.client = client
        self.message_template = message_template
        self._sent = set()  # MVP 级内存去重（生产需持久化）

    def load_recent_orders(self) -> List[Dict[str, Any]]:
        # TODO: 接入真实订单系统或企微会话存档关联数据
        now = datetime.now(timezone.utc)
        return [
            {
                "order_id": "o-1001",
                "external_userid": "wm_user_001",
                "customer_name": "张三",
                "purchase_time": now - timedelta(hours=25),
            },
            {
                "order_id": "o-1002",
                "external_userid": "wm_user_002",
                "customer_name": "李四",
                "purchase_time": now - timedelta(hours=5),
            },
        ]

    def run_once(self) -> None:
        now = datetime.now(timezone.utc)
        orders = self.load_recent_orders()
        for order in orders:
            order_id = order["order_id"]
            if order_id in self._sent:
                continue

            hours_passed = (now - order["purchase_time"]).total_seconds() / 3600
            if hours_passed < 24:
                continue

            msg = TemplateRenderer.render(
                self.message_template,
                {
                    "customer_name": order["customer_name"],
                    "hours": int(hours_passed),
                },
            )
            self.client.send_text_message(order["external_userid"], msg)
            self._sent.add(order_id)
            logging.info("followup sent: order_id=%s external_userid=%s", order_id, order["external_userid"])


def load_config(path: str) -> Dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def build_client(cfg: Dict[str, Any]) -> WeComClient:
    wc = cfg["wecom"]
    return WeComClient(
        WeComConfig(
            corp_id=os.getenv("WECOM_CORP_ID", wc.get("corp_id", "")),
            corp_secret=os.getenv("WECOM_CORP_SECRET", wc.get("corp_secret", "")),
            agent_id=os.getenv("WECOM_AGENT_ID", wc.get("agent_id", "1000002")),
            token_url=wc["token_url"],
            send_url=wc["send_url"],
            dry_run=bool(wc.get("dry_run", True)),
        )
    )


def main() -> None:
    base_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    config_path = os.path.join(base_dir, "config.yaml")
    cfg = load_config(config_path)

    client = build_client(cfg)
    template = cfg["templates"]["after_sales_followup"]
    service = AfterSalesFollowupService(client, template)

    scheduler = BlockingScheduler(timezone="UTC")
    cron_expr = cfg["triggers"]["after_sales_followup_cron"]

    # 按 cron 定时执行售后回访扫描（默认每 10 分钟）
    minute, hour, day, month, day_of_week = cron_expr.split()
    scheduler.add_job(
        service.run_once,
        trigger="cron",
        minute=minute,
        hour=hour,
        day=day,
        month=month,
        day_of_week=day_of_week,
        id="after_sales_followup_job",
        replace_existing=True,
    )

    # 启动时先跑一次，便于本地快速验证
    service.run_once()
    logging.info("scheduler started, cron=%s", cron_expr)
    scheduler.start()


if __name__ == "__main__":
    main()

