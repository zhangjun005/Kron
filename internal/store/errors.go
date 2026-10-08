package store

import "errors"

// Sentinel errors returned by Writer.Update.
//
// Errors are wrapped at the call site with fmt.Errorf; callers
// should use errors.Is / errors.As for discrimination. Never
// compare with ==.
var (
	// ErrEmptyPatch is returned when Update is called with a
	// UpdatePatch whose every business field is nil. The library
	// refuses to write a file with only UpdatedAt changed — that
	// almost always means the caller forgot to set something.
	//
	// Mirrors assumption.ErrEmptyPatch.
	ErrEmptyPatch = errors.New("update patch is empty")

	// ErrCreatorChangeNotAllowed is returned when UpdatePatch
	// includes CreatedBy but the caller did not pass
	// WithAllowCreatorChange. CreatedBy is identity-bearing;
	// changing it has social / forensic implications and is a
	// separate opt-in from the patch itself.
	//
	// Mirrors assumption.ErrCreatorChangeNotAllowed.
	ErrCreatorChangeNotAllowed = errors.New("changing created_by requires WithAllowCreatorChange opt-in")
)
