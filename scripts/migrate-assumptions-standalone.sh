#!/bin/bash
#
# scripts/migrate-assumptions-standalone.sh
#
# B-3 (RFC 2026-10-08-assumptions-standalone): migrate inline
# assumptions (A-path) to standalone .kron/assumptions/<id>.md
# files (B-path). The real work happens in the Go subcommand
# `kron migrate assumptions --to-standalone`; this bash wrapper
# exists for users who prefer invoking a script and for CI
# pipelines that grep for a stable filename.
#
# Usage:
#   MIGRATE_DRYRUN=1 scripts/migrate-assumptions-standalone.sh
#   scripts/migrate-assumptions-standalone.sh --yes
#
# Idempotency: re-running is a no-op (skipped entries). A
# divergent existing file is reported as an error; resolve
# manually.

set -euo pipefail

DRYRUN_FLAG=""
YES_FLAG=""
if [ "${MIGRATE_DRYRUN:-0}" = "1" ]; then
  DRYRUN_FLAG="--dry-run"
fi
for arg in "$@"; do
  case "$arg" in
    --yes|-y) YES_FLAG="--yes" ;;
    --dry-run) DRYRUN_FLAG="--dry-run" ;;
    --help|-h)
      sed -n '2,18p' "$0"
      exit 0
      ;;
    *) echo "unknown flag: $arg" >&2; exit 1 ;;
  esac
done

# Locate the kron binary: $KRON_BIN, else ./kron, else `go run`.
KRON="${KRON_BIN:-}"
if [ -z "$KRON" ]; then
  if [ -x "./kron" ]; then
    KRON="./kron"
  else
    KRON="go run ./cmd/kron"
  fi
fi

echo "==> migrating assumptions (B-3 standalone); DRYRUN=${MIGRATE_DRYRUN:-0}"
$KRON migrate assumptions --to-standalone $DRYRUN_FLAG $YES_FLAG
echo "==> done. Run 'kron lint' to verify the result."

# 5. (Bash-layer validation hook) After the Go subcommand returns
#    successfully, suggest `kron lint` for the user to confirm
#    the migration produced clean output. The Go tool does not
#    auto-run lint because doing so would couple two access
#    layers (CLI's migrate entry point + CLI's lint entry point)
#    in a single shell pipeline; the user can decide when to
#    invoke lint.
