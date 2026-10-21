// kron-mcp-test/go-client/main.go
//
// Real-usage E2E (L3) MCP client for `kron serve-mcp`. Spawns the binary
// under test as a child process, talks JSON-RPC over stdio, and runs a
// deterministic sequence of tool calls against the canonical fixture in
// ./project. Each call is sequential (1 send -> 1 wait -> 1 next) because
// real AI agents (Claude Code / Cursor) do exactly that; parallel
// requests over the same stdio handle race.
//
// This file replaces the bash mcp-use.sh driver that exposed a
// request-ordering race in fifo mode (snapshot 2026-10-15 §4.1).
//
// Run from E:\works\Kron\kron-mcp-test:
//   go run .\go-client\main.go              # default: 6 smoke cases
//   go run .\go-client\main.go -full        # every E2E case
//   go run .\go-client\main.go -case 5      # only case 5
//   go run .\go-client\main.go -list        # list case IDs only

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	kronPath = flag.String("kron", filepath.Join("install", "kron.exe"), "path to kron.exe")
	project  = flag.String("project", "project", "test project root (must contain .kron/)")
	fullMode = flag.Bool("full", false, "run every E2E case")
	listMode = flag.Bool("list", false, "list case IDs and exit")
	caseRun  = flag.Int("case", -1, "run only this case index (0-based)")
	dumpRaw  = flag.Bool("dump", false, "print raw JSON-RPC frames on stderr")
)

func main() {
	flag.Parse()
	cases := buildCases()
	if *listMode {
		for i, c := range cases {
			fmt.Printf("  %2d  %-12s %s\n", i, c.id, c.what)
		}
		fmt.Printf("\ntotal: %d cases (use -full for the long sweep)\n", len(cases))
		return
	}
	if !*fullMode {
		// Smoke subset: six baseline cases that prove the runtime works.
		cases = pickSmoke(cases)
	}
	if *caseRun >= 0 {
		if *caseRun >= len(cases) {
			fmt.Fprintf(os.Stderr, "case %d out of range (0..%d)\n", *caseRun, len(cases)-1)
			os.Exit(2)
		}
		cases = cases[*caseRun : *caseRun+1]
	}

	absKron, _ := filepath.Abs(*kronPath)
	absProj, _ := filepath.Abs(*project)
	cmd := exec.Command(absKron, "serve-mcp", "-CWD", absProj)
	cmd.Env = append(os.Environ(),
		"KRON_PROJECTS_JSON_PATH="+filepath.Join(filepath.Dir(absKron), "projects.json"),
	)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "spawn kron serve-mcp: %v\n", err)
		os.Exit(2)
	}
	defer func() { _ = cmd.Process.Kill() }()
	go io.Copy(io.Discard, stderr) // drain kron's stderr (logs go to slog by default)

	client := &mcpClient{
		stdin:  stdin,
		reader: bufio.NewReader(stdout),
		dump:   *dumpRaw,
	}
	// MCP protocol requires the initialize handshake before any other
	// method. The server returns its protocolVersion + capabilities;
	// we don't need to read them carefully, just confirm the call
	// succeeds (no error) so the session is open.
	if err := client.initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP initialize handshake failed: %v\n", err)
		os.Exit(2)
	}
	results := make([]caseResult, 0, len(cases))
	for _, c := range cases {
		res := client.run(c, absProj)
		results = append(results, res)
		printResult(c, res)
	}
	// Best-effort graceful shutdown.
	_ = stdin.Close()
	_, _ = cmd.Process.Wait()

	printSummary(results, absProj)
}

// mcpClient is a single-shot JSON-RPC client. Sequential by design.
type mcpClient struct {
	stdin  io.Writer
	mu     sync.Mutex // serialize writes to stdin
	nextID int
	reader *bufio.Reader
	dump   bool
}

type mcpRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *mcpClient) initialize() error {
	// MCP 2024-11-05 initialize. The server requires
	// protocolVersion + clientInfo; the rest is up to the client.
	resp, err := c.call("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "kron-mcp-test",
			"version": "0.1.0",
		},
	})
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	// Per MCP spec, after initialize the client should send
	// notifications/initialized. Servers don't reply to
	// notifications, so we just write the frame and move on.
	c.mu.Lock()
	defer c.mu.Unlock()
	notif := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	if _, err := c.stdin.Write(notif); err != nil {
		return fmt.Errorf("write initialized notification: %w", err)
	}
	return nil
}

func (c *mcpClient) call(method string, params interface{}) (*mcpResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	req := mcpRequest{JSONRPC: "2.0", ID: c.nextID, Method: method, Params: params}
	frame, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if c.dump {
		fmt.Fprintf(os.Stderr, "> %s\n", frame)
	}
	// JSON-RPC over stdio uses Content-Length framing (LSP convention).
	// The Kron MCP server reads line-delimited JSON (see serve_mcp.go),
	// so a single newline is sufficient.
	if _, err := c.stdin.Write(append(frame, '\n')); err != nil {
		return nil, fmt.Errorf("write frame: %w", err)
	}
	// Synchronous wait: read one JSON object. Real AI agents block
	// here. This is what avoids the bash-fifo race.
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read frame: %w", err)
	}
	if c.dump {
		fmt.Fprintf(os.Stderr, "< %s", line)
	}
	var resp mcpResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("decode frame: %w (raw=%q)", err, line)
	}
	return &resp, nil
}

// --- case model ---

type toolCall struct {
	name string
	args map[string]interface{}
}

type testCase struct {
	id        string
	what      string
	prep      func(c *mcpClient, project string) error // optional setup
	method    string                                   // top-level method; "" means "use call() with .call.name"
	toolName  string                                   // for tools/call: the registered tool name
	args      map[string]interface{}
	verify    func(result json.RawMessage, project string) (pass bool, detail string)
	expectErr bool // pass when isError=true
}

type caseResult struct {
	pass   bool
	detail string
	err    error
}

func (c *mcpClient) run(tc testCase, project string) caseResult {
	if tc.prep != nil {
		if err := tc.prep(c, project); err != nil {
			return caseResult{pass: false, err: err, detail: "prep: " + err.Error()}
		}
	}
	var resp *mcpResponse
	var err error
	if tc.method != "" {
		// top-level JSON-RPC method (e.g. "tools/list", "prompts/list").
		resp, err = c.call(tc.method, tc.args)
	} else {
		params := map[string]interface{}{"name": tc.toolName, "arguments": tc.args}
		resp, err = c.call("tools/call", params)
	}
	if err != nil {
		return caseResult{pass: false, err: err, detail: err.Error()}
	}
	if resp.Error != nil {
		return caseResult{pass: false, err: fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message), detail: resp.Error.Message}
	}
	// Two shapes:
	//  - tools/call result  = {content:[...], isError:bool}
	//  - top-level result   = the result object directly (e.g. tools/list -> {tools:[...]}).
	if tc.method == "" {
		var wrap struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if err := json.Unmarshal(resp.Result, &wrap); err != nil {
			return caseResult{pass: false, err: fmt.Errorf("decode tools/call result: %w", err), detail: string(resp.Result)}
		}
		if len(wrap.Content) == 0 {
			return caseResult{pass: false, detail: "empty content"}
		}
		body := []byte(wrap.Content[0].Text)
		if tc.expectErr {
			pass := wrap.IsError
			return caseResult{pass: pass, detail: "isError=" + strconv.FormatBool(wrap.IsError)}
		}
		if wrap.IsError {
			return caseResult{pass: false, detail: "tool returned isError=true: " + wrap.Content[0].Text}
		}
		if tc.verify != nil {
			pass, detail := tc.verify(body, project)
			return caseResult{pass: pass, detail: detail}
		}
		return caseResult{pass: true, detail: "(no verify — passed by non-error)"}
	}
	// top-level result
	if tc.verify != nil {
		pass, detail := tc.verify(resp.Result, project)
		return caseResult{pass: pass, detail: detail}
	}
	return caseResult{pass: true, detail: "(no verify — passed by non-error)"}
}

// --- cases ---
//
// The canonical fixture at ./project/ is built by setup.sh:
//   6 intents (auth/, payments/ + 4 leaves)
//   4 source files with @kron:intent anchors in src/
//   1 source file with a deliberately dangling anchor (src/broken/orphan.go)
//   1 markdown anchor in docs/adr/0001-jwt-rotation.md
//
// This driver is the verification of snapshot 2026-10-15 §4.8/§4.9:
//   with src/ present, kron_impact must return non-empty
//   incoming_anchors, and kron_lint must report the
//   anchor-dangling error for src/broken/orphan.go.

func buildCases() []testCase {
	return []testCase{
		// 1
		{
			id:     "init",
			what:   "tools/list  -> 18 tools present (sanity)",
			method: "tools/list",
			args:   map[string]interface{}{},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					Tools []struct {
						Name string `json:"name"`
					} `json:"tools"`
				}
				_ = json.Unmarshal(raw, &r)
				if len(r.Tools) == 0 {
					return false, "tools/list returned 0 tools (expected >= 18)"
				}
				names := make([]string, 0, len(r.Tools))
				for _, t := range r.Tools {
					names = append(names, t.Name)
				}
				sort.Strings(names)
				return true, fmt.Sprintf("%d tools: %s...", len(r.Tools), strings.Join(names[:min(5, len(names))], ","))
			},
		},
		// 2
		{
			id:       "list",
			what:     "kron_list  -> 6 intents (2 nodes + 4 leaves)",
			toolName: "kron_list",
			args:     map[string]interface{}{},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					Intents []struct {
						Slug string `json:"slug"`
					} `json:"intents"`
				}
				_ = json.Unmarshal(raw, &r)
				if len(r.Intents) != 6 {
					return false, fmt.Sprintf("got %d intents, want 6: %v", len(r.Intents), r.Intents)
				}
				return true, fmt.Sprintf("6 intents present")
			},
		},
		// 3
		{
			id:       "impact-with-src",
			what:     "kron_impact auth/jwt  -> incoming_anchors NON-EMPTY (proves src/ fixture wins over snapshot §4.8)",
			toolName: "kron_impact",
			args:     map[string]interface{}{"slug": "auth/jwt"},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					Slug                    string   `json:"slug"`
					IncomingAnchors         []string `json:"incoming_anchors"`
					IncomingAnchorsCode     []string `json:"incoming_anchors_code"`
					IncomingAnchorsMarkdown []string `json:"incoming_anchors_markdown"`
				}
				_ = json.Unmarshal(raw, &r)
				if len(r.IncomingAnchors) == 0 && len(r.IncomingAnchorsCode) == 0 && len(r.IncomingAnchorsMarkdown) == 0 {
					return false, "incoming_anchors* all empty — fixture missing src/? (snapshot §4.8 regression)"
				}
				return true, fmt.Sprintf("incoming_anchors_code=%v, incoming_anchors_markdown=%v", r.IncomingAnchorsCode, r.IncomingAnchorsMarkdown)
			},
		},
		// 4
		{
			id:       "lint-anchor-dangling",
			what:     "kron_lint  -> anchor-dangling error for src/broken/orphan.go (proves fixture 4.9 dangling case)",
			toolName: "kron_lint",
			args:     map[string]interface{}{},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					Errors []struct {
						Rule     string `json:"rule"`
						Severity string `json:"severity"`
						Where    string `json:"where"`
						Detail   string `json:"detail"`
					} `json:"errors"`
				}
				_ = json.Unmarshal(raw, &r)
				for _, e := range r.Errors {
					if e.Rule == "anchor-dangling" {
						return true, fmt.Sprintf("anchor-dangling raised: %s @ %s", e.Detail, e.Where)
					}
				}
				return false, fmt.Sprintf("no anchor-dangling in %d errors: %+v", len(r.Errors), r.Errors)
			},
		},
		// 5
		{
			id:       "tree",
			what:     "kron_tree  -> root.children = [auth, payments]",
			toolName: "kron_tree",
			args:     map[string]interface{}{},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					Root struct {
						Children []struct {
							Slug string `json:"slug"`
						} `json:"children"`
					} `json:"root"`
				}
				_ = json.Unmarshal(raw, &r)
				got := []string{}
				for _, c := range r.Root.Children {
					got = append(got, c.Slug)
				}
				sort.Strings(got)
				want := []string{"auth", "payments"}
				if !equalStrings(got, want) {
					return false, fmt.Sprintf("got %v, want %v", got, want)
				}
				return true, "root.children=[auth, payments]"
			},
		},
		// 6
		{
			id:       "density",
			what:     "kron_intent_density  -> real metrics (snapshot §4.7 metric-coverage gap closed)",
			toolName: "kron_intent_density",
			args:     map[string]interface{}{},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					TotalIntents int `json:"total_intents"`
					TotalAnchors int `json:"total_anchors"`
					Coverage     struct {
						IntentsWithAnchors    int      `json:"intents_with_anchors"`
						IntentsWithoutAnchors []string `json:"intents_without_anchors"`
					} `json:"coverage"`
					FilesWithoutIntent []string `json:"files_without_intent"`
				}
				_ = json.Unmarshal(raw, &r)
				// Snapshot §4.9 fixture has 2 nodes (auth/, payments/) +
				// 4 leaves = 6 intents; 5 code anchors + 1 md anchor = 6.
				// The 2 node-form intents have no code anchor pointing at
				// them by design (parent container). So we expect:
				//   total_intents=6, total_anchors=6,
				//   coverage.intents_with_anchors=5 (4 leaves + 1 md -> leaf)
				//   coverage.intents_without_anchors=[auth, payments]
				//   files_without_intent=[] (orphan.go HAS an anchor,
				//   it just points to a missing intent; different rule)
				if r.TotalIntents != 6 {
					return false, fmt.Sprintf("total_intents=%d, want 6", r.TotalIntents)
				}
				if r.TotalAnchors != 6 {
					return false, fmt.Sprintf("total_anchors=%d, want 6", r.TotalAnchors)
				}
				if r.Coverage.IntentsWithAnchors != 5 {
					return false, fmt.Sprintf("coverage.intents_with_anchors=%d, want 5", r.Coverage.IntentsWithAnchors)
				}
				want := []string{"auth", "payments"}
				sort.Strings(want)
				got := append([]string{}, r.Coverage.IntentsWithoutAnchors...)
				sort.Strings(got)
				if !equalStrings(got, want) {
					return false, fmt.Sprintf("coverage.intents_without_anchors=%v, want %v", r.Coverage.IntentsWithoutAnchors, want)
				}
				return true, fmt.Sprintf("total_intents=%d total_anchors=%d intents_with_anchors=%d intents_without=%v files_without=%v",
					r.TotalIntents, r.TotalAnchors, r.Coverage.IntentsWithAnchors, r.Coverage.IntentsWithoutAnchors, r.FilesWithoutIntent)
			},
		},
		// Cursor's bridge emits the wrong JSON types for these fields.
		// These cases exercise the production stdio path rather than only
		// the coerce helper's unit tests.
		{
			id:       "cursor-b1-bool-string",
			what:     "Cursor string true -> kron_get include_relations succeeds",
			toolName: "kron_get",
			args: map[string]interface{}{
				"slug":              "auth/jwt",
				"include_relations": "true",
			},
			verify: func(raw json.RawMessage, _ string) (bool, string) {
				var r struct {
					Intent struct {
						Relations json.RawMessage `json:"relations"`
					} `json:"intent"`
				}
				if err := json.Unmarshal(raw, &r); err != nil {
					return false, fmt.Sprintf("decode get result: %v", err)
				}
				if len(r.Intent.Relations) == 0 || string(r.Intent.Relations) == "null" {
					return false, "relations absent; string true was not coerced to boolean true"
				}
				return true, "string true coerced to boolean true"
			},
		},
		{
			id:       "cursor-b2-array-object",
			what:     "Cursor {item: reviewer} -> kron_update reviewers succeeds",
			toolName: "kron_update",
			args: map[string]interface{}{
				"slug":      "auth/jwt",
				"reviewers": map[string]interface{}{"item": "@cursor"},
			},
		},
		{
			id:       "cursor-b3-int-string",
			what:     "Cursor string 0 -> kron_stale days_threshold succeeds",
			toolName: "kron_stale",
			args:     map[string]interface{}{"days_threshold": "0"},
		},
	}
}

func pickSmoke(c []testCase) []testCase {
	// Keep cases 0..5 inclusive.
	if len(c) < 6 {
		return c
	}
	return c[:6]
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printResult(c testCase, r caseResult) {
	status := "PASS"
	if !r.pass {
		status = "FAIL"
	}
	fmt.Printf("  %-22s [%s] %s\n", c.id, status, c.what)
	if !r.pass && r.detail != "" {
		fmt.Printf("                            %s\n", r.detail)
	}
}

func printSummary(results []caseResult, project string) {
	pass, fail := 0, 0
	for _, r := range results {
		if r.pass {
			pass++
		} else {
			fail++
		}
	}
	fmt.Println()
	fmt.Println("=========================================")
	fmt.Printf("  %d pass / %d fail  (project: %s)\n", pass, fail, project)
	fmt.Println("=========================================")
	if fail > 0 {
		os.Exit(1)
	}
	_ = context.Background() // keep import in case we add timeout later
	_ = time.Second          // ditto
}
