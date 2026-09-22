// Package parser handles CLI argument parsing and markdown body extraction for Kron.
//
// It parses command-line flags, extracts anchors from markdown bodies,
// and identifies relative links. It does not perform file I/O — the
// caller passes pre-read content in.
package parser
