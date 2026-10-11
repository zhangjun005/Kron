// Package model defines the domain types for Kron.
//
// Types here are pure data structures with zero external dependencies.
// All file I/O and parsing logic lives in internal/store and internal/parser.
package model

// Config holds runtime configuration for Kron.
// It is loaded from .kron/config.toml; missing fields use defaults.
type Config struct {
	// IntentsDir is the root directory for intent files.
	// Defaults to ".kron/intents" if empty.
	IntentsDir string `toml:"intents_dir"`
}
