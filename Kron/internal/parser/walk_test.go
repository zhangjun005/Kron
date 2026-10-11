package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBinary_UTF8Text(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.go")
	require.NoError(t, os.WriteFile(p, []byte("package x\n"), 0o644))
	bin, err := IsBinary(p)
	require.NoError(t, err)
	assert.False(t, bin)
}

func TestIsBinary_NULByte(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "blob.bin")
	require.NoError(t, os.WriteFile(p, []byte{'P', 'K', 0x00, 0x03}, 0o644))
	bin, err := IsBinary(p)
	require.NoError(t, err)
	assert.True(t, bin)
}

func TestIsBinary_UTF16LE(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "utf16.txt")
	require.NoError(t, os.WriteFile(p, []byte{0xFF, 0xFE, 'a', 0x00}, 0o644))
	bin, err := IsBinary(p)
	require.NoError(t, err)
	assert.True(t, bin)
}

func TestIsBinary_UTF16BE(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "utf16be.txt")
	require.NoError(t, os.WriteFile(p, []byte{0xFE, 0xFF, 0x00, 'a'}, 0o644))
	bin, err := IsBinary(p)
	require.NoError(t, err)
	assert.True(t, bin)
}

func TestIsBinary_OpenError(t *testing.T) {
	_, err := IsBinary(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestWalkSourceFiles_Empty(t *testing.T) {
	var seen []string
	err := WalkSourceFiles(t.TempDir(), func(p string) error {
		seen = append(seen, p)
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, seen)
}

func TestWalkSourceFiles_SkipsDefaultDirs(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "real.go", "package x")
	writeSourceFile(t, dir, ".git/HEAD", "ref: refs/heads/main")
	writeSourceFile(t, dir, ".kron/intents/x.md", "x")
	writeSourceFile(t, dir, "node_modules/lib/index.js", "module.exports = {}")
	writeSourceFile(t, dir, "vendor/g.go", "package g")

	var seen []string
	err := WalkSourceFiles(dir, func(p string) error {
		seen = append(seen, p)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, seen, 1)
	assert.Contains(t, seen[0], "real.go")
}

func TestWalkSourceFiles_SkipsBinary(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "real.go", "package x")
	writeSourceFile(t, dir, "blob.bin", string([]byte{'P', 'K', 0x00, 0x03}))

	var seen []string
	err := WalkSourceFiles(dir, func(p string) error {
		seen = append(seen, p)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, seen, 1)
	assert.Contains(t, seen[0], "real.go")
}

func TestWalkSourceFiles_PropagatesVisitorError(t *testing.T) {
	dir := t.TempDir()
	writeSourceFile(t, dir, "a.go", "package a")
	writeSourceFile(t, dir, "b.go", "package b")

	want := filepath.Join(dir, "a.go")
	err := WalkSourceFiles(dir, func(p string) error {
		if p == want {
			return assert.AnError
		}
		return nil
	})
	require.ErrorIs(t, err, assert.AnError)
}

func TestWalkSourceFiles_MissingRoot(t *testing.T) {
	// Walking a non-existent root must return nil, not an error,
	// so a fresh repo with no source tree is indistinguishable
	// from an empty one.
	err := WalkSourceFiles(filepath.Join(t.TempDir(), "ghost"), func(string) error {
		t.Fatal("visitor should not be called for missing root")
		return nil
	})
	require.NoError(t, err)
}
