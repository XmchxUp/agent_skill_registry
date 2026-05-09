# Agent Skill Registry MVP

Agent Skill Registry is a runnable MVP for the ADP Skill supply-chain loop described in `docs/agent-skill-registry-design.md`.

It is intentionally small and dependency-free so it can run in an offline development environment. The implementation is not a production registry yet; it is a vertical slice that validates the core workflow.

## What Works

- FDE-oriented Workbench page at `/`
- Create Skill Drafts
- Run smoke Skill Evaluation
- Publish immutable Published Skills
- Generate signed OCI-like Skill Artifacts on local disk
- Generate Skill Lockfiles for Agent Profiles
- Export and import Offline Deployment Bundles
- Simulate Local Skill Registry import
- Simulate runtime signature verification and Skill invocation
- Record Skill Invocation Traces
- Revoke Skill digests and block future invocation
- Enforce MVP policy checks:
  - no secret values in Skill packages
  - no wildcard network egress
  - evaluation must pass before publication

## Run

```bash
GOCACHE=/tmp/agent-skill-registry-go-build go run ./cmd/agent-skill-registry -addr :18080 -data ./data
```

Open:

```text
http://localhost:18080
```

The service seeds one demo Published Skill on first start.

## Test

```bash
GOCACHE=/tmp/agent-skill-registry-go-build go test ./...
```

`GOCACHE` is set to `/tmp` because some sandboxed environments have a read-only home cache directory.

## API Examples

Create a draft:

```bash
curl -s -X POST http://localhost:18080/api/drafts \
  -H 'content-type: application/json' \
  -d '{
    "namespace": "finance",
    "name": "invoice-normalizer",
    "version": "0.1.0",
    "description": "Normalize invoice text.",
    "visibility": "project-private",
    "source": "workbench",
    "created_by": "fde",
    "runtime_payload": {
      "mode": "in_process",
      "interface": "adp.skill.runtime/v1",
      "kind": "template",
      "entrypoint": "runtime/template",
      "template": "Invoice {{invoice}} normalized"
    },
    "permission_manifest": {
      "data_domains": ["finance.invoice"],
      "network": {
        "egress": [
          {
            "name": "ocr",
            "target": "service:ocr.default.svc.cluster.local",
            "ports": [443]
          }
        ]
      },
      "filesystem": {
        "read": ["/mnt/input"],
        "write": ["/tmp/adp-skill"]
      },
      "secrets": [
        {
          "name": "ocr-token",
          "required": true
        }
      ],
      "kubernetes": {
        "api_access": false
      }
    },
    "evaluation": {
      "cases": [
        {
          "name": "smoke",
          "input": { "invoice": "INV-1" },
          "expected_contains": ["INV-1", "normalized"]
        }
      ]
    }
  }'
```

Evaluate and publish:

```bash
curl -s -X POST http://localhost:18080/api/drafts/finance/invoice-normalizer:0.1.0/evaluate
curl -s -X POST http://localhost:18080/api/drafts/finance/invoice-normalizer:0.1.0/publish
```

Resolve an Agent Profile lockfile:

```bash
curl -s -X POST http://localhost:18080/api/agent-profiles/resolve \
  -H 'content-type: application/json' \
  -d '{
    "id": "finance/invoice-agent",
    "version": "1.0.0",
    "skills": [
      {
        "id": "finance/invoice-normalizer",
        "version": "0.1.0"
      }
    ]
  }'
```

Invoke a Skill:

```bash
curl -s -X POST http://localhost:18080/api/runtime/invoke \
  -H 'content-type: application/json' \
  -d '{
    "agent_profile_id": "finance/invoice-agent",
    "skill_id": "finance/invoice-normalizer",
    "version": "0.1.0",
    "input": {
      "invoice": "INV-2"
    }
  }'
```

## Current MVP Boundaries

- Artifact storage is local disk, not a real OCI registry.
- Signatures use HMAC for local demonstration, not Sigstore/cosign.
- Runtime execution supports template and echo payloads only.
- Controller and K3S integration are simulated inside the service.
- Policy evaluation is built-in Go logic, not OPA/Rego yet.
- Workbench is functional but minimal.

