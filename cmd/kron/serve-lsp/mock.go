package lsp

import (
	"github.com/xxx/kron/internal/model"
)

// mockAnchorsForFile is a temporary placeholder for the real
// parser.AnchorsForFile. The internal anchor scanner is owned by the
// internal/ layer and lands on master separately; until then this stub
// returns no anchors so serve-lsp compiles and the initialize →
// hover/definition → shutdown handshake stays wired end-to-end, with
// hover/definition degrading to "no content".
//
// TODO(serve-lsp): once parser.AnchorsForFile lands on master, switch
// the injection in Run() to the real implementation and delete this file.
func mockAnchorsForFile(filePath string) ([]model.Anchor, error) {
	return []model.Anchor{}, nil
}
