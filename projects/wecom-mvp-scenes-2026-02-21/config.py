import os

CONFIG = {
    "aftersales": {
        "delay_days": int(os.getenv("AFTERSALES_DELAY_DAYS", "3")),
        "template": os.getenv("AFTERSALES_TEMPLATE", "您好{customer_name}，购买后使用体验如何？请回复1-5分满意度。")
    },
    "renewal": {
        "days_before": [int(x) for x in os.getenv("RENEWAL_DAYS_BEFORE", "7,3,0").split(",")],
        "template": os.getenv("RENEWAL_TEMPLATE", "您好{customer_name}，您的服务将在{days_left}天后到期，回复续费了解方案。")
    },
    "reactivation": {
        "silent_days": int(os.getenv("REACTIVATION_SILENT_DAYS", "30")),
        "template": os.getenv("REACTIVATION_TEMPLATE", "您好{customer_name}，我们为您准备了专属优惠/新品信息，回复领取。")
    }
}

