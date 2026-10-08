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

	// ErrAssumptionAlreadyDeleted is returned by Delete when the file is
	// already in the .trash directory. (Soft-delete is idempotent in
	// effect: re-deleting a trashed file is a no-op for the user, but
	// the library surfaces this so callers can distinguish "fresh
	// delete" from "no-op".)
	ErrAssumptionAlreadyDeleted = errors.New("assumption already in trash")

	// ErrAssumptionNotInTrash is returned by Restore when the file is
	// not in the .trash directory. Restoring a live assumption is a
	// logical error in the caller — the library refuses so the caller
	// can react (e.g., surface "nothing to restore").
	ErrAssumptionNotInTrash = errors.New("assumption is not in trash")

	// ErrEmptyPatch is returned by Update when the UpdatePatch has
	// every business field nil. Library refuses to write a file with
	// only UpdatedAt changed — the caller almost certainly forgot to
	// set something.
	ErrEmptyPatch = errors.New("update patch is empty")

	// ErrCreatorChangeNotAllowed is returned by Update when the
	// UpdatePatch includes CreatedBy but the caller did NOT pass
	// WithAllowCreatorChange. Library does NOT interpret "Actor"
	// beyond identity formatting; the caller is responsible for any
	// confirmation UI / audit log before calling with the opt-in.
	ErrCreatorChangeNotAllowed = errors.New("changing created_by requires WithAllowCreatorChange opt")
)
