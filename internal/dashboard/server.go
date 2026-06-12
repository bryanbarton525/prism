package dashboard

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/bryanbarton525/prism/internal/events"
	"github.com/bryanbarton525/prism/pkg/observe"
)

type Server struct {
	store *events.Store
}

type PageData struct {
	Summary events.Summary
	Recent  []eventsView
}

type eventsView struct {
	Timestamp string
	RunID     string
	Source    string
	AgentID   string
	Status    string
	Skills    string
	Bundle    string
}

func New(store *events.Store) *Server {
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/summary", s.summary)
	mux.HandleFunc("/api/events", s.eventList)
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	sum, _ := s.store.Summary(r.Context())
	items, _ := s.store.List(r.Context(), listOptionsFromRequest(r, 20))
	_ = page.Execute(w, PageData{Summary: sum, Recent: viewEvents(items)})
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	sum, err := s.store.Summary(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, sum)
}

func (s *Server) eventList(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.List(r.Context(), events.ListOptions{
		Limit:     intParam(r, "limit", 100),
		Status:    r.URL.Query().Get("status"),
		Agent:     r.URL.Query().Get("agent"),
		Source:    r.URL.Query().Get("source"),
		Actor:     r.URL.Query().Get("actor"),
		Workspace: r.URL.Query().Get("workspace"),
		Skill:     r.URL.Query().Get("skill"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
}

func listOptionsFromRequest(r *http.Request, limit int) events.ListOptions {
	return events.ListOptions{
		Limit:     intParam(r, "limit", limit),
		Status:    r.URL.Query().Get("status"),
		Agent:     r.URL.Query().Get("agent"),
		Source:    r.URL.Query().Get("source"),
		Actor:     r.URL.Query().Get("actor"),
		Workspace: r.URL.Query().Get("workspace"),
		Skill:     r.URL.Query().Get("skill"),
	}
}

func viewEvents(items []observe.RunEvent) []eventsView {
	out := make([]eventsView, 0, len(items))
	for _, item := range items {
		bundle := item.BundleID
		if item.BundleVersion != "" {
			bundle += "@" + item.BundleVersion
		}
		out = append(out, eventsView{
			Timestamp: item.Timestamp.Format("2006-01-02 15:04:05"),
			RunID:     item.RunID,
			Source:    item.Source,
			AgentID:   item.AgentID,
			Status:    item.Status,
			Skills:    fmt.Sprint(item.Skills),
			Bundle:    bundle,
		})
	}
	return out
}

func intParam(r *http.Request, name string, def int) int {
	value := r.URL.Query().Get(name)
	if value == "" {
		return def
	}
	var out int
	if _, err := fmt.Sscanf(value, "%d", &out); err != nil || out <= 0 {
		return def
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

var page = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Prism Dashboard</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, -apple-system, sans-serif; margin: 0; color: #17202a; background: #f7f8fa; }
    header { padding: 20px 28px; background: #ffffff; border-bottom: 1px solid #dde2e8; }
    main { padding: 24px 28px; display: grid; gap: 18px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
    .card { background: #ffffff; border: 1px solid #dde2e8; border-radius: 8px; padding: 16px; }
    .metric { font-size: 32px; font-weight: 700; }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th, td { text-align: left; border-bottom: 1px solid #dde2e8; padding: 8px 6px; vertical-align: top; }
    pre { white-space: pre-wrap; }
  </style>
</head>
<body>
  <header><h1>Prism Dashboard</h1></header>
  <main>
    <section class="grid">
      <div class="card"><div>Total runs</div><div class="metric">{{.Summary.Total}}</div></div>
      <div class="card"><div>Context warnings</div><div class="metric">{{.Summary.ContextBudgetWarnings}}</div></div>
      <div class="card"><div>Policy denials</div><div class="metric">{{.Summary.PolicyDenials}}</div></div>
      <div class="card"><div>Prompt tokens</div><div class="metric">{{.Summary.PromptTokensEstimate}}</div></div>
      <div class="card"><div>Graph executions</div><div class="metric">{{.Summary.GraphExecutions}}</div></div>
      <div class="card"><div>Validation failures</div><div class="metric">{{.Summary.ValidationFailures}}</div></div>
      <div class="card"><div>Timeouts</div><div class="metric">{{.Summary.Timeouts}}</div></div>
    </section>
    <section class="card"><h2>Recent runs</h2><table><thead><tr><th>Time</th><th>Status</th><th>Source</th><th>Agent</th><th>Skills</th><th>Bundle</th></tr></thead><tbody>{{range .Recent}}<tr><td>{{.Timestamp}}</td><td>{{.Status}}</td><td>{{.Source}}</td><td>{{.AgentID}}</td><td>{{.Skills}}</td><td>{{.Bundle}}</td></tr>{{end}}</tbody></table></section>
    <section class="card"><h2>Status</h2><pre>{{printf "%v" .Summary.StatusCounts}}</pre></section>
    <section class="card"><h2>Top agents</h2><pre>{{printf "%v" .Summary.TopAgents}}</pre></section>
    <section class="card"><h2>Top skills</h2><pre>{{printf "%v" .Summary.TopSkills}}</pre></section>
    <section class="card"><h2>Plugins</h2><pre>{{printf "%v" .Summary.TopPlugins}}</pre></section>
    <section class="card"><h2>Bundles</h2><pre>{{printf "%v" .Summary.BundleVersions}}</pre></section>
  </main>
</body>
</html>`))

func Serve(addr string, store *events.Store) error {
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	fmt.Printf("Prism dashboard: http://%s\n", addr)
	return http.ListenAndServe(addr, New(store).Handler())
}
