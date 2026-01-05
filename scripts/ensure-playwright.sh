#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="${ROOT_DIR}/web"
CACHE_DIR="${WEB_DIR}/node_modules/.cache/ms-playwright"
LOG_DIR="${PLAYWRIGHT_LOG_DIR:-${ROOT_DIR}/logs}"
INSTALL_LOG="${LOG_DIR}/playwright-install.log"
DOWNLOAD_HOST="${PLAYWRIGHT_DOWNLOAD_HOST:-https://playwright.azureedge.net}"

PYTHON_BIN=""

if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
elif command -v python >/dev/null 2>&1; then
  if python - <<'PY'
import sys

sys.exit(0 if sys.version_info >= (3, 0) else 1)
PY
  then
    PYTHON_BIN="python"
  fi
fi

if [[ -z "${PYTHON_BIN}" ]]; then
  echo "Python 3 is required to patch Playwright progress logging; install python3." >&2
  exit 1
fi

mkdir -p "${LOG_DIR}"

patch_playwright_progress() {
  local fetcher="${WEB_DIR}/node_modules/playwright-core/lib/server/registry/browserFetcher.js"
  local downloader="${WEB_DIR}/node_modules/playwright-core/lib/server/registry/oopDownloadBrowserMain.js"

  if [[ ! -f "${fetcher}" ]]; then
    return 0
  fi

  if grep -q "totalSize ? percentage" "${fetcher}"; then
    return 0
  fi

  "${PYTHON_BIN}" - "${fetcher}" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()
guard = "const totalSize = totalBytes && isFinite(totalBytes) ? totalBytes : 0;\n    const percentage = totalSize ? downloadedBytes / totalSize : 0;"
if guard in text:
    sys.exit(0)

needle = "const percentage = downloadedBytes / totalBytes;"
if needle not in text:
    sys.exit(0)

text = text.replace(
    needle,
    guard,
)
text = text.replace(
    'const percentageString = String(percentage * 100 | 0).padStart(3);\n      console.log(`|${"\\u25A0".repeat(row * stepWidth)}${" ".repeat((totalRows - row) * stepWidth)}| ${percentageString}% of ${toMegabytes(totalBytes)}`);',
    'const percentageString = String(totalSize ? percentage * 100 | 0 : 0).padStart(3);\n      const totalLabel = totalSize ? toMegabytes(totalSize) : "unknown";\n      console.log(`|${"\\u25A0".repeat(row * stepWidth)}${" ".repeat((totalRows - row) * stepWidth)}| ${percentageString}% of ${totalLabel}`);',
)
path.write_text(text)
PY

  if [[ -f "${downloader}" ]] && ! grep -q "totalBytes > 0 && downloadedBytes !== totalBytes" "${downloader}"; then
    "${PYTHON_BIN}" - "${downloader}" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text()
needle = 'if (downloadedBytes !== totalBytes) {'
replacement = 'if (totalBytes > 0 && downloadedBytes !== totalBytes) {'
if needle in text:
    text = text.replace(needle, replacement)
    path.write_text(text)
PY
  fi
}

if [[ -d "${CACHE_DIR}" ]]; then
  echo "Playwright browsers already installed at ${CACHE_DIR}."
  exit 0
fi

if [[ ! -d "${WEB_DIR}" ]]; then
  echo "web directory not found at ${WEB_DIR}" >&2
  exit 1
fi

if [[ ! -d "${WEB_DIR}/node_modules" ]]; then
  echo "web/node_modules not found; run 'npm install' in web/ first" >&2
  exit 1
fi

echo "Installing Playwright browsers (first run may take a while)..."
patch_playwright_progress
if (cd "${WEB_DIR}" && PLAYWRIGHT_DOWNLOAD_HOST="${DOWNLOAD_HOST}" npx playwright install --with-deps 2>&1 | tee "${INSTALL_LOG}"); then
  echo "Playwright browsers installed"
else
  echo "Playwright browser install failed; see ${INSTALL_LOG}" >&2
  exit 1
fi
