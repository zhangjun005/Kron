package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
)

// This file centralises the slug → on-disk path mapping that
// store.Writer and store.Reader share. Both the writer and the
// reader need the same shape decisions:
//
//	kind=leaf, slug="auth"          →  .kron/intents/auth.md
//	kind=node, slug="auth"          →  .kron/intents/auth/README.md
//	kind=leaf, slug="auth/jwt"      →  .kron/intents/auth/jwt.md
//	kind=node, slug="auth/login"    →  .kron/intents/auth/login/README.md
//	(kind=any) slug="README"        →  .kron/intents/README.md (top-level, NEVER README/README.md)
//
// The split is:
//
//	resolveWritePath(root, kind, slug) → absolute path or ErrSlugCollision
//	resolveReadPath(root, slug)        → absolute path or ErrIntentNotFound
//	                                  (already in reader.go as resolveIntentPath;
//	                                   see resolveReadPath below for the
//	                                   shared helper that resolveIntentPath
//	                                   delegates to)
//	KindFromPath(absPath)              → model.IntentKind inferred from
//	                                  a file's on-disk path (used by
//	                                  Reader.Load to backfill Intent.Kind)
//
// RFC: 2026-10-08-writer-readme-symmetry §3.3
// RFC: 2026-10-08-intent-tree-api      §3.1 A1 (the read side)

// resolveWritePath returns the absolute on-disk path for an intent
// being WRITTEN under the given kind. The two physical forms
// (leaf / node) are mutually exclusive per slug — if a file in
// the OTHER form already exists, returns ErrSlugCollision.
//
// Decision table:
//
//	kind="", kind=leaf  → <root>/.kron/intents/<slug>.md
//	kind=node           → <root>/.kron/intents/<slug>/README.md
//
// Edge case: slug == "README" (top-level node intent) is treated
// as a leaf-shaped write to ".kron/intents/README.md". A node
// intent at the root is still physically a README.md, so the
// shape is the same; we special-case the path to avoid creating
// a ".kron/intents/README/README.md" recursion.
//
// If the OTHER form already exists on disk, returns ErrSlugCollision
// so the caller can decide (surface to user, move existing file,
// or pick a different slug).
//
// If the same form already exists, returns the path WITHOUT error
// — the write caller's "exists" check is the right gate; this
// function only knows about shape, not presence.
func resolveWritePath(root string, kind model.IntentKind, slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("store: slug is empty")
	}
	if !kind.Valid() {
		return "", fmt.Errorf("store: invalid intent kind %q (must be leaf|node)", kind)
	}

	// Top-level README.md is special: always leaf-shaped.
	// This avoids ".kron/intents/README/README.md" recursion and
	// matches the read-side collapseReadmeSlug() which maps
	// "README" → "README" verbatim.
	if slug == "README" {
		return filepath.Join(root, model.KronDir, model.IntentDir, "README.md"), nil
	}

	var target string
	if kind == model.IntentKindNode {
		target = filepath.Join(root, model.KronDir, model.IntentDir, slug, "README.md")
	} else {
		// Default (empty + leaf) — canonical <slug>.md form.
		target = filepath.Join(root, model.KronDir, model.IntentDir, slug+model.IntentExtension)
	}

	// Reject if the OTHER form is already on disk.
	var other string
	if kind == model.IntentKindNode {
		other = filepath.Join(root, model.KronDir, model.IntentDir, slug+model.IntentExtension)
	} else {
		other = filepath.Join(root, model.KronDir, model.IntentDir, slug, "README.md")
	}

	if _, err := os.Stat(other); err == nil {
		return "", fmt.Errorf("%w: slug %q already exists as %s", ErrSlugCollision, slug, otherFormLabel(kind))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("store: stat %s: %w", other, err)
	}

	return target, nil
}

// otherFormLabel returns a short label for the on-disk form that
// the caller did NOT request, used in ErrSlugCollision messages.
func otherFormLabel(kind model.IntentKind) string {
	if kind == model.IntentKindNode {
		return "leaf (.md)"
	}
	return "node (README.md)"
}

// resolveReadPath is the shared helper backing Reader.resolveIntentPath.
// It implements the README shorthand: try canonical <slug>.md first,
// fall back to <slug>/README.md if absent. This is the read-side
// symmetric counterpart of resolveWritePath.
//
// The error contract matches the previous (pre-PR-B) behaviour:
//
//   - canonical exists     → canonical path, no error
//   - only README exists   → README path, no error
//   - neither exists       → ErrIntentNotFound (wrapped with slug)
//   - stat failure other
//     than NotExist         → wrapped fs error
//
// Special case: slug == "README" is treated as a leaf-shaped lookup
// of ".kron/intents/README.md" (the top-level node intent).
func resolveReadPath(root string, slug string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("store: slug is empty")
	}

	canonical := filepath.Join(root, model.KronDir, model.IntentDir, slug+model.IntentExtension)
	if _, err := os.Stat(canonical); err == nil {
		return canonical, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("store: stat %s: %w", canonical, err)
	}

	// Canonical missing — try the README shorthand.
	// Skip for top-level "README" (canonical form IS the README).
	if slug == "README" {
		return "", fmt.Errorf("%w: %s", model.ErrIntentNotFound, slug)
	}
	readme := filepath.Join(root, model.KronDir, model.IntentDir, slug, "README.md")
	if _, err := os.Stat(readme); err == nil {
		return readme, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("store: stat %s: %w", readme, err)
	}
	return "", fmt.Errorf("%w: %s", model.ErrIntentNotFound, slug)
}

// KindFromPath infers model.IntentKind from the on-disk filename.
// It is the read-side counterpart of the write-time Kind decision
// in resolveWritePath, and the canonical way to backfill
// Intent.Kind after a Load (RFC 2026-10-08-writer-readme-symmetry
// §3.3 + §6: Kind is a write-time routing decision that is NOT
// persisted to frontmatter, so the only authoritative source at
// read time is the file path).
//
// Decision:
//
//	<slug>/README.md  →  IntentKindNode
//	<slug>.md         →  IntentKindLeaf  (default; also matches
//	                                      top-level "README.md")
//	anything else     →  IntentKindLeaf  (defensive default; if a
//	                                      caller passes a non-intent
//	                                      path, returning Leaf is
//	                                      safer than a hard error
//	                                      because the type already
//	                                      defaults to "" which
//	                                      Valid() accepts as Leaf)
//
// The check is purely on the trailing path component, so it is
// O(1) and has no I/O. It works for any depth: "a/b/c/README.md"
// and "a/b/c.md" both return the right kind.
func KindFromPath(absPath string) model.IntentKind {
	if absPath == "" {
		return model.IntentKindLeaf
	}
	base := filepath.Base(absPath)
	if base != "README.md" {
		// Not a README.md at all — must be a leaf file
		// (<slug>.md). Covers "auth.md" and "a/b/c.md".
		return model.IntentKindLeaf
	}
	// base == "README.md": the intent's slug is the parent dir.
	// The top-level README.md (".kron/intents/README.md") is
	// leaf-shaped on disk per resolveWritePath (avoids the
	// README/README.md recursion). All deeper README.md files
	// are node-shaped anchors for their parent directory.
	//
	// We compare by the parent dir's base name to avoid coupling
	// to the absolute root the caller joined: the top-level
	// README.md is the one whose immediate parent directory's
	// base is the model.IntentDir.
	if filepath.Base(filepath.Dir(absPath)) == model.IntentDir {
		return model.IntentKindLeaf
	}
	return model.IntentKindNode
}
