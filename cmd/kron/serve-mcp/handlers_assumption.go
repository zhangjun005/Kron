package servemcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/xxx/kron/internal/assumption"
	"github.com/xxx/kron/internal/identity"
	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/relations"
)

// =====================================================================
// kron_assume_create
// =====================================================================

// HandleAssumptionCreate returns the mcp.ToolHandlerFor binding for
// kron_assume_create.
//
// Wire contract:
//
//	in:  { id, text, default_severity, body? }
//	out: { ok: bool, path: string }
//
// Returns ErrAssumptionExists (→ IsError=true) if the file already
// exists. The .kron/ skeleton must already exist (kron_init must have
// been called); otherwise ErrConfigInvalid (→ IsError=true) is returned.
func HandleAssumptionCreate(ctx context.Context, _ *mcp.CallToolRequest, in AssumptionCreateInput) (
	*mcp.CallToolResult, AssumptionCreateOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AssumptionCreateOutput{}, err
	}
	if err := parser.ValidateSlug(in.ID); err != nil {
		return nil, AssumptionCreateOutput{}, err
	}
	if in.Text == "" {
		return nil, AssumptionCreateOutput{}, fmt.Errorf("kron_assume_create: text is required")
	}
	sev := model.Severity(in.DefaultSeverity)
	if sev != model.SeverityHard && sev != model.SeveritySoft {
		return nil, AssumptionCreateOutput{}, fmt.Errorf("kron_assume_create: default_severity must be hard or soft, got %q", in.DefaultSeverity)
	}

	root := repoRootFrom(ctx)

	// Ensure .kron/assumptions/ exists so assumption.NewWriter can
	// create the sub-dirs it needs. Mirrors kron_init idempotent
	// behaviour; mkdirAll is safe to call repeatedly.
	if err := ensureAssumptionDir(root); err != nil {
		return nil, AssumptionCreateOutput{}, err
	}

	w, err := assumption.NewWriter(root)
	if err != nil {
		return nil, AssumptionCreateOutput{}, fmt.Errorf("kron_assume_create: open writer: %w", err)
	}

	handle := identity.Handle("", identity.GitUser())
	body := ""
	if in.Body != "" {
		body = in.Body
	}

	createParams := assumption.CreateParams{
		ID:              in.ID,
		Text:            in.Text,
		DefaultSeverity: sev,
		CreatedBy:       handle,
		Body:            &body,
	}
	if err := w.Create(ctx, createParams); err != nil {
		return nil, AssumptionCreateOutput{}, err
	}
	InvalidateCache(root)
	return nil, AssumptionCreateOutput{
		OK:   true,
		Path: fmt.Sprintf(".kron/assumptions/%s.md", in.ID),
	}, nil
}

// =====================================================================
// kron_assume_list
// =====================================================================

// HandleAssumptionList returns the mcp.ToolHandlerFor binding for
// kron_assume_list.
//
// Wire contract:
//
//	in:  { status?: "active" | "superseded" }
//	out: { assumptions: [{id, text, default_severity, status,
//	                       created_by, updated_at, expires_at,
//	                       verified_at, verified_by, reviewers}] }
//
// If status is absent or "active", returns only non-superseded
// assumptions. If "superseded", returns only superseded entries.
// Returns an empty array (not an error) if no assumptions exist.
func HandleAssumptionList(ctx context.Context, _ *mcp.CallToolRequest, in AssumptionListInput) (
	*mcp.CallToolResult, AssumptionListOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AssumptionListOutput{}, err
	}
	root := repoRootFrom(ctx)

	r, err := assumption.NewReader(root)
	if err != nil {
		return nil, AssumptionListOutput{}, err
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, AssumptionListOutput{}, fmt.Errorf("kron_assume_list: list: %w", err)
	}

	filterSuperseded := in.Status == "superseded"
	var out []AssumptionSummary
	for _, af := range all {
		fm := af.Frontmatter
		if filterSuperseded && fm.Status != model.StatusSuperseded {
			continue
		}
		if !filterSuperseded && fm.Status == model.StatusSuperseded {
			continue
		}
		summary := AssumptionSummary{
			ID:              fm.ID,
			Text:            fm.Text,
			DefaultSeverity: string(fm.DefaultSeverity),
			Status:          string(fm.Status),
			CreatedBy:       fm.CreatedBy,
			UpdatedAt:       fm.UpdatedAt,
			ExpiresAt:       fm.ExpiresAt,
			VerifiedAt:      fm.VerifiedAt,
			VerifiedBy:      fm.VerifiedBy,
			Reviewers:       fm.Reviewers,
		}
		out = append(out, summary)
	}
	if out == nil {
		out = []AssumptionSummary{}
	}
	return nil, AssumptionListOutput{Assumptions: out}, nil
}

// =====================================================================
// kron_assume_update
// =====================================================================

// HandleAssumptionUpdate returns the mcp.ToolHandlerFor binding for
// kron_assume_update.
//
// Wire contract:
//
//	in:  { id, text?, default_severity?, body?, status?,
//	       expires_at?, verified_at?, verified_by?, reviewers? }
//	out: { ok: bool, path: string }
//
// PATCH semantics: nil / absent = no change. Nil slice = clear.
// Returns ErrAssumptionNotFound (→ IsError=true) if the file does not
// exist.
func HandleAssumptionUpdate(ctx context.Context, _ *mcp.CallToolRequest, in AssumptionUpdateInput) (
	*mcp.CallToolResult, AssumptionUpdateOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AssumptionUpdateOutput{}, err
	}
	if err := parser.ValidateSlug(in.ID); err != nil {
		return nil, AssumptionUpdateOutput{}, err
	}

	root := repoRootFrom(ctx)
	handle := identity.Handle("", identity.GitUser())

	w, err := assumption.NewWriter(root)
	if err != nil {
		return nil, AssumptionUpdateOutput{}, fmt.Errorf("kron_assume_update: open writer: %w", err)
	}

	patch := assumption.UpdatePatch{Actor: handle}
	if in.Text != nil {
		patch.Text = in.Text
	}
	if in.DefaultSeverity != nil {
		sev := model.Severity(*in.DefaultSeverity)
		if sev != model.SeverityHard && sev != model.SeveritySoft {
			return nil, AssumptionUpdateOutput{}, fmt.Errorf("kron_assume_update: default_severity must be hard or soft, got %q", *in.DefaultSeverity)
		}
		patch.DefaultSeverity = &sev
	}
	if in.Body != nil {
		patch.Body = in.Body
	}
	if in.Status != nil {
		st := model.Status(*in.Status)
		if st != model.StatusDraft && st != model.StatusActive && st != model.StatusSuperseded {
			return nil, AssumptionUpdateOutput{}, fmt.Errorf("kron_assume_update: status must be draft, active, or superseded, got %q", *in.Status)
		}
		patch.Status = &st
	}
	if in.ExpiresAt != nil {
		patch.ExpiresAt = in.ExpiresAt
	}
	if in.VerifiedAt != nil {
		patch.VerifiedAt = in.VerifiedAt
	}
	if in.VerifiedBy != nil {
		patch.VerifiedBy = in.VerifiedBy
	}
	if in.Reviewers != nil {
		patch.Reviewers = &in.Reviewers
	}

	if err := w.Update(ctx, in.ID, patch); err != nil {
		return nil, AssumptionUpdateOutput{}, err
	}
	InvalidateCache(root)
	return nil, AssumptionUpdateOutput{
		OK:   true,
		Path: fmt.Sprintf(".kron/assumptions/%s.md", in.ID),
	}, nil
}

// =====================================================================
// kron_assume_delete
// =====================================================================

// HandleAssumptionDelete returns the mcp.ToolHandlerFor binding for
// kron_assume_delete.
//
// Wire contract:
//
//	in:  { id }
//	out: { ok: bool, trashed_path: string,
//	       referencing_intents: [slug] }
//
// Soft-deletes by moving .kron/assumptions/<id>.md to
// .kron/.trash/assumptions/<id>.md and setting Status=superseded.
// referencing_intents is the sorted list of intents that reference
// this assumption ID in their frontmatter.Assumptions[].id at the
// time of deletion. G-6: this uses internal/relations.ReferencingIntents
// to compute the reverse view before the file is moved.
//
// Returns ErrAssumptionNotFound (→ IsError=true) if the file does not
// exist. Deletion of an already-trashed assumption returns
// ErrAssumptionAlreadyDeleted (→ IsError=true).
func HandleAssumptionDelete(ctx context.Context, _ *mcp.CallToolRequest, in AssumptionDeleteInput) (
	*mcp.CallToolResult, AssumptionDeleteOutput, error,
) {
	if err := assertCallerMCP(ctx); err != nil {
		return nil, AssumptionDeleteOutput{}, err
	}
	if err := parser.ValidateSlug(in.ID); err != nil {
		return nil, AssumptionDeleteOutput{}, err
	}

	root := repoRootFrom(ctx)
	handle := identity.Handle("", identity.GitUser())

	// Compute referencing intents BEFORE moving the file, mirroring
	// the same pattern as kron_delete: the list is a courtesy warning,
	// not a hard block.
	var referencingIntents []string
	if all, lerr := LoadAllCached(ctx, root); lerr == nil {
		referencingIntents = relations.ReferencingIntents(all, in.ID)
	}

	w, err := assumption.NewWriter(root)
	if err != nil {
		return nil, AssumptionDeleteOutput{}, fmt.Errorf("kron_assume_delete: open writer: %w", err)
	}
	if err := w.Delete(ctx, in.ID, handle); err != nil {
		return nil, AssumptionDeleteOutput{}, err
	}

	sort.Strings(referencingIntents)
	return nil, AssumptionDeleteOutput{
		OK:                 true,
		TrashedPath:        fmt.Sprintf(".kron/.trash/assumptions/%s.md", in.ID),
		ReferencingIntents: referencingIntents,
	}, nil
}

// =====================================================================
// shared helpers
// =====================================================================

// ensureAssumptionDir creates .kron/assumptions/ if it does not exist.
// Safe to call repeatedly (mkdirAll is idempotent). This mirrors the
// idempotent behaviour of kron_init and ensures that assumption.NewWriter
// can create its .trash/ sub-dir on top of a pre-existing .kron/ root.
func ensureAssumptionDir(root string) error {
	asmDir := filepath.Join(root, model.KronDir, "assumptions")
	if err := os.MkdirAll(asmDir, 0o755); err != nil {
		return fmt.Errorf("ensure assumption dir: mkdir %s: %w", asmDir, err)
	}
	return nil
}
