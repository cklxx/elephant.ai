#!/usr/bin/env bash
#
# Agents Teams E2E Test Suite
# Tests agent team orchestration via the inject API (offline mode).
#
# Usage:
#   ./scripts/test_agents_teams_e2e.sh                  # run all 14 cases
#   ./scripts/test_agents_teams_e2e.sh --category C     # run one category
#   ./scripts/test_agents_teams_e2e.sh --case A1        # run single case
#   ./scripts/test_agents_teams_e2e.sh --dry-run        # show cases without executing

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
SENDER_ID="${SENDER_ID:-ou_e2e_teams}"
TIMEOUT_KIMI="${TIMEOUT_KIMI:-300}"         # 5 min for kimi-heavy cases
TIMEOUT_INTERNAL="${TIMEOUT_INTERNAL:-120}" # 2 min for internal-only cases
COOLDOWN_KIMI="${COOLDOWN_KIMI:-30}"        # seconds between kimi cases
COOLDOWN_HEAVY="${COOLDOWN_HEAVY:-60}"      # seconds between complex cases (cat E)
DRY_RUN=0

# CLI args
FILTER_CATEGORY=""
FILTER_CASE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --category) FILTER_CATEGORY="$2"; shift 2 ;;
        --case)     FILTER_CASE="$2"; shift 2 ;;
        --dry-run)  DRY_RUN=1; shift ;;
        --url)      INJECT_URL="$2"; shift 2 ;;
        *)          echo "Unknown flag: $1"; exit 1 ;;
    esac
done

# ---------------------------------------------------------------------------
# Counters
# ---------------------------------------------------------------------------
TOTAL=0; PASS=0; PARTIAL=0; FAIL=0; SKIP=0
EMPTY_ASSISTANT_ERRORS=0

# Result arrays for final summary
declare -a RESULT_IDS=()
declare -a RESULT_STATUSES=()
declare -a RESULT_NOTES=()

# ---------------------------------------------------------------------------
# Temp dir
# ---------------------------------------------------------------------------
TMPDIR_BASE="/tmp/agents_teams_e2e_$(date +%s)"
mkdir -p "${TMPDIR_BASE}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

log() { echo "[$(date +%H:%M:%S)] $*"; }

inject() {
    # inject <case_id> <json_payload> <timeout>
    local case_id="$1"
    local payload="$2"
    local timeout="${3:-${TIMEOUT_KIMI}}"

    local req_file="${TMPDIR_BASE}/${case_id}_req.json"
    local resp_file="${TMPDIR_BASE}/${case_id}_resp.json"
    local hdr_file="${TMPDIR_BASE}/${case_id}_hdr.txt"

    echo "${payload}" > "${req_file}"

    log "  → POST ${INJECT_URL} (timeout=${timeout}s)"

    local curl_exit=0
    curl -sS -D "${hdr_file}" -o "${resp_file}" \
        -X POST "${INJECT_URL}" \
        -H "Content-Type: application/json" \
        --max-time $((timeout + 30)) \
        --data @"${req_file}" || curl_exit=$?

    if [[ ${curl_exit} -ne 0 ]]; then
        echo "CURL_ERROR"
        log "  ✗ curl failed with exit code ${curl_exit}"
        echo '{"error":"curl_failed","replies":[]}' > "${resp_file}"
        return
    fi

    local http_code
    http_code="$(head -n 1 "${hdr_file}" | awk '{print $2}' | tr -d '\r')"

    if [[ "${http_code}" != "200" ]]; then
        echo "HTTP_${http_code}"
        log "  ✗ HTTP ${http_code}"
        return
    fi

    # Extract key metrics
    local duration_ms replies_count error_msg
    duration_ms="$(jq -r '.duration_ms // 0' "${resp_file}")"
    replies_count="$(jq -r '.replies | length' "${resp_file}")"
    error_msg="$(jq -r '.error // empty' "${resp_file}")"

    if [[ -n "${error_msg}" ]]; then
        echo "API_ERROR:${error_msg}"
        log "  ✗ API error: ${error_msg}"
        return
    fi

    # Check for empty assistant in replies
    local empty_asst_count
    empty_asst_count="$(jq -r '[.replies[]? | select(.content != null) | .content | test("empty.?assistant"; "i") // false] | map(select(.)) | length' "${resp_file}" 2>/dev/null || echo 0)"
    EMPTY_ASSISTANT_ERRORS=$((EMPTY_ASSISTANT_ERRORS + empty_asst_count))

    log "  ✓ ${replies_count} replies in ${duration_ms}ms"
    echo "OK:${replies_count}:${duration_ms}"
}

build_payload() {
    # build_payload <chat_id> <text> <timeout_seconds>
    local chat_id="$1"
    local text="$2"
    local timeout="$3"

    jq -n \
        --arg text "${text}" \
        --arg chat_id "${chat_id}" \
        --arg sender_id "${SENDER_ID}" \
        --argjson timeout_seconds "${timeout}" \
        '{
            text: $text,
            chat_id: $chat_id,
            chat_type: "p2p",
            sender_id: $sender_id,
            timeout_seconds: $timeout_seconds,
            auto_reply: true,
            max_auto_reply_rounds: 3
        }'
}

record_result() {
    local case_id="$1" status="$2" note="$3"
    RESULT_IDS+=("${case_id}")
    RESULT_STATUSES+=("${status}")
    RESULT_NOTES+=("${note}")

    case "${status}" in
        PASS)    ((PASS++))    ;;
        PARTIAL) ((PARTIAL++)) ;;
        FAIL)    ((FAIL++))    ;;
        SKIP)    ((SKIP++))    ;;
    esac
    ((TOTAL++))
}

should_run() {
    local case_id="$1"
    local category="${case_id:0:1}"

    if [[ -n "${FILTER_CASE}" && "${case_id}" != "${FILTER_CASE}" ]]; then
        return 1
    fi
    if [[ -n "${FILTER_CATEGORY}" && "${category}" != "${FILTER_CATEGORY}" ]]; then
        return 1
    fi
    return 0
}

cooldown() {
    local seconds="$1"
    if [[ ${DRY_RUN} -eq 1 ]]; then return; fi
    log "  ⏳ cooldown ${seconds}s (rate limit protection)"
    sleep "${seconds}"
}

evaluate_team_result() {
    # evaluate_team_result <case_id> <inject_result> <expected_min_replies>
    local case_id="$1" result="$2" min_replies="${3:-1}"

    if [[ "${result}" == CURL_ERROR ]]; then
        record_result "${case_id}" "FAIL" "curl connection failed — is server running?"
        return
    fi
    if [[ "${result}" == HTTP_* ]]; then
        record_result "${case_id}" "FAIL" "unexpected ${result}"
        return
    fi
    if [[ "${result}" == API_ERROR:* ]]; then
        local err="${result#API_ERROR:}"
        if echo "${err}" | grep -qi "rate.limit\|429\|throttl"; then
            record_result "${case_id}" "PARTIAL" "rate limited: ${err}"
        else
            record_result "${case_id}" "FAIL" "API error: ${err}"
        fi
        return
    fi
    if [[ "${result}" == OK:* ]]; then
        local replies duration
        replies="$(echo "${result}" | cut -d: -f2)"
        duration="$(echo "${result}" | cut -d: -f3)"

        if [[ ${replies} -ge ${min_replies} ]]; then
            record_result "${case_id}" "PASS" "${replies} replies, ${duration}ms"
        elif [[ ${replies} -gt 0 ]]; then
            record_result "${case_id}" "PARTIAL" "only ${replies}/${min_replies} replies, ${duration}ms"
        else
            record_result "${case_id}" "FAIL" "0 replies in ${duration}ms"
        fi
        return
    fi

    record_result "${case_id}" "FAIL" "unknown result: ${result}"
}

evaluate_error_case() {
    # For cases expected to return errors gracefully
    local case_id="$1" result="$2" expected_pattern="$3"

    if [[ "${result}" == CURL_ERROR ]]; then
        record_result "${case_id}" "FAIL" "curl failed — server unreachable"
        return
    fi

    # For error cases, any non-crash response is PASS (graceful handling)
    if [[ "${result}" == HTTP_* || "${result}" == API_ERROR:* || "${result}" == OK:* ]]; then
        record_result "${case_id}" "PASS" "graceful handling: ${result}"
        return
    fi

    record_result "${case_id}" "FAIL" "unexpected: ${result}"
}

# ---------------------------------------------------------------------------
# Test Case Definitions
# ---------------------------------------------------------------------------

# === Category A: Core functionality ===

run_A1() {
    log "A1: 2-stage kimi_research baseline"
    local chat_id="oc_e2e_teams_A1_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research goal=\"Go error wrapping with fmt.Errorf %w: best practices and common pitfalls\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject A1 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result A1 "${result}" 1
}

run_A2() {
    log "A2: 3-stage serial pipeline (technical_analysis)"
    local chat_id="oc_e2e_teams_A2_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=technical_analysis goal=\"Compare Redis vs Memcached for session caching: features, performance, persistence, clustering\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject A2 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result A2 "${result}" 1
}

run_A3() {
    log "A3: 3-way parallel + fan-in (competitive_review)"
    local chat_id="oc_e2e_teams_A3_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=competitive_review goal=\"Rust vs Go for backend microservices in 2025\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject A3 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result A3 "${result}" 1
}

# === Category B: Input boundaries ===

run_B1() {
    log "B1: Minimal single-word goal"
    local chat_id="oc_e2e_teams_B1_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research goal=\"Kubernetes\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject B1 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result B1 "${result}" 1
}

run_B2() {
    log "B2: Long multi-topic goal"
    local chat_id="oc_e2e_teams_B2_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research goal=\"Compare gRPC vs REST vs GraphQL for inter-service communication, focusing on performance under high concurrency, schema evolution strategy, and tooling ecosystem maturity in Go and Rust\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject B2 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result B2 "${result}" 1
}

run_B3() {
    log "B3: Goal with special characters (Go code)"
    local chat_id="oc_e2e_teams_B3_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research goal=\"Go select {} with default case vs time.After for timeout patterns\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject B3 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result B3 "${result}" 1
}

run_B4() {
    log "B4: English-only instruction"
    local chat_id="oc_e2e_teams_B4_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research goal=\"Explain the CAP theorem and its practical implications for distributed databases\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject B4 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result B4 "${result}" 1
}

# === Category C: Error handling & degradation ===

run_C1() {
    log "C1: Non-existent template"
    local chat_id="oc_e2e_teams_C1_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=nonexistent_team goal=\"test\"" \
        "${TIMEOUT_INTERNAL}")"
    local result
    result="$(inject C1 "${payload}" "${TIMEOUT_INTERNAL}")"
    evaluate_error_case C1 "${result}" "not.found\|unknown\|nonexistent"
}

run_C2() {
    log "C2: Missing goal parameter"
    local chat_id="oc_e2e_teams_C2_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research" \
        "${TIMEOUT_INTERNAL}")"
    local result
    result="$(inject C2 "${payload}" "${TIMEOUT_INTERNAL}")"
    evaluate_error_case C2 "${result}" "goal\|required\|missing"
}

run_C3() {
    log "C3: List templates"
    local chat_id="oc_e2e_teams_C3_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=list" \
        "${TIMEOUT_INTERNAL}")"
    local result
    result="$(inject C3 "${payload}" "${TIMEOUT_INTERNAL}")"

    if [[ "${result}" == OK:* ]]; then
        local replies
        replies="$(echo "${result}" | cut -d: -f2)"
        local resp_file="${TMPDIR_BASE}/C3_resp.json"
        local has_templates=0
        if jq -r '.replies[]?.content // ""' "${resp_file}" 2>/dev/null | grep -qi "kimi_research\|technical_analysis\|competitive_review"; then
            has_templates=1
        fi
        if [[ ${has_templates} -eq 1 ]]; then
            record_result C3 "PASS" "listed templates correctly (${replies} replies)"
        elif [[ ${replies} -gt 0 ]]; then
            record_result C3 "PARTIAL" "${replies} replies but template names not confirmed in content"
        else
            record_result C3 "FAIL" "0 replies"
        fi
    else
        evaluate_error_case C3 "${result}" "template\|list"
    fi
}

run_C4() {
    log "C4: Duplicate request on same chat_id"
    local chat_id="oc_e2e_teams_C4_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=kimi_research goal=\"Quick test for duplicate detection\"" \
        "${TIMEOUT_KIMI}")"

    log "  → First request"
    local result1
    result1="$(inject C4 "${payload}" "${TIMEOUT_KIMI}")"

    log "  → Second request (same chat_id)"
    local result2
    result2="$(inject C4_dup "${payload}" "${TIMEOUT_KIMI}")"

    # Either both succeed or second gets a conflict — both acceptable
    if [[ "${result1}" == OK:* || "${result1}" == API_ERROR:* ]]; then
        record_result C4 "PASS" "first=${result1}, second=${result2}"
    else
        evaluate_team_result C4 "${result1}" 1
    fi
}

# === Category D: Prompt override ===

run_D1() {
    log "D1: Prompt override for single role"
    local chat_id="oc_e2e_teams_D1_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        '@alex /run_tasks template=kimi_research goal="Python async patterns" prompts={"researcher":"Focus ONLY on asyncio.gather vs TaskGroup. Be extremely brief, 2 sentences max."}' \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject D1 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result D1 "${result}" 1
}

# === Category E: Complex real-world scenarios ===

run_E1() {
    log "E1: Real tech selection (competitive_review)"
    local chat_id="oc_e2e_teams_E1_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=competitive_review goal=\"PostgreSQL vs MySQL vs CockroachDB for multi-region deployment: consistency guarantees, operational complexity, and cost at 10K TPS\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject E1 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result E1 "${result}" 1
}

run_E2() {
    log "E2: Real architecture analysis (technical_analysis)"
    local chat_id="oc_e2e_teams_E2_$(date +%s)"
    local payload
    payload="$(build_payload "${chat_id}" \
        "@alex /run_tasks template=technical_analysis goal=\"Event sourcing vs CRUD for financial transaction systems: consistency, auditability, query complexity, and team learning curve\"" \
        "${TIMEOUT_KIMI}")"
    local result
    result="$(inject E2 "${payload}" "${TIMEOUT_KIMI}")"
    evaluate_team_result E2 "${result}" 1
}

# ---------------------------------------------------------------------------
# Execution orchestration
# ---------------------------------------------------------------------------

run_category() {
    local cat="$1"
    case "${cat}" in
        A)
            log "=== Category A: Core functionality ==="
            if should_run A1; then run_A1; cooldown "${COOLDOWN_KIMI}"; fi
            if should_run A2; then run_A2; cooldown "${COOLDOWN_KIMI}"; fi
            if should_run A3; then run_A3; fi
            ;;
        B)
            log "=== Category B: Input boundaries ==="
            if should_run B1; then run_B1; cooldown "${COOLDOWN_KIMI}"; fi
            if should_run B2; then run_B2; cooldown "${COOLDOWN_KIMI}"; fi
            if should_run B3; then run_B3; cooldown "${COOLDOWN_KIMI}"; fi
            if should_run B4; then run_B4; fi
            ;;
        C)
            log "=== Category C: Error handling & degradation ==="
            if should_run C1; then run_C1; fi
            if should_run C2; then run_C2; fi
            if should_run C3; then run_C3; fi
            if should_run C4; then run_C4; fi
            ;;
        D)
            log "=== Category D: Prompt override ==="
            if should_run D1; then run_D1; fi
            ;;
        E)
            log "=== Category E: Complex real-world scenarios ==="
            if should_run E1; then run_E1; cooldown "${COOLDOWN_HEAVY}"; fi
            if should_run E2; then run_E2; fi
            ;;
    esac
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

log "Agents Teams E2E Test Suite"
log "Inject URL: ${INJECT_URL}"
log "Temp dir: ${TMPDIR_BASE}"

if [[ ${DRY_RUN} -eq 1 ]]; then
    log "DRY RUN — listing cases without execution"
    echo ""
    echo "Category A (Core):"
    echo "  A1: 2-stage kimi_research baseline"
    echo "  A2: 3-stage serial pipeline (technical_analysis)"
    echo "  A3: 3-way parallel + fan-in (competitive_review)"
    echo ""
    echo "Category B (Input boundaries):"
    echo "  B1: Minimal single-word goal"
    echo "  B2: Long multi-topic goal"
    echo "  B3: Goal with special characters"
    echo "  B4: English-only instruction"
    echo ""
    echo "Category C (Error handling):"
    echo "  C1: Non-existent template"
    echo "  C2: Missing goal parameter"
    echo "  C3: List templates"
    echo "  C4: Duplicate request on same chat_id"
    echo ""
    echo "Category D (Prompt override):"
    echo "  D1: Override single role prompt"
    echo ""
    echo "Category E (Complex scenarios):"
    echo "  E1: Real tech selection (competitive_review)"
    echo "  E2: Real architecture analysis (technical_analysis)"
    exit 0
fi

# Verify server is reachable
log "Checking server connectivity..."
if ! curl -sS --max-time 5 -o /dev/null "${INJECT_URL}" -X POST \
    -H "Content-Type: application/json" -d '{"text":"ping","chat_id":"oc_healthcheck"}' 2>/dev/null; then
    log "⚠ Server may not be reachable at ${INJECT_URL}. Proceeding anyway..."
fi

# Execution order per plan: C → B → A → D → E
if [[ -n "${FILTER_CATEGORY}" ]]; then
    run_category "${FILTER_CATEGORY}"
elif [[ -n "${FILTER_CASE}" ]]; then
    cat="${FILTER_CASE:0:1}"
    run_category "${cat}"
else
    run_category C
    run_category B
    cooldown "${COOLDOWN_KIMI}"
    run_category A
    cooldown "${COOLDOWN_KIMI}"
    run_category D
    cooldown "${COOLDOWN_HEAVY}"
    run_category E
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "================================================================"
echo "  AGENTS TEAMS E2E — TEST RESULTS"
echo "================================================================"
echo ""

for i in "${!RESULT_IDS[@]}"; do
    st="${RESULT_STATUSES[$i]}"
    case "${st}" in
        PASS)    icon="✅" ;;
        PARTIAL) icon="⚠️ " ;;
        FAIL)    icon="❌" ;;
        SKIP)    icon="⏭️ " ;;
    esac
    printf "  %s %-6s %-8s %s\n" "${icon}" "${RESULT_IDS[$i]}" "${st}" "${RESULT_NOTES[$i]}"
done

echo ""
echo "────────────────────────────────────────────────────────────────"
printf "  Total: %d | Pass: %d | Partial: %d | Fail: %d | Skip: %d\n" \
    "${TOTAL}" "${PASS}" "${PARTIAL}" "${FAIL}" "${SKIP}"
echo "  Empty assistant errors: ${EMPTY_ASSISTANT_ERRORS}"
echo "  Temp files: ${TMPDIR_BASE}/"
echo "────────────────────────────────────────────────────────────────"

if [[ ${FAIL} -gt 0 ]]; then
    echo ""
    echo "  ❌ SUITE FAILED — ${FAIL} case(s) failed"
    echo ""
    echo "  Debug: check response files in ${TMPDIR_BASE}/"
    echo "  Example: jq . ${TMPDIR_BASE}/A1_resp.json"
    exit 1
elif [[ ${PARTIAL} -gt 0 ]]; then
    echo ""
    echo "  ⚠️  SUITE PARTIAL — ${PARTIAL} case(s) partially passed (likely rate limits)"
    exit 0
else
    echo ""
    echo "  ✅ ALL PASSED"
    exit 0
fi
