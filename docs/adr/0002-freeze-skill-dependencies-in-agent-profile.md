# Freeze Skill dependencies in Agent Profile

ADP publishes each Agent Profile with a Skill Lockfile that freezes direct and transitive Skill versions, digests, signature chains, and SBOM references. Customer-environment Controllers follow the lockfile exactly and do not resolve version ranges locally, trading runtime flexibility for offline reproducibility, auditability, and supply-chain integrity.
