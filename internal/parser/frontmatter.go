// Package parser handles pure-function parsing and serialisation of
// frontmatter and markdown content.
//
// It performs no I/O: store.Load/Write read bytes and call into here.
// This separation lets lint rules and CLI argument parsers reuse the
// same primitives without dragging in filesystem dependencies.
package parser

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/xxx/kron/internal/model"
)

// SplitMarkdown separates the YAML frontmatter block from the markdown body.
//
// On-disk format:
//
//	---
//	<yaml>
//	---
//	<body>
//
// Both fences MUST be exactly "---" on their own line (no leading whitespace,
// terminated by \n). Trailing whitespace on the fence line is tolerated.
//
// Behaviour:
//   - Empty file → (nil, "", nil).
//   - Missing opening fence → error (silent acceptance hides AI-write bugs
//     where the opening fence is forgotten).
//   - Missing closing fence → error.
//
// fmRaw includes the trailing newline after the YAML block; body preserves
// whatever follows the closing fence verbatim (including the leading newline
// if any).
func SplitMarkdown(data []byte) (fmRaw []byte, body string, err error) {
	if len(data) == 0 {
		return nil, "", nil
	}

	// Fast path: look for "\n---\n" (closing fence with surrounding newlines).
	// The opening fence must be the very first line.
	if !bytes.HasPrefix(data, []byte("---")) {
		return nil, "", errors.New("expected opening fence on line 1")
	}
	// The opening fence line is "---" optionally followed by \n or \r\n.
	after := data[3:]
	if len(after) > 0 && after[0] != '\n' && !(after[0] == '\r' && len(after) > 1 && after[1] == '\n') {
		return nil, "", errors.New("expected opening fence on line 1")
	}
	// Skip the newline(s) following the opening fence.
	if after[0] == '\r' {
		after = after[2:]
	} else {
		after = after[1:]
	}

	// Find the closing fence. We accept either "\n---\n", "\n---\r\n",
	// or "\n---" at end of file.
	const fence = "\n---"
	idx := bytes.Index(after, []byte(fence))
	if idx < 0 {
		return nil, "", errors.New("unterminated frontmatter block (missing closing ---)")
	}
	fmEnd := idx // offset in `after` where "\n---" begins; the leading \n
	// belongs to YAML, not the fence, so fmRaw = after[:idx+1] gives us
	// "yaml...\n" plus the final newline that terminated the YAML line.
	// Then we strip the leading "\n---" of the closing fence and its
	// trailing newline.
	fmRaw = after[:fmEnd+1] // includes the trailing \n before the fence

	rest := after[fmEnd+len(fence):] // skip "\n---"
	if len(rest) > 0 && rest[0] == '\r' {
		rest = rest[1:]
	}
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	return fmRaw, string(rest), nil
}

// ParseFrontmatter decodes raw YAML into a model.Frontmatter.
//
// Unknown fields are rejected: schema drift surfaces here rather than
// silently dropping data. Required fields (created_by, updated_at) are
// validated; missing required fields are returned as errors so callers
// can wrap with model.ErrFrontmatterInvalid.
func ParseFrontmatter(raw []byte) (model.Frontmatter, error) {
	var fm model.Frontmatter
	if len(bytes.TrimSpace(raw)) == 0 {
		return fm, errors.New("frontmatter is empty")
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil {
		return fm, fmt.Errorf("yaml decode: %w", err)
	}

	if err := validateFrontmatter(&fm); err != nil {
		return fm, err
	}
	return fm, nil
}

// SerializeMarkdown re-emits an Intent as a .md file:
//
//	---
//	<yaml>
//	---
//	<body>
//
// The body is emitted verbatim. The frontmatter is encoded by yaml.Marshal.
func SerializeMarkdown(intent *model.Intent) ([]byte, error) {
	if intent == nil {
		return nil, errors.New("intent is nil")
	}
	yml, err := yaml.Marshal(&intent.Frontmatter)
	if err != nil {
		return nil, fmt.Errorf("yaml encode: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yml)
	buf.WriteString("---\n")
	if intent.Body != "" {
		buf.WriteString(intent.Body)
		// Ensure trailing newline if body doesn't end with one.
		if !strings.HasSuffix(intent.Body, "\n") {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes(), nil
}

// validateFrontmatter checks required-field presence and value shape.
// It does NOT validate slug (caller's responsibility) or check for
// cross-references (lint's job).
func validateFrontmatter(fm *model.Frontmatter) error {
	if strings.TrimSpace(fm.CreatedBy) == "" {
		return errors.New("created_by is required")
	}
	if fm.UpdatedAt.IsZero() {
		return errors.New("updated_at is required")
	}
	switch fm.Status {
	case "", model.StatusDraft, model.StatusActive, model.StatusSuperseded:
		// ok
	default:
		return fmt.Errorf("status %q is invalid (allowed: draft, active, superseded)", fm.Status)
	}
	for i, a := range fm.Assumptions {
		if strings.TrimSpace(a.ID) == "" {
			return fmt.Errorf("assumptions[%d].id is required", i)
		}
		switch a.Severity {
		case model.SeverityHard, model.SeveritySoft:
			// ok
		case "":
			return fmt.Errorf("assumptions[%d].severity is required", i)
		default:
			return fmt.Errorf("assumptions[%d].severity %q is invalid", i, a.Severity)
		}
	}
	return nil
}

// ValidateSlug checks that slug conforms to the on-disk path shape:
// lowercase ASCII, digits, dashes, underscores, with optional "/" for
// grouping. No leading/trailing slash. No "..". No ".md" suffix.
//
// Examples of valid slugs:
//
//	"auth/jwt-sliding-window"
//	"payments/tencent-adapter"
//	"single-region-deployment"
//
// Examples of invalid slugs:
//
//	"Auth/JWT"             (uppercase)
//	"./auth"               (leading /)
//	"auth/"                (trailing /)
//	"../etc/passwd"        (path traversal)
//	"auth/jwt.md"          (extension included)
var slugSegmentRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("%w: empty", model.ErrSlugInvalid)
	}
	if strings.Contains(slug, "\\") {
		return fmt.Errorf("%w: %q contains backslash", model.ErrSlugInvalid, slug)
	}
	if strings.Contains(slug, "..") {
		return fmt.Errorf("%w: %q contains \"..\"", model.ErrSlugInvalid, slug)
	}
	if strings.HasSuffix(slug, model.IntentExtension) {
		return fmt.Errorf("%w: %q must not include %s", model.ErrSlugInvalid, slug, model.IntentExtension)
	}
	segments := strings.Split(slug, "/")
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("%w: %q has empty path segment", model.ErrSlugInvalid, slug)
		}
		if !slugSegmentRE.MatchString(seg) {
			return fmt.Errorf("%w: %q contains invalid characters", model.ErrSlugInvalid, slug)
		}
	}
	return nil
}
