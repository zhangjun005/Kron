package servemcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/xxx/kron/internal/parser"
	"github.com/xxx/kron/internal/store"
)

// handleIntentDensity returns the toolHandler for kron_intent_density.
//
// Wire contract: see docs/implementation/mcp.md §2 (kron_intent_density).
//
//	in:  {}
//	out: {
//	  total_intents: int,
//	  total_anchors: int,
//	  coverage: { intents_with_anchors: int, intents_without_anchors: [slug] },
//	  files_without_intent: [path]   // files >50 lines with 0 anchors
//	}
//
// "files without intent" is a rough heuristic: walk the source tree,
// skip the standard skip-list, count lines per file, keep those > 50
// with no anchor. The 50-line threshold matches the CLI intent-coverage
// gate; see docs/process/lint-rule.md for the rationale.
func handleIntentDensity(root string) toolHandler {
	return func(ctx context.Context, _ json.RawMessage) (any, error) {
		if err := assertCallerMCP(ctx); err != nil {
			return nil, err
		}

		r, err := store.NewReader(root)
		if err != nil {
			return nil, fmt.Errorf("kron_intent_density: open store: %w", err)
		}
		all, err := r.LoadAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("kron_intent_density: load: %w", err)
		}

		anchors, err := parser.ScanAnchors(root)
		if err != nil {
			return nil, fmt.Errorf("kron_intent_density: scan anchors: %w", err)
		}

		withAnchor := make(map[string]struct{}, len(anchors))
		anchoredFiles := make(map[string]struct{}, len(anchors))
		for _, a := range anchors {
			withAnchor[a.Slug] = struct{}{}
			anchoredFiles[a.FilePath] = struct{}{}
		}

		var withoutAnchor []string
		for _, in := range all {
			if _, ok := withAnchor[in.Slug]; !ok {
				withoutAnchor = append(withoutAnchor, in.Slug)
			}
		}
		sort.Strings(withoutAnchor)
		if withoutAnchor == nil {
			withoutAnchor = []string{}
		}

		orphans, err := scanOrphanFiles(root, anchoredFiles)
		if err != nil {
			return nil, fmt.Errorf("kron_intent_density: scan orphans: %w", err)
		}

		return densityResponse{
			TotalIntents: len(all),
			TotalAnchors: len(anchors),
			Coverage: coverageWire{
				IntentsWithAnchors:    len(withAnchor),
				IntentsWithoutAnchors: withoutAnchor,
			},
			FilesWithoutIntent: orphans,
		}, nil
	}
}

// scanOrphanFiles walks root and returns the repo-relative paths of
// source files that are > 50 lines long and have zero anchors. The
// 50-line threshold matches the CLI intent-coverage gate; see
// docs/process/lint-rule.md for the rationale.
func scanOrphanFiles(root string, anchored map[string]struct{}) ([]string, error) {
	const lineThreshold = 50
	skipDirs := map[string]struct{}{
		".git": {}, ".kron": {}, "node_modules": {}, "vendor": {},
		"dist": {}, "bin": {}, "target": {}, ".idea": {}, ".vscode": {},
	}

	var orphans []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := anchored[path]; ok {
			return nil
		}
		if isBinary, _ := quickBinaryCheck(path); isBinary {
			return nil
		}
		n, err := countLines(path, lineThreshold+1)
		if err != nil {
			return err
		}
		if n > lineThreshold {
			if rel, err := filepath.Rel(root, path); err == nil {
				orphans = append(orphans, rel)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(orphans)
	return orphans, nil
}

func quickBinaryCheck(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var head [512]byte
	n, _ := io.ReadFull(f, head[:])
	for i := 0; i < n; i++ {
		if head[i] == 0 {
			return true, nil
		}
	}
	return false, nil
}

func countLines(path string, capLines int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	count := 0
	for sc.Scan() {
		count++
		if count > capLines {
			break
		}
	}
	return count, sc.Err()
}

type densityResponse struct {
	TotalIntents       int          `json:"total_intents"`
	TotalAnchors       int          `json:"total_anchors"`
	Coverage           coverageWire `json:"coverage"`
	FilesWithoutIntent []string     `json:"files_without_intent"`
}

type coverageWire struct {
	IntentsWithAnchors    int      `json:"intents_with_anchors"`
	IntentsWithoutAnchors []string `json:"intents_without_anchors"`
}
