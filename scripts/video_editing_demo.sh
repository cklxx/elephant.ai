#!/usr/bin/env bash
set -euo pipefail

FFMPEG_BIN=${FFMPEG_BIN:-ffmpeg}
FFPROBE_BIN=${FFPROBE_BIN:-ffprobe}
RESOLUTION=${VIDEO_RESOLUTION:-1280x720}
SEGMENT_DURATION=${SEGMENT_DURATION:-2}
OUTPUT_PATH=${OUTPUT_PATH:-video_editing_demo.mp4}
FONT_FILE=${FONT_FILE:-/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf}
SIMULATE_MISSING_INPUT=${SIMULATE_MISSING_INPUT:-0}
SOURCE_MANIFEST=${SOURCE_MANIFEST:-}
PRIMARY_AUDIO_PATH=${PRIMARY_AUDIO_PATH:-}
SECONDARY_AUDIO_PATH=${SECONDARY_AUDIO_PATH:-}
AUDIO_VOLUME=${AUDIO_VOLUME:-0.2}
WATERMARK_TEXT=${WATERMARK_TEXT:-DEMO_WATERMARK}
WATERMARK_FONT_SIZE=${WATERMARK_FONT_SIZE:-48}
WATERMARK_OPACITY=${WATERMARK_OPACITY:-0.7}
WATERMARK_MARGIN=${WATERMARK_MARGIN:-40}
WATERMARK_POSITION=${WATERMARK_POSITION:-bottom-right}
WATERMARK_IMAGE_PATH=${WATERMARK_IMAGE_PATH:-}
WATERMARK_IMAGE_SCALE=${WATERMARK_IMAGE_SCALE:-1}
WATERMARK_IMAGE_OPACITY=${WATERMARK_IMAGE_OPACITY:-1}
SUBTITLE_FILE=${SUBTITLE_FILE:-}
SUBTITLE_CHARSET=${SUBTITLE_CHARSET:-UTF-8}
SUBTITLE_FORCE_STYLE=${SUBTITLE_FORCE_STYLE:-}
ENABLE_GPU=${ENABLE_GPU:-0}
PREFERRED_GPU_BACKEND=${PREFERRED_GPU_BACKEND:-auto}
VIDEO_CODEC=${VIDEO_CODEC:-}
VIDEO_PRESET=${VIDEO_PRESET:-}
PIXEL_FORMAT=${PIXEL_FORMAT:-yuv420p}
GPU_STATUS_FILE=${GPU_STATUS_FILE:-}
RUN_STATUS_FILE=${RUN_STATUS_FILE:-}
METRICS_LOG_PATH=${METRICS_LOG_PATH:-}

SCRIPT_START_TS=$(date +%s)
SCRIPT_START_ISO=$(date -Is)

workdir=$(mktemp -d -t video-edit-XXXXXX)
log() {
  echo "[video_editing_demo] $*"
}

cleanup() {
  rm -rf "$workdir"
}

escape_subtitle_value() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\'/\\\'}"
  echo "$value"
}

record_run_summary() {
  local exit_code="$1"
  local end_ts=$(date +%s)
  local end_iso=$(date -Is)
  local duration=$(awk -v start="$SCRIPT_START_TS" -v end="$end_ts" 'BEGIN { printf "%.3f", (end - start) }')
  local output_exists="no"
  local output_size="0"
  if [[ -f "$OUTPUT_PATH" ]]; then
    output_exists="yes"
    output_size=$(stat -c%s "$OUTPUT_PATH" 2>/dev/null || echo 0)
  fi
  local status="failed"
  if [[ "$exit_code" -eq 0 ]]; then
    status="succeeded"
  fi

  local target="$RUN_STATUS_FILE"
  if [[ -n "$target" ]]; then
    mkdir -p "$(dirname "$target")"
    cat >"$target" <<EOF
status=${status}
exit_code=${exit_code}
start_time=${SCRIPT_START_ISO}
end_time=${end_iso}
duration_seconds=${duration}
output_path=${OUTPUT_PATH}
output_exists=${output_exists}
output_size_bytes=${output_size}
video_codec=${VIDEO_CODEC}
video_preset=${VIDEO_PRESET}
pixel_format=${PIXEL_FORMAT}
gpu_backend=${GPU_BACKEND}
gpu_message=${GPU_STATUS_MESSAGE}
metrics_gap=orchestrator_export_missing
EOF

    log "Run summary written to $target (metrics gap still pending in orchestrator)"
  fi

  append_pseudo_metrics "$status" "$duration" "$output_size"
}

append_pseudo_metrics() {
  local status="$1"
  local duration="$2"
  local output_size="$3"
  local target="$METRICS_LOG_PATH"
  [[ -z "$target" ]] && return

  mkdir -p "$(dirname "$target")"
  local ts=$(date +%s)
  {
    echo "# pseudo metrics emitted $(date -Is); replace with Prometheus exporter once ready"
    echo "ffmpeg_demo_run_total{status=\"${status}\",gpu_backend=\"${GPU_BACKEND}\"} 1 ${ts}"
    echo "ffmpeg_demo_run_duration_seconds{status=\"${status}\",gpu_backend=\"${GPU_BACKEND}\"} ${duration}"
    echo "ffmpeg_demo_output_size_bytes{status=\"${status}\",gpu_backend=\"${GPU_BACKEND}\"} ${output_size}"
  } >>"$target"

  log "Pseudo metrics appended to $target (Prometheus exporter wiring still TODO)"
}

on_exit() {
  local exit_code=$?
  record_run_summary "$exit_code"
  cleanup
}

trap on_exit EXIT

GPU_BACKEND="cpu"
GPU_STATUS_MESSAGE="GPU disabled (set ENABLE_GPU=1 to attempt detection)"
FINAL_HWACCEL_ARGS=()
WATERMARK_TEXT_POS_X="w-text_w-40"
WATERMARK_TEXT_POS_Y="h-text_h-40"
WATERMARK_OVERLAY_POS_X="main_w-overlay_w-40"
WATERMARK_OVERLAY_POS_Y="main_h-overlay_h-40"
RESOLVED_WATERMARK_POSITION="bottom-right"
SUBTITLE_ENABLED=0

determine_watermark_position() {
  local raw_pos="${WATERMARK_POSITION:-bottom-right}"
  local normalized=$(echo "$raw_pos" | tr '[:upper:]' '[:lower:]')
  local margin="${WATERMARK_MARGIN:-40}"
  case "$normalized" in
    bottom-right)
      WATERMARK_TEXT_POS_X="w-text_w-${margin}"
      WATERMARK_TEXT_POS_Y="h-text_h-${margin}"
      WATERMARK_OVERLAY_POS_X="main_w-overlay_w-${margin}"
      WATERMARK_OVERLAY_POS_Y="main_h-overlay_h-${margin}"
      ;;
    bottom-left)
      WATERMARK_TEXT_POS_X="${margin}"
      WATERMARK_TEXT_POS_Y="h-text_h-${margin}"
      WATERMARK_OVERLAY_POS_X="${margin}"
      WATERMARK_OVERLAY_POS_Y="main_h-overlay_h-${margin}"
      ;;
    top-right)
      WATERMARK_TEXT_POS_X="w-text_w-${margin}"
      WATERMARK_TEXT_POS_Y="${margin}"
      WATERMARK_OVERLAY_POS_X="main_w-overlay_w-${margin}"
      WATERMARK_OVERLAY_POS_Y="${margin}"
      ;;
    top-left)
      WATERMARK_TEXT_POS_X="${margin}"
      WATERMARK_TEXT_POS_Y="${margin}"
      WATERMARK_OVERLAY_POS_X="${margin}"
      WATERMARK_OVERLAY_POS_Y="${margin}"
      ;;
    center)
      WATERMARK_TEXT_POS_X="(w-text_w)/2"
      WATERMARK_TEXT_POS_Y="(h-text_h)/2"
      WATERMARK_OVERLAY_POS_X="(main_w-overlay_w)/2"
      WATERMARK_OVERLAY_POS_Y="(main_h-overlay_h)/2"
      ;;
    *)
      log "Unknown WATERMARK_POSITION=${raw_pos}; defaulting to bottom-right"
      WATERMARK_TEXT_POS_X="w-text_w-${margin}"
      WATERMARK_TEXT_POS_Y="h-text_h-${margin}"
      WATERMARK_OVERLAY_POS_X="main_w-overlay_w-${margin}"
      WATERMARK_OVERLAY_POS_Y="main_h-overlay_h-${margin}"
      normalized="bottom-right"
      ;;
  esac
  RESOLVED_WATERMARK_POSITION="$normalized"
}

configure_video_encoder() {
  local backend="${PREFERRED_GPU_BACKEND:-auto}"
  local enable="${ENABLE_GPU:-0}"
  VIDEO_CODEC=${VIDEO_CODEC:-libx264}
  VIDEO_PRESET=${VIDEO_PRESET:-veryfast}

  if [[ "$enable" != "1" ]]; then
    GPU_STATUS_MESSAGE="GPU disabled (ENABLE_GPU!=1); using CPU encoder ${VIDEO_CODEC}"
    return
  fi

  if { [[ "$backend" == "auto" ]] || [[ "$backend" == "cuda" ]]; } && command -v nvidia-smi >/dev/null 2>&1; then
    GPU_BACKEND="cuda"
    VIDEO_CODEC=${VIDEO_CODEC:-h264_nvenc}
    VIDEO_PRESET=${VIDEO_PRESET:-p4}
    FINAL_HWACCEL_ARGS=(-hwaccel cuda -hwaccel_output_format cuda)
    GPU_STATUS_MESSAGE="ENABLE_GPU=1: detected NVIDIA stack via nvidia-smi; switching to h264_nvenc"
    return
  fi

  if [[ "$backend" == "cuda" ]]; then
    GPU_STATUS_MESSAGE="ENABLE_GPU=1 but CUDA backend requested and nvidia-smi not found; falling back to CPU"
    return
  fi

  if { [[ "$backend" == "auto" ]] || [[ "$backend" == "vaapi" ]]; } && command -v vainfo >/dev/null 2>&1; then
    GPU_BACKEND="vaapi-pending"
    GPU_STATUS_MESSAGE="Detected VAAPI stack via vainfo but automation is TODO; see docs/local_av/ffmpeg_pipeline/unresolved_work.md"
    return
  fi

  if [[ "$backend" == "vaapi" ]]; then
    GPU_STATUS_MESSAGE="ENABLE_GPU=1 but VAAPI backend is not yet wired; falling back to CPU"
    return
  fi

  GPU_STATUS_MESSAGE="ENABLE_GPU=1 but no supported backend detected (missing nvidia-smi/vainfo); using CPU"
}

record_gpu_status() {
  local target="$1"
  [[ -z "$target" ]] && return
  cat >"$target" <<EOF
backend=${GPU_BACKEND}
encoder=${VIDEO_CODEC}
preset=${VIDEO_PRESET}
message=${GPU_STATUS_MESSAGE}
timestamp=$(date -Is)
EOF
}

calc_manifest_duration() {
  local manifest="$1"
  local total="0"
  while IFS= read -r line || [[ -n "$line" ]]; do
    line=${line%$'\r'}
    [[ -z "$line" ]] && continue
    [[ "${line:0:4}" != "file" ]] && continue
    local path=${line#file }
    path=${path# } # trim leading space if present
    path=${path#\'}
    path=${path#\"}
    path=${path%\'}
    path=${path%\"}
    if [[ ! -f "$path" ]]; then
      log "Warning: manifest entry $path is missing; duration calculation may be inaccurate"
      continue
    fi
    local duration
    if duration=$("$FFPROBE_BIN" -v error -show_entries format=duration \
      -of default=nokey=1:noprint_wrappers=1 "$path" 2>/dev/null); then
      total=$(awk -v acc="$total" -v cur="$duration" 'BEGIN { printf "%.6f", acc + cur }')
    else
      log "Warning: unable to probe duration for $path"
    fi
  done <"$manifest"
  printf "%.2f" "$total"
}

make_segment() {
  local color="$1"
  local label="$2"
  local target="$3"
  local vf="format=yuv420p"
  if [[ -f "$FONT_FILE" ]]; then
    vf+=",drawtext=fontfile='${FONT_FILE}':text='${label}':fontsize=64:fontcolor=white:x=(w-text_w)/2:y=(h-text_h)/2"
  fi
  "$FFMPEG_BIN" -hide_banner -loglevel error -y \
    -f lavfi -i "color=c=${color}:s=${RESOLUTION}:d=${SEGMENT_DURATION}" \
    -vf "$vf" \
    "$target"
}

write_manifest() {
  local manifest="$1"
  shift
  : >"$manifest"
  for path in "$@"; do
    echo "file '$path'" >>"$manifest"
  done
}

if [[ -n "$SOURCE_MANIFEST" ]]; then
  if [[ ! -f "$SOURCE_MANIFEST" ]]; then
    log "Provided SOURCE_MANIFEST=$SOURCE_MANIFEST does not exist"
    exit 1
  fi
  log "Using user-provided manifest $SOURCE_MANIFEST"
  cp "$SOURCE_MANIFEST" "$workdir/segments.txt"
  log "Reminder: script does not auto-scale or insert filtergraph rules for mismatched sources yet"
else
  log "Generating base segments in $workdir"
  make_segment "#1a73e8" "SCENE_1" "$workdir/scene1.mp4"
  make_segment "#fbbc04" "SCENE_2" "$workdir/scene2.mp4"
  write_manifest "$workdir/segments.txt" "$workdir/scene1.mp4" "$workdir/scene2.mp4"
fi

configure_video_encoder
log "$GPU_STATUS_MESSAGE"
log "Video encoder: ${VIDEO_CODEC} (preset=${VIDEO_PRESET}, pixel_format=${PIXEL_FORMAT})"
record_gpu_status "$GPU_STATUS_FILE"

if [[ "$SIMULATE_MISSING_INPUT" == "1" ]]; then
  missing_path="$workdir/missing_scene.mp4"
  broken_manifest="$workdir/broken_segments.txt"
  write_manifest "$broken_manifest" "$workdir/scene1.mp4" "$missing_path"
  log "Simulating FFM-006 (missing segment) via $broken_manifest"
  set +e
  "$FFMPEG_BIN" -hide_banner -loglevel error -xerror -y \
    -f concat -safe 0 -i "$broken_manifest" -c copy "$workdir/broken_story.mp4"
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    log "Expected ffmpeg to fail when a segment is missing but it succeeded"
    exit 1
  fi
  log "FFmpeg exited with status $status as expected; see stderr above for missing path diagnostics."
fi

log "Concatenating segments"
"$FFMPEG_BIN" -hide_banner -loglevel error -y \
  -f concat -safe 0 -i "$workdir/segments.txt" -c copy "$workdir/story.mp4"

log "Adding watermark overlay and music bed"
total_duration=$(calc_manifest_duration "$workdir/segments.txt")
determine_watermark_position
log "Watermark text: ${WATERMARK_TEXT} (position=${RESOLVED_WATERMARK_POSITION}, margin=${WATERMARK_MARGIN}, font_size=${WATERMARK_FONT_SIZE}, opacity=${WATERMARK_OPACITY})"
if [[ -n "$WATERMARK_IMAGE_PATH" ]]; then
  if [[ -f "$WATERMARK_IMAGE_PATH" ]]; then
    log "PNG watermark: ${WATERMARK_IMAGE_PATH} (scale=${WATERMARK_IMAGE_SCALE}, opacity=${WATERMARK_IMAGE_OPACITY}, position=${RESOLVED_WATERMARK_POSITION})"
  else
    log "Configured WATERMARK_IMAGE_PATH=$WATERMARK_IMAGE_PATH does not exist"
    exit 1
  fi
else
  log "PNG watermark disabled (set WATERMARK_IMAGE_PATH to overlay logos; subtitle styling still TODO)"
fi

if [[ -n "$SUBTITLE_FILE" ]]; then
  if [[ -f "$SUBTITLE_FILE" ]]; then
    SUBTITLE_ENABLED=1
    log "Subtitles enabled: ${SUBTITLE_FILE} (charset=${SUBTITLE_CHARSET}, force_style=${SUBTITLE_FORCE_STYLE:-default}; 字幕样式模板仍未交付)"
  else
    log "Configured SUBTITLE_FILE=$SUBTITLE_FILE does not exist"
    exit 1
  fi
else
  log "Subtitles disabled (pass SUBTITLE_FILE=/path/to/demo.srt to show captions; full模板/字体回退仍缺)"
fi

input_args=("-i" "$workdir/story.mp4")
audio_labels=()
next_input=1
overlay_input_label=""

if [[ -n "$WATERMARK_IMAGE_PATH" ]]; then
  input_args+=("-loop" "1" "-t" "$total_duration" "-i" "$WATERMARK_IMAGE_PATH")
  overlay_input_label="[${next_input}:v]"
  ((next_input++))
fi

if [[ -n "$PRIMARY_AUDIO_PATH" ]]; then
  if [[ ! -f "$PRIMARY_AUDIO_PATH" ]]; then
    log "PRIMARY_AUDIO_PATH=$PRIMARY_AUDIO_PATH not found"
    exit 1
  fi
  input_args+=("-i" "$PRIMARY_AUDIO_PATH")
  audio_labels+=("[${next_input}:a]")
  ((next_input++))
fi

if [[ -n "$SECONDARY_AUDIO_PATH" ]]; then
  if [[ ! -f "$SECONDARY_AUDIO_PATH" ]]; then
    log "SECONDARY_AUDIO_PATH=$SECONDARY_AUDIO_PATH not found"
    exit 1
  fi
  input_args+=("-i" "$SECONDARY_AUDIO_PATH")
  audio_labels+=("[${next_input}:a]")
  ((next_input++))
fi

if [[ ${#audio_labels[@]} -eq 0 ]]; then
  input_args+=("-f" "lavfi" "-i" "sine=frequency=640:sample_rate=48000:duration=${total_duration}")
  audio_labels=("[${next_input}:a]")
  ((next_input++))
fi

audio_filter=""
if [[ ${#audio_labels[@]} -eq 1 ]]; then
  audio_filter="${audio_labels[0]}volume=${AUDIO_VOLUME}[a]"
else
  mix_inputs=""
  for label in "${audio_labels[@]}"; do
    mix_inputs+="$label"
  done
  audio_filter="${mix_inputs}amix=inputs=${#audio_labels[@]}:duration=longest[a_mix];[a_mix]volume=${AUDIO_VOLUME}[a]"
fi

video_filter_parts=()
video_label_index=0
current_video_label="[v${video_label_index}]"
video_filter_parts+=("[0:v]format=yuv420p${current_video_label}")

if [[ -f "$FONT_FILE" ]]; then
  video_label_index=$((video_label_index + 1))
  next_label="[v${video_label_index}]"
  video_filter_parts+=("${current_video_label}drawtext=fontfile='${FONT_FILE}':text='${WATERMARK_TEXT}':fontsize=${WATERMARK_FONT_SIZE}:fontcolor=white@${WATERMARK_OPACITY}:x=${WATERMARK_TEXT_POS_X}:y=${WATERMARK_TEXT_POS_Y}${next_label}")
  current_video_label="$next_label"
else
  log "Font file $FONT_FILE not found; skipping text watermark (subtitle presets still pending per docs/local_av/ffmpeg_pipeline/unresolved_work.md)"
fi

if [[ "$SUBTITLE_ENABLED" -eq 1 ]]; then
  subtitle_path=$(escape_subtitle_value "$SUBTITLE_FILE")
  subtitle_charset=$(escape_subtitle_value "$SUBTITLE_CHARSET")
  subtitle_filter="subtitles='${subtitle_path}':charenc='${subtitle_charset}'"
  if [[ -n "$SUBTITLE_FORCE_STYLE" ]]; then
    subtitle_style=$(escape_subtitle_value "$SUBTITLE_FORCE_STYLE")
    subtitle_filter+=":force_style='${subtitle_style}'"
  fi
  video_label_index=$((video_label_index + 1))
  next_label="[v${video_label_index}]"
  video_filter_parts+=("${current_video_label}${subtitle_filter}${next_label}")
  current_video_label="$next_label"
fi

if [[ -n "$overlay_input_label" ]]; then
  overlay_label_index=0
  current_overlay_label="[wm${overlay_label_index}]"
  video_filter_parts+=("${overlay_input_label}format=rgba${current_overlay_label}")

  if [[ "$WATERMARK_IMAGE_SCALE" != "1" ]] && [[ "$WATERMARK_IMAGE_SCALE" != "1.0" ]]; then
    overlay_label_index=$((overlay_label_index + 1))
    next_overlay="[wm${overlay_label_index}]"
    video_filter_parts+=("${current_overlay_label}scale=iw*${WATERMARK_IMAGE_SCALE}:ih*${WATERMARK_IMAGE_SCALE}${next_overlay}")
    current_overlay_label="$next_overlay"
  fi

  if [[ "$WATERMARK_IMAGE_OPACITY" != "1" ]] && [[ "$WATERMARK_IMAGE_OPACITY" != "1.0" ]]; then
    overlay_label_index=$((overlay_label_index + 1))
    next_overlay="[wm${overlay_label_index}]"
    video_filter_parts+=("${current_overlay_label}colorchannelmixer=aa=${WATERMARK_IMAGE_OPACITY}${next_overlay}")
    current_overlay_label="$next_overlay"
  fi

  video_label_index=$((video_label_index + 1))
  next_label="[v${video_label_index}]"
  video_filter_parts+=("${current_video_label}${current_overlay_label}overlay=${WATERMARK_OVERLAY_POS_X}:${WATERMARK_OVERLAY_POS_Y}${next_label}")
  current_video_label="$next_label"
fi

filter_complex_video=$(IFS=';'; echo "${video_filter_parts[*]}")
full_filter="${filter_complex_video};${audio_filter}"

"$FFMPEG_BIN" -hide_banner -loglevel error -y \
  "${input_args[@]}" \
  "${FINAL_HWACCEL_ARGS[@]}" \
  -filter_complex "$full_filter" \
  -map "$current_video_label" -map "[a]" -c:v "$VIDEO_CODEC" -preset "$VIDEO_PRESET" -pix_fmt "$PIXEL_FORMAT" -c:a aac -shortest \
  "$workdir/final.mp4"

mkdir -p "$(dirname "$OUTPUT_PATH")"
cp "$workdir/final.mp4" "$OUTPUT_PATH"

log "Wrote demo clip to $OUTPUT_PATH"
log "Probe summary:"
"$FFPROBE_BIN" -hide_banner -loglevel error \
  -select_streams v:0 -show_entries stream=codec_name,width,height,r_frame_rate \
  -of default=noprint_wrappers=1 "$OUTPUT_PATH"
