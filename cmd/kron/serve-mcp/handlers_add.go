package servemcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/identity"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// HandleAdd returns the mcp.ToolHandlerFor binding for kron_add.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_add).
//
//	in:  { slug, symbol?, why?, kind? }
//	out: { ok: bool, path: string }
//
// "symbol" populates Frontmatter.Symbol; "why" is appended to a
// scaffolded body that follows the same shape as CLI `kron add`.
// "kind" selects the on-disk physical form (leaf or node) — see
// RFC 2026-10-08-writer-readme-symmetry §3.5.
//
// The .kron/ skeleton must already exist; otherwise -32006
// (Unprocessable) is returned. Reusing an existing slug returns
// -32005 (Conflict), matching the mcp.md contract for ErrIntentExists.
func HandleAdd(ctx context.Context, _ *mcp.CallToolRequest, in AddInput) (
	*mcp.CallToolResult, AddOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AddOutput{}, err
	}
	if in.Slug == "" {
		return nil, AddOutput{}, fmt.Errorf("%w: kron_add requires non-empty slug", model.ErrSlugInvalid)
	}
	if err := parser.ValidateSlug(in.Slug); err != nil {
		return nil, AddOutput{}, err
	}

	// Parse kind via model.IntentKind. Empty string is treated as
	// "leaf" (the default) for backwards compatibility with callers
	// that pre-date RFC 2026-10-08-writer-readme-symmetry.
	kind := model.IntentKind(in.Kind)
	if !kind.Valid() {
		return nil, AddOutput{}, fmt.Errorf("kron_add: invalid kind %q (must be leaf or node)", in.Kind)
	}
	if kind == "" {
		kind = model.IntentKindLeaf
	}

	root := repoRootFrom(ctx)
	kronDir := filepath.Join(root, model.KronDir)
	if _, err := os.Stat(kronDir); err != nil {
		if os.IsNotExist(err) {
			return nil, AddOutput{}, fmt.Errorf("%w: .kron/ not found; run kron_init first", model.ErrConfigInvalid)
		}
		return nil, AddOutput{}, fmt.Errorf("kron_add: stat %s: %w", kronDir, err)
	}

	w, err := store.NewWriter(root)
	if err != nil {
		return nil, AddOutput{}, fmt.Errorf("kron_add: open store: %w", err)
	}
	if w.Exists(ctx, in.Slug) {
		return nil, AddOutput{}, fmt.Errorf("%w: %s", model.ErrIntentExists, in.Slug)
	}

	// Identity is resolved via internal/identity: prefer an explicit
	// client-derived handle, fall back to git config user.name, fall
	// back to "agent:unknown". Identity is an attribute, not an
	// authz gate (architecture.md §2.3).
	handle := identity.Handle("", identity.GitUser())

	fm := model.Frontmatter{
		CreatedBy: handle,
		UpdatedAt: time.Now().UTC(),
	}
	if in.Symbol != "" {
		fm.Symbol = []string{in.Symbol}
	}

	// G-5: populate Frontmatter.Assumptions[] and auto-create
	// registry files for any IDs not yet in the registry.
	if len(in.Assumptions) > 0 {
		asmFw, err := maybeOpenAssumptionWriter(root)
		if err != nil {
			return nil, AddOutput{}, fmt.Errorf("kron_add: open assumption writer: %w", err)
		}
		asms, asmErrs := resolveAssumptionsForIntent(ctx, root, asmFw, handle, in.Assumptions)
		// Surface errors as a structured error; the first error wins.
		if len(asmErrs) > 0 {
			return nil, AddOutput{}, asmErrs[0]
		}
		fm.Assumptions = asms
	}

	body := parser.IntentBodyTemplate(in.Slug)
	if in.Why != "" {
		body = body + "\n## Why\n\n" + in.Why + "\n"
	}

	intent := &model.Intent{
		Slug:        in.Slug,
		Kind:        kind,
		Frontmatter: fm,
		Body:        body,
	}
	if err := w.Write(ctx, intent); err != nil {
		return nil, AddOutput{}, fmt.Errorf("kron_add: write: %w", err)
	}

	// Echo the path the user actually got. For node form this is
	// <slug>/README.md, not <slug>.md. Writer.Write sets
	// intent.SourcePath to the resolved absolute path; we
	// return a repo-relative string for the JSON contract.
	relPath, err := filepath.Rel(root, intent.SourcePath)
	if err != nil {
		// Fall back to model.IntentPath if rel fails (shouldn't
		// happen because both paths live under root).
		relPath = model.IntentPath(in.Slug)
	}
	// Invalidate the LoadAll cache so subsequent read handlers
	// pick up the new intent without a stale cached result.
	InvalidateCache(root)
	return nil, AddOutput{OK: true, Path: filepath.ToSlash(relPath)}, nil
}

// scaffoldedBody delegates to parser.IntentBodyTemplate. The wrapper
// is kept so the handler reads naturally; the actual template lives
// in internal/parser where it is shared with the CLI add path and
// any future access layer. The CLI uses the Chinese variant
// (parser.IntentBodyTemplateCN) to preserve its prior wording.
func scaffoldedBody(slug string) string {
	return parser.IntentBodyTemplate(slug)
}

// =====================================================================
// assumption helpers (shared with kron_update)
// =====================================================================

// maybeOpenAssumptionWriter returns a Writer for the assumption registry
// or nil if the .kron/assumptions/ directory does not exist yet.
// The nil return is not an error — it signals that no registry
// auto-creation is possible (pre-migration repos).
func maybeOpenAssumptionWriter(root string) (*assumption.Writer, error) {
	// Ensure the dir exists so NewWriter can set up .trash/.
	// Idempotent: MkdirAll is safe to call on an existing path.
	asmDir := filepath.Join(root, model.KronDir, "assumptions")
	if err := os.MkdirAll(asmDir, 0o755); err != nil {
		return nil, fmt.Errorf("assumption dir: %w", err)
	}
	w, err := assumption.NewWriter(root)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// resolveAssumptionsForIntent converts []AssumptionRef (wire) to
// []model.Assumption (internal) and auto-creates registry files
// for any IDs not yet in the registry. It is called by kron_add
// and kron_update when they receive non-empty assumptions[] in the
// input.
//
// rationale validation (non-empty; mirrors lint
// RuleAssumptionRationaleRequired) happens at write time so the
// caller gets an immediate "kron_add: assumption %q: rationale is
// required" rather than writing a partial entry only to be
// rejected by `kron_lint` on the next call. The lint rule stays
// in place for writers that bypass this helper (manual edits, the
// CLI `kron update` path) and for the historical "write first,
// fix later" workflow on partial drafts.
//
// Returns the resolved slice and any errors encountered during
// auto-creation. If errors exist, the caller surfaces the first
// one (fail-fast).
func resolveAssumptionsForIntent(ctx context.Context, root string, w *assumption.Writer, actor string, refs []AssumptionRef) ([]model.Assumption, []error) {
	if w == nil {
		// Pre-migration repo: no registry, use raw refs directly.
		out := make([]model.Assumption, 0, len(refs))
		for _, r := range refs {
			out = append(out, model.Assumption{
				ID:         r.ID,
				Severity:   model.Severity(r.Severity),
				Rationale:  r.Rationale,
				ExpiresAt:  r.ExpiresAt,
				VerifiedAt: r.VerifiedAt,
				VerifiedBy: r.VerifiedBy,
			})
		}
		return out, nil
	}

	// Check which IDs are new and need registry files.
	existingIDs := make(map[string]bool)
	if ar, arErr := assumption.NewReader(root); arErr == nil {
		all, _ := ar.List(ctx)
		for _, af := range all {
			existingIDs[af.Frontmatter.ID] = true
		}
	}

	created := make(map[string]bool)
	var errs []error

	for _, r := range refs {
		sev := model.Severity(r.Severity)
		if sev != model.SeverityHard && sev != model.SeveritySoft {
			errs = append(errs, fmt.Errorf("kron_add: assumption %q: severity must be hard or soft, got %q", r.ID, r.Severity))
			continue
		}
		if r.ID == "" {
			errs = append(errs, fmt.Errorf("kron_add: assumption id is required"))
			continue
		}
		// rationale-required: mirror the lint rule at write time
		// (see lint.RuleAssumptionRationaleRequired). The rule is
		// documented as "non-empty"; we reject whitespace-only as
		// well because the parse layer would happily round-trip
		// "   " as a 3-character rationale and the lint rule
		// would not catch the failure (its check is `== ""`, not
		// `len < 10` as the surrounding docs suggest). This
		// pre-write check is the load-bearing one; the lint
		// rule is a backstop for edits that bypass the helper.
		if strings.TrimSpace(r.Rationale) == "" {
			errs = append(errs, fmt.Errorf("kron_add: assumption %q: rationale is required", r.ID))
			continue
		}

		// Auto-create registry stub for new IDs.
		if !existingIDs[r.ID] && !created[r.ID] {
			stubText := fmt.Sprintf("Auto-created stub for assumption %q.", r.ID)
			createParams := assumption.CreateParams{
				ID:              r.ID,
				Text:            stubText,
				DefaultSeverity: sev,
				CreatedBy:       actor,
			}
			if err := w.Create(ctx, createParams); err != nil {
				// Already exists (race): not an error.
				if !errors.Is(err, assumption.ErrAssumptionExists) {
					errs = append(errs, fmt.Errorf("kron_add: auto-create assumption %q: %w", r.ID, err))
					continue
				}
			}
			created[r.ID] = true
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}

	// Convert wire refs to model.Assumptions.
	out := make([]model.Assumption, 0, len(refs))
	for _, r := range refs {
		out = append(out, model.Assumption{
			ID:         r.ID,
			Severity:   model.Severity(r.Severity),
			Rationale:  r.Rationale,
			ExpiresAt:  r.ExpiresAt,
			VerifiedAt: r.VerifiedAt,
			VerifiedBy: r.VerifiedBy,
		})
	}
	return out, nil
}
