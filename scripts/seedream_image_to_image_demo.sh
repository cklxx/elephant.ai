#!/usr/bin/env bash

###############################################################################
# Seedream Image-to-Image Demo
#
# Usage:
#   ARK_API_KEY=xxxx ./scripts/seedream_image_to_image_demo.sh \
#       --model doubao-seedream-4-0-250828 \
#       --prompt "Quick connectivity test"
#
# This script fabricates a tiny 32x32 PNG, base64-encodes it, and sends a request
# directly to the Volcano Engine Seedream image generation endpoint using curl.
# It mirrors the payload structure used by the Alex seedream_image_to_image tool.
###############################################################################

set -euo pipefail

MODEL="doubao-seedream-4-0-250828"
PROMPT="Connectivity test via curl demo"
WATERMARK="false"
RESPONSE_FORMAT="b64_json"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --model)
      MODEL="$2"
      shift 2
      ;;
    --prompt)
      PROMPT="$2"
      shift 2
      ;;
    --watermark)
      WATERMARK="$2"
      shift 2
      ;;
    --response-format)
      RESPONSE_FORMAT="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "${ARK_API_KEY:-}" ]]; then
  echo "ERROR: ARK_API_KEY environment variable must be set." >&2
  exit 1
fi

TMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t seedream-demo)
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

FAKE_PNG="$TMP_DIR/fake.png"
PAYLOAD="$TMP_DIR/payload.json"
RESPONSE="$TMP_DIR/response.json"

# Write a 1x1 transparent PNG.
python3 - "$FAKE_PNG" <<'PY'
import base64, sys
png_bytes = base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAIAAAD8GO2jAAAAAXNSR0IArs4c6QAAAERlWElmTU0AKgAAAAgAAYdpAAQAAAABAAAAGgAAAAAAA6ABAAMAAAABAAEAAKACAAQAAAABAAAAIKADAAQAAAABAAAAIAAAAACshmLzAAABv0lEQVRIDWP8//8/Ay0BEy0NB5lNcwtYCPrg7du3v3//ZmFhgZB///4FspnAQEBAgKB2whb8+fPn5cuXYmJijx49EhERAXK/fv0KJAUFBYmxgHE0kglFAs1T0agFhKKAAWc+ACbfD1cWMjJz/Pn5gVvCiFPSDG7W/3//vn/5/PTaOQ4eflEFFQ4ePrgUJgOnBYyMjP///OA9mfiNTYBB6Tqyzn///l7es/7inBYWHn7roh51C0dkWTQ2vkhmE9P/ySn5X8SSlUsQWRszC6uSiR2nuCy/vLq8HsJnyGoQbGBQYIIfP368fv0aKP7985vfv35gKgCKfPn04cf3b0AGpCzBqgYoiD2I/v379+vXL6ArOHiEEW5BZXHz8kMEgKYAHYQqieDRvCzC4oNn928/u3WRkYkZ4QyCrP//uITENE1sMRViseDa4S1HJhezcXFhqsYl8vfXTzFDF02T7ZgKsFjAzMzCxsHBxs6BqRqXyF8mRlY2Nqyy+JIpVg2kCtLcAixB9A9Y7f7++ZeFhEgGqv/35zdWz2GxgE9CTtzIlYWNHasGrIL//v4WUTHEKkXzfEDzOBi1AGvEIgsO/SACABXM3d0H75J4AAAAAElFTkSuQmCC")
with open(sys.argv[1], "wb") as fh:
    fh.write(png_bytes)
PY

B64_IMAGE=$(python3 - "$FAKE_PNG" <<'PY'
import base64, sys
with open(sys.argv[1], "rb") as fh:
    print(base64.b64encode(fh.read()).decode("ascii"), end="")
PY
)

cat >"$PAYLOAD" <<JSON
{
  "model": "$MODEL",
  "prompt": "$PROMPT",
  "response_format": "$RESPONSE_FORMAT",
  "watermark": $WATERMARK,
  "image": "data:image/png;base64,$B64_IMAGE"
}
JSON

echo "Sending request to Seedream image endpoint..."
set +e
HTTP_STATUS=$(curl -sS -o "$RESPONSE" -w "%{http_code}" \
  https://ark.cn-beijing.volces.com/api/v3/images/generations \
  -H "Authorization: Bearer $ARK_API_KEY" \
  -H "Content-Type: application/json" \
  --data-binary @"$PAYLOAD")
CURL_EXIT=$?
set -e

if [[ $CURL_EXIT -ne 0 ]]; then
  echo "curl failed with exit code $CURL_EXIT" >&2
  cat "$RESPONSE" >&2 || true
  exit $CURL_EXIT
fi

echo "HTTP status: $HTTP_STATUS"
echo "Response body:"
cat "$RESPONSE"
echo
