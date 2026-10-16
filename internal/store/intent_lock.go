package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xxx/kron/internal/model"
	"github.com/xxx/kron/internal/parser"
)

// withIntentLock serializes mutations to one logical intent across Writer
// instances and processes. The stable sidecar path remains the same when the
// intent file itself is atomically replaced.
func (w *Writer) withIntentLock(slug string, mutate func() error) (result error) {
	if err := parser.ValidateSlug(slug); err != nil {
		return fmt.Errorf("%w: %s", model.ErrSlugInvalid, slug)
	}
	unlock, err := w.lockIntent(slug)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			result = errors.Join(result, fmt.Errorf("store: unlock intent %s: %w", slug, unlockErr))
		}
	}()
	return mutate()
}

func (w *Writer) lockIntent(slug string) (func() error, error) {
	key := sha256.Sum256([]byte(slug))
	lockDir := filepath.Join(w.Root, model.KronDir, ".lock")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create lock directory: %w", err)
	}
	lockPath := filepath.Join(lockDir, hex.EncodeToString(key[:])+".flock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("store: open intent lock: %w", err)
	}
	if err := lockOSFile(file); err != nil {
		closeErr := file.Close()
		return nil, errors.Join(fmt.Errorf("store: acquire intent lock: %w", err), closeErr)
	}
	return func() error {
		unlockErr := unlockOSFile(file)
		closeErr := file.Close()
		return errors.Join(unlockErr, closeErr)
	}, nil
}
