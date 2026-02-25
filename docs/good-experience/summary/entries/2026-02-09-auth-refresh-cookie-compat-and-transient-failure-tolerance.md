Summary: Stabilized post-login refresh persistence with URL-safe cookie handling + legacy decode compatibility on backend, and auth-only session clearing on frontend refresh failures.
Impact: Reduced forced re-login on transient refresh errors and tightened group-chat model scope resolution to chat-level semantics.
