#!/usr/bin/env python3
"""wiki-knowledge skill — 飞书知识库管理。

通过 channel tool 的 wiki actions 管理飞书知识库空间和节点。
当前为框架实现，实际调用通过 channel tool 的 list_wiki_*/create_wiki_*/get_wiki_* actions。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json
import os
import urllib.error
import urllib.request


def _lark_api(method: str, path: str, body: dict | None = None) -> dict:
    """Call Lark Open API (placeholder — needs tenant_access_token)."""
    base = "https://open.feishu.cn/open-apis"
    token = os.environ.get("LARK_TENANT_TOKEN", "")
    if not token:
        return {"error": "LARK_TENANT_TOKEN not set, wiki operations unavailable"}

    url = f"{base}{path}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def list_spaces(args: dict) -> dict:
    params = ""
    if args.get("page_size"):
        params += f"?page_size={args['page_size']}"
    if args.get("page_token"):
        sep = "&" if params else "?"
        params += f"{sep}page_token={args['page_token']}"

    result = _lark_api("GET", f"/wiki/v2/spaces{params}")
    if "error" in result:
        return {"success": False, **result}
    spaces = result.get("data", {}).get("items", [])
    return {"success": True, "spaces": spaces, "count": len(spaces)}


def list_nodes(args: dict) -> dict:
    space_id = args.get("space_id", "")
    if not space_id:
        return {"success": False, "error": "space_id is required"}

    params = ""
    if args.get("parent_node_token"):
        params += f"?parent_node_token={args['parent_node_token']}"
    if args.get("page_size"):
        sep = "&" if params else "?"
        params += f"{sep}page_size={args['page_size']}"

    result = _lark_api("GET", f"/wiki/v2/spaces/{space_id}/nodes{params}")
    if "error" in result:
        return {"success": False, **result}
    nodes = result.get("data", {}).get("items", [])
    return {"success": True, "nodes": nodes, "count": len(nodes)}


def create_node(args: dict) -> dict:
    space_id = args.get("space_id", "")
    if not space_id:
        return {"success": False, "error": "space_id is required"}

    body = {
        "obj_type": args.get("obj_type", "docx"),
    }
    if args.get("parent_node_token"):
        body["parent_node_token"] = args["parent_node_token"]
    if args.get("title"):
        body["node_type"] = "origin"
        body["title"] = args["title"]

    result = _lark_api("POST", f"/wiki/v2/spaces/{space_id}/nodes", body)
    if "error" in result:
        return {"success": False, **result}
    node = result.get("data", {}).get("node", {})
    return {"success": True, "node": node, "message": "知识节点已创建"}


def get_node(args: dict) -> dict:
    node_token = args.get("node_token", "")
    if not node_token:
        return {"success": False, "error": "node_token is required"}

    result = _lark_api("GET", f"/wiki/v2/spaces/get_node?token={node_token}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "node": result.get("data", {}).get("node", {})}


def run(args: dict) -> dict:
    action = args.pop("action", "list_spaces")

    handlers = {
        "list_spaces": list_spaces,
        "list_nodes": list_nodes,
        "create_node": create_node,
        "get_node": get_node,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}, valid: {list(handlers)}"}
    return handler(args)


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}

    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success", False) else 1)


if __name__ == "__main__":
    main()
