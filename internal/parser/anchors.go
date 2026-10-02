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

// anchorMarker is the substring every line-level intent anchor must
// contain. The surrounding comment prefix is intentionally not fixed:
// "//" (Go/C/Rust/JS), "#" (Python/Ruby/Shell), "--" (SQL/Lua/Haskell),
// "%" (Erlang/MATLAB) are all fine as long as the literal text
// "@kron:intent" appears on the same line followed by the slug.
const anchorMarker = "@kron:intent"

// ScanAnchors walks dir recursively and returns every line-level
// "@kron:intent <slug>" annotation it finds in source files.
//
// Scope:
//   - All regular files under dir are scanned (no extension filter in
//     v1: source files have too many extensions to enumerate, and
//     false positives on binary files are avoided by a content sniff
//     at the top of the file).
//   - Skip-list: .git, .kron, node_modules, vendor, dist, bin, target,
//     .idea, .vscode (matches docs/implementation/cli.md §4).
//   - Each scanned file is read line-by-line; an anchor is any line
//     that contains anchorMarker followed by a slug-shaped token.
//   - LineNumber is 1-indexed.
//
// The returned slice is sorted by (FilePath, LineNumber) for
// deterministic lint output.
func ScanAnchors(dir string) ([]model.Anchor, error) {
	skipDirs := map[string]struct{}{
		".git":         {},
		".kron":        {},
		"node_modules": {},
		"vendor":       {},
		"dist":         {},
		"bin":          {},
		"target":       {},
		".idea":        {},
		".vscode":      {},
	}

	var anchors []model.Anchor
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		// Quick content sniff: peek at the first 512 bytes and bail
		// if any NUL byte is present, or if a UTF-16 BOM is detected.
		// Both indicate the file is not a plain-text source file that
		// could carry "// @kron:intent" annotations.
		//
		// The NUL check is what catches UTF-8 files written by tools
		// that re-encode ASCII as UTF-16 (PowerShell's default), since
		// every ASCII byte is followed by a 0x00 NUL. The explicit BOM
		// checks are belt-and-suspenders for files that open in a
		// non-NUL-padded multibyte encoding (e.g. UTF-16 with non-ASCII
		// characters immediately after the BOM).
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()

		var head [512]byte
		n, _ := io.ReadFull(f, head[:])
		if bytes.IndexByte(head[:n], 0) >= 0 {
			return nil
		}
		if n >= 2 {
			// UTF-16 LE: FF FE. UTF-16 BE: FE FF.
			if (head[0] == 0xFF && head[1] == 0xFE) ||
				(head[0] == 0xFE && head[1] == 0xFF) {
				return nil
			}
		}

		// Reset to the start of the file.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seek %s: %w", path, err)
		}

		fileAnchors, err := scanFile(f, path)
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
