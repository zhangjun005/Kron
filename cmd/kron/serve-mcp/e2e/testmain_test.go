// TestMain wires process-scoped cleanup for the e2e test package.
// The built binary lives in a process-scoped directory (NOT a
// per-test t.TempDir) because t.TempDir is cleaned up at test end
// and races with parallel / subsequent tests that may still be
// spawning the binary.
package e2e

import (
	"os"
	"testing"
)

// TestMain delegates to the standard library's test runner after
// cleaning up the process-scoped build directory.  Go's testing
// package invokes TestMain in the main goroutine, blocking until
// m.Run() returns.
func TestMain(m *testing.M) {
	exit := m.Run()
	if processBuildDir != "" {
		_ = os.RemoveAll(processBuildDir)
	}
	os.Exit(exit)
}
