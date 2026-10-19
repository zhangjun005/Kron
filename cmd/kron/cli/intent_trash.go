package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/xxx/kron/internal/store"
)

func runTrash(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		fmt.Fprintln(errOut, "Usage: kron trash <slug>")
		return errUsage
	}
	w, err := store.NewWriter(".")
	if err != nil {
		return fmt.Errorf("kron trash: open repository: %w", err)
	}
	if err := w.MoveToTrash(context.Background(), args[0]); err != nil {
		fmt.Fprintf(errOut, "kron trash %s: %v\n", args[0], err)
		return errUsage
	}
	fmt.Fprintf(out, "kron: trashed %s\n", args[0])
	return nil
}

func runRestore(args []string, out, errOut io.Writer) error {
	if len(args) != 1 {
		fmt.Fprintln(errOut, "Usage: kron restore <slug>")
		return errUsage
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("kron restore: get working directory: %w", err)
	}
	w, err := store.NewWriter(cwd)
	if err != nil {
		return fmt.Errorf("kron restore: open repository: %w", err)
	}
	if err := w.RestoreFromTrash(context.Background(), args[0]); err != nil {
		fmt.Fprintf(errOut, "kron restore %s: %v\n", args[0], err)
		return errUsage
	}
	fmt.Fprintf(out, "kron: restored %s\n", args[0])
	return nil
}
