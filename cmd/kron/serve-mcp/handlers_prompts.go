package servemcp

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/lint"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/relations"
)

// =====================================================================
// 3 Prompts — RFC docs/rfc/2026-10-14-mcp-prompts.md
// =====================================================================
//
// Why these 3 prompts (vs the originally proposed
// kron-assumption-audit / kron-stale-report pair in
// docs/mcp/missing-capabilities.md §C-2): the user (zhangjun005)
// renamed the set on 2026-10-14 to give the AI Agent a single
// vocabulary across the development loop:
//
//   - kron-implement   ← pick the right intent for a new feature
//   - kron-drift-check ← reconcile code changes against intent state
//   - kron-review      ← full active-intent audit for PR review
//
// The handlers all follow the same pattern:
//
//  1. assertCallerMCP(ctx)         — defensive guard (every tool does
//     this; the prompt dispatch path is the same SDK middleware
//     path, and the same caller injection. See serve_mcp.go +
//     tools.go.)
//  2. repoRootFrom(ctx)            — repo root flows in via the
//     same callerInjectMiddleware every tool uses.
//  3. LoadAllCached(ctx, root)     — singleflight + mtime cache
//     (see cache.go). Reuses the same LoadAll every tool uses, so
//     prompt rendering picks up the warm cache for free.
//  4. Compute, then construct a single user-role message that
//     embeds the rendered markdown (the AI client sees a
//     `## User Request` + `## Matched Intents` + `## Action` block;
//     see per-handler docs for the exact section names).
//
// Errors that come back from the handler are returned as the second
// return value; the SDK wraps them into the `error` field of the
// JSON-RPC response. Tool-level IsError semantics don't apply to
// prompts (MCP spec: prompts/get either returns the rendered
// messages or fails the whole call). We surface argument validation
// errors as the second return value too — that gives the AI caller
// a clean "missing required argument" message it can fix.

// kronImplementPrompt renders the kron-implement template.
//
// Wire contract: see docs/rfc/2026-10-14-mcp-prompts.md §2.1
//
//	in:  { arguments: { feature: string } }
//	out: { description, messages: [{ role: "user", content: { text: "..." } }] }
//
// `feature` is REQUIRED. Empty / missing → error.
//
// The handler picks the top-N intents whose slug / symbol / first
// frontmatter line most closely matches the `feature` string (case
// insensitive substring match, ranked by number of hits), then
// computes each match's `relations.Prerequisites` so the AI caller
// has the full "read first, then implement" picture. 0 matches is
// not an error — the rendered message says "no match, consider
// kron_add".
func kronImplementPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, err
	}
	feature := strings.TrimSpace(getArg(req, "feature"))
	if feature == "" {
		return nil, fmt.Errorf("kron-implement: feature argument is required")
	}

	root := repoRootFrom(ctx)
	all, err := LoadAllCached(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("kron-implement: load intents: %w", err)
	}

	matches := matchIntentsByKeyword(all, feature)
	ranked := make([]promptIntentMatch, 0, len(matches))
	for _, m := range matches {
		prereqs := relations.Prerequisites(all, m.Slug)
		ranked = append(ranked, promptIntentMatch{
			Slug:          m.Slug,
			Status:        string(m.Frontmatter.Status),
			Prerequisites: prereqs,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Slug < ranked[j].Slug
	})

	var body strings.Builder
	fmt.Fprintf(&body, "## User Request\n%s\n\n", feature)
	if len(ranked) == 0 {
		fmt.Fprintf(&body, "## Matched Intents\n(no existing intent matches %q; consider kron_add a new one)\n\n", feature)
	} else {
		body.WriteString("## Matched Intents\n")
		for _, m := range ranked {
			fmt.Fprintf(&body, "- slug: %s  status: %s\n  prerequisites: %s\n", m.Slug, m.Status, joinOrNone(m.Prerequisites))
		}
		body.WriteString("\n")
	}
	body.WriteString("## Action\n1. Read each matched intent + its prerequisites via kron_get.\n2. If a prerequisite is missing, kron_add it first.\n3. Apply changes via kron_update (PATCH semantics).\n")

	return &mcp.GetPromptResult{
		Description: "Pick the right intent to implement a new feature, with prerequisites.",
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: body.String()},
		}},
	}, nil
}

// kronDriftCheckPrompt renders the kron-drift-check template.
//
// Wire contract: see docs/rfc/2026-10-14-mcp-prompts.md §2.1
//
//	in:  { arguments: { days_threshold?: string (decimal) } }
//	out: { description, messages: [{ role: "user", content: { text: "..." } }] }
//
// `days_threshold` is optional; missing / unparseable → default 90.
//
// The handler surfaces the kron_stale output: superseded candidates
// (active intents older than threshold) and expired hard
// assumptions. The message frames it as "what the AI should update
// / verify" so the AI Agent has a checklist to walk after editing
// code.
func kronDriftCheckPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, err
	}
	threshold := parseIntArg(getArg(req, "days_threshold"), 90)

	root := repoRootFrom(ctx)

	// Mirror kron_stale's data path: ComputeStaleReport over the
	// loaded set to get the structured view (SupersededCandidates
	// + ExpiredAssumptions). ComputeStaleReport is the structured
	// sibling of RunWith (see internal/lint/lint.go:331-380).
	all, err := LoadAllCached(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("kron-drift-check: load intents: %w", err)
	}
	ar, arErr := assumption.NewReader(root)
	if arErr != nil {
		// No registry is fine — the stale signal just degrades to
		// "active-old" without expired-assumption drill-down.
		ar = nil
	}
	report := lint.ComputeStaleReport(all, lint.RunOptions{
		StaleDaysThreshold: threshold,
	}.WithNow(time.Now().UTC()))
	_ = ar // ar isn't threaded through ComputeStaleReport (the
	// structured view takes the already-loaded intents only; the
	// full lint.RunWith path is the one that consumes AssumptionReader,
	// but we want the structured view here, not the diag list).

	// Translate lint's expired-assumption findings into a sorted
	// list of "what's drifted" for the prompt.
	expired := make([]string, 0, len(report.ExpiredAssumptions))
	for _, e := range report.ExpiredAssumptions {
		expired = append(expired, e.AssumptionID)
	}
	sort.Strings(expired)

	var body strings.Builder
	fmt.Fprintf(&body, "## Drift Check (days_threshold=%d)\n\n", threshold)
	if len(report.SupersededCandidates) == 0 && len(expired) == 0 {
		body.WriteString("No drift detected.\n")
	} else {
		if len(report.SupersededCandidates) > 0 {
			body.WriteString("### Superseded candidates (active intents older than threshold)\n")
			for _, slug := range report.SupersededCandidates {
				fmt.Fprintf(&body, "- %s\n", slug)
			}
			body.WriteString("\n")
		}
		if len(expired) > 0 {
			body.WriteString("### Expired hard assumptions\n")
			for _, id := range expired {
				fmt.Fprintf(&body, "- %s\n", id)
			}
			body.WriteString("\n")
		}
	}
	body.WriteString("## Action\n1. For each superseded candidate, decide: kron_update (status=superseded + reason) or kron_update (refresh updated_at).\n2. For each expired hard assumption, verify or relax severity via kron_assume_update.\n")

	return &mcp.GetPromptResult{
		Description: "Surface code-vs-intent drift (stale intents + expired hard assumptions).",
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: body.String()},
		}},
	}, nil
}

// kronReviewPrompt renders the kron-review template.
//
// Wire contract: see docs/rfc/2026-10-14-mcp-prompts.md §2.1
//
//	in:  { arguments: { status?: string } }
//	out: { description, messages: [{ role: "user", content: { text: "..." } }] }
//
// `status` is optional; missing / unparseable → default "active".
//
// The handler runs lint over the loaded set and groups findings
// by rule so the AI reviewer has a per-rule checklist. The
// message also lists every active intent's slug + first H1 so the
// AI has a "what's in scope" preview at the top.
func kronReviewPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, err
	}
	status := strings.TrimSpace(getArg(req, "status"))
	if status == "" {
		status = "active"
	}

	root := repoRootFrom(ctx)
	all, err := LoadAllCached(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("kron-review: load intents: %w", err)
	}
	ar, arErr := assumption.NewReader(root)
	if arErr != nil {
		ar = nil
	}
	diags, lerr := lint.RunWith(ctx, root, lint.RunOptions{})
	if lerr != nil {
		return nil, fmt.Errorf("kron-review: lint: %w", lerr)
	}
	_ = ar // ar isn't needed here — RunWith internally re-opens the
	// assumption registry on its own (see internal/lint/lint.go).

	// Group findings by rule code.
	byRule := make(map[string][]string)
	for _, d := range diags {
		byRule[string(d.Rule)] = append(byRule[string(d.Rule)], d.Where)
	}
	ruleKeys := make([]string, 0, len(byRule))
	for k := range byRule {
		ruleKeys = append(ruleKeys, k)
	}
	sort.Strings(ruleKeys)

	var body strings.Builder
	fmt.Fprintf(&body, "## Review (status=%s)\n\n", status)

	body.WriteString("### In-scope intents\n")
	scoped := make([]string, 0, len(all))
	for _, intent := range all {
		if string(intent.Frontmatter.Status) == status {
			scoped = append(scoped, intent.Slug)
		}
	}
	sort.Strings(scoped)
	if len(scoped) == 0 {
		body.WriteString("(no intents match the status filter)\n\n")
	} else {
		for _, s := range scoped {
			fmt.Fprintf(&body, "- %s\n", s)
		}
		body.WriteString("\n")
	}

	body.WriteString("### Findings by rule\n")
	if len(byRule) == 0 {
		body.WriteString("No findings.\n")
	} else {
		for _, rule := range ruleKeys {
			locs := byRule[rule]
			sort.Strings(locs)
			fmt.Fprintf(&body, "- %s (%d): %s\n", rule, len(locs), joinOrNone(locs))
		}
	}
	body.WriteString("\n## Action\n1. For each finding, follow the rule-specific fix (kron_update / kron_assume_update / kron_add).\n2. After fixing, re-run kron_lint to confirm zero errors before merging.\n")

	return &mcp.GetPromptResult{
		Description: "Full active-intent audit for PR review.",
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: body.String()},
		}},
	}, nil
}

// =====================================================================
// shared helpers
// =====================================================================

// promptIntentMatch is the per-match row used in kron-implement's
// rendered output. The fields are kept tiny (slug + status +
// prerequisites) because the AI Agent reads the rest via
// kron_get; the prompt is the index, not the dump.
type promptIntentMatch struct {
	Slug          string
	Status        string
	Prerequisites []string
}

// matchIntentsByKeyword returns intents whose slug / frontmatter.symbol
// set / first frontmatter line match the keyword (case-insensitive
// substring). Sorted by hit count desc, then slug asc. Naive — the
// AI does the real semantic match in its own reasoning; we just give
// it a ranked index to read from.
func matchIntentsByKeyword(intents []*model.Intent, feature string) []*model.Intent {
	needle := strings.ToLower(feature)
	type scored struct {
		intent *model.Intent
		hits   int
	}
	var hits []scored
	for _, intent := range intents {
		score := 0
		if strings.Contains(strings.ToLower(intent.Slug), needle) {
			score += 3
		}
		for _, sym := range intent.Frontmatter.Symbol {
			if strings.Contains(strings.ToLower(sym), needle) {
				score += 2
			}
		}
		// First non-empty line of the body is usually a # title.
		for _, line := range strings.Split(intent.Body, "\n") {
			if strings.HasPrefix(line, "# ") && strings.Contains(strings.ToLower(line), needle) {
				score++
				break
			}
		}
		if score > 0 {
			hits = append(hits, scored{intent: intent, hits: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].hits != hits[j].hits {
			return hits[i].hits > hits[j].hits
		}
		return hits[i].intent.Slug < hits[j].intent.Slug
	})
	out := make([]*model.Intent, len(hits))
	for i, h := range hits {
		out[i] = h.intent
	}
	return out
}

// getArg returns the value of req's "arguments" map for key, or "" if
// the request / params / arguments are nil. The SDK delivers prompt
// arguments as map[string]string per the spec (see
// mcp.GetPromptParams.Arguments), so we don't need any coercion.
func getArg(req *mcp.GetPromptRequest, key string) string {
	if req == nil || req.Params == nil {
		return ""
	}
	return req.Params.Arguments[key]
}

// parseIntArg parses s as a decimal int; returns fallback on any
// error or empty string. Used for days_threshold on kron-drift-check.
func parseIntArg(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

// joinOrNone renders a []string for human-readable prompt output:
// empty slice → "(none)", non-empty → "a, b, c".
func joinOrNone(xs []string) string {
	if len(xs) == 0 {
		return "(none)"
	}
	return strings.Join(xs, ", ")
}
