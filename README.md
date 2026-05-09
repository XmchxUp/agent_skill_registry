# Agent Skill Registry MVP

Agent Skill Registry is a runnable MVP for the ADP Skill supply-chain loop described in `docs/agent-skill-registry-design.md`.

It is intentionally small and dependency-free so it can run in an offline development environment. The implementation is not a production registry yet; it is a vertical slice that validates the core workflow.

## What Works

- FDE-oriented Workbench page at `/`
- Search/filter Published Skills by namespace, source, visibility, runtime mode, and text query
- Create Skill Drafts
- Run smoke Skill Evaluation
- Publish immutable Published Skills
- Ingest a pinned Community Skill source, reject failed scans/licenses, and re-sign it as a Z-scoped Published Skill
- Generate signed OCI-like Skill Artifacts on local disk
- Generate Skill Lockfiles for Agent Profiles
- Export and import Offline Deployment Bundles
- Simulate Local Skill Registry import
- Simulate Controller Lockfile verification, read-only Skill mounting, and Kubernetes permission resource mapping
- Simulate runtime signature verification and Skill invocation
- Let a running Agent create a gated runtime Skill Draft that must still pass evaluation before local publication
- Record Skill Invocation Traces
- Revoke Skill digests and block future invocation
- Export and import signed offline Revocation Lists
- Enforce MVP policy checks:
  - no secret values in Skill packages
  - no wildcard network egress
  - no failed Community Skill scan
  - no unknown/unlicensed Community Skill license
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

Ingest a pinned Community Skill:

```bash
curl -s -X POST http://localhost:18080/api/community/ingestions \
  -H 'content-type: application/json' \
  -d '{
    "source_url": "oci://community.example/skills/invoice-reader",
    "source_version": "2.0.0",
    "source_digest": "sha256:community-invoice-reader",
    "license": "Apache-2.0",
    "scan": {
      "status": "pass",
      "critical_vulnerabilities": 0
    },
    "skill": {
      "namespace": "community",
      "name": "invoice-reader",
      "version": "0.1.0",
      "description": "Community Skill normalized and re-signed by ADP.",
      "runtime_payload": {
        "mode": "in_process",
        "interface": "adp.skill.runtime/v1",
        "kind": "template",
        "entrypoint": "runtime/template",
        "template": "Community invoice {{invoice}} normalized"
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
            "required": false
          }
        ],
        "kubernetes": {
          "api_access": false
        }
      },
      "evaluation": {
        "cases": [
          {
            "name": "community smoke",
            "input": { "invoice": "INV-1" },
            "expected_contains": ["INV-1", "normalized"]
          }
        ]
      }
    }
  }'
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

Prepare a simulated customer Controller mount plan:

```bash
curl -s -X POST http://localhost:18080/api/controller/mount \
  -H 'content-type: application/json' \
  -d '{
    "agent_profile": {
      "id": "finance/invoice-agent",
      "version": "1.0.0",
      "skills": [
        {
          "id": "finance/invoice-normalizer",
          "version": "0.1.0"
        }
      ]
    }
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

Create a runtime Skill Draft from a running Agent:

```bash
curl -s -X POST http://localhost:18080/api/runtime/drafts \
  -H 'content-type: application/json' \
  -d '{
    "namespace": "local",
    "name": "runtime-observation",
    "version": "0.1.0",
    "description": "Draft created by a running Agent from local task experience.",
    "runtime_payload": {
      "mode": "in_process",
      "interface": "adp.skill.runtime/v1",
      "kind": "template",
      "entrypoint": "runtime/template",
      "template": "Runtime observation {{text}}"
    },
    "permission_manifest": {
      "data_domains": ["local.observation"],
      "draft_creation": {
        "allowed": true
      },
      "kubernetes": {
        "api_access": false
      }
    },
    "evaluation": {
      "cases": [
        {
          "name": "runtime draft smoke",
          "input": { "text": "OK" },
          "expected_contains": ["Runtime", "OK"]
        }
      ]
    }
  }'
```

Runtime-created drafts are not deployable directly. They must pass `/api/drafts/{id}/evaluate` and then use `/api/drafts/{id}/publish?local=true` to become Local Published Skills.

Export and import a signed offline Revocation List:

```bash
curl -s -X POST http://localhost:18080/api/revocations/export
curl -s -X POST http://localhost:18080/api/revocations/import \
  -H 'content-type: application/json' \
  -d @revocations.json
```

## Current MVP Boundaries

- Artifact storage is local disk, not a real OCI registry.
- Signatures use HMAC for local demonstration, not Sigstore/cosign.
- Runtime execution supports template and echo payloads only.
- Controller and K3S integration are modeled as a deterministic mount plan, not applied to a real cluster.
- Policy evaluation is built-in Go logic, not OPA/Rego yet.
- Community Skill ingestion uses supplied source/scan/license metadata; it does not pull from a real external registry.
- Permission mapping emits Kubernetes-shaped resources but does not create NetworkPolicy, ServiceAccount, RBAC, projected Secret, volume, or securityContext objects in K3S.
- Workbench is functional but minimal.
