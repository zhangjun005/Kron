package assumption

import "errors"

var (
	// ErrAssumptionNotFound is returned when an assumption id does not exist
	// in the registry.
	ErrAssumptionNotFound = errors.New("assumption not found")

	// ErrAssumptionFileIdMismatch is returned when a .kron/assumptions/<id>.md
	// file's frontmatter id field does not match its file name.
	ErrAssumptionFileIdMismatch = errors.New("assumption file id mismatch")

	// ErrAssumptionDirNotFound is returned when NewReader is called but
	// .kron/assumptions/ does not exist.
	ErrAssumptionDirNotFound = errors.New(".kron/assumptions/ directory not found")

	// ErrAssumptionExists is returned when Create is called but the file already exists.
	ErrAssumptionExists = errors.New("assumption file already exists")
)
