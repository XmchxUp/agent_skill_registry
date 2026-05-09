# Require declarative Skill permissions

Each Published Skill must include a Skill Permission Manifest that declares required data, network, filesystem, secret, model, tool, and cluster permissions. ADP validates this declaration before publishing, and the customer-environment Controller translates it into runtime controls such as NetworkPolicy, RBAC, volume mounts, secret projection, and sandbox settings, because production Skills must not rely on voluntary code behavior for security boundaries.
