// Package model defines the domain types for Kron.
//
// Types here are pure data structures with zero external dependencies.
// All file I/O and parsing logic lives in internal/store and internal/parser.
package model

import (
	"errors"
)

// Sentinel errors for known failure modes. Wrap with fmt.Errorf and use
// errors.Is/As for discrimination. Never compare with ==.
var (
	ErrIntentNotFound     = errors.New("intent not found")
	ErrIntentExists       = errors.New("intent already exists")
	ErrAnchorDangling     = errors.New("anchor points to non-existent intent")
	ErrFrontmatterInvalid = errors.New("frontmatter is invalid")
	ErrSlugInvalid        = errors.New("intent slug is invalid")
	ErrConfigInvalid      = errors.New("config file is invalid")
)
