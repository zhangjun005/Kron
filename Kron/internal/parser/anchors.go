package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxx/kron/internal/model"
)

// anchorMarker is the substring every line-level intent anchor must
// contain. The surrounding comment prefix is intentionally not fixed:
// "//" (Go/C/Rust/JS), "#" (Python/Ruby/Shell), "--" (SQL/Lua/Haskell),
// "%" (Erlang/MATLAB) are all fine as long as the literal text
// "@kron:intent" appears on the same line followed by the slug.
const anchorMarker = "@kron:intent"

// ScanAnchors walks dir recursively and returns every line-level
// "@kron:intent <slug>" annotation it finds in source files.
//
// The walk uses parser.WalkSourceFiles, so the skip-list (default
// ignored directories) and the binary-file heuristic are shared with
// any other source-tree scanner (intent-density orphan detection,
// future "find dead @kron:intent" check, etc.). See walk.go for
// the canonical skip list.
//
// Each scanned file is read line-by-line; an anchor is any line that
// contains anchorMarker followed by a slug-shaped token. LineNumber
// is 1-indexed.
//
// FilePath convention (RFC 2026-10-08-path-conventions.md §2.1):
// model.Anchor.FilePath is always repo-relative, forward-slash
// separated (filepath.ToSlash of filepath.Rel(dir, abs)). It is an
// *identifier*, not an IO path — callers that need to open the
// file must join it against their own root. This keeps the value
// stable across processes and platforms (Windows backslashes are
// not allowed in the emitted FilePath).
//
// The returned slice is sorted by (FilePath, LineNumber) for
// deterministic lint output.
func ScanAnchors(dir string) ([]model.Anchor, error) {
	// Pre-resolve the walk root so filepath.Rel below uses a stable
	// absolute baseline regardless of how `dir` was passed (".", "..",
	// "C:\path", or "/abs/path"). If Abs fails we fall back to the
	// raw dir — the walk still works, but FilePath values will be
	// relative to whatever string the caller passed in.
	rootAbs, _ := filepath.Abs(dir)

	var anchors []model.Anchor
	err := WalkSourceFiles(dir, func(path string) error {
		// RFC 2026-10-08-md-anchors.md §1.1: code anchors live
		// in source files; markdown anchors are produced by
		// ScanMarkdownAnchors. Skip .md here so the two
		// surfaces don't double-count or fight for the same
		// line. isMarkdownFile is the same predicate
		// ScanMarkdownAnchors uses (case-sensitive .md suffix).
		if isMarkdownFile(path) {
			return nil
		}
		relPath := toRepoRelative(path, rootAbs)
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()

		fileAnchors, err := scanFile(f, relPath)
		if err != nil {
			return fmt.Errorf("scan %s: %w", path, err)
		}
		anchors = append(anchors, fileAnchors...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Deterministic ordering: (FilePath, LineNumber).
	sortAnchors(anchors)
	return anchors, nil
}

// toRepoRelative converts an absolute (or walk-relative) file path
// into a forward-slash, root-relative identifier. It is the single
// point where Anchor.FilePath gets its canonical form per
// RFC 2026-10-08-path-conventions.md.
//
//	rootAbs = ""           → filePath is returned as-is (best effort)
//	rootAbs given, abs under root → Rel + ToSlash
//	rootAbs given, abs outside root → Rel + ToSlash (gives "../..." —
//	                            access layer decides how to surface)
func toRepoRelative(filePath, rootAbs string) string {
	if rootAbs == "" {
		return filePath
	}
	rel, err := filepath.Rel(rootAbs, filePath)
	if err != nil {
		return filePath
	}
	return filepath.ToSlash(rel)
}

// SlugsForFile returns the set of intent slugs that have a
// "@kron:intent <slug>" line in filePath. The set is returned as a
// sorted slice so callers can iterate it deterministically.
//
// filePath may be absolute or repo-relative. The file is opened and
// parsed directly (no walk), so this is the right entry point when
// the caller already knows the file (e.g. kron_assume_check with a
// file_path argument) and only needs the slugs that one file carries.
//
// Empty filePath or a path that cannot be opened returns an error.
// A file with no anchors returns an empty (non-nil) slice.
func SlugsForFile(filePath string) ([]string, error) {
	if filePath == "" {
		return nil, fmt.Errorf("parser: SlugsForFile: filePath is empty")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("parser: open %s: %w", filePath, err)
	}
	defer f.Close()

	fileAnchors, err := scanFile(f, filePath)
	if err != nil {
		return nil, err
	}

	// Dedupe (a single file can carry the same anchor multiple times)
	// and sort for deterministic output.
	seen := make(map[string]struct{}, len(fileAnchors))
	out := make([]string, 0, len(fileAnchors))
	for _, a := range fileAnchors {
		if _, ok := seen[a.Slug]; ok {
			continue
		}
		seen[a.Slug] = struct{}{}
		out = append(out, a.Slug)
	}
	sortStrings(out)
	return out, nil
}

// scanFile scans a single open file for "@kron:intent <slug>" lines.
// Lines are limited to a generous max line length so a pathological
// minified file cannot blow memory.
func scanFile(r io.Reader, path string) ([]model.Anchor, error) {
	const maxLineBytes = 1 << 20 // 1 MiB

	var out []model.Anchor
	sc := bufio.NewScanner(r)
	// Allow long lines; default is 64 KiB which clips minified JS.
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		slug, ok := parseAnchorLine(line)
		if !ok {
			continue
		}
		// Code anchors (Go/TS/Python/... source files). Markdown
		// anchors are produced by ScanMarkdownAnchors with
		// model.AnchorKindMarkdown so the two streams are
		// distinguishable downstream (RFC
		// 2026-10-08-md-anchors.md §2.5).
		out = append(out, model.Anchor{
			Slug:       slug,
			FilePath:   path,
			LineNumber: lineNo,
			Kind:       model.AnchorKindCode,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseAnchorLine returns (slug, true) if line contains a syntactically
// valid "@kron:intent <slug>" annotation, or ("", false) otherwise.
//
// Accepted forms (whitespace-tolerant):
//
//	// @kron:intent auth/jwt-sliding-window
//	#  @kron:intent single-region   some trailing comment
//	-- @kron:intent auth/jwt
//
// Rules:
//   - The marker "@kron:intent" must appear as a whole token (delimited
//     by whitespace or start-of-line on the left, whitespace on the
//     right). This avoids matching emails or strings that happen to
//     contain "@kron:intent" as a substring.
//   - The slug is everything from the next non-whitespace byte up to
//     the next whitespace (or end of line). Trailing comments on the
//     same line are ignored.
//   - The slug MUST pass ValidateSlug; otherwise the annotation is
//     treated as malformed and skipped (so a typo in the slug does
//     not produce a "dangling anchor" error pointing at a non-slug).
func parseAnchorLine(line string) (string, bool) {
	idx := findAnchorMarker(line)
	if idx < 0 {
		return "", false
	}
	after := line[idx+len(anchorMarker):]
	// Require whitespace (or end of line) immediately after the marker.
	if len(after) == 0 || !isAnchorSpace(after[0]) {
		return "", false
	}
	rest := strings.TrimLeft(after, " \t")
	// Slug ends at the first whitespace.
	end := len(rest)
	for i, r := range rest {
		if r == ' ' || r == '\t' {
			end = i
			break
		}
	}
	slug := rest[:end]
	if err := ValidateSlug(slug); err != nil {
		return "", false
	}
	return slug, true
}

// findAnchorMarker locates the "@kron:intent" marker in line as a
// whole token. It returns the byte offset, or -1 if not present.
//
// Whole-token rules: either start-of-line, or a non-whitespace
// non-letter byte (so "@kron:intent" in an email or identifier
// does not match), AND followed by a whitespace byte.
func findAnchorMarker(line string) int {
	const marker = anchorMarker
	off := 0
	for {
		i := strings.Index(line[off:], marker)
		if i < 0 {
			return -1
		}
		abs := off + i

		// Left boundary: start of line, or non-letter non-digit byte.
		leftOK := abs == 0 || !isIdentByte(line[abs-1])

		// Right boundary: at end of line, or a whitespace byte.
		rpos := abs + len(marker)
		rightOK := rpos == len(line) || isAnchorSpace(line[rpos])

		if leftOK && rightOK {
			return abs
		}
		off = abs + 1
	}
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') ||
		b == '_' || b == '-'
}

func isAnchorSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func sortAnchors(a []model.Anchor) {
	// Inlined insertion sort: anchors lists are typically <10k entries,
	// and sort.Slice has reflection overhead worth avoiding in a hot
	// scan path. Keep it simple.
	for i := 1; i < len(a); i++ {
		for j := i; j > 0; j-- {
			if anchorLess(a[j], a[j-1]) {
				a[j], a[j-1] = a[j-1], a[j]
				continue
			}
			break
		}
	}
}

func anchorLess(a, b model.Anchor) bool {
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	return a.LineNumber < b.LineNumber
}
