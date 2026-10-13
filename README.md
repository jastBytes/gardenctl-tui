# gtui – a terminal UI for gardenctl

`gtui` lets you browse your Gardener landscapes (garden → project → shoot, or
garden → seed → shoot), see the health of every shoot at a glance, and start a
shell, `k9s`, `kubectl` or any other tool **already targeted at the selected
cluster**. When the tool exits you are back in the TUI.

![gtui demo](docs/demo.gif)

> The GIF is generated from [`docs/demo.tape`](docs/demo.tape) with
> [vhs](https://github.com/charmbracelet/vhs): `make demo-gif`.

Your own shell is never touched. Every launched tool gets its own gardenctl
session (`GCTL_SESSION_ID`) and its own `KUBECONFIG`, so you can have k9s on one
shoot and a shell on another at the same time, and your parent shell keeps
whatever it was targeting before.

## Requirements

- [`gardenctl`](https://github.com/gardener/gardenctl-v2) **v2.10.0 or newer**
  on your `PATH`, configured with your gardens (`~/.garden/gardenctl-v2.yaml`).
  gtui checks the version on start-up and warns if it is missing or too old.
- `bash`, only if you use the provider-env option (`p`).
- The tools you want to launch (`k9s`, `kubectl`, `sofka`, …). Tools that are
  not installed are shown greyed out in the launcher menu.

## Install

```sh
# Homebrew (once the tap is published)
brew install --cask jastbytes/tap/gtui

# from source
go install github.com/jastbytes/gardenctl-tui/cmd/gtui@latest
```

## Usage

```sh
gtui                 # all gardens from the gardenctl config
gtui --garden dev    # start with one garden (identity or alias)
gtui --dump          # print gardens, projects and shoots as text
gtui --demo          # built-in fake landscape, no cluster access needed
```

Flags: `--config` (gardenctl config file), `--gtui-config` (gtui config file),
`--version`.

### Keys

| Key | Action |
|---|---|
| `↑/↓`, `j/k` | move |
| `enter` | open; on a shoot: details; in the details: launcher menu |
| `esc`, `h` | back |
| `/` | fuzzy filter (`esc` clears it) |
| `tab` | seeds of the garden (seed → shoots on that seed) |
| `l` | launcher menu for the selected shoot or seed |
| `s` `k` `c` `f` | Shell / k9s / kubectl / sofka on the selected shoot |
| `S` `K` `C` `F` | the same on the shoot's **control plane** (seed namespace) |
| `*` | toggle favorite |
| `ctrl+r` | recent targets and favorites |
| `p` | toggle provider-env (cloud provider CLI credentials) for launched tools |
| `r` | refresh, `a` toggles auto-refresh |
| `?` | help |
| `q`, `ctrl+c` | quit |

Tool keys are active in the shoot list, the seed list, the shoot details and
the recent list. There they take precedence over the vim keys, so `k` starts
k9s; use the arrow keys to move.

Status colors: green = all conditions `True`, yellow = an operation is running
or a condition is `Progressing`/`Unknown`, red = failed operation, errors or a
`False` condition, grey = hibernated.

## Configuration

gtui reads `~/.garden/gtui.yaml` (or `$GCTL_HOME/gtui.yaml`). Everything is
optional; these are the defaults:

```yaml
refreshInterval: 30s     # "0" disables auto-refresh
providerEnv: false       # start with provider-env enabled
gardenctl: gardenctl     # gardenctl binary
rememberLocation: false  # reopen the view gtui was quit in
tools:
  - key: "s"
    name: "Shell"
    command: ["$SHELL"]
  - key: "k"
    name: "k9s"
    command: ["k9s"]
  - key: "c"
    name: "kubectl"
    command: ["$SHELL", "-c", "kubectl get nodes; exec $SHELL"]
  - key: "f"
    name: "sofka"
    command: ["sofka"]
```

- Placeholders in `command`: `$SHELL`, `{garden}`, `{project}`, `{seed}`,
  `{shoot}`, `{namespace}` (the shoot's project namespace).
- Each tool also gets a control-plane key, by default the upper-case key. Set
  `controlPlaneKey` to choose another one.
- The keys `q r a l h p / ? *` are reserved.

- With `rememberLocation: true`, gtui saves where you were when you quit
  (garden, project or seed, shoot list or shoot details, and the selected row)
  and opens that view again on the next start. If an item no longer exists,
  gtui stops at the last level it can open. Pressing a key while it is still
  restoring stops the restoring.

Recent targets, favorites and the last location are stored in
`~/.garden/gtui-state.yaml`.

### Showing the target in your prompt

Launched tools get `GTUI_TARGET=garden/project/shoot` (with ` (control plane)`
appended for control-plane targets). Add it to your prompt so that you always
know which cluster a subshell talks to:

```sh
# bash (~/.bashrc)
if [ -n "$GTUI_TARGET" ]; then PS1="[gtui: $GTUI_TARGET] $PS1"; fi

# zsh (~/.zshrc)
if [[ -n "$GTUI_TARGET" ]]; then PROMPT="%F{cyan}[gtui: $GTUI_TARGET]%f $PROMPT"; fi
```

```fish
# fish (~/.config/fish/config.fish)
if set -q GTUI_TARGET
    functions -c fish_prompt __gtui_orig_prompt
    function fish_prompt
        set_color cyan; echo -n "[gtui: $GTUI_TARGET] "; set_color normal
        __gtui_orig_prompt
    end
end
```

## How it works

Listing uses the Gardener API directly (read-only) with the garden kubeconfigs
from the gardenctl configuration. Targeting is left to gardenctl:

1. A new session ID (UUID) is created and used as `GCTL_SESSION_ID`.
2. `gardenctl target --garden G --project P --shoot S [--control-plane]` runs
   with that session ID (`--seed` instead of `--project` when you came through
   the seeds view). It runs in the terminal, so you can answer an
   access-restriction confirmation or follow a login prompt.
3. `gardenctl kubectl-env bash` returns `export KUBECONFIG='…';`, which gtui
   parses (gardenctl has no machine-readable output for this).
4. With provider-env on, the tool is started through
   `bash -c 'eval "$(gardenctl provider-env bash)"; exec tool'`, because that
   output may run provider CLI logins and cannot be parsed safely.
5. The tool runs with the terminal handed over (`tea.Exec`); afterwards the
   current view is refreshed.

On exit gtui deletes the gardenctl session directories it created
(`$TMPDIR/garden/sessions/<id>`), and only those. Details about the gardenctl
internals gtui relies on are in [docs/gardenctl-notes.md](docs/gardenctl-notes.md).

Credential plugins (e.g. OIDC via kubelogin) cannot read from the terminal while
the UI runs; log in once with `kubectl` beforehand if your plugin needs input.
Library log output is written to a temporary log file whose path is printed on
exit.

## Limitations (v1)

- Read-only: no creating, changing or deleting shoots.
- No resource views inside a cluster – that is what k9s is for.
- Linux and macOS only (subshell start and env parsing for PowerShell come later).
- Lists are loaded in full (no pagination/watch yet).

## Development

```sh
make build test lint
go test ./internal/ui/ -update   # refresh teatest golden files
gtui --demo                      # try the UI without a cluster
```

### Releasing

Tag `vX.Y.Z` and push the tag; the release workflow runs goreleaser. Before the
first release create the `jastbytes/homebrew-tap` repository and add a
`HOMEBREW_TAP_GITHUB_TOKEN` secret that can push to it.

## Relation to gardenctl

gtui is a separate tool that only uses gardenctl's CLI. Whether it should rather
become `gardenctl tui` is an open question for the gardenctl maintainers; see
[docs/upstream-issue.md](docs/upstream-issue.md) for the proposal.
