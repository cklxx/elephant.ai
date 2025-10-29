# Sandbox Environment Reuse Research

## Background

The current sandbox integration provisions a fresh sandbox session whenever the backend boots. Initialization requires a health
probe (`Shell.ExecCommand`) to ensure the remote runtime is reachable. Operators have reported long waits during this handshake,
so we evaluated strategies that would allow the runtime to be reused instead of rebuilt for every backend restart.

## Observations

- The Go server keeps a single shared `SandboxManager` instance that owns the sandbox SDK clients. Once the manager finishes its
  health check, the in-memory client pool is ready for reuse.
- Initialization cost is dominated by the remote health probe. API client construction is cheap, but the remote echo command can
take multiple seconds when the sandbox provider cold-starts infrastructure.
- The manager exposes cached environment metadata through `SandboxManager.Environment`, which means downstream components can
tolerate reused sessions as long as credentials remain valid.

## Reuse Options

1. **Connection Warm Pool**
   - Persist the sandbox base URL and bearer token between restarts.
   - On shutdown, serialize the manager's environment snapshot to disk and hydrate it on boot to avoid repeated `printenv` calls.
   - Risk: cached credentials can expire; must validate with a lightweight probe before trusting the cached snapshot.

2. **Session Pinning via Provider API**
   - Many hosted sandboxes expose APIs to request a specific session ID. We can extend `SandboxManager` to accept a `SessionID`
     hint so the health probe resumes an existing container.
   - Requires provider support. We would need to upgrade the SDK (`github.com/agent-infra/sandbox-sdk-go`) to surface a
     `ResumeSession` call and pass the pinned ID via configuration.

3. **Stateful Sidecar**
   - Introduce a sidecar process that keeps the sandbox session alive independently of the backend server. The backend would
     communicate with the sidecar over a local socket; restarting the backend would not terminate the sandbox.
   - Complexity: additional deployment surface plus lifecycle coordination between the backend and the sidecar.

4. **Lazy Health Checks with Backoff**
   - Skip the blocking health probe during boot. Instead, queue tool invocations until the first sandbox call succeeds.
   - This reduces perceived startup time but defers failure detection. For operators this can still be acceptable if paired with
     progressive status updates in the UI.

## Recommendation

- Short Term: implement cached environment snapshots and reuse them on boot (Option 1). Pair with the new progress events to
  surface when a refresh probe is still running.
- Medium Term: explore session pinning once the sandbox provider exposes a resume API (Option 2). This would give deterministic
  reuse and minimize cold-start penalties.
- Avoid introducing a dedicated sidecar unless reuse requirements exceed what cached snapshots + pinning can provide.

## Next Steps

1. Extend configuration to optionally persist sandbox credentials and environment caches.
2. Prototype provider-specific session pinning to measure savings.
3. Add telemetry around sandbox initialization latency so we can quantify the impact of reuse strategies.
