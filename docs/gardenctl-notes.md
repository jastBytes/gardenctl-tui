# gardenctl-v2 notes

Findings from reading `gardener/gardenctl-v2` (master, `VERSION` = `v2.17.0-dev`,
Go 1.26). These are the parts of gardenctl that gtui depends on.

## Config file

- Location: `$GCTL_HOME/<name>.yaml`, falling back to `~/.garden/<name>.yaml`.
  `<name>` is `$GCTL_CONFIG_NAME` or `gardenctl-v2` (`pkg/cmd/cmd.go`, `initConfig`).
  gardenctl also accepts `--config <file>`.
- Format (`pkg/config/config.go`):

  ```yaml
  linkKubeconfig: true          # optional, default true; env GCTL_LINK_KUBECONFIG overrides
  gardens:
    - identity: landscape-dev   # the garden name used for targeting (required)
      name: dev                 # optional alias
      kubeconfig: ~/.garden/dev.yaml   # "~" is expanded
      context: my-context       # optional, overrides current-context
      patterns: [...]           # optional regex patterns
      accessRestrictions: [...]
      kubeconfigAccessLevelDefaults: {shoots: admin, seeds: viewer}
  provider: {...}
  ```

  Note the confusing YAML keys: `identity` is the name, `name` is the alias.
- Garden clients are built with
  `clientcmd.NewNonInteractiveDeferredLoadingClientConfig({ExplicitPath: kubeconfig}, {CurrentContext: context})`.

## Sessions

- Session ID comes from `GCTL_SESSION_ID` (must match `^[\w-]{1,128}$`), else a
  UUID extracted from `TERM_SESSION_ID`, else gardenctl fails
  (`internal/util/factory.go`, `GetSessionID`).
- Session directory: `${TMPDIR:-/tmp}/garden/sessions/<sid>` (created with 0700).
  Older versions used `${TMPDIR}/garden/<sid>`; gardenctl migrates that on first use.
- Contents: `target.yaml` (current target), `kubeconfig.yaml` (symlink to the
  targeted cluster's kubeconfig when `linkKubeconfig` is true),
  `kubeconfig.<md5>.yaml` files, `provider-env/` temp data.
- Consequence for gtui: a fresh UUID per launched tool gives a fully isolated
  gardenctl state. Cleanup = `os.RemoveAll($TMPDIR/garden/sessions/<sid>)` for
  the IDs gtui created (never anything else).

## `gardenctl target`

- `gardenctl target --garden G --project P --shoot S [--control-plane]`
- Seed route: `gardenctl target --garden G --seed SEED`
- Also `--access-level admin|viewer|auto` (`--admin`, `--viewer` shorthands).
- `gardenctl target view -o json` prints the current target as JSON:
  `{"garden":..,"project":..,"seed":..,"shoot":..,"controlPlane":true}`.

## `gardenctl kubectl-env <shell>`

- **No machine-readable output** (`-o` is explicitly absent; there is a test for it).
- bash/zsh output (`pkg/env/templates/kubernetes.tmpl`):

  ```
  export KUBECONFIG='/tmp/garden/sessions/<sid>/kubeconfig.yaml';

  # Run this command to configure kubectl for your shell:
  # eval $(gardenctl kubectl-env bash)
  ```

  The value is POSIX single-quoted with `'` escaped as `'"'"'`
  (`pkg/env/escape.go`, `shellEscapePOSIX`). gtui parses this line (see
  `internal/session/parse.go`) and has tests for the quoting.
- With `linkKubeconfig: true` (default) the path is always
  `<sessionDir>/kubeconfig.yaml`; otherwise a `kubeconfig.<hash>.yaml` is written.
- Alternative that was considered: `gardenctl kubeconfig --raw` prints the
  kubeconfig itself, but then gtui would have to manage the file. Parsing
  `kubectl-env` keeps gardenctl in charge of the file.

## `gardenctl provider-env <shell>`

- Output for bash is not a plain list of variables: it reads secrets from temp
  files with `$(< file)` and deletes them with `rm -f`, and for gcloud/az it runs
  login commands. Parsing it is therefore not safe.
- gtui instead evaluates it inside the child process:
  `bash -c 'eval "$(gardenctl provider-env bash)" && exec "$@"' gtui <tool...>`.
  Requires `bash` on the PATH.

## `gardenctl version`

- `gardenctl version -o json` → `{"gitVersion":"v2.x.y", ...}` (k8s `version.Info`).
- `gardenctl version --short` → `Version: v2.x.y`.

## Library vs. CLI

gardenctl's `pkg/` packages are importable but not documented as public API.
gtui only shells out to the `gardenctl` binary for targeting and environment
generation, and reads the gardenctl config file and the Gardener API itself
(read-only) for listing.
