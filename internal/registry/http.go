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
	mux.HandleFunc("/api/community/ingestions", h.ingestCommunity)
	mux.HandleFunc("/api/skills", h.skills)
	mux.HandleFunc("/api/skills/", h.skill)
	mux.HandleFunc("/api/agent-profiles/resolve", h.resolve)
	mux.HandleFunc("/api/controller/mount", h.mount)
	mux.HandleFunc("/api/offline-bundles/export", h.exportBundle)
	mux.HandleFunc("/api/offline-bundles/import", h.importBundle)
	mux.HandleFunc("/api/runtime/drafts", h.createRuntimeDraft)
	mux.HandleFunc("/api/runtime/invoke", h.invoke)
	mux.HandleFunc("/api/revocations/export", h.exportRevocations)
	mux.HandleFunc("/api/revocations/import", h.importRevocations)
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
	filter := SkillSearchFilter{
		Namespace:   r.URL.Query().Get("namespace"),
		Query:       r.URL.Query().Get("q"),
		Visibility:  r.URL.Query().Get("visibility"),
		Source:      r.URL.Query().Get("source"),
		RuntimeMode: r.URL.Query().Get("runtime_mode"),
	}
	writeJSON(w, http.StatusOK, h.service.SearchPublished(filter))
}

func (h *handler) ingestCommunity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req CommunityIngestRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ingestion, err := h.service.IngestCommunitySkill(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ingestion)
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

func (h *handler) mount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ControllerMountRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan, err := h.service.PrepareControllerMount(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
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

func (h *handler) createRuntimeDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req CreateDraftRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	draft, err := h.service.CreateRuntimeDraft(req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, draft)
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

func (h *handler) exportRevocations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	list, err := h.service.ExportRevocationList()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *handler) importRevocations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var list SignedRevocationList
	if err := readJSON(r, &list); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	imported, err := h.service.ImportRevocationList(list)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, imported)
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
		case map[string]*CommunityIngestion:
			return len(x)
		case map[string]*Revocation:
			return len(x)
		case map[string]ControllerMountPlan:
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
      background: #f2f5f8;
      color: #172033;
      --bg: #f2f5f8;
      --surface: #ffffff;
      --surface-raised: #ffffff;
      --surface-soft: #f7f9fc;
      --line: #d8dee7;
      --line-strong: #c4cedb;
      --ink: #172033;
      --muted: #617086;
      --muted-strong: #42526a;
      --accent: #0f766e;
      --accent-dark: #0a5d58;
      --accent-soft: #e4f3f1;
      --blue: #2563eb;
      --blue-soft: #e8efff;
      --amber: #b45309;
      --amber-soft: #fff2df;
      --danger: #b42318;
      --danger-soft: #fdebea;
      --steel: #48586f;
      --shadow: 0 1px 2px rgba(15, 23, 42, .08), 0 12px 28px rgba(15, 23, 42, .06);
    }
    * { box-sizing: border-box; }
    html { min-height: 100%; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--ink);
      font-size: 14px;
      line-height: 1.45;
    }
    .topbar {
      background: var(--surface);
      border-bottom: 1px solid var(--line);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      padding: 18px 28px;
      position: sticky;
      top: 0;
      z-index: 10;
    }
    .brand { display: grid; gap: 4px; min-width: 0; }
    .eyebrow {
      color: var(--accent);
      font-size: 11px;
      font-weight: 780;
      text-transform: uppercase;
      letter-spacing: .08em;
    }
    h1 { margin: 0; font-size: 23px; line-height: 1.2; letter-spacing: 0; }
    .subtitle { color: var(--muted); font-size: 13px; max-width: 720px; }
    .status-pill {
      align-items: center;
      background: var(--accent-soft);
      border: 1px solid #b9ded9;
      border-radius: 999px;
      color: var(--accent-dark);
      display: inline-flex;
      font-size: 13px;
      font-weight: 760;
      gap: 8px;
      min-height: 34px;
      padding: 7px 12px;
      white-space: nowrap;
    }
    .status-pill::before {
      background: var(--accent);
      border-radius: 999px;
      content: "";
      height: 8px;
      width: 8px;
    }
    main { padding: 22px 28px 42px; max-width: 1440px; margin: 0 auto; }
    .grid { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 12px; margin-bottom: 18px; }
    .metric {
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      min-height: 92px;
      padding: 14px;
      position: relative;
    }
    .metric::before {
      background: var(--accent);
      border-radius: 999px;
      content: "";
      height: 5px;
      left: 14px;
      position: absolute;
      right: 14px;
      top: 0;
    }
    .metric:nth-child(2)::before,
    .metric:nth-child(5)::before { background: var(--blue); }
    .metric:nth-child(3)::before,
    .metric:nth-child(6)::before { background: var(--amber); }
    .metric strong { display: block; font-size: 30px; line-height: 1; margin: 14px 0 8px; }
    .metric span {
      color: var(--muted);
      display: block;
      font-size: 11px;
      font-weight: 780;
      letter-spacing: .06em;
      text-transform: uppercase;
    }
    .tools {
      align-items: stretch;
      display: grid;
      gap: 14px;
      grid-template-columns: minmax(360px, 460px) minmax(0, 1fr);
      margin-bottom: 16px;
    }
    section {
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
      margin: 14px 0;
      overflow: hidden;
    }
    .result-panel { display: flex; flex-direction: column; }
    section h2 {
      align-items: center;
      background: var(--surface-soft);
      border-bottom: 1px solid var(--line);
      display: flex;
      gap: 10px;
      margin: 0;
      min-height: 48px;
      padding: 13px 16px;
      font-size: 15px;
      line-height: 1.2;
    }
    section h2::before {
      background: var(--accent);
      border-radius: 999px;
      content: "";
      flex: 0 0 auto;
      height: 9px;
      width: 9px;
    }
    form { padding: 16px; display: grid; gap: 12px; }
    .field-grid { display: grid; gap: 12px; grid-template-columns: repeat(3, minmax(0, 1fr)); }
    label {
      color: #344357;
      display: grid;
      font-size: 12px;
      font-weight: 740;
      gap: 6px;
    }
    input, textarea, select {
      background: #fff;
      border: 1px solid var(--line-strong);
      border-radius: 6px;
      color: var(--ink);
      font: inherit;
      min-height: 38px;
      outline: none;
      padding: 9px 10px;
      transition: border-color .16s ease, box-shadow .16s ease;
      width: 100%;
    }
    input:focus, textarea:focus, select:focus {
      border-color: var(--accent);
      box-shadow: 0 0 0 3px rgba(15, 118, 110, .14);
    }
    input::placeholder, textarea::placeholder { color: #98a3b3; }
    textarea { min-height: 96px; resize: vertical; }
    button {
      align-items: center;
      border: 1px solid transparent;
      border-radius: 6px;
      cursor: pointer;
      display: inline-flex;
      font: inherit;
      font-size: 13px;
      font-weight: 760;
      justify-content: center;
      min-height: 36px;
      padding: 8px 12px;
      transition: background .16s ease, border-color .16s ease, transform .16s ease;
      white-space: nowrap;
    }
    button:hover { transform: translateY(-1px); }
    button:active { transform: translateY(0); }
    button { background: var(--accent); color: #fff; }
    button:hover { background: var(--accent-dark); }
    button.secondary { background: #fff; border-color: var(--line-strong); color: var(--steel); }
    button.secondary:hover { background: #f6f8fb; border-color: #aeb9c8; }
    button.danger { background: var(--danger); color: #fff; }
    button.danger:hover { background: #921b13; }
    button:disabled { opacity: .55; cursor: default; transform: none; }
    .table-scroll { overflow-x: auto; }
    table { border-collapse: collapse; font-size: 13px; min-width: 760px; width: 100%; }
    th, td { border-bottom: 1px solid #e8edf3; padding: 12px 14px; text-align: left; vertical-align: top; }
    th {
      background: #f8fafc;
      color: #526278;
      font-size: 11px;
      font-weight: 780;
      letter-spacing: .05em;
      text-transform: uppercase;
    }
    tr:hover td { background: #fbfcfd; }
    tr:last-child td { border-bottom: 0; }
    code {
      background: #edf3f7;
      border: 1px solid #dce5ed;
      border-radius: 5px;
      color: #26364a;
      display: inline-block;
      font-size: 12px;
      line-height: 1.35;
      max-width: 100%;
      overflow-wrap: anywhere;
      padding: 2px 6px;
    }
    pre {
      background: #0f172a;
      color: #d8e4ef;
      flex: 1;
      font-size: 12px;
      line-height: 1.55;
      margin: 0;
      min-height: 420px;
      overflow: auto;
      padding: 16px;
      white-space: pre-wrap;
    }
    .badge {
      align-items: center;
      border-radius: 999px;
      display: inline-flex;
      font-size: 12px;
      font-weight: 760;
      gap: 6px;
      line-height: 1;
      padding: 6px 9px;
      white-space: nowrap;
    }
    .badge::before {
      border-radius: 999px;
      content: "";
      height: 6px;
      width: 6px;
    }
    .badge.ok {
      background: var(--accent-soft);
      border: 1px solid #b9ded9;
      color: var(--accent-dark);
    }
    .badge.ok::before { background: var(--accent); }
    .badge.neutral {
      background: var(--blue-soft);
      border: 1px solid #cbd8ff;
      color: #1d4ed8;
    }
    .badge.neutral::before { background: var(--blue); }
    .badge.warn {
      background: var(--amber-soft);
      border: 1px solid #fed7aa;
      color: var(--amber);
    }
    .badge.warn::before { background: var(--amber); }
    .bad { color: var(--danger); font-weight: 760; }
    .muted { color: var(--muted); }
    .empty { padding: 18px; color: var(--muted); background: #fbfcfd; }
    .actions { display: flex; flex-wrap: wrap; gap: 8px; padding-top: 4px; }
    .row-actions { display: flex; flex-wrap: wrap; gap: 6px; min-width: 230px; }
    .row-actions button { padding: 6px 9px; min-height: 30px; font-size: 12px; }
    @media (max-width: 1120px) {
      .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
      .tools { grid-template-columns: 1fr; }
      pre { min-height: 320px; }
    }
    @media (max-width: 760px) {
      .topbar { align-items: flex-start; flex-direction: column; padding: 16px; position: static; }
      main { padding: 16px; }
      .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .field-grid { grid-template-columns: 1fr; }
      table { min-width: 680px; }
    }
    @media (max-width: 560px) {
      .grid { grid-template-columns: 1fr; }
      .actions button, .row-actions button { width: 100%; }
      .tools { gap: 10px; }
    }
  </style>
</head>
<body>
  <header class="topbar">
    <div class="brand">
      <div class="eyebrow">ADP Internal</div>
      <h1>Agent Skill Registry</h1>
      <div class="subtitle">Workbench, Registry, Lockfile, Offline Bundle, Local Runtime simulation</div>
    </div>
    <div class="status-pill">MVP Ready</div>
  </header>
  <main>
    <div class="grid">
      <div class="metric"><strong>{{count .Drafts}}</strong><span>Drafts</span></div>
      <div class="metric"><strong>{{count .Published}}</strong><span>Published Skills</span></div>
      <div class="metric"><strong>{{count .CommunityIngestions}}</strong><span>Community Ingestions</span></div>
      <div class="metric"><strong>{{count .Revocations}}</strong><span>Revocations</span></div>
      <div class="metric"><strong>{{count .MountPlans}}</strong><span>Mount Plans</span></div>
      <div class="metric"><strong>{{count .Traces}}</strong><span>Invocation Traces</span></div>
    </div>

    <div class="tools">
      <section>
        <h2>Create Skill Draft</h2>
        <form id="draft-form">
          <div class="field-grid">
            <label>Namespace <input name="namespace" value="finance" required></label>
            <label>Name <input name="name" value="invoice-normalizer" required></label>
            <label>Version <input name="version" value="0.1.0" required></label>
          </div>
          <label>Description <input name="description" value="Normalize invoice text into a predictable response."></label>
          <label>Template <textarea name="template">Invoice [[invoice]] normalized by Agent Skill Registry</textarea></label>
          <label>Smoke input value <input name="smoke_input" value="INV-1001"></label>
          <label>Expected output contains <input name="expected" value="INV-1001"></label>
          <label>Network target <input name="network_target" value="service:ocr.default.svc.cluster.local"></label>
          <div class="actions">
            <button type="submit">Create Draft</button>
            <button class="secondary" type="button" id="ingest-community">Ingest Community</button>
            <button class="secondary" type="button" id="refresh-state">Refresh</button>
            <button class="danger" type="button" id="export-revocations">Export Revocations</button>
          </div>
        </form>
      </section>

      <section class="result-panel">
        <h2>API Result</h2>
        <pre id="result">Use the Workbench controls to exercise the MVP flow.</pre>
      </section>
    </div>

    <section>
      <h2>Published Skills</h2>
      {{if .Published}}
      <div class="table-scroll">
      <table>
        <thead><tr><th>ID</th><th>Status</th><th>Runtime</th><th>Digest</th><th>Actions</th></tr></thead>
        <tbody>
          {{range .Published}}
          <tr>
            <td><code>{{.ID}}</code><br><span class="muted">{{.Description}}</span></td>
            <td><span class="badge ok">{{.Status}}</span></td>
            <td>{{.RuntimePayload.Mode}}<br><span class="muted">{{.RuntimePayload.Interface}}</span></td>
            <td><code>{{short .Digest}}</code><br><span class="muted">{{.SignatureScope}}</span></td>
            <td>
              <div class="row-actions">
                <button data-action="invoke" data-id="{{.ID}}">Invoke</button>
                <button class="secondary" data-action="lock" data-id="{{.ID}}">Lockfile</button>
                <button class="secondary" data-action="bundle" data-id="{{.ID}}">Export Bundle</button>
                <button class="secondary" data-action="mount" data-id="{{.ID}}">Mount Plan</button>
                <button class="danger" data-action="revoke" data-digest="{{.Digest}}">Revoke</button>
              </div>
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>
      </div>
      {{else}}<div class="empty">No Published Skills.</div>{{end}}
    </section>

    <section>
      <h2>Community Ingestions</h2>
      {{if .CommunityIngestions}}
      <div class="table-scroll">
      <table>
        <thead><tr><th>ID</th><th>Source</th><th>Status</th><th>Published Skill</th></tr></thead>
        <tbody>
          {{range .CommunityIngestions}}
          <tr><td><code>{{.ID}}</code></td><td>{{.SourceURL}}<br><span class="muted">{{.SourceVersion}} {{.License}}</span></td><td><span class="badge ok">{{.Status}}</span></td><td><code>{{.PublishedSkillID}}</code></td></tr>
          {{end}}
        </tbody>
      </table>
      </div>
      {{else}}<div class="empty">No Community Skill ingestions yet.</div>{{end}}
    </section>

    <section>
      <h2>Controller Mount Plans</h2>
      {{if .MountPlans}}
      <div class="table-scroll">
      <table>
        <thead><tr><th>ID</th><th>Agent Profile</th><th>Status</th><th>Skills</th></tr></thead>
        <tbody>
          {{range .MountPlans}}
          <tr><td><code>{{.ID}}</code></td><td>{{.AgentProfile.ID}}:{{.AgentProfile.Version}}</td><td><span class="badge ok">{{.Status}}</span></td><td>{{len .Skills}}</td></tr>
          {{end}}
        </tbody>
      </table>
      </div>
      {{else}}<div class="empty">No Controller mount plans yet.</div>{{end}}
    </section>

    <section>
      <h2>Drafts</h2>
      {{if .Drafts}}
      <div class="table-scroll">
      <table>
        <thead><tr><th>ID</th><th>Status</th><th>Source</th><th>Updated</th></tr></thead>
        <tbody>
          {{range .Drafts}}
          <tr>
            <td><code>{{.ID}}</code></td>
            <td><span class="badge warn">{{.Status}}</span></td>
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
      </div>
      {{else}}<div class="empty">No Drafts.</div>{{end}}
    </section>

    <section>
      <h2>Recent Traces</h2>
      {{if .Traces}}
      <div class="table-scroll">
      <table>
        <thead><tr><th>Invocation</th><th>Skill</th><th>Latency</th><th>Output Summary</th></tr></thead>
        <tbody>
          {{range .Traces}}
          <tr><td><code>{{.InvocationID}}</code></td><td>{{.SkillID}}</td><td>{{.LatencyMillis}} ms</td><td>{{.OutputSummary}}</td></tr>
          {{end}}
        </tbody>
      </table>
      </div>
      {{else}}<div class="empty">No runtime invocations yet.</div>{{end}}
    </section>
  </main>
  <script>
    const result = document.querySelector("#result");
    const resultCacheKey = "agent-skill-registry:last-result";
    const show = (value) => {
      const rendered = typeof value === "string" ? value : JSON.stringify(value, null, 2);
      result.textContent = rendered;
      return rendered;
    };
    const showAndRefresh = (value) => {
      const rendered = show(value);
      sessionStorage.setItem(resultCacheKey, rendered);
      window.setTimeout(() => location.reload(), 300);
    };
    const open = String.fromCharCode(123, 123);
    const close = String.fromCharCode(125, 125);
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
    const cachedResult = sessionStorage.getItem(resultCacheKey);
    if (cachedResult) {
      result.textContent = cachedResult;
      sessionStorage.removeItem(resultCacheKey);
    }
    document.querySelector("#draft-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const submitter = event.submitter || event.currentTarget.querySelector("button[type='submit']");
      if (submitter) submitter.disabled = true;
      const data = new FormData(event.currentTarget);
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
        showAndRefresh(await api("/api/drafts", body));
      } catch (error) {
        show(error);
      } finally {
        if (submitter) submitter.disabled = false;
      }
    });
    document.querySelector("#refresh-state").addEventListener("click", () => location.reload());
    document.querySelector("#export-revocations").addEventListener("click", async () => {
      try {
        show(await api("/api/revocations/export", {}));
      } catch (error) {
        show(error);
      }
    });
    document.querySelector("#ingest-community").addEventListener("click", async () => {
      const form = document.querySelector("#draft-form");
      const data = new FormData(form);
      const sourceName = "community-" + String(data.get("name"));
      const body = {
        source_url: "oci://community.example/skills/" + sourceName,
        source_version: "0.1.0",
        source_digest: "sha256:" + sourceName,
        license: "Apache-2.0",
        scan: { status: "pass", critical_vulnerabilities: 0, high_vulnerabilities: 0 },
        skill: {
          namespace: "community",
          name: sourceName,
          version: data.get("version"),
          description: "Ingested community baseline for " + data.get("description"),
          visibility: "company-wide",
          runtime_payload: {
            mode: "in_process",
            interface: "adp.skill.runtime/v1",
            kind: "template",
            entrypoint: "runtime/template",
            template: "Community " + data.get("template").replaceAll("[[", open).replaceAll("]]", close)
          },
          permission_manifest: {
            data_domains: ["community.demo"],
            network: { egress: [{ name: "declared-service", target: data.get("network_target"), ports: [443] }] },
            filesystem: { read: ["/mnt/input"], write: ["/tmp/adp-skill"] },
            secrets: [{ name: "community-token", required: false }],
            kubernetes: { api_access: false }
          },
          evaluation: {
            cases: [{ name: "community smoke", input: { invoice: data.get("smoke_input") }, expected_contains: [data.get("expected")] }]
          }
        }
      };
      try {
        showAndRefresh(await api("/api/community/ingestions", body));
      } catch (error) {
        show(error);
      }
    });
    document.addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const action = button.dataset.action;
      const id = button.dataset.id;
      try {
        if (action === "evaluate") showAndRefresh(await api("/api/drafts/" + encodeURIComponent(id).replaceAll("%2F", "/") + "/evaluate"));
        if (action === "publish") showAndRefresh(await api("/api/drafts/" + encodeURIComponent(id).replaceAll("%2F", "/") + "/publish"));
        if (action === "publish-local") showAndRefresh(await api("/api/drafts/" + encodeURIComponent(id).replaceAll("%2F", "/") + "/publish?local=true"));
        if (action === "invoke") showAndRefresh(await api("/api/runtime/invoke", { agent_profile_id: "demo-agent", skill_id: id, input: { text: "runtime", invoice: "INV-1001" } }));
        if (action === "lock") {
          const parts = id.split(":");
          show(await api("/api/agent-profiles/resolve", { id: "demo-agent", version: "0.1.0", skills: [{ id: parts[0], version: parts[1] }] }));
        }
        if (action === "bundle") {
          const parts = id.split(":");
          show(await api("/api/offline-bundles/export", { id: "demo-agent", version: "0.1.0", skills: [{ id: parts[0], version: parts[1] }] }));
        }
        if (action === "mount") {
          const parts = id.split(":");
          showAndRefresh(await api("/api/controller/mount", { agent_profile: { id: "demo-agent", version: "0.1.0", skills: [{ id: parts[0], version: parts[1] }] } }));
        }
        if (action === "revoke") showAndRefresh(await api("/api/revocations", { target_digest: button.dataset.digest, reason: "Workbench revocation" }));
      } catch (error) {
        show(error);
      }
    });
  </script>
</body>
</html>`
