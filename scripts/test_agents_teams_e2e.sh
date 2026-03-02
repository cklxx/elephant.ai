#!/usr/bin/env bash
#
# Agent Teams E2E Test Suite — Local (Claude-only)
#
# Tests the full agent team pipeline via the dev inject endpoint.
# All external agent roles use claude_code — no kimi or codex quota required.
#
# Prerequisites:
#   - Server running:  alex dev restart backend
#   - Claude CLI available and authenticated
#
# Usage:
#   ./scripts/test_agents_teams_e2e.sh                    # all cases
#   ./scripts/test_agents_teams_e2e.sh --category C       # single category
#   ./scripts/test_agents_teams_e2e.sh --case A1          # single case
#   ./scripts/test_agents_teams_e2e.sh --dry-run          # list cases

set -euo pipefail

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------
INJECT_URL="${INJECT_URL:-http://127.0.0.1:9090/api/dev/inject}"
SENDER_ID="${SENDER_ID:-ou_e2e_claude_teams}"
TIMEOUT_FAST="${TIMEOUT_FAST:-120}"  # single-stage claude tasks
TIMEOUT_SLOW="${TIMEOUT_SLOW:-300}"  # multi-stage or debate tasks
COOLDOWN="${COOLDOWN:-5}"            # seconds between cases
DRY_RUN=0

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
# Counters and state
# ---------------------------------------------------------------------------
TOTAL=0; PASS=0; PARTIAL=0; FAIL=0; SKIP=0
declare -a RESULT_IDS=()
declare -a RESULT_STATUSES=()
declare -a RESULT_NOTES=()
TMPDIR_BASE="/tmp/agent_teams_e2e_$(date +%s)"
mkdir -p "${TMPDIR_BASE}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log() { echo "[$(date +%H:%M:%S)] $*"; }

inject() {
    local case_id="$1" payload="$2" timeout="${3:-${TIMEOUT_FAST}}"
    local req="${TMPDIR_BASE}/${case_id}_req.json"
    local resp="${TMPDIR_BASE}/${case_id}_resp.json"
    local hdr="${TMPDIR_BASE}/${case_id}_hdr.txt"

    echo "${payload}" > "${req}"
    log "  → POST ${INJECT_URL} (timeout=${timeout}s)"

    local curl_exit=0
    curl -sS -D "${hdr}" -o "${resp}" \
        -X POST "${INJECT_URL}" \
        -H "Content-Type: application/json" \
        --max-time $((timeout + 30)) \
        --data @"${req}" || curl_exit=$?

    if [[ ${curl_exit} -ne 0 ]]; then
        log "  ✗ curl failed (exit ${curl_exit})"
        echo "CURL_ERROR"
        return
    fi

    local http_code
    http_code="$(head -n 1 "${hdr}" | awk '{print $2}' | tr -d '\r')"
    if [[ "${http_code}" != "200" ]]; then
        log "  ✗ HTTP ${http_code}"
        echo "HTTP_${http_code}"
        return
    fi

    local replies duration err
    replies="$(jq -r '.replies | length' "${resp}" 2>/dev/null || echo 0)"
    duration="$(jq -r '.duration_ms // 0' "${resp}" 2>/dev/null || echo 0)"
    err="$(jq -r '.error // empty' "${resp}" 2>/dev/null || true)"

    if [[ -n "${err}" ]]; then
        log "  ✗ API error: ${err}"
        echo "API_ERROR:${err}"
        return
    fi

    log "  ✓ ${replies} replies in ${duration}ms"
    echo "OK:${replies}:${duration}"
}

make_payload() {
    local chat_id="$1" text="$2" timeout="$3"
    jq -n \
        --arg text "${text}" \
        --arg chat_id "${chat_id}" \
        --arg sender_id "${SENDER_ID}" \
        --argjson timeout_seconds "${timeout}" \
        '{text: $text, chat_id: $chat_id, chat_type: "p2p",
          sender_id: $sender_id, timeout_seconds: $timeout_seconds,
          auto_reply: true, max_auto_reply_rounds: 3}'
}

record() {
    local id="$1" status="$2" note="$3"
    RESULT_IDS+=("${id}")
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

eval_ok() {
    local id="$1" result="$2" min="${3:-1}"
    case "${result}" in
        CURL_ERROR)  record "${id}" FAIL "curl failed — is server running?"; return ;;
        HTTP_*)      record "${id}" FAIL "${result}"; return ;;
        API_ERROR:*) record "${id}" FAIL "${result#API_ERROR:}"; return ;;
        OK:*)
            local n; n="$(echo "${result}" | cut -d: -f2)"
            local ms; ms="$(echo "${result}" | cut -d: -f3)"
            if [[ ${n} -ge ${min} ]]; then
                record "${id}" PASS "${n} replies, ${ms}ms"
            elif [[ ${n} -gt 0 ]]; then
                record "${id}" PARTIAL "only ${n}/${min} replies"
            else
                record "${id}" FAIL "0 replies"
            fi
            ;;
        *) record "${id}" FAIL "unknown result: ${result}" ;;
    esac
}

eval_error() {
    local id="$1" result="$2"
    case "${result}" in
        CURL_ERROR) record "${id}" FAIL "curl failed — is server running?"; return ;;
        OK:*|HTTP_*|API_ERROR:*) record "${id}" PASS "graceful: ${result}"; return ;;
        *) record "${id}" FAIL "unknown: ${result}" ;;
    esac
}

should_run() {
    local id="$1" cat="${1:0:1}"
    [[ -n "${FILTER_CASE}" && "${id}" != "${FILTER_CASE}" ]] && return 1
    [[ -n "${FILTER_CATEGORY}" && "${cat}" != "${FILTER_CATEGORY}" ]] && return 1
    return 0
}

cooldown() {
    [[ ${DRY_RUN} -eq 1 ]] && return
    log "  ⏳ cooldown ${COOLDOWN}s"
    sleep "${COOLDOWN}"
}

resp_content() {
    # resp_content <case_id> — returns joined reply content for grepping
    local rf="${TMPDIR_BASE}/${1}_resp.json"
    jq -r '[.replies[]?.content // ""] | join(" ")' "${rf}" 2>/dev/null || echo ""
}

# ---------------------------------------------------------------------------
# Category A: Core templates
# ---------------------------------------------------------------------------

run_A1() {
    log "A1: claude_research — single-stage research"
    local id="A1" chat="oc_e2e_teams_A1_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_research goal=\"Go context propagation: patterns, pitfalls, and best practices for cancellation in concurrent programs\"" \
        "${TIMEOUT_FAST}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")" 1
}

run_A2() {
    log "A2: claude_analysis — parallel dual-perspective + synthesis"
    local id="A2" chat="oc_e2e_teams_A2_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_analysis goal=\"PostgreSQL vs CockroachDB for multi-region deployment: consistency, operational complexity, cost at 5K TPS\"" \
        "${TIMEOUT_SLOW}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_SLOW}")" 1
}

run_A3() {
    log "A3: claude_debate — analyst + auto-challenger (debate_mode) + reviewer"
    local id="A3" chat="oc_e2e_teams_A3_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_debate goal=\"Event sourcing vs CRUD for a financial audit system: which provides stronger long-term auditability with acceptable query complexity?\"" \
        "${TIMEOUT_SLOW}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_SLOW}")" 1
}

# ---------------------------------------------------------------------------
# Category B: Input edge cases
# ---------------------------------------------------------------------------

run_B1() {
    log "B1: claude_research — minimal single-word goal"
    local id="B1" chat="oc_e2e_teams_B1_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_research goal=\"Kubernetes\"" \
        "${TIMEOUT_FAST}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")" 1
}

run_B2() {
    log "B2: claude_research — long multi-constraint goal"
    local id="B2" chat="oc_e2e_teams_B2_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_research goal=\"Compare gRPC vs REST vs GraphQL for inter-service communication: latency under high concurrency, schema evolution, streaming support, tooling in Go, and team onboarding cost for a 20-engineer org\"" \
        "${TIMEOUT_FAST}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")" 1
}

run_B3() {
    log "B3: claude_research — goal with code syntax"
    local id="B3" chat="oc_e2e_teams_B3_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_research goal=\"Go select{} with default case vs time.After() for timeout: when each is correct and when it causes goroutine leaks\"" \
        "${TIMEOUT_FAST}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")" 1
}

# ---------------------------------------------------------------------------
# Category C: Error handling
# ---------------------------------------------------------------------------

run_C1() {
    log "C1: non-existent template → graceful error"
    local id="C1" chat="oc_e2e_teams_C1_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=nonexistent_template goal=\"test\"" \
        "${TIMEOUT_FAST}")"
    eval_error "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")"
}

run_C2() {
    log "C2: missing goal parameter → graceful error"
    local id="C2" chat="oc_e2e_teams_C2_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_research" \
        "${TIMEOUT_FAST}")"
    eval_error "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")"
}

run_C3() {
    log "C3: template=list → lists claude templates"
    local id="C3" chat="oc_e2e_teams_C3_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=list" \
        "${TIMEOUT_FAST}")"
    local result
    result="$(inject "${id}" "${p}" "${TIMEOUT_FAST}")"
    if [[ "${result}" == OK:* ]]; then
        local content; content="$(resp_content "${id}")"
        if echo "${content}" | grep -qiE "claude_research|claude_analysis|claude_debate"; then
            record "${id}" PASS "listed new claude templates"
        else
            record "${id}" PARTIAL "replied but claude template names not in content"
        fi
    else
        eval_error "${id}" "${result}"
    fi
}

# ---------------------------------------------------------------------------
# Category D: Context inheritance (content-level verification)
# ---------------------------------------------------------------------------

run_D1() {
    log "D1: claude_analysis — synthesizer merges both analyst perspectives"
    local id="D1" chat="oc_e2e_teams_D1_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_analysis goal=\"Saga vs 2PC for distributed transactions in Go microservices: choose the better default for a new greenfield service\"" \
        "${TIMEOUT_SLOW}")"
    local result
    result="$(inject "${id}" "${p}" "${TIMEOUT_SLOW}")"
    if [[ "${result}" == OK:* ]]; then
        local n ms content
        n="$(echo "${result}" | cut -d: -f2)"
        ms="$(echo "${result}" | cut -d: -f3)"
        content="$(resp_content "${id}")"
        # Synthesizer prompt asks for "points of agreement", "key tensions", "final verdict"
        if echo "${content}" | grep -qiE "agreement|tension|verdict|both|synthesiz|recommend"; then
            record "${id}" PASS "${n} replies (${ms}ms) — synthesis content confirmed"
        elif [[ ${n} -ge 1 ]]; then
            record "${id}" PARTIAL "${n} replies but synthesis keywords not detected"
        else
            record "${id}" FAIL "0 replies"
        fi
    else
        eval_ok "${id}" "${result}" 1
    fi
}

run_D2() {
    log "D2: claude_debate — reviewer sees analyst + challenger via InheritContext"
    local id="D2" chat="oc_e2e_teams_D2_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        "@alex /run_tasks template=claude_debate goal=\"Clean Architecture vs simple layered approach for a new team: pick one and defend it\"" \
        "${TIMEOUT_SLOW}")"
    local result
    result="$(inject "${id}" "${p}" "${TIMEOUT_SLOW}")"
    if [[ "${result}" == OK:* ]]; then
        local n ms content
        n="$(echo "${result}" | cut -d: -f2)"
        ms="$(echo "${result}" | cut -d: -f3)"
        content="$(resp_content "${id}")"
        # Reviewer prompt asks about "challenger", "confidence level", "verdict"
        if echo "${content}" | grep -qiE "challenger|confidence|verdict|debate|critique|exposed"; then
            record "${id}" PASS "${n} replies (${ms}ms) — debate verdict content confirmed"
        elif [[ ${n} -ge 1 ]]; then
            record "${id}" PARTIAL "${n} replies but debate keywords not detected"
        else
            record "${id}" FAIL "0 replies"
        fi
    else
        eval_ok "${id}" "${result}" 1
    fi
}

# ---------------------------------------------------------------------------
# Category E: Prompt override
# ---------------------------------------------------------------------------

run_E1() {
    log "E1: claude_research — custom role prompt via prompts= override"
    local id="E1" chat="oc_e2e_teams_E1_$(date +%s)"
    local p; p="$(make_payload "${chat}" \
        '@alex /run_tasks template=claude_research goal="Python asyncio event loop internals" prompts={"researcher":"Explain in exactly 3 bullet points, each under 20 words. Topic: {GOAL}"}' \
        "${TIMEOUT_FAST}")"
    eval_ok "${id}" "$(inject "${id}" "${p}" "${TIMEOUT_FAST}")" 1
}

# ---------------------------------------------------------------------------
# Execution
# ---------------------------------------------------------------------------

run_category() {
    local cat="$1"
    case "${cat}" in
        A)
            log "=== Category A: Core templates ==="
            should_run A1 && { run_A1; cooldown; }
            should_run A2 && { run_A2; cooldown; }
            should_run A3 && run_A3
            ;;
        B)
            log "=== Category B: Input edge cases ==="
            should_run B1 && { run_B1; cooldown; }
            should_run B2 && { run_B2; cooldown; }
            should_run B3 && run_B3
            ;;
        C)
            log "=== Category C: Error handling ==="
            should_run C1 && { run_C1; cooldown; }
            should_run C2 && { run_C2; cooldown; }
            should_run C3 && run_C3
            ;;
        D)
            log "=== Category D: Context inheritance ==="
            should_run D1 && { run_D1; cooldown; }
            should_run D2 && run_D2
            ;;
        E)
            log "=== Category E: Prompt override ==="
            should_run E1 && run_E1
            ;;
    esac
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

if [[ ${DRY_RUN} -eq 1 ]]; then
    cat <<'EOF'
Agent Teams E2E — Case List (claude-only)

Category A: Core templates
  A1  claude_research     single-stage research
  A2  claude_analysis     parallel dual-perspective + synthesis
  A3  claude_debate       analyst + auto-challenger + reviewer (debate_mode)

Category B: Input edge cases
  B1  claude_research     minimal single-word goal
  B2  claude_research     long multi-constraint goal
  B3  claude_research     goal with code syntax

Category C: Error handling
  C1  nonexistent         graceful template-not-found error
  C2  missing goal        graceful missing-param error
  C3  template=list       lists available claude templates

Category D: Context inheritance (content-level)
  D1  claude_analysis     synthesizer merges both analyst perspectives
  D2  claude_debate       reviewer sees analyst + challenger (InheritContext)

Category E: Prompt override
  E1  claude_research     custom role prompt via prompts= param
EOF
    exit 0
fi

log "Agent Teams E2E — Claude-only local suite"
log "Inject URL : ${INJECT_URL}"
log "Temp dir   : ${TMPDIR_BASE}"
echo ""

# Connectivity check
if ! curl -sS --max-time 5 -o /dev/null -X POST "${INJECT_URL}" \
    -H "Content-Type: application/json" \
    -d '{"text":"ping","chat_id":"oc_healthcheck"}' 2>/dev/null; then
    log "⚠ Server may not be reachable — proceeding anyway"
fi

if [[ -n "${FILTER_CASE}" ]]; then
    run_category "${FILTER_CASE:0:1}"
elif [[ -n "${FILTER_CATEGORY}" ]]; then
    run_category "${FILTER_CATEGORY}"
else
    run_category C   # fast error cases first
    run_category A
    cooldown
    run_category B
    cooldown
    run_category D
    run_category E
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "================================================================"
echo "  AGENT TEAMS E2E — RESULTS (claude-only)"
echo "================================================================"
for i in "${!RESULT_IDS[@]}"; do
    st="${RESULT_STATUSES[$i]}"
    case "${st}" in
        PASS)    icon="✅" ;;
        PARTIAL) icon="⚠️ " ;;
        FAIL)    icon="❌" ;;
        *)       icon="⏭️ " ;;
    esac
    printf "  %s  %-6s  %-8s  %s\n" "${icon}" "${RESULT_IDS[$i]}" "${st}" "${RESULT_NOTES[$i]}"
done
echo "────────────────────────────────────────────────────────────────"
printf "  Total: %d | ✅ %d | ⚠️  %d | ❌ %d\n" "${TOTAL}" "${PASS}" "${PARTIAL}" "${FAIL}"
echo "  Response files: ${TMPDIR_BASE}/"
echo "  Debug:          jq . ${TMPDIR_BASE}/A2_resp.json"
echo "────────────────────────────────────────────────────────────────"

if [[ ${FAIL} -gt 0 ]]; then
    echo "  ❌ ${FAIL} case(s) failed"; exit 1
elif [[ ${PARTIAL} -gt 0 ]]; then
    echo "  ⚠️  ${PARTIAL} case(s) partial"; exit 0
else
    echo "  ✅ ALL PASSED"; exit 0
fi
