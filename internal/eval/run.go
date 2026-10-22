package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	prismbundle "github.com/bryanbarton525/prism"
	"github.com/bryanbarton525/prism/internal/app"
	"github.com/bryanbarton525/prism/internal/buildinfo"
	"github.com/bryanbarton525/prism/internal/downstreammcp"
	"github.com/bryanbarton525/prism/internal/llm/runtime"
	prismmcp "github.com/bryanbarton525/prism/internal/mcp"
	"github.com/bryanbarton525/prism/internal/plugins"
	"github.com/bryanbarton525/prism/internal/result"
	"github.com/bryanbarton525/prism/internal/toolmodel"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type ModelCall struct {
	Started    time.Time             `json:"started"`
	Request    runtime.ChatRequest   `json:"request"`
	Response   *runtime.ChatResponse `json:"response,omitempty"`
	Error      string                `json:"error,omitempty"`
	DurationMS int64                 `json:"duration_ms"`
}
type FixtureCall struct {
	Started    time.Time           `json:"started"`
	DurationMS int64               `json:"duration_ms"`
	Response   *sdk.CallToolResult `json:"response,omitempty"`
	Server     string              `json:"server"`
	Tool       string              `json:"tool"`
	Arguments  map[string]any      `json:"arguments"`
	Matched    bool                `json:"matched"`
}
type Trial struct {
	ControlMode    string                   `json:"control_mode,omitempty"`
	Routing        bool                     `json:"routing,omitempty"`
	Routes         []app.RouteDecision      `json:"routes,omitempty"`
	Recommendation *app.RecommendationTrace `json:"recommendation,omitempty"`
	MCPResponse    *sdk.CallToolResult      `json:"mcp_response,omitempty"`
	CaseID         string                   `json:"case_id"`
	Model          string                   `json:"model"`
	Repetition     int                      `json:"repetition"`
	DurationMS     int64                    `json:"duration_ms"`
	Calls          []ModelCall              `json:"calls"`
	ToolCalls      []FixtureCall            `json:"tool_calls"`
	Result         result.RunResult         `json:"result"`
	MCPError       bool                     `json:"mcp_error"`
	Error          string                   `json:"error,omitempty"`
	Grade          Grade                    `json:"grade"`
}
type Generation struct {
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}
type Report struct {
	ControlMode string         `json:"control_mode,omitempty"`
	LayaMode    string         `json:"laya_mode,omitempty"`
	LayaURL     string         `json:"laya_url,omitempty"`
	Version     int            `json:"version"`
	Grader      string         `json:"grader"`
	Suite       Suite          `json:"suite"`
	SuiteHash   string         `json:"suite_hash"`
	Build       buildinfo.Info `json:"build"`
	Engine      runtime.Engine `json:"engine"`
	Models      []string       `json:"models"`
	Repeat      int            `json:"repeat"`
	Generation  Generation     `json:"generation"`
	Started     time.Time      `json:"started"`
	Completed   bool           `json:"completed"`
	Trials      []Trial        `json:"trials"`
}
type Options struct {
	LexicalAdvice bool
	LayaMode      string
	LayaURL       string
	Models        []string
	Repeat        int
	Timeout       time.Duration
	Generation    Generation
	OnTrial       func(Trial) error
}

// Run exercises the production run_agent MCP adapter and agent loop. Only the
// model endpoint is live: downstream MCP tools are isolated HTTP replay servers.
func Run(ctx context.Context, s Suite, rt runtime.ModelRuntime, o Options) (Report, error) {
	if o.LayaURL != "" && o.LayaMode == "" {
		o.LayaMode = "first"
	}
	if o.LayaURL == "" {
		o.LayaMode = ""
	}
	r := Report{LayaMode: o.LayaMode, LayaURL: o.LayaURL, Version: Version, Grader: GraderVersion, Suite: s, SuiteHash: Hash(s), Build: buildinfo.Current(), Engine: rt.Engine(), Models: o.Models, Repeat: o.Repeat, Generation: o.Generation, Started: time.Now().UTC()}
	if o.LexicalAdvice {
		if o.LayaURL != "" {
			return r, fmt.Errorf("lexical advice and Laya are separate evaluation modes")
		}
		r.ControlMode = "lexical"
		r.Version = artifactVersion(r.ControlMode)
	}
	if err := s.Validate(); err != nil {
		return r, err
	}
	if len(o.Models) == 0 || o.Repeat < 1 || o.Repeat > 100 || o.Timeout <= 0 || o.Generation.MaxTokens < 1 || math.IsNaN(o.Generation.Temperature) || math.IsInf(o.Generation.Temperature, 0) || o.Generation.Temperature < 0 || o.Generation.Temperature > 2 {
		return r, fmt.Errorf("models, 1–100 repetitions, positive timeout and max tokens required")
	}
	if o.LayaURL != "" {
		if o.LayaMode != "first" && o.LayaMode != "advisory" {
			return r, fmt.Errorf("laya mode must be first or advisory")
		}
		if _, err := toolmodel.NewDecisionClient(o.LayaURL, "", "laya-english"); err != nil {
			return r, err
		}
	}
	seen := map[string]bool{}
	for _, model := range o.Models {
		if model == "" || seen[model] {
			return r, fmt.Errorf("model IDs must be nonempty and unique")
		}
		seen[model] = true
	}
	for repetition := 1; repetition <= o.Repeat; repetition++ {
		for _, c := range s.Cases {
			for _, model := range o.Models {
				if err := ctx.Err(); err != nil {
					return r, err
				}
				trialCtx, cancel := context.WithTimeout(ctx, o.Timeout)
				t := runTrial(trialCtx, c, model, repetition, rt, o.Generation, o.LayaURL, o.LayaMode, r.ControlMode)
				cancel()
				r.Trials = append(r.Trials, t)
				if o.OnTrial != nil {
					if err := o.OnTrial(t); err != nil {
						return r, err
					}
				}
			}
		}
	}
	r.Completed = true
	return r, nil
}

type tracedRuntime struct {
	runtime.ModelRuntime
	model      string
	generation Generation
	calls      []ModelCall
}

func (r *tracedRuntime) Chat(ctx context.Context, req runtime.ChatRequest) (*runtime.ChatResponse, error) {
	req.Model = r.model
	req.Temperature = &r.generation.Temperature
	req.MaxTokens = r.generation.MaxTokens
	data, _ := json.Marshal(req)
	var saved runtime.ChatRequest
	_ = json.Unmarshal(data, &saved)
	start := time.Now()
	response, err := r.ModelRuntime.Chat(ctx, req)
	call := ModelCall{Started: start.UTC(), Request: saved, DurationMS: time.Since(start).Milliseconds()}
	if response != nil {
		data, _ = json.Marshal(response)
		_ = json.Unmarshal(data, &call.Response)
	}
	if err != nil {
		call.Error = err.Error()
	}
	r.calls = append(r.calls, call)
	return response, err
}

type fixturePlugin struct{ name, content string }

func (p fixturePlugin) Name() string { return p.name }
func (p fixturePlugin) Tools() []plugins.ToolSpec {
	return []plugins.ToolSpec{{Name: "fixture.context", ReadOnly: true}}
}
func (p fixturePlugin) Call(context.Context, plugins.ToolCall) (plugins.ToolResult, error) {
	return plugins.ToolResult{Label: "fixed-eval-context", Content: p.content}, nil
}

type replayClient struct {
	*downstreammcp.Client
	public []downstreammcp.Server
}

func (c replayClient) Servers() []downstreammcp.Server { return c.public }
func runTrial(ctx context.Context, c Case, model string, repetition int, rt runtime.ModelRuntime, g Generation, layaURL, layaMode, controlMode string) (t Trial) {
	start := time.Now()
	t = Trial{CaseID: c.ID, Model: model, Repetition: repetition}
	t.ControlMode = controlMode
	t.Routing = layaURL != "" && layaMode == "first"
	if (layaURL != "" && !t.Routing) || controlMode == "lexical" {
		t.Recommendation = &app.RecommendationTrace{Error: "recommendation step was not reached"}
	}
	traced := &tracedRuntime{ModelRuntime: rt, model: model, generation: g}
	var mu sync.Mutex
	defer func() {
		t.DurationMS = time.Since(start).Milliseconds()
		t.Calls = traced.calls
		mu.Lock()
		defer mu.Unlock()
		t.Grade = GradeTrial(c, t)
	}()
	servers := map[string]*sdk.Server{}
	var order []string
	for _, fixture := range c.Tools {
		f := fixture
		srv := servers[f.Server]
		if srv == nil {
			srv = sdk.NewServer(&sdk.Implementation{Name: f.Server, Version: "fixture-v1"}, nil)
			servers[f.Server] = srv
			order = append(order, f.Server)
		}
		srv.AddTool(&sdk.Tool{Name: f.Name, Description: f.Description, InputSchema: f.InputSchema}, func(_ context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			started := time.Now()
			var args map[string]any
			err := json.Unmarshal(req.Params.Arguments, &args)
			matched := err == nil && Hash(args) == Hash(f.Arguments)
			body := f.Response
			isError := f.IsError
			if !matched {
				body = map[string]any{"error": "fixture argument mismatch"}
				isError = true
			}
			data, _ := json.Marshal(body)
			response := &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(data)}}, StructuredContent: body, IsError: isError}
			mu.Lock()
			t.ToolCalls = append(t.ToolCalls, FixtureCall{Started: started.UTC(), DurationMS: time.Since(started).Milliseconds(), Server: f.Server, Tool: f.Name, Arguments: args, Matched: matched, Response: response})
			mu.Unlock()
			return response, nil
		})
	}
	state := downstreammcp.State{}
	var public []downstreammcp.Server
	for _, name := range order {
		srv := servers[name]
		httpServer := httptest.NewServer(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, nil))
		defer httpServer.Close()
		state.Servers = append(state.Servers, downstreammcp.Server{Name: name, Transport: downstreammcp.TransportStreamableHTTP, URL: httpServer.URL, MaxBytes: c.MaxBytes})
		public = append(public, downstreammcp.Server{Name: name, Transport: downstreammcp.TransportStreamableHTTP, URL: "http://fixture.invalid/" + name})
	}
	var inventory []byte
	client := replayClient{Client: downstreammcp.New(state), public: public}
	// Never include expected arguments, outcomes or grader checks in model context.
	var toolInventory []map[string]any
	for _, f := range c.Tools {
		toolInventory = append(toolInventory, map[string]any{"server": f.Server, "name": f.Name, "description": f.Description, "input_schema": f.InputSchema})
	}
	inventory, _ = json.Marshal(toolInventory)
	cfg := app.Config{BundleFS: prismbundle.BundleFS(), ModelRuntime: traced, DownstreamMCP: client, RuntimePlugins: plugins.NewRegistry(fixturePlugin{"linear", "Use the MCP bridge to retrieve evidence. This is a synthetic evaluation workspace."}, fixturePlugin{"mcp", string(inventory)})}
	if controlMode == "lexical" {
		cfg.ToolRecommendationAgents = []string{c.AgentID}
		cfg.RecommendationObserver = func(trace app.RecommendationTrace) { t.Recommendation = &trace }
	}
	if layaURL != "" {
		cfg.DecisionURL, cfg.DecisionModel = layaURL, "laya-english"
		cfg.ToolRecommendationAgents = []string{c.AgentID}
		cfg.DecisionRoutingMode = layaMode
		cfg.RouteObserver = func(route app.RouteDecision) { t.Routes = append(t.Routes, route) }
		cfg.RecommendationObserver = func(trace app.RecommendationTrace) { t.Recommendation = &trace }
	}
	runner, err := app.New(cfg)
	if err != nil {
		t.Error = err.Error()
		return
	}
	server := prismmcp.NewServer(runner, prismmcp.Config{})
	ct, st := sdk.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Error = err.Error()
		return
	}
	defer ss.Close()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "prism-eval", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Error = err.Error()
		return
	}
	defer cs.Close()
	arguments := map[string]any{"agent_id": c.AgentID, "task": c.Task, "skill_names": c.Skills, "format": "json"}
	if len(c.DecisionSchema) > 0 {
		arguments["decision_schema"] = c.DecisionSchema
	}
	response, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: "run_agent", Arguments: arguments})
	if err != nil {
		t.Error = err.Error()
		return
	}
	t.MCPResponse = response
	t.MCPError = response.IsError
	data, err := json.Marshal(response.StructuredContent)
	if err == nil {
		err = json.Unmarshal(data, &t.Result)
	}
	if err != nil {
		t.Error = err.Error()
	}
	return
}
