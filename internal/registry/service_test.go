package registry

import (
	"testing"
)

func TestMVPFlow(t *testing.T) {
	source := newTestService(t)
	req := CreateDraftRequest{
		Namespace: "finance",
		Name: "invoice-normalizer",
		Version: "0.1.0",
		Description: "Normalize invoice text.",
		Visibility: "project-private",
		Source: "test",
		CreatedBy: "fde",
		RuntimePayload: RuntimePayload{
			Mode: "in_process",
			Interface: "adp.skill.runtime/v1",
			Kind: "template",
			Template: "Invoice {{invoice}} normalized",
		},
		PermissionManifest: PermissionManifest{
			DataDomains: []string{"finance.invoice"},
			Network: NetworkPermission{
				Egress: []NetworkEgress{{Name: "ocr", Target: "service:ocr.default.svc.cluster.local", Ports: []int{443}}},
			},
			Filesystem: FilePermission{Read: []string{"/mnt/input"}, Write: []string{"/tmp/adp-skill"}},
			Secrets: []SecretReference{{Name: "ocr-token", Required: true}},
			Kubernetes: KubernetesAccess{APIAccess: false},
		},
		Assets: SkillAssets{
			KnowledgeRefs: []KnowledgeAsset{{Name: "finance-policy", URI: "knowledge://finance", Version: "2026-05", Digest: "sha256:demo"}},
		},
		Evaluation: SkillEvaluation{
			Cases: []EvaluationCase{{Name: "smoke", Input: map[string]string{"invoice": "INV-1"}, ExpectedContains: []string{"INV-1", "normalized"}}},
		},
	}
	draft, err := source.CreateDraft(req)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	result, err := source.EvaluateDraft(draft.ID)
	if err != nil {
		t.Fatalf("evaluate draft: %v", err)
	}
	if !result.Passed {
		t.Fatalf("evaluation failed: %+v", result)
	}
	published, err := source.PublishDraft(draft.ID, false)
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if published.Digest == "" || published.Signature == "" {
		t.Fatalf("published skill missing digest/signature: %+v", published)
	}

	profile := AgentProfileRequest{
		ID: "finance/invoice-agent",
		Version: "1.0.0",
		Skills: []AgentProfileSkillRef{{ID: "finance/invoice-normalizer", Version: "0.1.0"}},
	}
	lock, err := source.ResolveLockfile(profile)
	if err != nil {
		t.Fatalf("resolve lockfile: %v", err)
	}
	if len(lock.Skills) != 1 || lock.Skills[0].Digest != published.Digest {
		t.Fatalf("unexpected lockfile: %+v", lock)
	}
	bundle, err := source.ExportBundle(profile)
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	if len(bundle.Skills) != 1 || bundle.Signature == "" {
		t.Fatalf("unexpected bundle: %+v", bundle)
	}

	customer := newTestService(t)
	imported, err := customer.ImportBundle(*bundle)
	if err != nil {
		t.Fatalf("import bundle: %v", err)
	}
	if imported.SkillCount != 1 {
		t.Fatalf("unexpected import count: %+v", imported)
	}
	invoked, err := customer.Invoke(InvokeRequest{
		AgentProfileID: "finance/invoice-agent",
		SkillID: "finance/invoice-normalizer",
		Version: "0.1.0",
		Input: map[string]string{"invoice": "INV-2"},
	})
	if err != nil {
		t.Fatalf("invoke imported skill: %v", err)
	}
	if invoked.Output != "Invoice INV-2 normalized" {
		t.Fatalf("unexpected output: %q", invoked.Output)
	}

	rev, err := customer.Revoke(published.Digest, "test revocation")
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if rev.TargetDigest != published.Digest {
		t.Fatalf("unexpected revocation: %+v", rev)
	}
	if _, err := customer.Invoke(InvokeRequest{
		AgentProfileID: "finance/invoice-agent",
		SkillID: "finance/invoice-normalizer",
		Version: "0.1.0",
		Input: map[string]string{"invoice": "INV-3"},
	}); err == nil {
		t.Fatalf("expected revoked skill invocation to fail")
	}
}

func TestPolicyRejectsSecretValues(t *testing.T) {
	service := newTestService(t)
	_, err := service.CreateDraft(CreateDraftRequest{
		Namespace: "finance",
		Name: "bad-secret",
		Version: "0.1.0",
		RuntimePayload: RuntimePayload{Template: "hello {{text}}"},
		PermissionManifest: PermissionManifest{
			Secrets: []SecretReference{{Name: "token", Value: "plaintext-secret"}},
		},
	})
	if err == nil {
		t.Fatalf("expected secret value to be rejected")
	}
}

func TestPolicyRejectsWildcardEgress(t *testing.T) {
	service := newTestService(t)
	draft, err := service.CreateDraft(CreateDraftRequest{
		Namespace: "finance",
		Name: "bad-egress",
		Version: "0.1.0",
		RuntimePayload: RuntimePayload{Template: "hello {{text}}"},
		PermissionManifest: PermissionManifest{
			Network: NetworkPermission{Egress: []NetworkEgress{{Name: "all", Target: "*", Ports: []int{443}}}},
		},
		Evaluation: SkillEvaluation{
			Cases: []EvaluationCase{{Name: "smoke", Input: map[string]string{"text": "world"}, ExpectedContains: []string{"world"}}},
		},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := service.EvaluateDraft(draft.ID); err != nil {
		t.Fatalf("evaluate draft: %v", err)
	}
	if _, err := service.PublishDraft(draft.ID, false); err == nil {
		t.Fatalf("expected wildcard egress publication to fail")
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService(t.TempDir(), "test-signing-key")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service
}

