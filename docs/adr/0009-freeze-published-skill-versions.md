# Freeze Published Skill versions

Published Skill versions are immutable and multiple versions of the same Skill may coexist. ADP never rewrites a published artifact and never uses a floating latest pointer for deployment, because reproducibility, rollback safety, and auditability matter more than runtime convenience in offline private deployments.
