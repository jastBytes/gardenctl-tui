// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

// Package testutil contains helpers shared by tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeGardenctl mimics the parts of gardenctl that gtui uses: it keeps the
// target per GCTL_SESSION_ID under $TMPDIR/garden/sessions/<id>.
// `target view -o json` prints the stored target.
// FAKE_RESTRICTED=1 makes `target` ask for confirmation on stdin;
// FAKE_TARGET_ERROR=<msg> makes `target` fail.
const fakeGardenctl = `#!/bin/sh
set -e
need_sid() { [ -n "$GCTL_SESSION_ID" ] || { echo "Error: environment variable GCTL_SESSION_ID is required" >&2; exit 1; }; }
dir="$TMPDIR/garden/sessions/$GCTL_SESSION_ID"
case "$1" in
  version) echo '{"gitVersion":"v2.16.0","gitCommit":"abc"}' ;;
  target)
    need_sid
    if [ "$2" = view ]; then
      [ -f "$dir/target" ] || { echo '{}'; exit 0; }
      g= p= sd= sh= cp=false
      set -- $(cat "$dir/target")
      while [ $# -gt 0 ]; do
        case "$1" in
          --garden) g=$2; shift ;; --project) p=$2; shift ;; --seed) sd=$2; shift ;;
          --shoot) sh=$2; shift ;; --control-plane) cp=true ;;
        esac
        shift
      done
      printf '{"garden":"%s","project":"%s","seed":"%s","shoot":"%s","controlPlane":%s}\n' "$g" "$p" "$sd" "$sh" "$cp"
      exit 0
    fi
    if [ -n "$FAKE_TARGET_ERROR" ]; then echo "Error: $FAKE_TARGET_ERROR" >&2; exit 1; fi
    if [ -n "$FAKE_RESTRICTED" ]; then
      echo "Access restriction: this cluster is restricted"
      printf 'Do you want to continue? [y/N]: '
      ans=""; read ans || true
      [ "$ans" = y ] || { echo "Error: operation aborted" >&2; exit 1; }
    fi
    mkdir -p "$dir"
    shift
    echo "$*" > "$dir/target"
    printf 'apiVersion: v1\nkind: Config\n' > "$dir/kubeconfig.yaml"
    echo "Successfully targeted"
    [ "$KUBECONFIG" = "$dir/kubeconfig.yaml" ] || echo "WARN The KUBECONFIG environment variable does not point to the current target of gardenctl." ;;
  kubectl-env)
    need_sid
    [ -f "$dir/target" ] || { echo "Error: no target" >&2; exit 1; }
    echo "export KUBECONFIG='$dir/kubeconfig.yaml';"
    echo
    echo "# Run this command to configure kubectl for your shell:"
    echo "# eval \$(gardenctl kubectl-env bash)" ;;
  *) echo "Error: unknown command $1" >&2; exit 1 ;;
esac
`

// FakeGardenctl writes a fake gardenctl script, points TMPDIR to a fresh
// directory and returns the script path and that directory.
func FakeGardenctl(t testing.TB) (bin, tmp string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "gardenctl")
	if err := os.WriteFile(bin, []byte(fakeGardenctl), 0o700); err != nil { // #nosec G306 -- test script
		t.Fatal(err)
	}
	tmp = t.TempDir()
	t.Setenv("TMPDIR", tmp)
	return bin, tmp
}
