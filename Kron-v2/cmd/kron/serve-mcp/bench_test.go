package servemcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Bench harness for the 13 kron_* tools. Goals:
//
//  1. Identify which tools scale poorly with repo size.
//  2. Surface tools that do redundant I/O (re-walk when a cached
//     view would do).
//  3. Provide a baseline (-count=3, ns/op) that future PRs can
//     regress against.
//
// Scaling axes: 10 / 100 / 500 intents. The 500-intent run is the
// primary regression target (real repos land in the 50-500 range).
// Each cell runs a single CallTool invocation; we measure
// end-to-end (including SDK transport + JSON-RPC envelope) so
// the numbers reflect what a real MCP client experiences.
//
// How to run:
//
//	go test ./cmd/kron/serve-mcp/ -bench=BenchTool -benchtime=3x -run=^$ -timeout=10m
//	go test ./cmd/kron/serve-mcp/ -bench=BenchTool -benchtime=3x -run=^$ -count=3
//
// The first form is fast (3 iterations per cell); the second is
// the multi-run variant used for the 2026-10-09 baseline report.

// --- fixtures -------------------------------------------------------

// seedNIntents creates a .kron/ tree with n intents. Intent slugs
// are "intent-<i>" and bodies contain a first H1 so kron_tree's
// title projection has something to render.
//
// Anchors are NOT seeded (so kron_impact.incoming_anchors and the
// anchor-walk portion of kron_lint exercise their empty path — a
// realistic "kron_lint on a brand-new repo" baseline).
func seedNIntentsT(t *testing.T, root string, n int) {
	t.Helper()
	seedNIntentsInto(t, root, n)
}

// seedNIntentsB is the *testing.B counterpart of seedNIntentsT.
func seedNIntentsB(b *testing.B, root string, n int) {
	b.Helper()
	seedNIntentsInto(b, root, n)
}

// seedNIntentsInto is the shared body for both. The two wrappers
// above exist only because *testing.T and *testing.B don't share
// a single interface we can pass to newTestSession; the body is
// otherwise identical.
func seedNIntentsInto(tb testing.TB, root string, n int) {
	ts := newTestSessionForTB(tb, root)
	defer ts.Cleanup()

	// kron_init first.
	_, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "kron_init", Arguments: map[string]any{},
	})
	if err != nil {
		tb.Fatalf("kron_init: %v", err)
	}

	// Seed intents via direct file write (faster than kron_add per
	// intent, and decouples "fixture setup" from "what we measure").
	// We still use the testdata sample intent shape so the parser
	// accepts them as valid.
	intentsDir := filepath.Join(root, ".kron", "intents")
	for i := 0; i < n; i++ {
		slug := fmt.Sprintf("intent-%04d", i)
		body := fmt.Sprintf("# Intent %04d\n\n## Why\n\nplaceholder\n", i)
		// Append body after sample wire.
		full := sampleIntentWire + body
		path := filepath.Join(intentsDir, slug+".md")
		if err := os.WriteFile(path, []byte(full), 0o644); err != nil {
			tb.Fatalf("write %s: %v", path, err)
		}
	}
}

// newTestSessionForTB dispatches to the right newTestSession
// based on whether the caller is a Test or Benchmark. Both go-sdk
// session constructors are otherwise identical — see
// newTestSession and newTestSessionFromB.
func newTestSessionForTB(tb testing.TB, root string) *testSession {
	if t, ok := tb.(*testing.T); ok {
		return newTestSession(t, root)
	}
	if b, ok := tb.(*testing.B); ok {
		return newTestSessionFromB(b, root)
	}
	panic(fmt.Sprintf("newTestSessionForTB: unsupported %T", tb))
}

// newTestSessionFromB mirrors newTestSession but uses *testing.B
// instead of *testing.T. The only methods newTestSession calls on
// its *testing.T argument are t.Helper and t.Fatalf — we replicate
// the call shape against *testing.B here so the bench harness can
// use the same session constructor. The cleanup is identical.
func newTestSessionFromB(b *testing.B, root string) *testSession {
	b.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "kron-bench", Version: "v1.2.0"}, nil)
	server.AddReceivingMiddleware(callerInjectMiddleware(root))
	registerTools(server)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		b.Fatalf("server.Connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "bench-client", Version: "v1.0.0"}, nil).
		Connect(context.Background(), ct, nil)
	if err != nil {
		_ = ss.Close()
		b.Fatalf("client.Connect: %v", err)
	}
	cleanup := func() {
		_ = cs.Close()
		_ = ss.Close()
	}
	return &testSession{Server: server, Session: cs, Cleanup: cleanup}
}

// benchCase names a single (tool, args) pair to benchmark. The
// args map is what the MCP client would send.
type benchCase struct {
	name string
	args map[string]any
}

// benchCases returns the 13 (tool, args) pairs we benchmark.
// Ordering is alphabetical-by-tool-name to match the canonical
// MCP tools/list ordering used by registerTools.
func benchCases() []benchCase {
	return []benchCase{
		{name: "kron_add", args: map[string]any{"slug": "bench-add"}}, // 0-intent case; ignored when intent count > 0
		{name: "kron_assume_check", args: map[string]any{"file_path": ""}},
		{name: "kron_delete", args: map[string]any{"slug": "intent-0000"}},
		{name: "kron_get", args: map[string]any{"slug": "intent-0000", "include_relations": true}},
		{name: "kron_impact", args: map[string]any{"slug": "intent-0000"}},
		{name: "kron_init", args: map[string]any{}},
		{name: "kron_intent_density", args: map[string]any{}},
		{name: "kron_lint", args: map[string]any{}},
		{name: "kron_list", args: map[string]any{}},
		{name: "kron_restore", args: map[string]any{"slug": "intent-0000"}},
		{name: "kron_stale", args: map[string]any{"days_threshold": 90}},
		{name: "kron_tree", args: map[string]any{}},
		{name: "kron_update", args: map[string]any{"slug": "intent-0000", "symbol": []string{"X"}}},
	}
}

// --- benchmark entry points ----------------------------------------

// BenchmarkTool10 runs every tool against a 10-intent fixture.
// Fastest run; use to spot "this tool is O(1) by construction"
// regressions.
func BenchmarkTool10(b *testing.B) { benchAll(b, 10) }

// BenchmarkTool100 runs every tool against a 100-intent fixture.
func BenchmarkTool100(b *testing.B) { benchAll(b, 100) }

// BenchmarkTool500 runs every tool against a 500-intent fixture.
// Primary regression target.
func BenchmarkTool500(b *testing.B) { benchAll(b, 500) }

// benchAll runs the full bench matrix for a given intent count.
// One sub-bench per tool; b.ResetTimer() after fixture setup so
// the seeding cost is excluded.
func benchAll(b *testing.B, n int) {
	cases := benchCases()

	for _, c := range cases {
		c := c
		b.Run(c.name, func(b *testing.B) {
			// Setup: build a fresh dir + session for EACH iteration.
			// The session has an InMemoryTransport pipe; reusing it
			// across iterations would confound transport cost with
			// tool cost. A fresh tempdir + Init keeps the only
			// shared cost (session construction) in the per-iter
			// overhead we want to measure.
			//
			// Why we do NOT just pre-seed and reuse: many tools
			// mutate the repo (kron_add, kron_delete, kron_update).
			// Re-running on a mutated repo would measure a
			// different state. A clean per-iter dir is the only
			// way to keep "this is a clean N-intent repo"
			// invariant.
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				dir := b.TempDir()
				seedNIntentsB(b, dir, n)
				ts := newTestSessionFromB(b, dir)
				b.ResetTimer()

				_, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
					Name:      c.name,
					Arguments: c.args,
				})
				b.StopTimer()
				ts.Cleanup()
				if err != nil {
					b.Fatalf("%s call: %v", c.name, err)
				}
			}
		})
	}
}

// --- aggregate timing report ---------------------------------------

// TestBenchReport is a non-benchmark test that runs each tool once
// against 10/100/500 intents and prints a per-tool timing table.
// Useful for one-off "where is my time going" investigations
// without going through -bench.
//
//	go test ./cmd/kron/serve-mcp/ -run TestBenchReport -v
//
// The numbers are wall-clock (not ns/op) so they include
// goroutine scheduling + GC, which is what a human operator
// experiences. Output is tab-separated so it's easy to copy into
// a spreadsheet or render as a markdown table.
func TestBenchReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping bench report in -short mode")
	}
	sizes := []int{10, 100, 500}
	tools := benchCases()

	// results[tool][size] = mean wall-clock over 3 runs.
	results := make(map[string]map[int]time.Duration, len(tools))
	for _, c := range tools {
		results[c.name] = make(map[int]time.Duration, len(sizes))
		for _, n := range sizes {
			results[c.name][n] = benchMean(t, c.name, c.args, n, 3)
		}
	}

	// Print markdown table.
	t.Log("\n=== kron_* tool timing (mean of 3 runs, wall-clock) ===")
	t.Logf("| tool | 10 intents | 100 intents | 500 intents |")
	t.Logf("|---|---|---|---|")
	// Stable order: alphabetical.
	names := make([]string, 0, len(tools))
	for _, c := range tools {
		names = append(names, c.name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Logf("| %s | %s | %s | %s |",
			name,
			formatDur(results[name][10]),
			formatDur(results[name][100]),
			formatDur(results[name][500]),
		)
	}

	// Print scaling ratio (500/10) — a tool that re-walks the repo
	// per call will show ratio > 5; a tool that precomputes will
	// show ratio ≈ 1-2.
	t.Log("\n=== scaling ratio (500/10) — high = re-walking, low = precomputed ===")
	t.Logf("| tool | 500/10 ratio |")
	t.Logf("|---|---|")
	for _, name := range names {
		r10 := results[name][10]
		r500 := results[name][500]
		if r10 == 0 {
			continue
		}
		ratio := float64(r500) / float64(r10)
		t.Logf("| %s | %.1fx |", name, ratio)
	}
}

// benchMean runs tool once n times and returns the mean wall-clock
// duration. We use time.Since (not b.N) because the Go benchmark
// framework's -benchtime convention doesn't apply inside a Test
// function — we want raw wall-clock so the report matches what a
// user "feels".
func benchMean(t *testing.T, tool string, args map[string]any, intentCount, runs int) time.Duration {
	t.Helper()
	durs := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		dir := t.TempDir()
		seedNIntentsT(t, dir, intentCount)
		ts := newTestSession(t, dir)
		start := time.Now()
		_, err := ts.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      tool,
			Arguments: args,
		})
		elapsed := time.Since(start)
		ts.Cleanup()
		if err != nil {
			t.Fatalf("%s call: %v", tool, err)
		}
		durs = append(durs, elapsed)
	}
	// Trimmed mean: drop highest, average the rest. Robust against
	// one noisy run on a shared CI box.
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	if len(durs) > 2 {
		durs = durs[:len(durs)-1]
	}
	var sum time.Duration
	for _, d := range durs {
		sum += d
	}
	return sum / time.Duration(len(durs))
}

// formatDur renders a duration as "1.2ms" / "350µs" / "1.5s" for
// the markdown table. Compact + column-aligned.
func formatDur(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.0fµs", float64(d.Microseconds()))
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
