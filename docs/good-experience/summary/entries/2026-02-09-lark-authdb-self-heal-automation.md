Summary: Introduced self-healing for local Lark/AuthDB flow by cleaning orphan `alex-server lark` processes and retrying auth DB setup on `too many clients already`.
Impact: Reduced manual recovery steps and stabilized local bootstrap when supervisor restart loops previously exhausted auth Postgres connections.
