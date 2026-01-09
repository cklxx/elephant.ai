#!/bin/bash

# Alex Configuration Update Script
# Updates alex configuration to use Google Gemini API

set -e

# Help function
show_help() {
    echo "Alex Configuration Update Script"
    echo ""
    echo "Usage: $0 [OPTIONS] [API_KEY]"
    echo ""
    echo "OPTIONS:"
    echo "  -h, --help     Show this help message"
    echo "  -m, --model    Specify model (default: gemini-2.5-flash)"
    echo "  -u, --url      Specify base URL (default: Google Gemini API)"
    echo ""
    echo "ENVIRONMENT VARIABLES:"
    echo "  GOOGLE_API_KEY     Google API key"
    echo "  GEMINI_MODEL       Gemini model to use"
    echo "  GEMINI_BASE_URL    Base URL for Gemini API"
    echo ""
    echo "EXAMPLES:"
    echo "  $0 xxx"
    echo "  GOOGLE_API_KEY=your_key $0"
    echo "  $0 -m gemini-2.5-pro your_api_key"
    echo ""
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            ;;
        -m|--model)
            GEMINI_MODEL="$2"
            shift 2
            ;;
        -u|--url)
            GEMINI_BASE_URL="$2"
            shift 2
            ;;
        -*)
            echo "Unknown option: $1"
            show_help
            ;;
        *)
            # If no API key set and this looks like an API key, use it
            if [ -z "${GOOGLE_API_KEY:-}" ] && [[ "$1" =~ ^AIza ]]; then
                GOOGLE_API_KEY="$1"
            fi
            shift
            ;;
    esac
done

# Google Gemini API Configuration
API_KEY="${GOOGLE_API_KEY:-xxx}"
MODEL="${GEMINI_MODEL:-gemini-2.5-pro}"
BASE_URL="${GEMINI_BASE_URL:-https://generativelanguage.googleapis.com/v1beta/openai}"

echo "🔧 Updating alex configuration for Google Gemini API..."

# Check if API key is provided
if [ -z "$API_KEY" ] || [ "$API_KEY" = "xxx" ]; then
    echo "⚠️  Using default/no API key. For production use, please provide your own API key."
    echo "Set GOOGLE_API_KEY environment variable or use --help for usage information."
    echo ""
    read -p "Continue with default key? (y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted. Use --help for more information."
        exit 1
    fi
fi

# Validate API key format (basic check for Google API key)
if [[ ! "$API_KEY" =~ ^AIza[0-9A-Za-z_-]{35}$ ]]; then
    echo "⚠️  Warning: API key format doesn't match expected Google API key pattern"
    echo "Expected format: AIza[35 characters]"
    echo "Current key: ${API_KEY:0:10}..."
    read -p "Continue anyway? (y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Check if alex binary exists
if [ ! -f "./alex" ]; then
    echo "❌ Alex binary not found. Please run 'make build' first."
    exit 1
fi

# Configuration file path
CONFIG_FILE="$HOME/.alex/config.yaml"
mkdir -p "$(dirname "$CONFIG_FILE")"

# Backup existing configuration
echo "📦 Backing up existing configuration..."
if [ -f "$CONFIG_FILE" ]; then
    cp "$CONFIG_FILE" "$CONFIG_FILE.backup-$(date +%Y%m%d-%H%M%S)"
fi

# Update configuration using yq (YAML processor)
echo "🔧 Updating configuration file..."
if command -v yq >/dev/null 2>&1; then
    export API_KEY BASE_URL MODEL
    if [ -f "$CONFIG_FILE" ]; then
        yq -i \
          '.runtime.api_key = strenv(API_KEY) |
           .runtime.base_url = strenv(BASE_URL) |
           .runtime.llm_provider = "openai" |
           .runtime.llm_model = strenv(MODEL)' \
          "$CONFIG_FILE"
    else
        cat > "$CONFIG_FILE" << EOF
runtime:
  llm_provider: "openai"
  llm_model: "$MODEL"
  api_key: "$API_KEY"
  base_url: "$BASE_URL"
  max_tokens: 12000
  temperature: 0.7
  max_iterations: 25
EOF
    fi
elif command -v python3 >/dev/null 2>&1 && python3 - << 'PY' >/dev/null 2>&1
import yaml  # noqa: F401
PY
then
    python3 - "$CONFIG_FILE" "$API_KEY" "$BASE_URL" "$MODEL" << 'PY'
import os
import sys
import yaml

path, api_key, base_url, model = sys.argv[1:5]
data = {}
if os.path.exists(path):
    with open(path, "r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh) or {}
runtime = data.get("runtime") or {}
runtime["api_key"] = api_key
runtime["base_url"] = base_url
runtime["llm_provider"] = "openai"
runtime["llm_model"] = model
data["runtime"] = runtime
os.makedirs(os.path.dirname(path), exist_ok=True)
with open(path, "w", encoding="utf-8") as fh:
    yaml.safe_dump(data, fh, sort_keys=False)
PY
else
    if [ -f "$CONFIG_FILE" ]; then
        echo "⚠️  yq/python3+pyyaml not found; refusing to overwrite $CONFIG_FILE."
        echo "Install yq or python3 with pyyaml and retry."
        exit 1
    fi
    echo "⚠️  yq/python3+pyyaml not found. Creating simple configuration..."
    # Create a simple configuration file
    cat > "$CONFIG_FILE" << EOF
runtime:
  llm_provider: "openai"
  llm_model: "$MODEL"
  api_key: "$API_KEY"
  base_url: "$BASE_URL"
  max_tokens: 12000
  temperature: 0.7
  max_iterations: 25
EOF
fi

echo "✅ Configuration updated successfully!"
echo ""
echo "📋 Current configuration:"
if [ -f "$CONFIG_FILE" ]; then
    if command -v yq >/dev/null 2>&1; then
        echo "  🔑 API Key: $(yq -r '.runtime.api_key | .[0:10] + "..."' "$CONFIG_FILE")"
        echo "  🌐 Base URL: $(yq -r '.runtime.base_url' "$CONFIG_FILE")"
        echo "  🤖 Provider: $(yq -r '.runtime.llm_provider' "$CONFIG_FILE")"
        echo "  🤖 Model: $(yq -r '.runtime.llm_model' "$CONFIG_FILE")"
        echo "  🎯 Max Tokens: $(yq -r '.runtime.max_tokens' "$CONFIG_FILE")"
        echo "  🌡️  Temperature: $(yq -r '.runtime.temperature' "$CONFIG_FILE")"
    else
        echo "  Configuration file: $CONFIG_FILE"
        echo "  Install 'yq' to see detailed configuration display"
    fi
else
    echo "  ❌ Configuration file not found"
fi
echo ""
echo "🚀 Alex is now configured to use Google Gemini API!"
echo "📝 Model: $MODEL"
echo "🌐 Base URL: $BASE_URL"
echo "💡 You can now start using: ./alex -i"
echo ""
echo "🔧 To verify: ./alex config"
