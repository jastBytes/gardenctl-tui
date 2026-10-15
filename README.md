# gtui – a terminal UI for gardenctl

[![CI](https://img.shields.io/github/actions/workflow/status/jastBytes/gardenctl-tui/ci.yml?branch=main&label=CI)](https://github.com/jastBytes/gardenctl-tui/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jastBytes/gardenctl-tui)](https://github.com/jastBytes/gardenctl-tui/releases/latest)
[![License](https://img.shields.io/github/license/jastBytes/gardenctl-tui)](LICENSE)

`gtui` lets you browse your Gardener landscapes (garden → project → shoot, or
garden → seed → shoot), see the health of every shoot at a glance, and start a
shell, `k9s`, `kubectl` or any other tool **already targeted at the selected
cluster**. When the tool exits you are back in the TUI – or, if you prefer, the
tool opens in a new terminal window and gtui stays usable.

![gtui demo](docs/demo.gif)

> The GIF is generated from [`docs/demo.tape`](docs/demo.tape) with
> [vhs](https://github.com/charmbracelet/vhs): `make demo-gif`.

Your own shell is never touched. Every launched tool gets its own gardenctl
session (`GCTL_SESSION_ID`) and its own `KUBECONFIG`, so you can have k9s on one
shoot and a shell on another at the same time, and your parent shell keeps
whatever it was targeting before.

> [!NOTE]
> gtui has been vibe coded: most of its code was written with an AI coding
> assistant. It is covered by tests and CI, but use it at your own risk. The
> tools it starts get real cluster credentials, so try `gtui --demo` first and
> please report anything odd.

## Requirements

- [`gardenctl`](https://github.com/gardener/gardenctl-v2) **v2.10.0 or newer**
  on your `PATH`, configured with your gardens (`~/.garden/gardenctl-v2.yaml`).
  gtui checks the version on start-up and warns if it is missing or too old.
- `bash`, only if you use the provider-env option (`p`).
- The tools you want to launch (`k9s`, `kubectl`, `sofka`, …). Tools that are
  not installed are shown greyed out in the launcher menu.

## Install

```sh
# Homebrew
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
| `A` | all shoots of the garden (in the project list) |
| `l` | launcher menu for the selected shoot or seed |
| `s` `9` `c` `f` | Shell / k9s / kubectl / sofka on the selected shoot |
| `S` `K` `C` `F` | the same on the shoot's **control plane** (seed namespace) |
| `*` | toggle favorite |
| `!` | shoot lists: show only shoots with problems (red or yellow) |
| `o` | shoot lists: sort by name, status (problems first), Kubernetes version (oldest first) or last operation (newest first) |
| `y` | copy the name, path, `gardenctl target` command or technical ID |
| `v` | the shoot's YAML manifest (`y` there copies it) |
| `ctrl+r` | favorites (with live status) and recent targets |
| `p` | toggle provider-env (cloud provider CLI credentials) for launched tools |
| `w` | toggle where tools start: this terminal or a new terminal window |
| `r` | refresh, `a` toggles auto-refresh |
| `?` | help |
| `q`, `ctrl+c` | quit |

Tool keys are active in the shoot list, the seed list, the shoot details and
the recent list. A tool bound to a vim key (`j`, `k`, `g`, `G`) takes
precedence over it there; use the arrow keys to move then.

Shoot lists start with a summary (`5 shoots  ✗ 1 error  ✓ 2 healthy …`), and
the project list shows the number of shoots per project with the problems
(`3 ✗1 ⟳1`: one red, one yellow), colored with the worst status. The counts
need permission to list shoots in all namespaces; without it the column stays
empty.

Running operations show their progress (`Reconcile 63% ▰▰▰▰▰▱▱▱`), and
`Updated` says how long ago the last operation changed. A row whose status
just changed is marked with `▮` for ten seconds. When the terminal is too
narrow, less important columns (purpose, owner, age) are hidden instead of
cutting off the others.

The shoot details also show the DNS domain, the API server, the worker pools
(machine type, image, node range, zones), the maintenance window and
auto-updates, hibernation schedules and the state of credentials rotations.

### Favorites

Mark shoots with `*`; `ctrl+r` lists them with their live status, together
with your recent targets. While gtui runs, it watches the project namespaces
of your favorites and recent shoots (no cluster-wide permissions needed). When a favorite turns
red, gtui says so in the status line and rings the terminal bell; with
`notify: [bell, desktop]` you also get a desktop notification (`notify-send`
on Linux, `osascript` on macOS), `notify: []` turns both off. A favorite that
recovers is reported in the status line. With `favoritesOnStart: true` gtui
opens the favorites view on start. Without watches (`watch: false`) the
favorites view reloads every `refreshInterval` while it is open, and there are
no notifications.

Copying uses the system clipboard (`pbcopy`, `wl-copy`, `xclip`/`xsel`) when
available and also sends the text to the terminal as an OSC 52 sequence, which
works over SSH. In tmux this needs `set -g set-clipboard on`.

Status: `✓` green = all conditions `True`, `⟳` yellow = an operation is
running or a condition is `Progressing`/`Unknown`, `✗` red = failed operation,
errors or a `False` condition, `☾` grey = hibernated, `?` = unknown. The
symbols carry the status without colors too: with
[`NO_COLOR`](https://no-color.org) set, gtui uses no colors and shows the
selected row in reverse video.

## Configuration

gtui reads `~/.garden/gtui.yaml` (or `$GCTL_HOME/gtui.yaml`). Everything is
optional; these are the defaults:

```yaml
refreshInterval: 5s      # "0" disables auto-refresh
watch: true              # keep shoot lists up to date with watches
notify: [bell]           # when a favorite turns red: bell and/or desktop
favoritesOnStart: false  # open the favorites view on start
providerEnv: false       # start with provider-env enabled
newWindow: false         # start tools in a new terminal window (toggle: w)
terminal: []             # command that opens a window, e.g. [tmux, new-window]
gardenctl: gardenctl     # gardenctl binary
rememberLocation: false  # reopen the view gtui was quit in
tools:
  - key: "s"
    name: "Shell"
    command: ["$SHELL"]
  - key: "9"
    name: "k9s"
    command: ["k9s"]
    controlPlaneKey: "K"
  - key: "c"
    name: "kubectl"
    command: ["$SHELL", "-c", "kubectl get nodes; exec $SHELL"]
  - key: "f"
    name: "sofka"
    command: ["sofka"]
```

- `refreshInterval` reloads the current view every interval (e.g. `10s`,
  `1m`). `a` pauses and resumes it while gtui runs; `0` turns it off
  completely (then `a` cannot turn it on). `r` refreshes once.
- With `watch: true` (the default) the shoot lists and the shoot counts of the
  project list are not reloaded but kept up to date with a Kubernetes watch:
  changes appear within a second, and the API server only sends what changed.
  The header shows `● live` then. `a` pauses the updates (`r` shows the
  latest list once), and lost connections are re-established with backoff. If
  you may not watch shoots, gtui falls back to reloading every
  `refreshInterval`. Projects, seeds and the shoot details are small and still
  reloaded every interval.
- Placeholders in `command`: `$SHELL`, `{garden}`, `{project}`, `{seed}`,
  `{shoot}`, `{namespace}` (the shoot's project namespace).
- Each tool also gets a control-plane key, by default the upper-case key. Set
  `controlPlaneKey` to choose another one.
- The keys `q r a l h p w o v y / ? * !` are reserved.
- With `rememberLocation: true`, gtui saves where you were when you quit
  (garden, project or seed, shoot list or shoot details, and the selected row)
  and opens that view again on the next start. If an item no longer exists,
  gtui stops at the last level it can open. Pressing a key while it is still
  restoring stops the restoring.

### Same terminal or new window

By default a launched tool takes over gtui's terminal until it exits. Press `w`
(or set `newWindow: true`) to start tools in a new terminal window instead;
gtui stays usable and you can start further tools right away. Targeting
(`gardenctl target`) still runs briefly in gtui's terminal, because it may ask
for a confirmation.

`terminal` is the command prefix that opens the window; the tool's command is
appended. Without it gtui uses tmux (`tmux new-window`), zellij, kitty, WezTerm,
Ghostty, Alacritty or gnome-terminal when it runs inside one of them, and
otherwise `x-terminal-emulator -e`. Examples:

```yaml
terminal: [tmux, split-window, -h]        # a tmux pane instead of a window
terminal: [kitty, "@", launch, --type=tab] # a kitty tab (needs allow_remote_control)
terminal: [wezterm, cli, spawn, --]       # a WezTerm tab
```

The session environment (`KUBECONFIG`, `GCTL_SESSION_ID`, …) is passed through
`env(1)`, so it also reaches terminals that reuse a running instance. Sessions of
tools started in a window are not deleted when gtui exits, as the window may
still use them.

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
   current view is refreshed. In new-window mode it is started detached in the
   configured terminal instead.

On exit gtui deletes the gardenctl session directories it created
(`$TMPDIR/garden/sessions/<id>`), and only those (except sessions of tools
started in a new window). Details about the gardenctl
internals gtui relies on are in [docs/gardenctl-notes.md](docs/gardenctl-notes.md).

Credential plugins (e.g. OIDC via kubelogin) cannot read from the terminal while
the UI runs; log in once with `kubectl` beforehand if your plugin needs input.
Library log output is written to a temporary log file whose path is printed on
exit.

## Limitations (v1)

- Read-only: no creating, changing or deleting shoots.
- No resource views inside a cluster – that is what k9s is for.
- Linux and macOS only (subshell start and env parsing for PowerShell come later).
- Lists are loaded in full (no pagination); only shoots are watched.

## Development

```sh
make all                         # build, vet, test, lint
make vuln                        # govulncheck
go test ./internal/ui/ -update   # refresh teatest golden files
gtui --demo                      # try the UI without a cluster
```

CI (`.github/workflows/ci.yml`) runs golangci-lint, `go vet`, the tests with the
race detector and coverage, a `go mod tidy` check, govulncheck and
`goreleaser check`; binaries are only built by the release workflow. Dependabot
keeps Go modules and GitHub Actions up to date.

### Releasing

[Release Drafter](https://github.com/release-drafter/release-drafter) keeps a
draft release with the merged pull requests, grouped by label (`enhancement`,
`bug`, `documentation`, `dependencies`; `skip-changelog` leaves a pull request
out). It proposes the next version: `enhancement` or `minor` bumps the minor
version, `major` the major version, anything else the patch version. Pull
requests are labeled from their files, branch and title where possible.

To release, edit the draft on GitHub if needed and publish it. Publishing
creates the tag, and the release workflow runs goreleaser, which adds the
binaries to that release (keeping its notes) and updates the Homebrew cask. A
tag pushed without publishing a release gets a release with goreleaser's own
changelog. The release workflow needs a `HOMEBREW_TAP_GITHUB_TOKEN` secret that
can push to `jastbytes/homebrew-tap`.

## Relation to gardenctl

gtui is a separate tool that only uses gardenctl's CLI. Whether it should rather
become `gardenctl tui` is an open question for the gardenctl maintainers; see
[docs/upstream-issue.md](docs/upstream-issue.md) for the proposal.

## License

gtui is licensed under the [Apache License 2.0](LICENSE). See
[CONTRIBUTING.md](CONTRIBUTING.md) to contribute and [SECURITY.md](SECURITY.md)
to report vulnerabilities.
