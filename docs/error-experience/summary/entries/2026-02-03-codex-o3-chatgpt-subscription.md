Summary: Codex external-agent defaults used `model="o3"`, which Codex CLI rejects when authenticated via ChatGPT subscription, causing bg_dispatch failures.
Remediation: Switch defaults/examples to `gpt-5.2-codex` and add a regression test for the default model.
