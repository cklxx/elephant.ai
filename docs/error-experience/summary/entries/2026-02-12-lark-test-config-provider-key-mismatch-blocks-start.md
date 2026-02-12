Summary: `scripts/lark/test.sh start` failed repeatedly because `~/.alex/test.yaml` had provider/base_url/api_key mismatches (`codex`/`anthropic` with `sk-kimi-*`), preventing dual-agent startup validation.
Remediation: enforce provider-family alignment in test config and add startup precheck for actionable failure hints.
