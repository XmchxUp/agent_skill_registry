package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPMVPFlow(t *testing.T) {
	service := newTestService(t)
	mux := http.NewServeMux()
	RegisterHandlers(mux, service)

	home := httptest.NewRecorder()
	mux.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	if home.Code != http.StatusOK {
		t.Fatalf("home status = %d", home.Code)
	}
	if !strings.Contains(home.Body.String(), "Agent Skill Registry MVP") {
		t.Fatalf("home page did not render title")
	}
	if !strings.Contains(home.Body.String(), "showAndRefresh(await api(\"/api/drafts\"") {
		t.Fatalf("home page should auto-refresh after creating a draft")
	}

	draft := postJSON[SkillDraft](t, mux, "/api/drafts", CreateDraftRequest{
		Namespace:   "finance",
		Name:        "http-skill",
		Version:     "0.1.0",
		Description: "HTTP flow skill.",
		Visibility:  "project-private",
		Source:      "httptest",
		CreatedBy:   "fde",
		RuntimePayload: RuntimePayload{
			Mode:      "in_process",
			Interface: "adp.skill.runtime/v1",
			Kind:      "template",
			Template:  "HTTP {{value}}",
		},
		PermissionManifest: PermissionManifest{
			DataDomains: []string{"finance.invoice"},
			Network: NetworkPermission{
				Egress: []NetworkEgress{{Name: "svc", Target: "service:demo.default.svc.cluster.local", Ports: []int{443}}},
			},
			Filesystem: FilePermission{Read: []string{"/mnt/input"}, Write: []string{"/tmp/adp-skill"}},
			Secrets:    []SecretReference{{Name: "demo-token", Required: true}},
			Kubernetes: KubernetesAccess{APIAccess: false},
		},
		Evaluation: SkillEvaluation{
			Cases: []EvaluationCase{{Name: "http smoke", Input: map[string]string{"value": "OK"}, ExpectedContains: []string{"HTTP OK"}}},
		},
	})
	if draft.ID != "finance/http-skill:0.1.0" {
		t.Fatalf("unexpected draft id: %s", draft.ID)
	}

	eval := postJSON[EvaluationResult](t, mux, "/api/drafts/finance/http-skill:0.1.0/evaluate", nil)
	if !eval.Passed {
		t.Fatalf("evaluation failed: %+v", eval)
	}
	published := postJSON[PublishedSkill](t, mux, "/api/drafts/finance/http-skill:0.1.0/publish", nil)
	if published.Digest == "" {
		t.Fatalf("published skill missing digest")
	}
	lock := postJSON[SkillLockfile](t, mux, "/api/agent-profiles/resolve", AgentProfileRequest{
		ID:      "finance/http-agent",
		Version: "1.0.0",
		Skills:  []AgentProfileSkillRef{{ID: "finance/http-skill", Version: "0.1.0"}},
	})
	if len(lock.Skills) != 1 {
		t.Fatalf("unexpected lockfile: %+v", lock)
	}
	mount := postJSON[ControllerMountPlan](t, mux, "/api/controller/mount", ControllerMountRequest{Lockfile: lock})
	if len(mount.Skills) != 1 || mount.Skills[0].MountPath == "" {
		t.Fatalf("unexpected mount plan: %+v", mount)
	}
	bundle := postJSON[OfflineBundle](t, mux, "/api/offline-bundles/export", AgentProfileRequest{
		ID:      "finance/http-agent",
		Version: "1.0.0",
		Skills:  []AgentProfileSkillRef{{ID: "finance/http-skill", Version: "0.1.0"}},
	})
	if bundle.Signature == "" {
		t.Fatalf("bundle missing signature")
	}
	imported := postJSON[ImportedBundle](t, mux, "/api/offline-bundles/import", bundle)
	if imported.SkillCount != 1 {
		t.Fatalf("unexpected import result: %+v", imported)
	}
	invoked := postJSON[InvokeResponse](t, mux, "/api/runtime/invoke", InvokeRequest{
		AgentProfileID: "finance/http-agent",
		SkillID:        "finance/http-skill",
		Version:        "0.1.0",
		Input:          map[string]string{"value": "OK"},
	})
	if invoked.Output != "HTTP OK" {
		t.Fatalf("unexpected invoke output: %q", invoked.Output)
	}
	revocation := postJSON[Revocation](t, mux, "/api/revocations", map[string]string{
		"target_digest": published.Digest,
		"reason":        "httptest revoke",
	})
	if revocation.TargetDigest != published.Digest {
		t.Fatalf("unexpected revocation: %+v", revocation)
	}
	revocations := postJSON[SignedRevocationList](t, mux, "/api/revocations/export", nil)
	if len(revocations.Revocations) != 1 || revocations.Signature == "" {
		t.Fatalf("unexpected revocation list: %+v", revocations)
	}
	importedRevocations := postJSON[ImportedRevocationList](t, mux, "/api/revocations/import", revocations)
	if importedRevocations.RevocationCount != 1 {
		t.Fatalf("unexpected revocation import: %+v", importedRevocations)
	}
	community := postJSON[CommunityIngestion](t, mux, "/api/community/ingestions", CommunityIngestRequest{
		SourceURL:     "oci://community.example/http-derived",
		SourceVersion: "0.1.0",
		SourceDigest:  "sha256:http-derived",
		License:       "Apache-2.0",
		Scan:          CommunityScanResult{Status: "pass"},
		Skill: CreateDraftRequest{
			Namespace:   "community",
			Name:        "http-derived",
			Version:     "0.1.0",
			Description: "HTTP community flow skill.",
			RuntimePayload: RuntimePayload{
				Mode:      "in_process",
				Interface: "adp.skill.runtime/v1",
				Kind:      "template",
				Template:  "Derived {{value}} normalized",
			},
			PermissionManifest: PermissionManifest{
				DataDomains: []string{"community.demo"},
				Network: NetworkPermission{
					Egress: []NetworkEgress{{Name: "svc", Target: "service:demo.default.svc.cluster.local", Ports: []int{443}}},
				},
				Filesystem: FilePermission{Read: []string{"/mnt/input"}, Write: []string{"/tmp/adp-skill"}},
				Kubernetes: KubernetesAccess{APIAccess: false},
			},
			Evaluation: SkillEvaluation{
				Cases: []EvaluationCase{{Name: "community smoke", Input: map[string]string{"value": "OK"}, ExpectedContains: []string{"Derived", "normalized"}}},
			},
		},
	})
	if community.Status != ingestionStatusPublished || community.PublishedSkillID == "" {
		t.Fatalf("unexpected community ingestion: %+v", community)
	}
	runtimeReq := CreateDraftRequest{
		Namespace:   "local",
		Name:        "runtime-http",
		Version:     "0.1.0",
		Description: "Runtime-created HTTP draft.",
		RuntimePayload: RuntimePayload{
			Mode:      "in_process",
			Interface: "adp.skill.runtime/v1",
			Kind:      "template",
			Template:  "Runtime {{value}}",
		},
		PermissionManifest: PermissionManifest{
			DataDomains:   []string{"runtime.demo"},
			DraftCreation: DraftCreation{Allowed: true},
		},
		Evaluation: SkillEvaluation{
			Cases: []EvaluationCase{{Name: "runtime smoke", Input: map[string]string{"value": "OK"}, ExpectedContains: []string{"Runtime OK"}}},
		},
	}
	runtimeDraft := postJSON[SkillDraft](t, mux, "/api/runtime/drafts", runtimeReq)
	if runtimeDraft.CreatedBy != "agent-runtime" || runtimeDraft.Status != draftStatusDraft {
		t.Fatalf("unexpected runtime draft: %+v", runtimeDraft)
	}
}

func postJSON[T any](t *testing.T, mux http.Handler, path string, body any) T {
	t.Helper()
	var payload []byte
	var err error
	if body == nil {
		payload = []byte("{}")
	} else {
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
	}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("content-type", "application/json")
	mux.ServeHTTP(recorder, req)
	if recorder.Code < 200 || recorder.Code >= 300 {
		t.Fatalf("POST %s status=%d body=%s", path, recorder.Code, recorder.Body.String())
	}
	var out T
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v\nbody=%s", path, err, recorder.Body.String())
	}
	return out
}
