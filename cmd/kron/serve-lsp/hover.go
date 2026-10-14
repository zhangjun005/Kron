package lsp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"github.com/xxx/kron/internal/model"
)

// Hover returns the rendered intent for the anchor under the cursor.
// It returns (nil, nil) — an empty hover — when the position is not on
// an anchor line, the file cannot be read, or the anchor dangles (its
// slug has no matching intent file). A dangling anchor is not an error:
// the RFC requires hover to degrade to no content, never to report a
// protocol failure.
func (s *server) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	slug, ok := s.anchorSlugAt(ctx, &params.TextDocumentPositionParams)
	if !ok {
		return nil, nil
	}

	intent, err := s.reader.Load(ctx, slug)
	if err != nil {
		// Dangling anchor (RFC §6): not-found is the expected failure
		// mode for a stale @kron:intent line; degrade to no hover
		// without surfacing an LSP error. Any other error (frontmatter
		// corruption, I/O failure) is logged via the notifier so the
		// editor's Output panel surfaces it.
		if errors.Is(err, model.ErrIntentNotFound) {
			return nil, nil
		}
		s.logf(ctx, protocol.MessageTypeError, "kron serve-lsp: hover: load %s: %v", slug, err)
		return nil, nil
	}

	return &protocol.Hover{
		Contents: &protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: renderIntentHover(intent),
		},
	}, nil
}

// renderIntentHover builds the markdown shown in the hover popup: the
// slug (with lifecycle status, if set) as the heading, then a short
// body excerpt as the summary.
func renderIntentHover(intent *model.Intent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s**", intent.Slug)
	if intent.Frontmatter.Status != "" {
		fmt.Fprintf(&b, " · %s", intent.Frontmatter.Status)
	}
	if summary := summarizeBody(intent.Body); summary != "" {
		b.WriteString("\n\n")
		b.WriteString(summary)
	}
	return b.String()
}

// summarizeBody returns the first non-empty paragraph of the intent
// body, truncated to a hover-friendly length.
func summarizeBody(body string) string {
	const maxLen = 200

	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if idx := strings.Index(body, "\n\n"); idx >= 0 {
		body = body[:idx]
	}
	body = strings.TrimSpace(body)
	// Truncate by rune, not byte: intent bodies are often Chinese, and
	// a byte slice would cut a 3-byte rune in half and emit invalid
	// UTF-8 (mojibake) in the hover popup.
	if runes := []rune(body); len(runes) > maxLen {
		body = string(runes[:maxLen]) + "…"
	}
	return body
}
