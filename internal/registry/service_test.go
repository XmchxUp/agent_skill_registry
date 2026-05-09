package registry

import (
	"testing"
)

func TestMVPFlow(t *testing.T) {
	source := newTestService(t)
	req := testDraftRequest("invoice-normalizer", "Invoice {{invoice}} normalized")
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
		ID:      "finance/invoice-agent",
		Version: "1.0.0",
		Skills:  []AgentProfileSkillRef{{ID: "finance/invoice-normalizer", Version: "0.1.0"}},
	}
	lock, err := source.ResolveLockfile(profile)
	if err != nil {
		t.Fatalf("resolve lockfile: %v", err)
	}
	if len(lock.Skills) != 1 || lock.Skills[0].Digest != published.Digest {
		t.Fatalf("unexpected lockfile: %+v", lock)
	}
	plan, err := source.PrepareControllerMount(ControllerMountRequest{Lockfile: *lock})
	if err != nil {
		t.Fatalf("prepare controller mount: %v", err)
	}
	if len(plan.Skills) != 1 || !plan.Skills[0].ReadOnly || !plan.Skills[0].HotLoadReady {
		t.Fatalf("unexpected mount plan: %+v", plan)
	}
	if len(plan.Skills[0].PermissionResources.NetworkPolicies) != 1 {
		t.Fatalf("mount plan did not map network policy: %+v", plan.Skills[0].PermissionResources)
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
		SkillID:        "finance/invoice-normalizer",
		Version:        "0.1.0",
		Input:          map[string]string{"invoice": "INV-2"},
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
		SkillID:        "finance/invoice-normalizer",
		Version:        "0.1.0",
		Input:          map[string]string{"invoice": "INV-3"},
	}); err == nil {
		t.Fatalf("expected revoked skill invocation to fail")
	}
}

func TestCommunityIngestionPublishesResignedSkill(t *testing.T) {
	service := newTestService(t)
	ingestion, err := service.IngestCommunitySkill(CommunityIngestRequest{
		SourceURL:     "oci://community.example/skills/invoice-reader",
		SourceVersion: "2.0.0",
		SourceDigest:  "sha256:community-invoice-reader",
		License:       "Apache-2.0",
		Scan:          CommunityScanResult{Status: "pass"},
		Skill:         testDraftRequest("community-invoice-reader", "Community {{invoice}} normalized"),
	})
	if err != nil {
		t.Fatalf("ingest community skill: %v", err)
	}
	if ingestion.Status != ingestionStatusPublished || ingestion.PublishedSkillID == "" {
		t.Fatalf("unexpected ingestion: %+v", ingestion)
	}
	published, err := service.GetPublished(ingestion.PublishedSkillID)
	if err != nil {
		t.Fatalf("get ingested skill: %v", err)
	}
	if published.SignatureScope != "z-global" || published.Source == "" {
		t.Fatalf("community skill was not re-signed as global Published Skill: %+v", published)
	}
	if published.Provenance.Source != published.Source {
		t.Fatalf("provenance did not preserve normalized community source: %+v", published.Provenance)
	}
}

func TestCommunityIngestionRejectsFailedScan(t *testing.T) {
	service := newTestService(t)
	_, err := service.IngestCommunitySkill(CommunityIngestRequest{
		SourceURL:     "oci://community.example/skills/bad",
		SourceVersion: "1.0.0",
		SourceDigest:  "sha256:bad",
		License:       "Apache-2.0",
		Scan:          CommunityScanResult{Status: "fail", CriticalVulnerabilities: 1},
		Skill:         testDraftRequest("bad-community", "Bad {{invoice}}"),
	})
	if err == nil {
		t.Fatalf("expected failed community scan to be rejected")
	}
}

func TestRuntimeDraftRequiresExplicitPermissionAndCanPublishLocal(t *testing.T) {
	service := newTestService(t)
	req := testDraftRequest("runtime-created", "Runtime {{invoice}} normalized")
	if _, err := service.CreateRuntimeDraft(req); err == nil {
		t.Fatalf("expected runtime draft without permission to fail")
	}
	req.PermissionManifest.DraftCreation.Allowed = true
	draft, err := service.CreateRuntimeDraft(req)
	if err != nil {
		t.Fatalf("create runtime draft: %v", err)
	}
	if draft.Status != draftStatusDraft || draft.CreatedBy != "agent-runtime" || draft.Source != "runtime-agent" {
		t.Fatalf("unexpected runtime draft: %+v", draft)
	}
	if _, err := service.PublishDraft(draft.ID, true); err == nil {
		t.Fatalf("expected unevaluated runtime draft publication to fail")
	}
	if _, err := service.EvaluateDraft(draft.ID); err != nil {
		t.Fatalf("evaluate runtime draft: %v", err)
	}
	local, err := service.PublishDraft(draft.ID, true)
	if err != nil {
		t.Fatalf("publish runtime draft locally: %v", err)
	}
	if !local.Local || local.SignatureScope != "customer-local" {
		t.Fatalf("unexpected Local Published Skill: %+v", local)
	}
}

func TestRevocationListImportBlocksCustomerMountAndInvoke(t *testing.T) {
	source := newTestService(t)
	draft, err := source.CreateDraft(testDraftRequest("revoked-skill", "Invoice {{invoice}} normalized"))
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := source.EvaluateDraft(draft.ID); err != nil {
		t.Fatalf("evaluate draft: %v", err)
	}
	published, err := source.PublishDraft(draft.ID, false)
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	profile := AgentProfileRequest{
		ID:      "finance/revoked-agent",
		Version: "1.0.0",
		Skills:  []AgentProfileSkillRef{{ID: "finance/revoked-skill", Version: "0.1.0"}},
	}
	bundle, err := source.ExportBundle(profile)
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	if _, err := source.Revoke(published.Digest, "emergency revocation"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revocations, err := source.ExportRevocationList()
	if err != nil {
		t.Fatalf("export revocation list: %v", err)
	}

	customer := newTestService(t)
	if _, err := customer.ImportBundle(*bundle); err != nil {
		t.Fatalf("import bundle: %v", err)
	}
	if _, err := customer.ImportRevocationList(*revocations); err != nil {
		t.Fatalf("import revocation list: %v", err)
	}
	if _, err := customer.PrepareControllerMount(ControllerMountRequest{AgentProfile: profile}); err == nil {
		t.Fatalf("expected revoked skill mount to fail")
	}
	if _, err := customer.Invoke(InvokeRequest{
		AgentProfileID: "finance/revoked-agent",
		SkillID:        "finance/revoked-skill",
		Version:        "0.1.0",
		Input:          map[string]string{"invoice": "INV-9"},
	}); err == nil {
		t.Fatalf("expected revoked skill invocation to fail")
	}
}

func TestPolicyRejectsSecretValues(t *testing.T) {
	service := newTestService(t)
	_, err := service.CreateDraft(CreateDraftRequest{
		Namespace:      "finance",
		Name:           "bad-secret",
		Version:        "0.1.0",
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
		Namespace:      "finance",
		Name:           "bad-egress",
		Version:        "0.1.0",
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

func testDraftRequest(name, template string) CreateDraftRequest {
	return CreateDraftRequest{
		Namespace:   "finance",
		Name:        name,
		Version:     "0.1.0",
		Description: "Normalize invoice text.",
		Visibility:  "project-private",
		Source:      "test",
		CreatedBy:   "fde",
		RuntimePayload: RuntimePayload{
			Mode:      "in_process",
			Interface: "adp.skill.runtime/v1",
			Kind:      "template",
			Template:  template,
		},
		PermissionManifest: PermissionManifest{
			DataDomains: []string{"finance.invoice"},
			Network: NetworkPermission{
				Egress: []NetworkEgress{{Name: "ocr", Target: "service:ocr.default.svc.cluster.local", Ports: []int{443}}},
			},
			Filesystem: FilePermission{Read: []string{"/mnt/input"}, Write: []string{"/tmp/adp-skill"}},
			Secrets:    []SecretReference{{Name: "ocr-token", Required: true}},
			Kubernetes: KubernetesAccess{APIAccess: false},
		},
		Assets: SkillAssets{
			KnowledgeRefs: []KnowledgeAsset{{Name: "finance-policy", URI: "knowledge://finance", Version: "2026-05", Digest: "sha256:demo"}},
		},
		Evaluation: SkillEvaluation{
			Cases: []EvaluationCase{{Name: "smoke", Input: map[string]string{"invoice": "INV-1"}, ExpectedContains: []string{"INV-1", "normalized"}}},
		},
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
