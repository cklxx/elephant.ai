# ALEX Quick Start Guide

## 🚀 Fastest Way to Get Started

### Option 1: Direct Query (Recommended) ⭐
```bash
./alex "list files in the current directory"
./alex "explain the project structure"
./alex "find all TODO comments in the codebase"
```

### Option 2: Interactive Mode (Most Compatible)
```bash
./alex --no-tui
```

Then type your requests:
```
> what is the architecture of this project?
> list all Go files
> /quit
```

### Option 3: Web Interface (Best UI)
```bash
./deploy.sh start
# Opens browser at http://localhost:3000
```

---

## ⚠️ If `./alex` Seems to Hang

It's **NOT hung** - it's waiting for your input!

```bash
# WRONG (appears to hang):
./alex
# ↑ Now waiting for you to type something

# RIGHT (shows prompt):
./alex --no-tui
# ↑ Shows ">" and waits for input

# RIGHT (executes directly):
./alex "your task here"
# ↑ Executes immediately
```

---

## 🔍 Diagnose Issues

```bash
# Run the diagnosis script
scripts/diagnose-startup.sh
```

Output will show:
- ✅ Binary is working
- ✅ Config is loaded
- ✅ API connection works
- ✅ Native UI functions properly

---

## 💡 Pro Tips

### Skip TUI (Most Reliable)
```bash
./alex --no-tui
```
- Works on all terminals
- Uses Lipgloss for nice formatting
- Better compatibility

### Direct Execution (No Interaction)
```bash
./alex "Your task" 2>&1 | tee output.txt
```
- Executes immediately
- Captures output to file
- No interactive prompts

### Pipe Input
```bash
echo "list files" | ./alex --no-tui
```
- Batch mode
- Works in scripts/CI/CD

### Enable Verbose Output
```bash
ALEX_VERBOSE=1 ./alex --no-tui
```
- Shows detailed tool execution
- Helpful for debugging

---

## 🎯 Common Tasks

### List Files in a Directory
```bash
./alex "list all .go files in cmd/alex"
```

### Analyze Code
```bash
./alex "explain how the ReAct engine works in this project"
```

### Find Something
```bash
./alex "search for all error handling in the codebase"
```

### Get Project Help
```bash
./alex "what is the structure and purpose of this project?"
```

---

## 🔧 Troubleshooting

### Problem: No output/appears frozen
**Solution**: Press Enter or use `--no-tui`
```bash
./alex --no-tui
# Now type your request and press Enter
```

### Problem: Colors look wrong
**Solution**: Force terminal type
```bash
TERM=xterm-256color ./alex --no-tui
```

### Problem: Using over SSH
**Solution**: Make sure TERM is set correctly
```bash
# On SSH server:
export TERM=xterm-256color
./alex --no-tui
```

### Problem: In Docker/CI
**Solution**: Allocate TTY explicitly
```bash
docker run -it your-image ./alex --no-tui
```

---

## 📊 What's Actually Happening

When you run `./alex`:

```
./alex
  ↓
[1] Try to start tcell-based TUI
    ├─ Success? → Enter interactive mode (waits for input)
    └─ Fail? → Fall back to native TUI (also waits for input)
  ↓
[2] Waits for your command
```

That's why it seems to "hang" - it's actually **waiting for you to type something**!

### The Fix
Use `--no-tui` to skip the TUI attempt and go straight to native mode:

```
./alex --no-tui
  ↓
[1] Skip TUI attempt entirely
  ↓
[2] Go directly to native TUI with Lipgloss styling
  ↓
[3] Show prompt and wait for input
```

---

## ✅ Verification

Run this to confirm everything works:

```bash
# Should show version
./alex --no-tui version

# Should run a task
./alex "list files"

# Should show help
./alex help
```

All three commands should complete without hanging.

---

## 🎓 Understanding the UI Modes

| Mode | Command | Compatibility | Speed | Best For |
|------|---------|---|---|---|
| **Native (Recommended)** | `./alex --no-tui` | ⭐⭐⭐⭐⭐ | Fast | All terminals |
| **TUI (tcell-based)** | `./alex` | ⭐⭐⭐ | Medium | Modern terminals |
| **Web** | `./deploy.sh start` | ⭐⭐⭐⭐⭐ | Network | Complex tasks |
| **Direct Query** | `./alex "task"` | ⭐⭐⭐⭐⭐ | Fast | Scripts/automation |

---

## 📞 Still Need Help?

1. Run diagnosis: `scripts/diagnose-startup.sh`
2. Check logs: `tail -f logs/web.log.old`
3. Try web interface: `./deploy.sh start`
4. Read full docs: `TERMINAL_UI_FIX_SUMMARY.md`

**Most common issue**: Just type something and press Enter! 😄
