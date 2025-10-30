#!/bin/bash
###############################################################################
# Sandbox Environment Verification Script
#
# Verifies that all pre-installed packages are available in the sandbox
###############################################################################

set -euo pipefail

C_GREEN='\033[0;32m'
C_RED='\033[0;31m'
C_CYAN='\033[0;36m'
C_RESET='\033[0m'

echo -e "${C_CYAN}=== Verifying Sandbox Environment ===${C_RESET}\n"

# Check if sandbox is running
if ! docker ps --filter "name=alex-sandbox" --format '{{.Names}}' | grep -q alex-sandbox; then
    echo -e "${C_RED}✗ Sandbox container is not running${C_RESET}"
    echo "Start it with: ./deploy.sh start"
    exit 1
fi

echo -e "${C_CYAN}Node.js Environment:${C_RESET}"
docker exec alex-sandbox bash -c "
    echo '  Node.js: '$(node --version)
    echo '  npm: '$(npm --version)
    echo '  TypeScript: '$(tsc --version)
    echo '  ts-node: '$(ts-node --version 2>&1 | head -1)
    echo '  Prettier: '$(prettier --version)
    echo '  ESLint: '$(eslint --version)
    echo '  pnpm: '$(pnpm --version)
    echo '  yarn: '$(yarn --version)
"

echo ""
echo -e "${C_CYAN}Python Environment:${C_RESET}"
docker exec alex-sandbox bash -c "
    echo '  Python: '$(python3 --version)
    echo '  pip: '$(pip3 --version | cut -d' ' -f1-2)
    python3 -c '
import sys
packages = {
    \"numpy\": \"NumPy\",
    \"pandas\": \"Pandas\",
    \"requests\": \"Requests\",
    \"fastapi\": \"FastAPI\",
    \"flask\": \"Flask\",
    \"pytest\": \"pytest\",
    \"black\": \"Black\",
    \"mypy\": \"mypy\",
    \"jupyter\": \"Jupyter\"
}
for module, name in packages.items():
    try:
        mod = __import__(module)
        version = getattr(mod, \"__version__\", \"OK\")
        print(f\"  {name}: {version}\")
    except ImportError:
        print(f\"  {name}: NOT FOUND\", file=sys.stderr)
        sys.exit(1)
'
"

echo ""
echo -e "${C_GREEN}✓ All packages verified successfully!${C_RESET}"
