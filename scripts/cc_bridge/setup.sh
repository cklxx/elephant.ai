#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

if [ ! -d .venv ]; then
    echo "Creating virtual environment..."
    python3 -m venv .venv
fi

echo "Installing dependencies..."
.venv/bin/pip install -q -r requirements.txt

echo "Done. Bridge script ready at: $(pwd)/cc_bridge.py"
echo "Python: $(pwd)/.venv/bin/python3"
