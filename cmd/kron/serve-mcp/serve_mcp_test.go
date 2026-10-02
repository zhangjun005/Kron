package servemcp

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_NotAvailable(t *testing.T) {
	var out, errOut bytes.Buffer
	err := Run(nil, &out, &errOut)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotImplemented), "Run must wrap ErrNotImplemented so main can map it to exit 1")
	assert.Empty(t, out.String(), "the placeholder must not write to stdout (the stdio server would consume it)")
	assert.Contains(t, errOut.String(), "not yet available")
}
