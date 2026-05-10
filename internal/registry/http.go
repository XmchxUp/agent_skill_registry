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
      --primary: #150f23;
      --ink-deep: #1f1633;
      --on-primary: #ffffff;
      --accent-lime: #c2ef4e;
      --accent-pink: #fa7faa;
      --accent-violet: #6a5fc1;
      --accent-violet-deep: #422082;
      --accent-violet-mid: #79628c;
      --surface-canvas-dark: #1f1633;
      --surface-canvas-light: #ffffff;
      --surface-night: #150f23;
      --surface-press-light: #f0f0f0;
      --surface-press-stronger: #efefef;
      --hairline-violet: #362d59;
      --hairline-cool: #cfcfdb;
      --hairline-cloud: #e5e7eb;
      --ink: #1f1633;
      --ink-press: #1a1a1a;
      --on-dark-muted: #bdb8c0;
      --on-dark-faint: #3f3849;
      --ring-focus: #9dc1f5;
      --shadow-1: rgba(0, 0, 0, 0.08) 0 2px 8px 0;
      --shadow-2: rgba(0, 0, 0, 0.1) 0 10px 15px -3px, rgba(0, 0, 0, 0.1) 0 4px 6px -4px;
    }

    * { box-sizing: border-box; }
    html { min-height: 100%; }
    body {
      margin: 0;
      background: var(--surface-canvas-dark);
      color: var(--on-primary);
      font-family: Rubik, -apple-system, system-ui, Segoe UI, Helvetica, Arial, sans-serif;
      font-size: 16px;
      line-height: 1.5;
    }

    /* Top Navigation - Dark Canvas */
    .topbar {
      background: var(--surface-canvas-dark);
      border-bottom: 1px solid var(--hairline-violet);
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 24px;
      padding: 32px 48px;
    }
    .brand { display: grid; gap: 8px; min-width: 0; max-width: 600px; }
    .eyebrow {
      color: var(--accent-lime);
      font-size: 15px;
      font-weight: 500;
      text-transform: uppercase;
      letter-spacing: 0.2px;
      line-height: 1.4;
    }
    h1 {
      margin: 0;
      font-family: "Sentri Display", Rubik, system-ui, sans-serif;
      font-size: 60px;
      font-weight: 500;
      line-height: 1.1;
      letter-spacing: 0;
    }
    .subtitle {
      color: var(--on-dark-muted);
      font-size: 16px;
      line-height: 2.0;
      max-width: 720px;
    }
    .status-pill {
      align-items: center;
      background: var(--on-dark-faint);
      border: 1px solid var(--hairline-violet);
      border-radius: 12px;
      color: var(--on-primary);
      display: inline-flex;
      font-size: 14px;
      font-weight: 700;
      gap: 8px;
      min-height: 34px;
      padding: 8px 16px;
      white-space: nowrap;
      text-transform: uppercase;
      letter-spacing: 0.2px;
    }
    .status-pill::before {
      background: var(--accent-lime);
      border-radius: 999px;
      content: "";
      height: 8px;
      width: 8px;
    }

    /* Main Content */
    main { padding: 48px 48px 80px; max-width: 1440px; margin: 0 auto; }

    /* Stats Grid - 4 columns for better spacing */
    .grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 24px; margin-bottom: 48px; }
    .metric {
      background: var(--surface-canvas-light);
      border: 1px solid var(--hairline-cloud);
      border-radius: 12px;
      padding: 32px 24px;
      position: relative;
      transition: transform 0.16s ease, box-shadow 0.16s ease;
    }
    .metric:hover {
      transform: translateY(-2px);
      box-shadow: var(--shadow-2);
    }
    .metric::before {
      background: var(--primary);
      border-radius: 999px;
      content: "";
      height: 5px;
      left: 24px;
      position: absolute;
      right: 24px;
      top: 0;
    }
    .metric:nth-child(2)::before,
    .metric:nth-child(5)::before { background: var(--accent-violet); }
    .metric:nth-child(3)::before,
    .metric:nth-child(6)::before { background: var(--accent-lime); }
    .metric strong {
      display: block;
      font-family: "Sentri Display", Rubik, system-ui, sans-serif;
      font-size: 60px;
      line-height: 1.1;
      margin: 16px 0 8px;
      color: var(--ink-deep);
    }
    .metric span {
      color: var(--on-dark-muted);
      display: block;
      font-size: 14px;
      font-weight: 700;
      letter-spacing: 0.2px;
      text-transform: uppercase;
    }

    /* Tools Section - Better balanced layout */
    .tools {
      align-items: stretch;
      display: grid;
      gap: 32px;
      grid-template-columns: 420px minmax(0, 1fr);
      margin-bottom: 48px;
    }
    section {
      background: var(--ink-deep);
      border: 1px solid var(--hairline-violet);
      border-radius: 18px;
      margin: 32px 0;
      overflow: hidden;
    }
    .result-panel { display: flex; flex-direction: column; }
    section h2 {
      align-items: center;
      background: var(--surface-night);
      border-bottom: 1px solid var(--hairline-violet);
      display: flex;
      gap: 12px;
      margin: 0;
      min-height: 56px;
      padding: 20px 24px;
      font-family: Rubik, system-ui, sans-serif;
      font-size: 20px;
      font-weight: 600;
      line-height: 1.25;
    }
    section h2::before {
      background: var(--accent-lime);
      border-radius: 999px;
      content: "";
      flex: 0 0 auto;
      height: 10px;
      width: 10px;
    }
    form { padding: 24px; display: grid; gap: 20px; }
    .field-grid { display: grid; gap: 20px; grid-template-columns: repeat(3, minmax(0, 1fr)); }
    label {
      color: var(--on-primary);
      display: grid;
      font-size: 14px;
      font-weight: 500;
      gap: 8px;
      line-height: 1.4;
    }
    label span {
      color: var(--on-dark-muted);
      font-size: 12px;
      font-weight: 400;
    }
    input, textarea, select {
      background: var(--surface-canvas-light);
      border: 1px solid var(--hairline-cool);
      border-radius: 6px;
      color: var(--ink-deep);
      font: inherit;
      min-height: 40px;
      outline: none;
      padding: 10px 12px;
      transition: border-color 0.16s ease, box-shadow 0.16s ease;
      width: 100%;
    }
    input:focus, textarea:focus, select:focus {
      border-color: var(--accent-lime);
      box-shadow: 0 0 0 3px rgba(194, 239, 78, 0.14);
    }
    input::placeholder, textarea::placeholder { color: var(--on-dark-muted); }
    textarea { min-height: 120px; resize: vertical; }
    button {
      align-items: center;
      border: 1px solid transparent;
      border-radius: 8px;
      cursor: pointer;
      display: inline-flex;
      font: inherit;
      font-size: 14px;
      font-weight: 700;
      justify-content: center;
      min-height: 40px;
      padding: 10px 16px;
      transition: background 0.16s ease, border-color 0.16s ease, transform 0.16s ease;
      white-space: nowrap;
      text-transform: uppercase;
      letter-spacing: 0.2px;
    }
    button:hover { transform: translateY(-1px); }
    button:active { transform: translateY(0); }
    button { background: var(--primary); color: var(--on-primary); }
    button:hover { background: var(--accent-violet-deep); }
    button.secondary { background: var(--on-dark-faint); color: var(--on-primary); }
    button.secondary:hover { background: var(--on-dark-muted); }
    button.danger { background: var(--accent-pink); color: var(--ink-deep); }
    button.danger:hover { background: #f85f92; }
    button:disabled { opacity: 0.5; cursor: default; transform: none; }
    .actions { display: flex; flex-wrap: wrap; gap: 12px; padding-top: 12px; }
    .row-actions { display: flex; flex-wrap: wrap; gap: 8px; min-width: 280px; }
    .row-actions button { padding: 8px 12px; min-height: 34px; font-size: 13px; }

    /* Tables */
    .table-scroll { overflow-x: auto; }
    table { border-collapse: collapse; font-size: 14px; min-width: 760px; width: 100%; }
    th, td { border-bottom: 1px solid var(--hairline-violet); padding: 16px; text-align: left; vertical-align: top; }
    th {
      background: var(--surface-night);
      color: var(--on-dark-muted);
      font-size: 14px;
      font-weight: 700;
      letter-spacing: 0.2px;
      text-transform: uppercase;
    }
    tr:hover td { background: var(--on-dark-faint); }
    tr:last-child td { border-bottom: 0; }

    /* Code and Pre */
    code {
      background: var(--surface-night);
      border: 1px solid var(--hairline-violet);
      border-radius: 4px;
      color: var(--accent-lime);
      display: inline-block;
      font-family: Monaco, Menlo, Ubuntu Mono, monospace;
      font-size: 14px;
      line-height: 1.5;
      max-width: 100%;
      overflow-wrap: anywhere;
      padding: 4px 8px;
    }
    pre {
      background: var(--surface-night);
      color: var(--on-primary);
      flex: 1;
      font-family: Monaco, Menlo, Ubuntu Mono, monospace;
      font-size: 14px;
      line-height: 1.5;
      margin: 0;
      min-height: 420px;
      overflow: auto;
      padding: 32px;
      white-space: pre-wrap;
    }

    /* Badges */
    .badge {
      align-items: center;
      border-radius: 4px;
      display: inline-flex;
      font-size: 12px;
      font-weight: 700;
      gap: 6px;
      line-height: 1;
      padding: 6px 10px;
      white-space: nowrap;
      text-transform: uppercase;
      letter-spacing: 0.2px;
    }
    .badge::before {
      border-radius: 999px;
      content: "";
      height: 6px;
      width: 6px;
    }
    .badge.ok {
      background: var(--accent-lime);
      border: 1px solid var(--accent-lime);
      color: var(--ink-deep);
    }
    .badge.ok::before { background: var(--accent-lime); }
    .badge.neutral {
      background: var(--accent-violet-mid);
      border: 1px solid var(--accent-violet-mid);
      color: var(--on-primary);
    }
    .badge.neutral::before { background: var(--accent-violet-mid); }
    .badge.warn {
      background: var(--accent-pink);
      border: 1px solid var(--accent-pink);
      color: var(--ink-deep);
    }
    .badge.warn::before { background: var(--accent-pink); }
    .badge.danger {
      background: var(--hairline-violet);
      border: 1px solid var(--hairline-violet);
      color: var(--on-dark-muted);
    }
    .badge.danger::before { background: var(--hairline-violet); }

    .muted { color: var(--on-dark-muted); }
    .empty { padding: 48px 32px; color: var(--on-dark-muted); background: var(--on-dark-faint); text-align: center; }

    /* Responsive */
    @media (max-width: 1120px) {
      .topbar { padding: 24px 32px; }
      main { padding: 32px; }
      .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
      .tools { grid-template-columns: 1fr; }
      pre { min-height: 320px; }
    }
    @media (max-width: 760px) {
      .topbar { align-items: flex-start; flex-direction: column; padding: 20px; position: static; }
      h1 { font-size: 48px; }
      main { padding: 20px; }
      .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .field-grid { grid-template-columns: 1fr; }
      table { min-width: 680px; }
    }
    @media (max-width: 560px) {
      .grid { grid-template-columns: 1fr; }
      .actions button, .row-actions button { width: 100%; }
      .tools { gap: 16px; }
      h1 { font-size: 36px; }
      .brand { gap: 4px; }
      .eyebrow { font-size: 13px; }
    }
  </style>
</head>
<body>
  <header class="topbar">
    <div class="brand">
      <div class="eyebrow">ADP Internal Platform</div>
      <h1>Agent Skill <span style="background: var(--accent-lime); color: var(--ink-deep); padding: 0 8px; border-radius: 4px;">Registry</span></h1>
      <div class="subtitle">Skill Workbench, Registry, Lockfile, Offline Bundle & Runtime simulation</div>
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
    const scrollToResult = () => {
      const resultPanel = document.querySelector(".result-panel");
      if (resultPanel) {
        resultPanel.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    };
    const show = (value) => {
      const rendered = typeof value === "string" ? value : JSON.stringify(value, null, 2);
      result.textContent = rendered;
      scrollToResult();
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
