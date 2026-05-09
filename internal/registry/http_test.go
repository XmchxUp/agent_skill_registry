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

	draft := postJSON[SkillDraft](t, mux, "/api/drafts", CreateDraftRequest{
		Namespace: "finance",
		Name: "http-skill",
		Version: "0.1.0",
		Description: "HTTP flow skill.",
		Visibility: "project-private",
		Source: "httptest",
		CreatedBy: "fde",
		RuntimePayload: RuntimePayload{
			Mode: "in_process",
			Interface: "adp.skill.runtime/v1",
			Kind: "template",
			Template: "HTTP {{value}}",
		},
		PermissionManifest: PermissionManifest{
			DataDomains: []string{"finance.invoice"},
			Network: NetworkPermission{
				Egress: []NetworkEgress{{Name: "svc", Target: "service:demo.default.svc.cluster.local", Ports: []int{443}}},
			},
			Filesystem: FilePermission{Read: []string{"/mnt/input"}, Write: []string{"/tmp/adp-skill"}},
			Secrets: []SecretReference{{Name: "demo-token", Required: true}},
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
		ID: "finance/http-agent",
		Version: "1.0.0",
		Skills: []AgentProfileSkillRef{{ID: "finance/http-skill", Version: "0.1.0"}},
	})
	if len(lock.Skills) != 1 {
		t.Fatalf("unexpected lockfile: %+v", lock)
	}
	bundle := postJSON[OfflineBundle](t, mux, "/api/offline-bundles/export", AgentProfileRequest{
		ID: "finance/http-agent",
		Version: "1.0.0",
		Skills: []AgentProfileSkillRef{{ID: "finance/http-skill", Version: "0.1.0"}},
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
		SkillID: "finance/http-skill",
		Version: "0.1.0",
		Input: map[string]string{"value": "OK"},
	})
	if invoked.Output != "HTTP OK" {
		t.Fatalf("unexpected invoke output: %q", invoked.Output)
	}
	revocation := postJSON[Revocation](t, mux, "/api/revocations", map[string]string{
		"target_digest": published.Digest,
		"reason": "httptest revoke",
	})
	if revocation.TargetDigest != published.Digest {
		t.Fatalf("unexpected revocation: %+v", revocation)
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

