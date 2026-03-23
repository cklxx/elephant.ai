---
name: desktop-automation
description: When you need to control macOS desktop apps (Atlas, Chrome, Finder) → automate via AppleScript.
triggers:
  intent_patterns:
    - "applescript|桌面自动化|desktop|macos|打开应用|切换窗口"
    - "打开.*app|open.*application|启动.*程序|launch"
    - "切换到|switch to|activate.*window|最小化|minimize|最大化|maximize"
    - "Finder|Spotify|Music|Terminal|Notes|备忘录|系统偏好"
    - "屏幕.*亮度|brightness|音量|volume|静音|mute|勿扰|do not disturb"
    - "复制.*粘贴|clipboard|剪贴板|自动.*键入|type.*automatically"
    - "窗口.*排列|tile.*window|分屏|split.*screen|全屏|fullscreen"
  context_signals:
    keywords: ["applescript", "desktop", "macos", "自动化", "窗口", "app", "应用", "切换", "打开", "启动", "Finder", "音量", "亮度"]
  confidence_threshold: 0.65
priority: 6
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# desktop-automation

macOS 桌面自动化：运行 AppleScript 控制应用程序。

## 调用

```bash
python3 skills/desktop-automation/run.py run --script 'tell application "Finder" to activate'
python3 skills/desktop-automation/run.py open_app --app Safari
```
