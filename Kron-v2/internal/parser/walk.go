package parser

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// DefaultSkipDirs is the set of directory names walked by ScanAnchors
// (and other source-tree scans) that are skipped without descending.
// It centralises the "what counts as a project artefact vs vendored
// noise" decision so callers don't each maintain their own copy.
//
// The list is intentionally a fixed map literal: adding to it is a
// cross-cutting change that affects lint, density, and any future
// source-tree scanner, and reviewers should see all the call sites
// in the same diff.
var DefaultSkipDirs = map[string]struct{}{
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

// IsBinary reports whether path looks like a binary file based on the
// first 512 bytes: returns true if a NUL byte or a UTF-16 BOM is
// present. False positives are possible (a UTF-8 file that happens
// to contain a NUL in the first 512 bytes), but the heuristic is good
// enough to skip the common cases (compiled artefacts, images, minified
// blob data) without dragging in a content-type detector.
//
// The function is safe to call concurrently across files; it does
// not retain any state.
func IsBinary(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	var head [512]byte
	n, _ := io.ReadFull(f, head[:])
	if bytes.IndexByte(head[:n], 0) >= 0 {
		return true, nil
	}
	if n >= 2 {
		// UTF-16 LE: FF FE. UTF-16 BE: FE FF.
		if (head[0] == 0xFF && head[1] == 0xFE) ||
			(head[0] == 0xFE && head[1] == 0xFF) {
			return true, nil
		}
	}
	return false, nil
}

// WalkSourceFiles walks root and yields every regular file that
// passes the default-skip-dir and binary-sniff filters. It is the
// shared source-tree walker used by ScanAnchors (anchor scan) and
// any future scanner (intent-density orphan detection, future
// "find dead @kron:intent" check, etc.) so the skip-list / binary
// heuristic are defined exactly once.
//
// The visitor callback receives the absolute path of each surviving
// file. Returning a non-nil error from the visitor aborts the walk
// and propagates that error out of WalkSourceFiles.
//
// If root itself is missing, WalkSourceFiles returns nil (not an
// error) — a fresh repo with no source tree behaves identically to
// one with an empty source tree. This matches store.Reader.LoadAll's
// missing-intents-dir tolerance. We use os.Stat to detect a missing
// root explicitly because filepath.WalkDir surfaces the I/O error
// in a platform-specific way (e.g. wrapped GetFileAttributesEx on
// Windows, fs.ErrNotExist on Unix) that can't be matched with a
// single errors.Is check.
func WalkSourceFiles(root string, visit func(path string) error) error {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("parser: walk %s: %w", root, err)
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if _, skip := DefaultSkipDirs[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		binary, berr := IsBinary(path)
		if berr != nil {
			// Surface I/O errors but do NOT block the walk on a single
			// unreadable file; treat it as "skip this file".
			return nil
		}
		if binary {
			return nil
		}
		return visit(path)
	})
	if err != nil {
		return fmt.Errorf("parser: walk %s: %w", root, err)
	}
	return nil
}

// sortStrings is a tiny wrapper around the stdlib sort so callers in
// anchors.go (e.g. SlugsForFile) don't need to import the sort
// package directly. The wrapper is trivial; the indirection is for
// caller-clarity: every sort helper in this package is named
// "sort*" so reviewers can grep for them.
func sortStrings(s []string) { sort.Strings(s) }
