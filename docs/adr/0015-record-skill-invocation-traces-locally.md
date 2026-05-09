# Record Skill Invocation Traces locally

Each Skill call produces a Skill Invocation Trace that records the Agent Profile, Skill identity, version, digest, permission context, latency, outcome, and redacted input/output summaries. Traces are stored in the customer environment by default and exported back to ADP only through customer-approved redacted diagnostic bundles, because runtime observability must not weaken the data isolation guarantees of offline deployments.
