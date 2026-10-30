package servemcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleIntentBodyTemplate = "## Why\n\nExplain the decision.\n\n## How\n\nExplain the implementation."

func writeIntentBodyTemplate(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".kron", "templates")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "intent-body.md"), []byte(content), 0o644))
}

func TestIntentBodyTemplate_AbsentMeansNoInjection(t *testing.T) {
	root := t.TempDir()
	assert.Empty(t, readIntentBodyTemplate(root))
	assert.NotContains(t, serverInstructionsFor(""), "Project intent body template")
	assert.Equal(t, "base", intentToolDescription("base", ""))
}

func TestIntentBodyTemplate_InstructionsAndToolDescription(t *testing.T) {
	root := t.TempDir()
	writeIntentBodyTemplate(t, root, sampleIntentBodyTemplate)

	tmpl := readIntentBodyTemplate(root)
	assert.Contains(t, tmpl, "## Why")
	assert.Contains(t, serverInstructionsFor(tmpl), "## Project intent body template")
	assert.Contains(t, serverInstructionsFor(tmpl), "## How")
	description := intentToolDescription("Create intent.", tmpl)
	assert.Contains(t, description, "## Why")
	assert.Contains(t, description, "This project's intent body template")
}

func TestIntentBodyTemplate_InjectedIntoKronAddDescriptionAndImplementPrompt(t *testing.T) {
	root := t.TempDir()
	writeIntentBodyTemplate(t, root, sampleIntentBodyTemplate)

	ts := newTestSession(t, root)
	defer ts.Cleanup()

	tools, err := ts.Session.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(t, err)
	var addDescription string
	for _, tool := range tools.Tools {
		if tool.Name == "kron_add" {
			addDescription = tool.Description
		}
	}
	require.NotEmpty(t, addDescription)
	assert.Contains(t, addDescription, "## Why", "kron_add description must carry the project body template")
	assert.Contains(t, addDescription, "## How")

	res, err := ts.Session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "kron-implement",
		Arguments: map[string]string{"feature": "absolutely-no-such-feature-keyword-xyzzy"},
	})
	require.NoError(t, err)
	text := textContent(t, res.Messages[0])
	assert.Contains(t, text, "## Project intent body template", "kron-implement must include the template")
	assert.Contains(t, text, "## How")
}

func TestIntentBodyTemplate_EmptyFileDisabled(t *testing.T) {
	root := t.TempDir()
	writeIntentBodyTemplate(t, root, "   \n\n")
	assert.Empty(t, readIntentBodyTemplate(root))
	assert.False(t, strings.Contains(serverInstructionsFor(readIntentBodyTemplate(root)), "Project intent body template"))
}
