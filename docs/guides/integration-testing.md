# 集成测试通用方案

> 适用于所有经过对话管道（LarkGateway → Agent → 外部执行）的功能特性。

## 核心模型

所有测试通过 `POST /api/dev/inject` (port 9090) 注入对话消息，三层验证：

| 层 | 验证内容 | 工具 |
|---|---|---|
| **Layer 1** | Agent 同步回复 | `jq .replies` |
| **Layer 2** | 内部执行路径 | `grep` log |
| **Layer 3** | 外部副作用（DB/文件/API/Kaku） | `curl`/`kaku cli` |

不同功能选择对应层：纯对话 → L1；Agent Teams → L1+L2；Kaku Runtime → L1+L2+L3。

## 基础设施

| 服务 | 地址 | 用途 |
|---|---|---|
| HTTP API | `localhost:8080` | 生产 API |
| Debug | `localhost:9090` | `POST /api/dev/inject` 测试专用 |

### inject 函数

```bash
INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
SENDER_ID="${SENDER_ID:-ou_e2e_test}"
TIMEOUT="${TIMEOUT:-120}"

inject() {
  local TEXT="$1" CHAT_ID="${2:-oc_e2e_default}" TIMEOUT_S="${3:-$TIMEOUT}"
  curl -s -X POST "$INJECT_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"text\":            $(printf '%s' "$TEXT" | jq -Rs .),
      \"chat_id\":         \"$CHAT_ID\",
      \"chat_type\":       \"p2p\",
      \"sender_id\":       \"$SENDER_ID\",
      \"timeout_seconds\": $TIMEOUT_S
    }"
}

assert_reply_contains() {
  local RESP="$1" KEYWORD="$2"
  echo "$RESP" | jq -r '.replies[].content' | grep -qi "$KEYWORD" \
    && echo "PASS: reply contains '$KEYWORD'" \
    || { echo "FAIL: reply missing '$KEYWORD'"; return 1; }
}

assert_no_error() {
  local ERR; ERR=$(echo "$1" | jq -r '.error // empty')
  [[ -z "$ERR" ]] && echo "PASS: no error" \
    || { echo "FAIL: error='$ERR'"; return 1; }
}
```

## 测试用例模板

```bash
tc_N_feature() {
  local CASE="TC-N"
  echo "=== $CASE: 功能描述 ==="

  # Layer 1: inject + 检查响应
  local RESP; RESP=$(inject "指令文本" "chat_id" 120)
  assert_no_error "$RESP"
  assert_reply_contains "$RESP" "关键词"

  # Layer 2: 日志轨迹
  sleep 2
  grep "期望日志关键词" ~/alex-service.log | tail -5

  # Layer 3: 外部状态（如适用）
  # curl / kaku cli get-text / 文件检查

  echo "$CASE: PASS"
}
```

## 脚本规范

测试脚本放在 `scripts/test/`，统一接口：

```bash
./scripts/test/<feature>-e2e.sh              # 全套
./scripts/test/<feature>-e2e.sh --case TC-1  # 单个用例
./scripts/test/<feature>-e2e.sh --dry-run    # 列出用例
```

## 已有套件

| 套件 | 脚本 | 覆盖功能 |
|---|---|---|
| Agent Teams | `scripts/test_agents_teams_e2e.sh` | 多 agent 协作 |
| Kaku Runtime | `scripts/test/kaku-runtime-e2e.sh` | CC/Codex session + hooks |

## 验收标准

| 维度 | 标准 |
|---|---|
| inject 成功 | HTTP 200，无 error |
| 有意义回复 | replies 非空 |
| 无回归 | 现有用例 PASS 率不下降 |
| 清理完整 | 无遗留测试 pane |
| 耗时合理 | 单用例 <5min，多 agent <15min |

## 故障排查

```bash
# 服务健康
curl -s http://localhost:8080/health | jq .

# 重启
CLAUDECODE= alex dev restart backend

# 日志路径
lsof -p $(lsof -i :8080 -Fp | head -1 | tr -d p) | grep "\.log"

# Kaku pane 清理
kaku cli list
kaku cli kill-pane --pane-id $PANE 2>/dev/null || true
```
