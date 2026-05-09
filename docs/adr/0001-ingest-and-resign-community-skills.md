# Ingest and re-sign Community Skills

Community Skills are treated as external inputs, not trusted runtime artifacts. ADP must ingest a specific Community Skill version, lock its provenance, run policy checks, and re-sign it under Z company's trust root before it can become a Published Skill, because customer environments are offline private deployments and must not directly trust community registries or upstream signatures.
