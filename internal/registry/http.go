package registry

import (
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"
)

func RegisterHandlers(mux *http.ServeMux, service *Service) {
	h := &handler{service: service}
	mux.HandleFunc("/", h.index)
	mux.HandleFunc("/api/state", h.state)
	mux.HandleFunc("/api/drafts", h.createDraft)
	mux.HandleFunc("/api/drafts/", h.draftAction)
	mux.HandleFunc("/api/skills", h.skills)
	mux.HandleFunc("/api/skills/", h.skill)
	mux.HandleFunc("/api/agent-profiles/resolve", h.resolve)
	mux.HandleFunc("/api/offline-bundles/export", h.exportBundle)
	mux.HandleFunc("/api/offline-bundles/import", h.importBundle)
	mux.HandleFunc("/api/runtime/invoke", h.invoke)
	mux.HandleFunc("/api/revocations", h.revoke)
}

type handler struct {
	service *Service
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	state := h.service.State()
	page.Execute(w, state)
}

func (h *handler) state(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.service.State())
}

func (h *handler) createDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req CreateDraftRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	draft, err := h.service.CreateDraft(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, draft)
}

func (h *handler) draftAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/drafts/")
	if strings.HasSuffix(rest, "/evaluate") {
		id := strings.TrimSuffix(rest, "/evaluate")
		result, err := h.service.EvaluateDraft(id)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if strings.HasSuffix(rest, "/publish") {
		id := strings.TrimSuffix(rest, "/publish")
		local := r.URL.Query().Get("local") == "true"
		published, err := h.service.PublishDraft(id, local)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, published)
		return
	}
	writeError(w, http.StatusNotFound, "unknown draft action")
}

func (h *handler) skills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, h.service.ListPublished())
}

func (h *handler) skill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/skills/")
	skill, err := h.service.GetPublished(id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (h *handler) resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req AgentProfileRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	lock, err := h.service.ResolveLockfile(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lock)
}

func (h *handler) exportBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req AgentProfileRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	bundle, err := h.service.ExportBundle(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bundle)
}

func (h *handler) importBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var bundle OfflineBundle
	if err := readJSON(r, &bundle); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	imported, err := h.service.ImportBundle(bundle)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, imported)
}

func (h *handler) invoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req InvokeRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := h.service.Invoke(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		TargetDigest string `json:"target_digest"`
		Reason       string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rev, err := h.service.Revoke(req.TargetDigest, req.Reason)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rev)
}

func readJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

var page = template.Must(template.New("index").Funcs(template.FuncMap{
	"count": func(v any) int {
		switch x := v.(type) {
		case map[string]*SkillDraft:
			return len(x)
		case map[string]*PublishedSkill:
			return len(x)
		case map[string]*Revocation:
			return len(x)
		case []SkillInvocationTrace:
			return len(x)
		default:
			return 0
		}
	},
	"short": func(value string) string {
		if len(value) > 18 {
			return value[:18] + "..."
		}
		return value
	},
}).Parse(indexHTML))

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Agent Skill Registry MVP</title>
  <style>
    :root {
      color-scheme: light;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f6f7f9;
      color: #17202a;
    }
    body { margin: 0; }
    header { background: #ffffff; border-bottom: 1px solid #d9dee7; padding: 18px 28px; }
    h1 { margin: 0; font-size: 22px; letter-spacing: 0; }
    main { padding: 24px 28px 40px; max-width: 1280px; margin: 0 auto; }
    .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-bottom: 20px; }
    .tools { display: grid; grid-template-columns: minmax(320px, 420px) 1fr; gap: 14px; align-items: start; }
    .metric { background: #fff; border: 1px solid #d9dee7; border-radius: 8px; padding: 14px; }
    .metric strong { display: block; font-size: 26px; margin-bottom: 4px; }
    section { background: #fff; border: 1px solid #d9dee7; border-radius: 8px; margin: 14px 0; overflow: hidden; }
    section h2 { margin: 0; padding: 14px 16px; font-size: 16px; border-bottom: 1px solid #e4e8ef; }
    form { padding: 14px 16px; display: grid; gap: 10px; }
    label { display: grid; gap: 5px; font-size: 13px; color: #3d4b5c; }
    input, textarea, select { box-sizing: border-box; width: 100%; border: 1px solid #cbd3df; border-radius: 6px; padding: 9px 10px; font: inherit; background: #fff; color: #17202a; }
    textarea { min-height: 86px; resize: vertical; }
    button { border: 0; border-radius: 6px; padding: 10px 12px; background: #145c9e; color: #fff; font-weight: 650; cursor: pointer; }
    button.secondary { background: #4d5d70; }
    button.danger { background: #a33b3b; }
    button:disabled { opacity: .55; cursor: default; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid #edf0f5; vertical-align: top; }
    th { color: #516070; font-weight: 600; background: #fafbfc; }
    code { background: #eef2f6; padding: 2px 5px; border-radius: 4px; }
    pre { margin: 0; padding: 14px 16px; background: #111827; color: #d5e2f2; min-height: 180px; overflow: auto; font-size: 12px; }
    .ok { color: #0f7b45; font-weight: 600; }
    .bad { color: #a33b3b; font-weight: 600; }
    .muted { color: #657384; }
    .empty { padding: 16px; color: #657384; }
    .actions { display: flex; flex-wrap: wrap; gap: 8px; }
    .row-actions { display: flex; flex-wrap: wrap; gap: 6px; }
    .row-actions button { padding: 7px 9px; font-size: 12px; }
    @media (max-width: 900px) { .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .tools { grid-template-columns: 1fr; } }
    @media (max-width: 560px) { main { padding: 16px; } .grid { grid-template-columns: 1fr; } th:nth-child(4), td:nth-child(4) { display: none; } }
  </style>
</head>
<body>
  <header>
    <h1>Agent Skill Registry MVP</h1>
    <div class="muted">Workbench, Registry, Lockfile, Offline Bundle, Local Runtime simulation</div>
  </header>
  <main>
    <div class="grid">
      <div class="metric"><strong>{{count .Drafts}}</strong><span>Drafts</span></div>
      <div class="metric"><strong>{{count .Published}}</strong><span>Published Skills</span></div>
      <div class="metric"><strong>{{count .Revocations}}</strong><span>Revocations</span></div>
      <div class="metric"><strong>{{count .Traces}}</strong><span>Invocation Traces</span></div>
    </div>

    <div class="tools">
      <section>
        <h2>Create Skill Draft</h2>
        <form id="draft-form">
          <label>Namespace <input name="namespace" value="finance" required></label>
          <label>Name <input name="name" value="invoice-normalizer" required></label>
          <label>Version <input name="version" value="0.1.0" required></label>
          <label>Description <input name="description" value="Normalize invoice text into a predictable response."></label>
          <label>Template <textarea name="template">Invoice [[invoice]] normalized by Agent Skill Registry</textarea></label>
          <label>Smoke input value <input name="smoke_input" value="INV-1001"></label>
          <label>Expected output contains <input name="expected" value="INV-1001"></label>
          <label>Network target <input name="network_target" value="service:ocr.default.svc.cluster.local"></label>
          <div class="actions">
            <button type="submit">Create Draft</button>
            <button class="secondary" type="button" id="refresh-state">Refresh</button>
          </div>
        </form>
      </section>

      <section>
        <h2>API Result</h2>
        <pre id="result">Use the Workbench controls to exercise the MVP flow.</pre>
      </section>
    </div>

    <section>
      <h2>Published Skills</h2>
      {{if .Published}}
      <table>
        <thead><tr><th>ID</th><th>Status</th><th>Runtime</th><th>Digest</th><th>Actions</th></tr></thead>
        <tbody>
          {{range .Published}}
          <tr>
            <td><code>{{.ID}}</code><br><span class="muted">{{.Description}}</span></td>
            <td class="ok">{{.Status}}</td>
            <td>{{.RuntimePayload.Mode}}<br><span class="muted">{{.RuntimePayload.Interface}}</span></td>
            <td><code>{{short .Digest}}</code><br><span class="muted">{{.SignatureScope}}</span></td>
            <td>
              <div class="row-actions">
                <button data-action="invoke" data-id="{{.ID}}">Invoke</button>
                <button class="secondary" data-action="lock" data-id="{{.ID}}">Lockfile</button>
                <button class="secondary" data-action="bundle" data-id="{{.ID}}">Export Bundle</button>
                <button class="danger" data-action="revoke" data-digest="{{.Digest}}">Revoke</button>
              </div>
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{else}}<div class="empty">No Published Skills.</div>{{end}}
    </section>

    <section>
      <h2>Drafts</h2>
      {{if .Drafts}}
      <table>
        <thead><tr><th>ID</th><th>Status</th><th>Source</th><th>Updated</th></tr></thead>
        <tbody>
          {{range .Drafts}}
          <tr>
            <td><code>{{.ID}}</code></td>
            <td>{{.Status}}</td>
            <td>{{.Source}}</td>
            <td>
              {{.UpdatedAt}}
              <div class="row-actions">
                <button data-action="evaluate" data-id="{{.ID}}">Evaluate</button>
                <button class="secondary" data-action="publish" data-id="{{.ID}}">Publish</button>
                <button class="secondary" data-action="publish-local" data-id="{{.ID}}">Publish Local</button>
              </div>
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{else}}<div class="empty">No Drafts.</div>{{end}}
    </section>

    <section>
      <h2>Recent Traces</h2>
      {{if .Traces}}
      <table>
        <thead><tr><th>Invocation</th><th>Skill</th><th>Latency</th><th>Output Summary</th></tr></thead>
        <tbody>
          {{range .Traces}}
          <tr><td><code>{{.InvocationID}}</code></td><td>{{.SkillID}}</td><td>{{.LatencyMillis}} ms</td><td>{{.OutputSummary}}</td></tr>
          {{end}}
        </tbody>
      </table>
      {{else}}<div class="empty">No runtime invocations yet.</div>{{end}}
    </section>
  </main>
  <script>
    const result = document.querySelector("#result");
    const show = (value) => {
      result.textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
    };
    const api = async (path, body) => {
      const response = await fetch(path, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body || {})
      });
      const json = await response.json();
      if (!response.ok) throw json;
      return json;
    };
    document.querySelector("#draft-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      const open = String.fromCharCode(123, 123);
      const close = String.fromCharCode(125, 125);
      const body = {
        namespace: data.get("namespace"),
        name: data.get("name"),
        version: data.get("version"),
        description: data.get("description"),
        visibility: "project-private",
        source: "workbench",
        created_by: "fde",
        runtime_payload: {
          mode: "in_process",
          interface: "adp.skill.runtime/v1",
          kind: "template",
          entrypoint: "runtime/template",
          template: data.get("template").replaceAll("[[", open).replaceAll("]]", close)
        },
        permission_manifest: {
          data_domains: ["finance.invoice"],
          network: { egress: [{ name: "declared-service", target: data.get("network_target"), ports: [443] }] },
          filesystem: { read: ["/mnt/input"], write: ["/tmp/adp-skill"] },
          secrets: [{ name: "service-token", required: true }],
          models: [{ name: "local-llm", purpose: "normalization" }],
          kubernetes: { api_access: false },
          draft_creation: { allowed: false }
        },
        assets: {
          prompts: ["Normalize the invoice input."],
          examples: ["invoice=" + data.get("smoke_input")],
          knowledge_refs: [{ name: "finance-policy", uri: "knowledge://finance-policy", version: "2026-05", digest: "sha256:demo" }]
        },
        evaluation: {
          cases: [{ name: "workbench smoke", input: { invoice: data.get("smoke_input") }, expected_contains: [data.get("expected")] }]
        }
      };
      try {
        show(await api("/api/drafts", body));
      } catch (error) {
        show(error);
      }
    });
    document.querySelector("#refresh-state").addEventListener("click", () => location.reload());
    document.addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const action = button.dataset.action;
      const id = button.dataset.id;
      try {
        if (action === "evaluate") show(await api("/api/drafts/" + encodeURIComponent(id).replaceAll("%2F", "/") + "/evaluate"));
        if (action === "publish") show(await api("/api/drafts/" + encodeURIComponent(id).replaceAll("%2F", "/") + "/publish"));
        if (action === "publish-local") show(await api("/api/drafts/" + encodeURIComponent(id).replaceAll("%2F", "/") + "/publish?local=true"));
        if (action === "invoke") show(await api("/api/runtime/invoke", { agent_profile_id: "demo-agent", skill_id: id, input: { text: "runtime", invoice: "INV-1001" } }));
        if (action === "lock") {
          const parts = id.split(":");
          show(await api("/api/agent-profiles/resolve", { id: "demo-agent", version: "0.1.0", skills: [{ id: parts[0], version: parts[1] }] }));
        }
        if (action === "bundle") {
          const parts = id.split(":");
          show(await api("/api/offline-bundles/export", { id: "demo-agent", version: "0.1.0", skills: [{ id: parts[0], version: parts[1] }] }));
        }
        if (action === "revoke") show(await api("/api/revocations", { target_digest: button.dataset.digest, reason: "Workbench revocation" }));
      } catch (error) {
        show(error);
      }
    });
  </script>
</body>
</html>`
