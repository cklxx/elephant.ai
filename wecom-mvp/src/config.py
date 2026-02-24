import os


class Settings:
    # 企业微信基础配置（请替换为真实值）
    CORP_ID = os.getenv("WECOM_CORP_ID", "wwxxxxxxxxxxxxxxxx")
    AGENT_ID = int(os.getenv("WECOM_AGENT_ID", "1000002"))
    APP_SECRET = os.getenv("WECOM_APP_SECRET", "your_app_secret")

    # 回调配置（需与企微后台一致）
    TOKEN = os.getenv("WECOM_TOKEN", "your_token")
    ENCODING_AES_KEY = os.getenv("WECOM_ENCODING_AES_KEY", "your_encoding_aes_key_43_chars")

    # 服务配置
    HOST = os.getenv("HOST", "0.0.0.0")
    PORT = int(os.getenv("PORT", "8000"))


settings = Settings()

