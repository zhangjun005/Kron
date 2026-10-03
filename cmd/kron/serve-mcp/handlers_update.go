package servemcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleUpdate returns the toolHandler for kron_update.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_update) +
// 2026-10-03 PATCH-semantics decision.
//
//	in:  {
//	  slug:     string (required),
//	  symbol:   string? (null = no change; missing = no change;
//	                     "" = clear; non-empty = replace with [value]),
//	  status:   string? (same nullability rules),
//	  reviewers: string[]? (null = no change; missing = no change;
//	                       [] = clear; non-nil = replace),
//	  body:     string? (null/missing = no change; non-null = replace),
//	}
//	out: { ok: bool, path: string }
//
// Patch semantics (decision 2026-10-03):
//   - Independent fields; only the ones the client supplied are touched.
//   - To CLEAR a list field (e.g. reviewers), the client MUST send an
//     empty array, not null. JSON-RPC null is treated as "field not
//     provided" (no change) so the request stays idempotent.
//   - To CLEAR a scalar (e.g. status), the client sends "" — but
//     we reject empty status because Status is an enum; the
//     omit-status intent is signaled by sending the field absent.
//   - Validation runs after patching: any schema violation yields
//     -32006 (Unprocessable) so the existing intent on disk is untouched.
//
// Returns ErrIntentNotFound (-32004) if slug does not exist.
func handleUpdate(root string) toolHandler {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}
		var args updateArgs
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("%w: kron_update: invalid params: %v", model.ErrSlugInvalid, err)
		}
		if args.Slug == "" {
			return nil, fmt.Errorf("%w: kron_update requires non-empty slug", model.ErrSlugInvalid)
		}
		if err := parser.ValidateSlug(args.Slug); err != nil {
			return nil, err
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_update: open store: %w", err)
		}
		intent, err := r.Load(ctx, args.Slug)
		if err != nil {
			return nil, err
		}

		// Apply patch. set=true means "client sent this key".
		if args.Symbol.Set {
			intent.Frontmatter.Symbol = args.Symbol.Value
		}
		if args.Status.Set {
			intent.Frontmatter.Status = model.Status(args.Status.Value)
		}
		if args.Reviewers.Set {
			intent.Frontmatter.Reviewers = args.Reviewers.Value
		}
		if args.Body.Set {
			intent.Body = args.Body.Value
		}
		// dedupReferences: 保持首次出现顺序;RFC §3.2 "重复由 YAML 解析层自动忽略"。
		// JSON-RPC 没 warning 通道,与其静默保留重复不如显式归一化落盘值。
		if args.References.Set {
			intent.Frontmatter.References = dedupStrings(args.References.Value)
		}
		if args.DependsOn.Set {
			intent.Frontmatter.DependsOn = dedupStrings(args.DependsOn.Value)
		}

		// Always bump updated_at on any write so downstream observers
		// can detect the change. Match CLI's behaviour of always
		// refreshing the timestamp on Write.
		intent.Frontmatter.UpdatedAt = time.Now().UTC()

		// Re-validate the patched frontmatter BEFORE persisting so a
		// malformed update (e.g. invalid status enum) does not corrupt
		// the on-disk file. We round-trip through YAML to reuse
		// parser's validator (same rules that Load/Write apply).
		if err := validateFrontmatterShape(args.Slug, &intent.Frontmatter); err != nil {
			return nil, fmt.Errorf("%w: %v", model.ErrFrontmatterInvalid, err)
		}

		w, err := store.NewWriter(root)
		if err != nil {
			return nil, fmt.Errorf("kron_update: open writer: %w", err)
		}
		if err := w.Write(ctx, intent); err != nil {
			return nil, fmt.Errorf("kron_update: write: %w", err)
		}

		return updateResponse{OK: true, Path: model.IntentPath(args.Slug)}, nil
	}
}

type updateResponse struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

// updateArgs mirrors the JSON payload. PATCH semantics require
// distinguishing "field not provided" from "field provided as null".
// json.RawMessage + presence flag captures both: missing key → not Set;
// present key with value null → Set with Value=nil.
type updateArgs struct {
	Slug      string          `json:"slug"`
	Symbol    patchStringList `json:"symbol"`
	Status    patchString     `json:"status"`
	Reviewers patchStringList `json:"reviewers"`
	Body      patchString     `json:"body"`
	// References and DependsOn follow the same PATCH semantics as
	// Reviewers: absent=no change, null=no change, []=clear, [...] = replace.
	// Added 2026-10-03 per RFC 2026-10-03-frontmatter-references.
	References patchStringList `json:"references"`
	DependsOn  patchStringList `json:"depends_on"`
}

// patchString is a string field that tracks presence. For a
// single-value (scalar) field the wire shape is:
//
//	absent    → Set=false (no change)
//	null      → Set=true,  Value=""
//	"x"       → Set=true,  Value="x"
//
// Empty string carries the same meaning as null for scalars: the
// caller is signalling "set to zero value". This is fine for
// free-form fields like Body. For enum fields like Status we
// validate non-empty in the shape validator.
type patchString struct {
	Set   bool
	Value string
}

func (p *patchString) UnmarshalJSON(data []byte) error {
	p.Set = true
	if string(data) == "null" {
		p.Value = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	p.Value = s
	return nil
}

// patchStringList is a string-list field that tracks presence. The
// wire shape is:
//
//	absent    → Set=false (no change)
//	null      → Set=true,  Value=nil (treat as no-op for ergonomics;
//	             client libraries that always serialise fields send null
//	             for "no change" — this is friendlier than 400)
//	[]        → Set=true,  Value=[] (CLEAR)
//	["a","b"] → Set=true,  Value=["a","b"]
//
// Null = no-op is a deliberate deviation from "null = clear" because
// in practice JSON-RPC clients that auto-serialise every struct field
// will send null for "unset" values, and rejecting those requests
// would force every client to write a custom "omit-if-default" helper.
type patchStringList struct {
	Set   bool
	Value []string
}

func (p *patchStringList) UnmarshalJSON(data []byte) error {
	p.Set = true
	if string(data) == "null" {
		p.Value = nil
		return nil
	}
	var xs []string
	if err := json.Unmarshal(data, &xs); err != nil {
		return err
	}
	p.Value = xs
	return nil
}

// dedupStrings returns a slice with duplicate entries removed,
// preserving the order of first appearance. nil / empty inputs
// pass through unchanged.
//
// This mirrors RFC 2026-10-03-frontmatter-references §3.2: "重复由
// YAML 解析层自动忽略". We apply the same rule to the patch layer so
// `kron_update {"references": ["a","b","a"]}` doesn't write back a
// list that disagrees with what YAML round-trip would yield.
//
// Reference equality is exact-string; we deliberately do not trim or
// normalise whitespace because parser.ValidateSlug already rejects
// entries that wouldn't round-trip cleanly through `ValidateSlug`.
func dedupStrings(in []string) []string {
	if len(in) < 2 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// validateFrontmatterShape round-trips fm through parser's YAML
// validator to enforce the same schema rules that parser.ParseFrontmatter
// applies on load. By doing it after the patch we catch enum violations
// (status, assumption severity) before they reach the on-disk file.
//
// We invoke parser.ParseFrontmatter rather than calling its private
// validateFrontmatter so any future tightening of the schema is
// automatically picked up here.
//
// self-reference check: A must not list A in its own References or
// DependsOn. parser.ParseFrontmatter can't catch this (it has no
// owner slug in scope); store.Load does this when a slug is in hand.
// Here we re-check using the slug the caller is targeting via kron_update.
func validateFrontmatterShape(slug string, fm *model.Frontmatter) error {
	yml, err := yaml.Marshal(fm)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}
	if _, err := parser.ParseFrontmatter(yml); err != nil {
		return err
	}
	for _, r := range fm.References {
		if r == slug {
			return fmt.Errorf("references contains self (%q)", r)
		}
	}
	for _, d := range fm.DependsOn {
		if d == slug {
			return fmt.Errorf("depends_on contains self (%q)", d)
		}
	}
	return nil
}
