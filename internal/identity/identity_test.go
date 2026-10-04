package identity

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitUser_NoGit(t *testing.T) {
	// On CI / sandboxes without git configured, GitUser returns "".
	// We don't assert that here (it depends on the test env) — but
	// GitUser must NEVER return something with a missing "@" prefix
	// or with stray whitespace.
	got := GitUser()
	if got != "" {
		assert.True(t, strings.HasPrefix(got, "@"))
		assert.Equal(t, strings.TrimSpace(got), got)
	}
}

func TestHandle_Empty(t *testing.T) {
	assert.Equal(t, DefaultUnknown, Handle())
	assert.Equal(t, DefaultUnknown, Handle(""))
	assert.Equal(t, DefaultUnknown, Handle("  ", "\t"))
}

func TestHandle_FirstNonEmptyWins(t *testing.T) {
	assert.Equal(t, "@alice", Handle("", "alice", "bob"))
	assert.Equal(t, "agent:gpt", Handle("", "agent:gpt"))
}

func TestHandle_AutoAtPrefix(t *testing.T) {
	assert.Equal(t, "@alice", Handle("alice"))
	assert.Equal(t, "@bob", Handle("bob"))
}

func TestHandle_PreservesExplicitAt(t *testing.T) {
	// Caller-supplied "@alice" must not become "@@alice".
	assert.Equal(t, "@alice", Handle("@alice"))
}

func TestHandle_PreservesAgentPrefix(t *testing.T) {
	// "agent:<model>" already has a prefix; do not prefix again.
	assert.Equal(t, "agent:claude-3-7", Handle("agent:claude-3-7"))
}

func TestHandle_TrimsWhitespace(t *testing.T) {
	assert.Equal(t, "@alice", Handle("  alice  "))
	assert.Equal(t, "agent:claude-3-7", Handle("\tagent:claude-3-7\n"))
}
