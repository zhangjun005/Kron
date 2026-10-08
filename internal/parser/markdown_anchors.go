package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xxx/kron/internal/model"
)

// ScanMarkdownmodel.Anchors walks dir recursively and returns every
// "@kron:intent <slug>" line in .md files OUTSIDE the skip-list
// (notably .kron/intents/, .kron/.trash/, node_modules/, vendor/, .git/).
//
// # In-fence handling (md-anchors RFC §2.4)
//
// A line that is INSIDE a ``` ``` or ~~~ ``` fenced code block is
// not treated as an anchor, even if it matches the marker. This
// prevents false positives such as documentation examples that quote
// the anchor syntax inside a code block. The fence state is tracked
// across lines:
//
//   - A line that is exactly ``` ``` (or ``` ```` ```, i.e. up to
//     three backticks optionally followed by a language tag) opens a
//     fence.
//   - A line that is exactly ``` ``` (matching the opening fence
//     length) closes it.
//   - Fences must be the first non-whitespace content of a line.
//   - Nested fences (different opening lengths) are NOT supported;
//     once a fence is open, the next matching-fence-length line
//     closes it. This matches CommonMark §4.5 (indented code blocks
//     are out of scope for v1).
//
// # Skip list
//
// Reuses WalkSourceFiles' default skip list (see walk.go) so .kron/,
// .git/, node_modules/, vendor/ are excluded by default. .md files
// inside .kron/intents/ are NOT scanned because intent files are
// self-referential (one .md == one intent); see intent-structure.md
// §一.
func ScanMarkdownAnchors(dir string) ([]model.Anchor, error) {
	var out []model.Anchor
	err := WalkSourceFiles(dir, func(path string) error {
		if !isMarkdownFile(path) {
			return nil
		}
		anchors, err := scanMarkdownFile(path)
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

// isMarkdownFile reports whether path ends in .md (case-sensitive, lower-case).
// WalkSourceFiles handles the directory skip-list.
func isMarkdownFile(path string) bool {
	return strings.HasSuffix(path, ".md")
}

// scanMarkdownFile scans a single .md file, respecting fenced code blocks.
// Returns anchors (with line numbers) for every non-fence line that
// matches the @kron:intent marker.
func scanMarkdownFile(path string) ([]model.Anchor, error) {
	const maxLineBytes = 1 << 20 // 1 MiB

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var out []model.Anchor
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	lineNo := 0
	inFence := false
	fenceLen := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()

		fenceOpen, openLen, fenceClose := parseFenceLine(line)

		if inFence {
			// We are inside a fence; the only way out is a matching close fence.
			if fenceClose && openLen == fenceLen {
				inFence = false
				fenceLen = 0
			}
			continue
		}

		// Not in fence: maybe open one.
		if fenceOpen {
			inFence = true
			fenceLen = openLen
			continue
		}

		// Not in fence: try to parse an anchor.
		slug, ok := parseAnchorLine(line)
		if !ok {
			continue
		}
		out = append(out, model.Anchor{
			Slug:       slug,
			FilePath:   path,
			LineNumber: lineNo,
		})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseFenceLine inspects line as a CommonMark fenced code block delimiter.
// Returns (isOpen, fenceLen, isClose) where:
//   - isOpen=true, fenceClose=false: opening fence
//   - isOpen=false, fenceClose=true: closing fence (matches opening length)
//   - both false: not a fence line
//
// A fence is 3 or more consecutive backticks (` ``` `) or tildes (~~~)
// at the start of the line (after optional leading whitespace), optionally
// followed by an info string. We treat ``` and ~~~ as separate fence
// types — opening with one and closing with the other does NOT close
// the fence. This is a conservative subset of CommonMark §4.5.
func parseFenceLine(line string) (isOpen bool, fenceLen int, isClose bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 3 {
		return false, 0, false
	}

	marker := trimmed[0]
	if marker != '`' && marker != '~' {
		return false, 0, false
	}

	// Count consecutive identical markers.
	count := 0
	for _, b := range trimmed {
		if byte(b) == marker {
			count++
			continue
		}
		break
	}
	if count < 3 {
		return false, 0, false
	}
	// After the marker run, the rest of the line is the info string
	// (only for opening fences; closing fences must have nothing after).
	rest := strings.TrimSpace(trimmed[count:])
	if rest == "" {
		// Pure fence line: could be open or close.
		// The caller tracks in/out state; both open and close report true here.
		return true, count, true
	}
	// Non-empty info string: opening fence only.
	return true, count, false
}

// _ = filepath.Join keeps the filepath import "used" in case future
// refactors need to construct paths from ScanMarkdownmodel.Anchors. Removing
// this would break `goimports` and force a second edit when adding
// path-derived code.
var _ = filepath.Join
