# Proposal for gardener/gardenctl-v2

*Draft for an issue in [gardener/gardenctl-v2](https://github.com/gardener/gardenctl-v2/issues).
Not filed yet; link it from the README once it exists.*

**Title:** Terminal UI on top of gardenctl – separate tool or `gardenctl tui`?

Hi gardenctl maintainers,

I built [gtui](https://github.com/jastbytes/gardenctl-tui), a terminal UI for
browsing Gardener landscapes (garden → project/seed → shoot) that starts a shell,
k9s or kubectl already targeted at the selected shoot or its control plane. It
uses gardenctl for all targeting:

- a fresh `GCTL_SESSION_ID` per launched tool,
- `gardenctl target --garden … --project … --shoot … [--control-plane]`,
- `gardenctl kubectl-env bash` to obtain `KUBECONFIG`,
- optionally `gardenctl provider-env bash`, evaluated in the child shell.

Questions:

1. Would you rather have this as `gardenctl tui` inside gardenctl, or keep it a
   separate tool that only depends on the CLI?
2. Would you accept a machine-readable output for `kubectl-env` (e.g.
   `gardenctl kubectl-env -o json` → `{"KUBECONFIG": "…"}`)? Today gtui parses
   the `export KUBECONFIG='…';` line of the bash output.
3. Is the session directory layout (`$TMPDIR/garden/sessions/<id>`) something a
   tool may rely on for cleanup, or would a `gardenctl session delete` (or
   similar) be preferable?
4. Are the Go packages under `pkg/` meant to be used as a library?

Thanks!
