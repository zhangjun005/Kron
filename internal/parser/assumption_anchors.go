package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxx/kron/internal/model"
)

// assumptionAnchorMarker is the line-level marker that an intent
// body uses to declare "this intent depends on assumption <id>".
//
// B-3 (Option B1 — keep structural reference): model.Frontmatter.
// Assumptions is the primary signal (per-intent severity, rationale,
// expires_at), but the orphan rule needs to know "which intent(s)
// use this assumption". An optional `@kron:assumes <id>` line in
// the intent body serves as a redundant, body-visible declaration
// for the orphan walker. Bodies may have any number of these
// lines, one per referenced assumption.
//
// Accepted forms (whitespace-tolerant, same shape as @kron:intent):
//
//	<!-- @kron:assumes single-region -->
//	// @kron:assumes csrf-protected
//	#  @kron:assumes auth/jwt-shared  (no — assumption IDs are flat,
//	                                  no slashes; this would be rejected)
//
// Assumption IDs are kebab-case slugs WITHOUT a "/" path separator
// (they map 1:1 to .kron/assumptions/<id>.md filenames), so the
// slug validation is the same as the assumption ID validation
// enforced by parser.ValidateSlug — same rules, same boundaries.
const assumptionAnchorMarker = "@kron:assumes"

// SlugsForAssumption walks the repo and returns the set of intent
// slugs that have a "@kron:assumes <id>" line in their body.
//
// This is the "who uses this assumption" reverse view that
// internal/lint.RuleAssumptionOrphan needs to detect an active
// assumption that is not referenced by any intent.
//
// The walk reaches .kron/intents/ (which the default
// parser.WalkSourceFiles skips). The skip list is narrower: we
// only avoid .git, .kron/.trash, .kron/assumptions, and the
// vendored noise directories. .kron/intents/<slug>.md IS walked
// because that is where the markers live.
//
// The returned slice is sorted by (FilePath, LineNumber) for
// deterministic lint output.
//
// Returns (slugs, nil) on success; empty slice if no anchors are
// found.
func SlugsForAssumption(root, id string) ([]string, error) {
	if root == "" {
		return nil, fmt.Errorf("parser: SlugsForAssumption: root is empty")
	}
	if id == "" {
		return nil, fmt.Errorf("parser: SlugsForAssumption: id is empty")
	}
	rootAbs, _ := filepath.Abs(root)

	seen := make(map[string]struct{})
	var out []string
	err := walkForAssumptionAnchors(root, rootAbs, func(path string) error {
		relPath := toRepoRelative(path, rootAbs)
		slugs, err := assumptionAnchorsInFile(path, relPath, id)
		if err != nil {
			return fmt.Errorf("scan %s: %w", path, err)
		}
		for _, s := range slugs {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortStrings(out)
	return out, nil
}

// walkForAssumptionAnchors walks root and yields every .md file
// under .kron/intents/ (NOT .kron/.trash/ or .kron/assumptions/,
// since neither can legitimately host a @kron:assumes marker:
// trash is retired, registry files describe their own assumption
// rather than referencing a sibling).
//
// The skip list is a SUBSET of parser.DefaultSkipDirs — we keep
// .git/, node_modules/, vendor/ etc. (noise) but allow .kron/
// and descend into .kron/intents/ specifically. Other .kron
// subdirectories (.kron/assumptions, .kron/.trash) are
// filtered out at the visitor level via isAssumptionAnchorTarget.
func walkForAssumptionAnchors(root, rootAbs string, visit func(path string) error) error {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("parser: walk %s: %w", root, err)
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			// Skip noise directories; allow .kron/ so we can
			// descend into .kron/intents/.
			if d.Name() == ".git" || d.Name() == "node_modules" ||
				d.Name() == "vendor" || d.Name() == "dist" ||
				d.Name() == "bin" || d.Name() == "target" ||
				d.Name() == ".idea" || d.Name() == ".vscode" {
				return filepath.SkipDir
			}
			// Inside .kron/: only intents/ matters.
			if d.Name() == "assumptions" || d.Name() == ".trash" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdownFile(path) {
			return nil
		}
		// Only .kron/intents/<...>.md files can host the
		// @kron:assumes marker. Other .md files (docs, README,
		// etc.) are skipped.
		rel := toRepoRelative(path, rootAbs)
		if !strings.HasPrefix(rel, ".kron/intents/") {
			return nil
		}
		return visit(path)
	})
}

// assumptionAnchorsInFile opens filePath and returns the slug of
// every line that contains the assumption anchor marker followed
// by the requested id. The slug emitted is the intent's own slug
// (derived from the file path relPath), not the assumption id —
// the caller wants to know "this intent references this assumption".
//
// We extract the intent slug from relPath by stripping the
// ".kron/intents/" prefix and the ".md" extension. The result is
// a stable, repo-relative slug consistent with model.Intent.Slug.
//
// emitPath is the value to record in model.Anchor.FilePath when
// surfacing these via ScanAssumptionAnchors (kept distinct from
// filePath so callers can pass a pre-formatted canonical form).
func assumptionAnchorsInFile(filePath, emitPath, id string) ([]string, error) {
	slug := intentSlugFromFilePath(emitPath)
	if slug == "" {
		// File is outside .kron/intents/, so its body cannot
		// legitimately declare "@kron:assumes" (an assumption
		// reference only makes sense in an intent body). Skip.
		return nil, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	// Cheap pre-check: skip the file entirely if it has no
	// "@kron:assumes" marker at all (avoids the per-line work
	// for the common case of unrelated docs).
	var head [1 << 20]byte // 1 MiB; bigger than any realistic .md
	n, _ := io.ReadFull(f, head[:])
	if n == 0 {
		return nil, nil
	}
	if !bytes.Contains(head[:n], []byte(assumptionAnchorMarker)) {
		return nil, nil
	}
	// Marker found in the pre-check — rewind to the start so
	// the line scanner can walk the whole file.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, assumptionAnchorMarker) {
			continue
		}
		assumeID, ok := parseAssumptionAnchorLine(line, id)
		_ = assumeID // we only need the bool; emit slug once per file
		if !ok {
			continue
		}
		out = append(out, slug)
		break // one match per file is enough
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// intentSlugFromFilePath converts a repo-relative .md path under
// .kron/intents/ to the intent slug (the same value stored in
// model.Intent.Slug). Returns "" if the file is not under
// .kron/intents/.
//
// Examples:
//
//	.kron/intents/auth.md            → "auth"
//	.kron/intents/auth/jwt.md        → "auth/jwt"
//	.kron/intents/auth/README.md     → "auth/README"
//	docs/notes.md                    → "" (outside intents dir)
func intentSlugFromFilePath(repoRelPath string) string {
	const intentsDir = ".kron/intents/"
	if !strings.HasPrefix(repoRelPath, intentsDir) {
		return ""
	}
	rest := repoRelPath[len(intentsDir):]
	rest = strings.TrimSuffix(rest, ".md")
	return rest
}

// parseAssumptionAnchorLine returns (assumptionID, true) if line
// contains a "@kron:assumes <id>" annotation whose id equals
// wantID, or ("", false) otherwise. The id comparison is exact
// string equality; we do not partial-match.
//
// The annotation must be token-isolated (delimited by whitespace
// or start-of-line on the left, whitespace on the right) to
// avoid matching emails or strings that happen to contain
// "@kron:assumes" as a substring.
func parseAssumptionAnchorLine(line, wantID string) (string, bool) {
	idx := findAnchorMarkerWith(line, assumptionAnchorMarker)
	if idx < 0 {
		return "", false
	}
	after := line[idx+len(assumptionAnchorMarker):]
	if len(after) == 0 || !isAnchorSpace(after[0]) {
		return "", false
	}
	rest := strings.TrimLeft(after, " \t")
	// Token ends at the next whitespace.
	end := len(rest)
	for i, r := range rest {
		if r == ' ' || r == '\t' {
			end = i
			break
		}
	}
	id := rest[:end]
	if id != wantID {
		return "", false
	}
	return id, true
}

// findAnchorMarkerWith is the marker-agnostic sibling of
// findAnchorMarker (anchors.go). It locates marker in line as a
// whole token and returns the byte offset, or -1 if not present.
//
// Whole-token rules: either start of line, or a non-letter
// non-digit byte (so "@kron:assumes" in an email or identifier
// does not match), AND followed by a whitespace byte.
func findAnchorMarkerWith(line, marker string) int {
	off := 0
	for {
		i := strings.Index(line[off:], marker)
		if i < 0 {
			return -1
		}
		abs := off + i
		leftOK := abs == 0 || !isIdentByte(line[abs-1])
		rpos := abs + len(marker)
		rightOK := rpos == len(line) || isAnchorSpace(line[rpos])
		if leftOK && rightOK {
			return abs
		}
		off = abs + 1
	}
}

// ScanAssumptionAnchors walks root and returns every "@kron:assumes
// <id>" line found in any .kron/intents/<...>.md file (the only
// legitimate surface for the marker).
//
// Each emitted model.Anchor carries:
//   - Slug: the assumption id (kebab-case; no "/")
//   - FilePath: the repo-relative path of the file containing the
//     anchor (forward-slash, RFC 2026-10-08-path-conventions.md
//     §2.1)
//   - LineNumber: 1-indexed
//   - Kind: model.AnchorKindCode (we reuse the code kind because
//     the marker is body-internal; the markdown-anchor-vs-code-
//     anchor split is a separate concern tracked on the intent
//     reference surface, not the assumption surface)
//
// Use case: future MCP tools (kron_assume_inventory,
// kron_assume_reverse) that need the full picture of "where is
// this assumption referenced from". Lint's RuleAssumptionOrphan
// uses SlugsForAssumption (a more focused wrapper) instead of
// this raw scan.
//
// The returned slice is sorted by (FilePath, LineNumber) for
// deterministic output.
func ScanAssumptionAnchors(root string) ([]model.Anchor, error) {
	rootAbs, _ := filepath.Abs(root)

	var out []model.Anchor
	err := walkForAssumptionAnchors(root, rootAbs, func(path string) error {
		relPath := toRepoRelative(path, rootAbs)
		anchors, err := scanAssumptionAnchorsInFile(path, relPath)
		if err != nil {
			return fmt.Errorf("scan %s: %w", path, err)
		}
		out = append(out, anchors...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortAnchors(out)
	return out, nil
}

// scanAssumptionAnchorsInFile reads filePath and returns every
// "@kron:assumes <id>" line as a model.Anchor with the slug set
// to the assumption id and FilePath set to emitPath.
func scanAssumptionAnchorsInFile(filePath, emitPath string) ([]model.Anchor, error) {
	const maxLineBytes = 1 << 20 // 1 MiB
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	// Cheap pre-check: skip the file if no marker is present.
	var head [1 << 20]byte // 1 MiB; bigger than any realistic .md
	n, _ := io.ReadFull(f, head[:])
	if n == 0 {
		return nil, nil
	}
	if !bytes.Contains(head[:n], []byte(assumptionAnchorMarker)) {
		return nil, nil
	}
	// Marker found — rewind so the scanner can walk the whole file.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	var out []model.Anchor
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if !strings.Contains(line, assumptionAnchorMarker) {
			continue
		}
		id, ok := parseAssumptionAnchorLineAny(line)
		if !ok {
			continue
		}
		out = append(out, model.Anchor{
			Slug:       id,
			FilePath:   emitPath,
			LineNumber: lineNo,
			Kind:       model.AnchorKindCode, // see ScanAssumptionAnchors doc
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseAssumptionAnchorLineAny is the "emit any id" sibling of
// parseAssumptionAnchorLine. It returns the first token after
// the marker, regardless of which assumption id the caller is
// looking for. Used by ScanAssumptionAnchors which collects all
// anchors regardless of which assumption they reference.
func parseAssumptionAnchorLineAny(line string) (string, bool) {
	idx := findAnchorMarkerWith(line, assumptionAnchorMarker)
	if idx < 0 {
		return "", false
	}
	after := line[idx+len(assumptionAnchorMarker):]
	if len(after) == 0 || !isAnchorSpace(after[0]) {
		return "", false
	}
	rest := strings.TrimLeft(after, " \t")
	end := len(rest)
	for i, r := range rest {
		if r == ' ' || r == '\t' {
			end = i
			break
		}
	}
	id := rest[:end]
	if !isValidAssumptionID(id) {
		return "", false
	}
	return id, true
}

// isValidAssumptionID returns true if id is a syntactically valid
// assumption identifier: kebab-case, no "/", no ".md" suffix. The
// rules are deliberately narrower than ValidateSlug because
// assumption files are flat in .kron/assumptions/ and never
// nested.
func isValidAssumptionID(id string) bool {
	if id == "" {
		return false
	}
	if strings.Contains(id, "/") {
		return false
	}
	if strings.HasSuffix(id, ".md") {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}
