Summary: Adding memory.Service to agent.ServiceBundle created a Go import cycle through memory/rag/utils/id back into agent.
Remediation: Keep memory out of agent/ports and inject memory service at the react engine layer.
