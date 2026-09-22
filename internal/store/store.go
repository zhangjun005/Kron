// Package store handles file I/O and frontmatter parsing for Kron intent files.
//
// It reads and writes .kron/intents/<slug>.md files. It does not know
// whether the caller is CLI, MCP, LSP, IDE, or GUI — that identity flows
// via context.Context, not via type knowledge.
//
// Errors are wrapped at the point of origin. Sentinel errors are defined
// for known failure modes (e.g., ErrIntentNotFound).
package store
