package dashboard

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/bryanbarton525/prism/internal/events"
)

type Server struct {
	store *events.Store
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
	_ = page.Execute(w, sum)
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
		Limit:  intParam(r, "limit", 100),
		Status: r.URL.Query().Get("status"),
		Agent:  r.URL.Query().Get("agent"),
		Source: r.URL.Query().Get("source"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
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
    pre { white-space: pre-wrap; }
  </style>
</head>
<body>
  <header><h1>Prism Dashboard</h1></header>
  <main>
    <section class="grid">
      <div class="card"><div>Total runs</div><div class="metric">{{.Total}}</div></div>
      <div class="card"><div>Context warnings</div><div class="metric">{{.ContextBudgetWarnings}}</div></div>
      <div class="card"><div>Policy denials</div><div class="metric">{{.PolicyDenials}}</div></div>
      <div class="card"><div>Prompt tokens</div><div class="metric">{{.PromptTokensEstimate}}</div></div>
      <div class="card"><div>Graph executions</div><div class="metric">{{.GraphExecutions}}</div></div>
      <div class="card"><div>Validation failures</div><div class="metric">{{.ValidationFailures}}</div></div>
      <div class="card"><div>Timeouts</div><div class="metric">{{.Timeouts}}</div></div>
    </section>
    <section class="card"><h2>Status</h2><pre>{{printf "%v" .StatusCounts}}</pre></section>
    <section class="card"><h2>Top agents</h2><pre>{{printf "%v" .TopAgents}}</pre></section>
    <section class="card"><h2>Top skills</h2><pre>{{printf "%v" .TopSkills}}</pre></section>
    <section class="card"><h2>Plugins</h2><pre>{{printf "%v" .TopPlugins}}</pre></section>
    <section class="card"><h2>Bundles</h2><pre>{{printf "%v" .BundleVersions}}</pre></section>
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
