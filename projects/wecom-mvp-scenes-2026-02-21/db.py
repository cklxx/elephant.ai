import os
import sqlite3
from datetime import datetime, timedelta

DB_PATH = os.getenv("DB_PATH", "data/wecom_mvp.db")


def get_conn():
    os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_db():
    conn = get_conn()
    cur = conn.cursor()
    cur.execute('''
    CREATE TABLE IF NOT EXISTS customers (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      external_userid TEXT UNIQUE,
      name TEXT,
      tags TEXT,
      purchased_at TEXT,
      expiry_at TEXT,
      last_interaction_at TEXT,
      webhook_url TEXT
    )
    ''')
    cur.execute('''
    CREATE TABLE IF NOT EXISTS followup_records (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      customer_id INTEGER,
      scene TEXT,
      scheduled_at TEXT,
      sent_at TEXT,
      status TEXT,
      template TEXT,
      message TEXT,
      replied INTEGER DEFAULT 0,
      FOREIGN KEY(customer_id) REFERENCES customers(id)
    )
    ''')
    conn.commit()
    conn.close()


def now_iso():
    return datetime.utcnow().isoformat()


def parse_iso(s: str):
    return datetime.fromisoformat(s)


def days_ago(days: int):
    return (datetime.utcnow() - timedelta(days=days)).isoformat()

