# Claude Code Visualizer - 故障排除指南

## 🔍 问题：Claude Code 没有触发 Hook

### 诊断步骤

#### 1. 运行自动诊断

```bash
./scripts/diagnose-visualizer-hooks.sh
```

如果所有检查都通过 ✅，继续下一步。

#### 2. 启用调试日志

在启动 Claude Code 时添加 `DEBUG=1`：

```bash
DEBUG=1 claude-code
```

或在当前 shell 中设置：

```bash
export DEBUG=1
claude-code
```

#### 3. 测试 Hook 是否被调用

在 Claude Code 中执行一个简单命令：

```
> Read the README.md file
```

然后立即检查日志：

```bash
tail -20 ~/.claude/visualizer-hook.log
```

**如果日志文件不存在或为空**：Hook 没有被触发！继续排查。

**如果日志显示事件**：Hook 被触发了，检查事件是否到达 API。

#### 4. 检查 Claude Code 版本和配置

```bash
claude-code --version
```

确认你使用的是 **Claude Code CLI**，而不是：
- VSCode 扩展（不支持 hooks）
- Claude.ai 网页版（不支持 hooks）
- 其他 IDE 插件

#### 5. 检查 hooks 是否启用

某些 Claude Code 版本可能需要显式启用 hooks：

```bash
# 检查 ~/.claude/config.json
cat ~/.claude/config.json | jq .hooks

# 如果 hooks.enabled 为 false，启用它
# 编辑 ~/.claude/config.json，添加：
{
  "hooks": {
    "enabled": true
  }
}
```

---

## 🐛 常见问题和解决方案

### 问题 1: Hook 日志显示 "Failed to read stdin"

**原因**：Claude Code 可能不是通过 stdin 传递事件

**解决方案**：

1. 检查 Claude Code 文档确认 hook 事件格式
2. 尝试使用环境变量而非 stdin：

编辑 `~/.claude/hooks/visualizer-hook.sh`，在开头添加：

```bash
# Debug: dump all environment variables
if [ "$DEBUG" = "1" ]; then
  env | grep -i claude >> "$LOG_FILE"
fi
```

---

### 问题 2: Hook 被触发但事件格式不对

**症状**：日志显示 `ERROR: Failed to parse hook_event_name`

**解决方案**：

检查实际的 JSON 格式：

```bash
# 查看原始输入
grep "Raw input:" ~/.claude/visualizer-hook.log | tail -5
```

根据实际格式调整 hook 脚本中的 jq 解析。

---

### 问题 3: Claude Code 版本过旧

**检查版本**：

```bash
claude-code --version
# 需要 >= 0.3.0 才支持 hooks
```

**更新 Claude Code**：

```bash
# Homebrew
brew upgrade claude-code

# npm
npm install -g @anthropic/claude-code

# 或从官网下载最新版本
```

---

### 问题 4: Hooks 功能未启用

某些安装可能默认禁用 hooks。

**检查并启用**：

```bash
# 创建或编辑 ~/.claude/config.json
cat > ~/.claude/config.json << 'EOF'
{
  "hooks": {
    "enabled": true,
    "timeout": 10000
  }
}
EOF

# 重启 Claude Code
```

---

### 问题 5: 权限问题

**检查权限**：

```bash
ls -la ~/.claude/hooks/visualizer-hook.sh
# 应该显示 -rwxr-xr-x

ls -la ~/.claude/hooks.json
# 应该显示 -rw-r--r--
```

**修复权限**：

```bash
chmod +x ~/.claude/hooks/visualizer-hook.sh
chmod 644 ~/.claude/hooks.json
```

---

## 🔄 替代方案：手动模拟 Claude Code

如果 hooks 始终无法工作，你可以：

### 方案 A：使用测试脚本

```bash
# 模拟 Claude Code 活动
./scripts/test-visualizer.sh
```

### 方案 B：创建包装脚本

创建 `~/bin/claude-code-with-visualizer.sh`：

```bash
#!/bin/bash
# Wrapper that logs tool calls and sends to visualizer

# Start visualizer event sender in background
{
  while true; do
    # Monitor Claude Code logs and extract tool calls
    tail -f ~/.claude/logs/latest.log | \
      grep -i "tool.*use" | \
      while read line; do
        # Parse and send to visualizer
        # (需要根据实际日志格式调整)
        echo "Tool call detected: $line"
      done
    sleep 1
  done
} &

# Run actual Claude Code
claude-code "$@"
```

### 方案 C：使用 API 直接发送

在你的工作流中，手动发送事件到 visualizer：

```bash
# 读取文件前
curl -X POST http://localhost:3002/api/visualizer/events \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "event": "tool-use",
    "tool": "Read",
    "path": "/path/to/file.ts",
    "status": "started",
    "details": {}
  }'
```

---

## 📋 完整调试检查清单

- [ ] ✅ Hook 脚本存在且可执行
- [ ] ✅ jq 已安装
- [ ] ✅ hooks.json 配置正确
- [ ] ✅ 开发服务器运行在 port 3002
- [ ] ✅ 手动测试 hook 脚本成功
- [ ] ⚠️ Claude Code 使用 CLI 版本（不是 IDE 插件）
- [ ] ⚠️ Claude Code 版本 >= 0.3.0
- [ ] ⚠️ Hooks 功能已启用
- [ ] ⚠️ 调试日志显示 hook 被触发
- [ ] ⚠️ 日志显示事件被正确解析
- [ ] ⚠️ 事件成功到达 API

---

## 🆘 获取帮助

如果以上方法都无效：

1. **收集诊断信息**：

```bash
# 运行完整诊断
./scripts/diagnose-visualizer-hooks.sh > diagnosis.txt

# 收集 Claude Code 信息
claude-code --version >> diagnosis.txt
cat ~/.claude/config.json >> diagnosis.txt
tail -50 ~/.claude/visualizer-hook.log >> diagnosis.txt
```

2. **检查 Claude Code 官方文档**：
   - Hooks API 参考
   - 版本兼容性
   - 已知问题

3. **查看项目 Issue**：
   - GitHub: https://github.com/anthropics/claude-code/issues
   - 搜索关键词: "hooks not working"

---

## ✅ 成功标志

当一切正常时，你应该看到：

```bash
# 1. Hook 日志显示活动
$ tail -5 ~/.claude/visualizer-hook.log
[2026-02-10T15:30:00Z] === Hook triggered ===
[2026-02-10T15:30:00Z] Event: tool-use
[2026-02-10T15:30:00Z] Tool: Read
[2026-02-10T15:30:00Z] Extracted path: /Users/.../file.ts
[2026-02-10T15:30:00Z] Event sent to http://localhost:3002/...

# 2. API 显示事件
$ curl -s 'http://localhost:3002/api/visualizer/events?limit=1' | jq .
{
  "events": [{
    "tool": "Read",
    "path": "/Users/.../file.ts",
    "status": "started"
  }],
  "count": 5
}

# 3. 可视化界面实时更新
# 螃蟹移动 🦀
# 文件夹变色 📁
# 事件日志更新 📊
```

---

**最后提示**：如果 Claude Code hooks 确实不工作，使用测试脚本 `./scripts/test-visualizer.sh` 仍然可以完整演示可视化功能！🎉
