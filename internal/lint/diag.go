package lint

// Diag is one diagnostic produced by Run.
//
// Rule and Severity are typed (not strings) so consumers get compile-time
// guarantees on which values are well-formed. Where carries enough
// context to point a human at the offending location; for repo-wide
// issues (e.g., LoadAll failure) it is the sentinel value "<root>".
type Diag struct {
	// Rule is the stable id of the check that produced this diagnostic.
	Rule Rule

	// Where is the repo-relative location of the issue. For anchor
	// diags it is "<relative path>:<line>"; for repo-wide diags it is
	// the literal "<root>".
	Where string

	// Detail is a human-readable explanation. It is intended for humans
	// reading lint output, not for machine parsing; use Rule for that.
	Detail string

	// Severity is "error" or "warning". Empty is treated as error.
	Severity Severity
}

// severity returns d.Severity, defaulting to SeverityError when empty.
// Used by cli/json output formatters; kept package-private so callers
// go through HasErrors / future Render helpers rather than comparing
// strings directly.
func (d Diag) severity() Severity {
	if d.Severity == "" {
		return SeverityError
	}
	return d.Severity
}