package model

// Reserved directory names under .kron/. These are the on-disk layout
// that store reads from and writes to. Lint and CLI consume them too.
//
// Centralising the constants here (rather than re-declaring in each
// package) avoids drift if the layout ever changes — e.g. a future
// "kron pack" command would read TrashDir and IntentDir from the same
// source.
const (
	// KronDir is the root of Kron-managed data within a repository.
	KronDir = ".kron"

	// IntentDir holds live intent files: <KronDir>/<IntentDir>/<slug>.md.
	// Stored as a relative path so paths in error messages are
	// repo-relative and survive running kron from subdirectories.
	IntentDir = "intents"

	// TrashDir holds soft-deleted intent files: <KronDir>/<TrashDir>/<slug>.md.
	// Hard-delete and GC are explicitly out of v1 (see architecture.md §5.2.1).
	TrashDir = ".trash"

	// ConfigFileName is the TOML config file name inside KronDir.
	ConfigFileName = "config.toml"

	// IntentExtension is the file extension for intent files.
	IntentExtension = ".md"
)

// IntentPath returns the on-disk relative path of an intent under .kron/intents/.
// The slug is the canonical "auth/jwt-sliding-window" form (no .md).
// No validation is performed here; call ValidateSlug first.
func IntentPath(slug string) string {
	return KronDir + "/" + IntentDir + "/" + slug + IntentExtension
}

// TrashPath returns the on-disk relative path of a soft-deleted intent.
func TrashPath(slug string) string {
	return KronDir + "/" + TrashDir + "/" + slug + IntentExtension
}
