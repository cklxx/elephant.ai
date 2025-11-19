#!/usr/bin/env bash
set -euo pipefail

FFMPEG_BIN=${FFMPEG_BIN:-ffmpeg}
FFPROBE_BIN=${FFPROBE_BIN:-ffprobe}
VIDEO_DURATION=${VIDEO_DURATION:-1}
VIDEO_RESOLUTION=${VIDEO_RESOLUTION:-128x128}
COLOR=${COLOR:-red}

log() {
  printf '%b\n' "$*"
}

require_binary() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: $1 is not available on PATH" >&2
    exit 1
  fi
}

require_binary "$FFMPEG_BIN"
require_binary "$FFPROBE_BIN"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

video_path="$workdir/ffmpeg_verify.mp4"

log "[1/3] Generating ${VIDEO_DURATION}s ${VIDEO_RESOLUTION} clip via $FFMPEG_BIN"
"$FFMPEG_BIN" -hide_banner -loglevel error -y \
  -f lavfi -i "color=c=${COLOR}:s=${VIDEO_RESOLUTION}:d=${VIDEO_DURATION}" \
  -c:v libx264 -preset veryfast -pix_fmt yuv420p "$video_path"

log "[2/3] Inspecting generated clip via $FFPROBE_BIN"
probe_output=$("$FFPROBE_BIN" -hide_banner -loglevel error \
  -select_streams v:0 \
  -show_entries stream=codec_name,width,height,r_frame_rate \
  -of default=noprint_wrappers=1 "$video_path")

expect_contains() {
  local needle=$1
  if ! grep -q "$needle" <<<"$probe_output"; then
    echo "error: expected '$needle' in ffprobe output" >&2
    echo "$probe_output" >&2
    exit 1
  fi
}

width=${VIDEO_RESOLUTION%x*}
height=${VIDEO_RESOLUTION#*x}
expect_contains "codec_name=h264"
expect_contains "width=${width}"
expect_contains "height=${height}"

log "[3/3] All FFmpeg/FFprobe checks passed"
log "Generated clip stored at: $video_path"
log "Probe summary:\n$probe_output"
