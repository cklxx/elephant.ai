#!/usr/bin/env python3
"""
AnyGen OpenAPI Client

Usage:
    python3 anygen.py create --api-key sk-xxx --operation slide --prompt "..."
    python3 anygen.py poll --api-key sk-xxx --task-id task_xxx
    python3 anygen.py download --api-key sk-xxx --task-id task_xxx --output ./
    python3 anygen.py run --api-key sk-xxx --operation slide --prompt "..." --output ./
"""

import argparse
import base64
import json
import os
import sys
import time
from datetime import datetime
from pathlib import Path

try:
    import requests
except ImportError:
    print("[ERROR] requests library not found. Install with: pip3 install requests")
    sys.exit(1)


API_BASE = "https://www.anygen.io"
POLL_INTERVAL = 3  # seconds
MAX_POLL_TIME = 600  # 10 minutes
CONFIG_DIR = Path.home() / ".config" / "anygen"
CONFIG_FILE = CONFIG_DIR / "config.json"
ENV_API_KEY = "ANYGEN_API_KEY"


def load_config():
    """Load configuration from file."""
    if not CONFIG_FILE.exists():
        return {}
    try:
        with open(CONFIG_FILE, "r") as f:
            return json.load(f)
    except (json.JSONDecodeError, IOError):
        return {}


def save_config(config):
    """Save configuration to file."""
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    with open(CONFIG_FILE, "w") as f:
        json.dump(config, f, indent=2)
    # Set file permissions to owner read/write only (600)
    CONFIG_FILE.chmod(0o600)


def get_api_key(args_api_key=None):
    """Get API key with priority: command line > env var > config file."""
    # 1. Command line argument
    if args_api_key:
        return args_api_key

    # 2. Environment variable
    env_key = os.environ.get(ENV_API_KEY)
    if env_key:
        return env_key

    # 3. Config file
    config = load_config()
    return config.get("api_key")


def log_info(msg):
    print(f"[INFO] {msg}")


def log_success(msg):
    print(f"[SUCCESS] {msg}")


def log_error(msg):
    print(f"[ERROR] {msg}")


def log_progress(status, progress):
    print(f"[PROGRESS] 状态: {status}, 进度: {progress}%")


def format_timestamp(ts):
    """Convert Unix timestamp to readable datetime."""
    if not ts:
        return "N/A"
    return datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M:%S")


def parse_headers(header_list):
    """Parse header list from command line into dict."""
    if not header_list:
        return None
    headers = {}
    for h in header_list:
        if ":" in h:
            key, value = h.split(":", 1)
            headers[key.strip()] = value.strip()
    return headers if headers else None


def encode_file(file_path):
    """Encode file to base64."""
    path = Path(file_path)
    if not path.exists():
        raise FileNotFoundError(f"File not found: {file_path}")

    with open(path, "rb") as f:
        content = f.read()

    # Determine MIME type
    suffix = path.suffix.lower()
    mime_types = {
        ".pdf": "application/pdf",
        ".png": "image/png",
        ".jpg": "image/jpeg",
        ".jpeg": "image/jpeg",
        ".gif": "image/gif",
        ".txt": "text/plain",
        ".doc": "application/msword",
        ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        ".ppt": "application/vnd.ms-powerpoint",
        ".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    }
    mime_type = mime_types.get(suffix, "application/octet-stream")

    return {
        "file_name": path.name,
        "file_type": mime_type,
        "file_data": base64.b64encode(content).decode("utf-8")
    }


def create_task(api_key, operation, prompt, language=None, slide_count=None,
                template=None, ratio=None, doc_format=None, files=None, extra_headers=None, style=None):
    """Create an async generation task."""
    log_info("创建任务中...")

    # Build auth token
    auth_token = api_key if api_key.startswith("Bearer ") else f"Bearer {api_key}"

    # Enhance prompt with style if provided
    final_prompt = prompt
    if style:
        final_prompt = f"{prompt}\n\n风格要求: {style}"
        log_info(f"已添加风格要求: {style}")

    # Build request body
    body = {
        "auth_token": auth_token,
        "operation": operation,
        "prompt": final_prompt
    }

    if language:
        body["language"] = language

    # Slide-specific parameters
    if operation == "slide":
        if slide_count:
            body["slide_count"] = slide_count
        if template:
            body["template"] = template
        if ratio:
            body["ratio"] = ratio

    # Doc-specific parameters
    if operation == "doc":
        if doc_format:
            body["doc_format"] = doc_format

    # Process files
    if files:
        encoded_files = []
        for file_path in files:
            try:
                encoded_files.append(encode_file(file_path))
                log_info(f"已添加附件: {file_path}")
            except FileNotFoundError as e:
                log_error(str(e))
                return None
        if encoded_files:
            body["files"] = encoded_files

    # Build headers
    headers = {"Content-Type": "application/json"}
    if extra_headers:
        headers.update(extra_headers)

    # Send request
    try:
        log_info(f"请求 URL: {API_BASE}/v1/openapi/tasks")
        if extra_headers:
            log_info(f"额外 Headers: {extra_headers}")
        response = requests.post(
            f"{API_BASE}/v1/openapi/tasks",
            json=body,
            headers=headers,
            timeout=30
        )
        log_info(f"响应状态码: {response.status_code}")
        log_info(f"响应内容: {response.text[:500] if response.text else 'Empty'}")
        if response.status_code != 200:
            log_error(f"HTTP 错误: {response.status_code}")
            return None
        result = response.json()
    except requests.RequestException as e:
        log_error(f"请求失败: {e}")
        return None
    except json.JSONDecodeError:
        log_error(f"响应解析失败: {response.text[:500] if response.text else 'Empty'}")
        return None

    if result.get("success"):
        task_id = result.get("task_id")
        log_success("任务创建成功!")
        print(f"Task ID: {task_id}")
        return task_id
    else:
        log_error(f"任务创建失败: {result.get('error', 'Unknown error')}")
        return None


def query_task(api_key, task_id, extra_headers=None):
    """Query task status."""
    auth_token = api_key if api_key.startswith("Bearer ") else f"Bearer {api_key}"

    headers = {"Authorization": auth_token}
    if extra_headers:
        headers.update(extra_headers)

    try:
        response = requests.get(
            f"{API_BASE}/v1/openapi/tasks/{task_id}",
            headers=headers,
            timeout=30
        )
        return response.json()
    except requests.RequestException as e:
        log_error(f"请求失败: {e}")
        return None
    except json.JSONDecodeError:
        log_error(f"响应解析失败: {response.text}")
        return None


def poll_task(api_key, task_id, max_time=MAX_POLL_TIME, extra_headers=None):
    """Poll task until completion or failure."""
    log_info(f"查询任务状态: {task_id}")

    start_time = time.time()
    last_progress = -1

    while True:
        elapsed = time.time() - start_time
        if elapsed > max_time:
            log_error(f"轮询超时 ({max_time}秒)")
            return None

        task = query_task(api_key, task_id, extra_headers)
        if not task:
            time.sleep(POLL_INTERVAL)
            continue

        status = task.get("status")
        progress = task.get("progress", 0)

        # Only log progress if it changed
        if progress != last_progress:
            log_progress(status, progress)
            last_progress = progress

        if status == "completed":
            output = task.get("output", {})
            log_success("任务完成!")
            print(f"文件名: {output.get('file_name', 'N/A')}")
            print(f"下载链接: {output.get('file_url', 'N/A')}")
            print(f"链接有效期至: {format_timestamp(output.get('expires_at'))}")
            if output.get("slide_count"):
                print(f"PPT 页数: {output.get('slide_count')}")
            if output.get("word_count"):
                print(f"字数: {output.get('word_count')}")
            return task

        elif status == "failed":
            log_error("任务失败!")
            print(f"错误信息: {task.get('error', 'Unknown error')}")
            return task

        time.sleep(POLL_INTERVAL)


def download_file(api_key, task_id, output_dir, extra_headers=None):
    """Download the generated file."""
    # First query task to get file URL
    task = query_task(api_key, task_id, extra_headers)
    if not task:
        return False

    if task.get("status") != "completed":
        log_error(f"任务未完成，当前状态: {task.get('status')}")
        return False

    output = task.get("output", {})
    file_url = output.get("file_url")
    file_name = output.get("file_name")

    if not file_url:
        log_error("无法获取下载链接")
        return False

    log_info("下载文件中...")

    try:
        response = requests.get(file_url, timeout=120)
        response.raise_for_status()
    except requests.RequestException as e:
        log_error(f"下载失败: {e}")
        return False

    # Ensure output directory exists
    output_path = Path(output_dir)
    output_path.mkdir(parents=True, exist_ok=True)

    # Save file
    file_path = output_path / file_name
    with open(file_path, "wb") as f:
        f.write(response.content)

    log_success(f"文件已保存: {file_path}")
    return True


def run_full_workflow(api_key, operation, prompt, output_dir, extra_headers=None, style=None, **kwargs):
    """Run the full workflow: create -> poll -> download."""
    # Create task
    task_id = create_task(api_key, operation, prompt, extra_headers=extra_headers, style=style, **kwargs)
    if not task_id:
        return False

    # Poll for completion
    task = poll_task(api_key, task_id, extra_headers=extra_headers)
    if not task or task.get("status") != "completed":
        return False

    # Download file
    if output_dir:
        return download_file(api_key, task_id, output_dir, extra_headers=extra_headers)

    return True


def main():
    parser = argparse.ArgumentParser(
        description="AnyGen OpenAPI Client",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Create a slide task
  python3 anygen.py create -k sk-xxx -o slide -p "关于AI的演示文稿"

  # Poll task status
  python3 anygen.py poll -k sk-xxx --task-id task_xxx

  # Download generated file
  python3 anygen.py download -k sk-xxx --task-id task_xxx --output ./

  # Run full workflow
  python3 anygen.py run -k sk-xxx -o slide -p "关于AI的演示文稿" --output ./
        """
    )

    subparsers = parser.add_subparsers(dest="command", help="Commands")

    # Common arguments
    def add_common_args(p):
        p.add_argument("--api-key", "-k", help="AnyGen API Key (sk-xxx). Can also use env ANYGEN_API_KEY or config file")
        p.add_argument("--header", "-H", action="append", dest="headers",
                       help="Extra HTTP header (format: 'Key:Value', can be used multiple times)")

    # Create command
    create_parser = subparsers.add_parser("create", help="Create a generation task")
    add_common_args(create_parser)
    create_parser.add_argument("--operation", "-o", required=True,
                               choices=["chat", "slide", "doc", "storybook", "data_analysis", "website"],
                               help="Operation type: chat, slide, doc, storybook, data_analysis, website")
    create_parser.add_argument("--prompt", "-p", required=True, help="Content prompt")
    create_parser.add_argument("--language", "-l", help="Language (zh-CN, en-US)")
    create_parser.add_argument("--slide-count", "-c", type=int, help="Number of slides")
    create_parser.add_argument("--template", "-t", help="Slide template")
    create_parser.add_argument("--ratio", "-r", choices=["16:9", "4:3"], help="Slide ratio")
    create_parser.add_argument("--doc-format", "-f", choices=["docx", "pdf"], help="Document format")
    create_parser.add_argument("--file", action="append", dest="files", help="Attachment file path (can be used multiple times)")
    create_parser.add_argument("--style", "-s", help="Style preference (e.g., '商务正式', '简约现代', '科技感')")

    # Poll command
    poll_parser = subparsers.add_parser("poll", help="Poll task status until completion")
    add_common_args(poll_parser)
    poll_parser.add_argument("--task-id", required=True, help="Task ID to poll")

    # Download command
    download_parser = subparsers.add_parser("download", help="Download generated file")
    add_common_args(download_parser)
    download_parser.add_argument("--task-id", required=True, help="Task ID")
    download_parser.add_argument("--output", required=True, help="Output directory")

    # Run command (full workflow)
    run_parser = subparsers.add_parser("run", help="Run full workflow: create -> poll -> download")
    add_common_args(run_parser)
    run_parser.add_argument("--operation", "-o", required=True,
                           choices=["chat", "slide", "doc", "storybook", "data_analysis", "website"],
                           help="Operation type: chat, slide, doc, storybook, data_analysis, website")
    run_parser.add_argument("--prompt", "-p", required=True, help="Content prompt")
    run_parser.add_argument("--language", "-l", help="Language (zh-CN, en-US)")
    run_parser.add_argument("--slide-count", "-c", type=int, help="Number of slides")
    run_parser.add_argument("--template", "-t", help="Slide template")
    run_parser.add_argument("--ratio", "-r", choices=["16:9", "4:3"], help="Slide ratio")
    run_parser.add_argument("--doc-format", "-f", choices=["docx", "pdf"], help="Document format")
    run_parser.add_argument("--file", action="append", dest="files", help="Attachment file path")
    run_parser.add_argument("--style", "-s", help="Style preference (e.g., '商务正式', '简约现代', '科技感')")
    run_parser.add_argument("--output", help="Output directory (optional)")

    # Config command
    config_parser = subparsers.add_parser("config", help="Manage configuration")
    config_subparsers = config_parser.add_subparsers(dest="config_action", help="Config actions")

    # config set
    config_set_parser = config_subparsers.add_parser("set", help="Set a config value")
    config_set_parser.add_argument("key", choices=["api_key", "default_language"], help="Config key")
    config_set_parser.add_argument("value", help="Config value")

    # config get
    config_get_parser = config_subparsers.add_parser("get", help="Get a config value")
    config_get_parser.add_argument("key", nargs="?", help="Config key (omit to show all)")

    # config delete
    config_delete_parser = config_subparsers.add_parser("delete", help="Delete a config value")
    config_delete_parser.add_argument("key", help="Config key to delete")

    # config path
    config_subparsers.add_parser("path", help="Show config file path")

    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        sys.exit(1)

    # Handle config command separately (doesn't need API key)
    if args.command == "config":
        if not args.config_action:
            config_parser.print_help()
            sys.exit(1)

        if args.config_action == "path":
            print(f"Config file: {CONFIG_FILE}")
            sys.exit(0)

        elif args.config_action == "set":
            config = load_config()
            config[args.key] = args.value
            save_config(config)
            # Mask API key in output
            display_value = args.value[:10] + "..." if args.key == "api_key" and len(args.value) > 10 else args.value
            log_success(f"已设置 {args.key} = {display_value}")
            sys.exit(0)

        elif args.config_action == "get":
            config = load_config()
            if args.key:
                value = config.get(args.key)
                if value:
                    # Mask API key
                    if args.key == "api_key" and len(value) > 10:
                        value = value[:10] + "..."
                    print(f"{args.key} = {value}")
                else:
                    print(f"{args.key} is not set")
            else:
                if config:
                    for k, v in config.items():
                        # Mask API key
                        if k == "api_key" and len(v) > 10:
                            v = v[:10] + "..."
                        print(f"{k} = {v}")
                else:
                    print("No config set")
            sys.exit(0)

        elif args.config_action == "delete":
            config = load_config()
            if args.key in config:
                del config[args.key]
                save_config(config)
                log_success(f"已删除 {args.key}")
            else:
                log_error(f"{args.key} not found in config")
            sys.exit(0)

    # For other commands, resolve API key
    api_key = get_api_key(getattr(args, 'api_key', None))
    if not api_key:
        log_error("未找到 API Key。请通过以下方式之一提供:")
        print("  1. 命令行参数: --api-key sk-xxx")
        print(f"  2. 环境变量: export {ENV_API_KEY}=sk-xxx")
        print(f"  3. 配置文件: python3 anygen.py config set api_key sk-xxx")
        sys.exit(1)

    # Parse extra headers
    extra_headers = parse_headers(args.headers) if hasattr(args, 'headers') else None

    if args.command == "create":
        task_id = create_task(
            api_key=api_key,
            operation=args.operation,
            prompt=args.prompt,
            language=args.language,
            slide_count=args.slide_count,
            template=args.template,
            ratio=args.ratio,
            doc_format=args.doc_format,
            files=args.files,
            extra_headers=extra_headers,
            style=args.style
        )
        sys.exit(0 if task_id else 1)

    elif args.command == "poll":
        task = poll_task(api_key, args.task_id, extra_headers=extra_headers)
        if task and task.get("status") == "completed":
            sys.exit(0)
        else:
            sys.exit(1)

    elif args.command == "download":
        success = download_file(api_key, args.task_id, args.output, extra_headers=extra_headers)
        sys.exit(0 if success else 1)

    elif args.command == "run":
        success = run_full_workflow(
            api_key=api_key,
            operation=args.operation,
            prompt=args.prompt,
            output_dir=args.output,
            extra_headers=extra_headers,
            language=args.language,
            slide_count=args.slide_count,
            template=args.template,
            ratio=args.ratio,
            doc_format=args.doc_format,
            files=args.files,
            style=args.style
        )
        sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
