package lsp

import (
	"context"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Definition resolves the anchor under the cursor to the location of
// the intent's .md file (top of file). Like Hover, a dangling anchor or
// a position that is not on an anchor line yields a nil result — the
// editor simply does not navigate — rather than a protocol error.
func (s *server) Definition(ctx context.Context, params *protocol.DefinitionParams) (protocol.DefinitionResult, error) {
	slug, ok := s.anchorSlugAt(&params.TextDocumentPositionParams)
	if !ok {
		return nil, nil
	}

	intent, err := s.reader.Load(ctx, slug)
	if err != nil {
		return nil, nil
	}

	loc := protocol.Location{
		URI: uri.File(intent.SourcePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		},
	}
	return protocol.LocationSlice{loc}, nil
}
