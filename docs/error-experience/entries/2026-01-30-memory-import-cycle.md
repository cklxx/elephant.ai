Error: Adding memory.Service to agent.ServiceBundle introduced an import cycle (agent -> memory -> rag -> httpclient -> errors -> logging -> utils/id -> agent), causing go list/tests to fail.
Remediation: Remove memory dependency from agent/ports and pass memory service via react engine config so memory stays in app/domain layers.
