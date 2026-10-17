# AGENTS.md

Guidance for AI coding agents working on this repository. Human-facing docs are
in [README.md](README.md) and [CONTRIBUTING.md](CONTRIBUTING.md); this file
summarizes what you need to make a change that passes CI and review.

## Project

`gtui` is a terminal UI (Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea))
for [gardenctl](https://github.com/gardener/gardenctl-v2). It browses Gardener
landscapes (garden → project → shoot, or garden → seed → shoot) read-only via
the Gardener API and starts tools (shell, k9s, kubectl, …) targeted at the
selected cluster, each in its own isolated gardenctl session.

## Layout

| Path | Contents |
|---|---|
| `cmd/gtui/` | `main.go` (flags, start-up), `--dump` and `--demo` modes |
| `internal/ui/` | Bubble Tea models: one file per view (`gardens.go`, `projects.go`, `shoots.go`, `seeds.go`, `detail.go`, `yaml.go`, `recent.go`, `goto.go`, `launcher_menu.go`), plus keys, messages, table rendering, watches, notifications, clipboard |
| `internal/ui/styles/` | lipgloss styles and color themes |
| `internal/ui/testdata/` | golden files for the teatest snapshot tests |
| `internal/gardener/` | `Client` interface for Projects/Shoots/Seeds, types, status logic, watches |
| `internal/gardener/fake/` | in-memory `Client` used by tests and `--demo` (`fake.Landscape()`) |
| `internal/session/` | gardenctl sessions: `GCTL_SESSION_ID`, `gardenctl target`, `kubectl-env` parsing, cleanup |
| `internal/launcher/` | builds and runs tool commands (placeholders, provider-env, new terminal window) |
| `internal/config/` | reads `gardenctl-v2.yaml` and gtui's own `gtui.yaml` (defaults, validation, reserved keys) |
| `internal/state/` | persists favorites, recent targets and last location (`gtui-state.yaml`) |
| `internal/testutil/` | shared test helpers, incl. a fake `gardenctl` shell script |
| `docs/` | gardenctl internals gtui relies on (`gardenctl-notes.md`), upstream proposal, demo tape/GIF |

## Commands

```sh
make all                         # build, vet, test, lint – run before every push
make build                       # binary in bin/gtui
make test                        # go test ./...
make lint                        # golangci-lint (version pinned in the Makefile, installed into bin/)
make fmt                         # gofmt + goimports via golangci-lint
make tidy                        # go mod tidy
make vuln                        # govulncheck
go test ./internal/ui/ -update   # regenerate golden files after intended UI changes
go run ./cmd/gtui --demo         # run the UI against the fake landscape, no cluster needed
```

CI (`.github/workflows/ci.yml`) runs golangci-lint, `go vet`,
`go test -race` with coverage, a `go mod tidy` diff check, govulncheck and
`goreleaser check`. Mirror it locally: `make all`, plus `go test -race ./...`
and `go mod tidy` with no diff when dependencies change.

## Conventions

- **License header**: every new Go file starts with

  ```go
  // SPDX-FileCopyrightText: 2026 Jan Steffen
  // SPDX-License-Identifier: Apache-2.0
  ```

- **Imports**: grouped by goimports with `github.com/jastbytes/gardenctl-tui`
  as the local prefix (stdlib, third party, then this module).
- **Lint**: `errorlint`, `gocritic`, `gosec`, `misspell`, `revive`,
  `unconvert`, `unparam` are enabled (`.golangci.yml`). Wrap errors with `%w`
  and compare with `errors.Is/As`.
- **Style**: match the surrounding code — short doc comments on non-trivial
  functions, a package comment in one file per package, no needless
  abstractions.
- **Read-only**: gtui never creates, changes or deletes Gardener resources.
  Keep it that way.
- **Targeting goes through gardenctl's CLI** (`session.Runner`), not through
  reimplemented gardenctl logic. When relying on gardenctl behavior, check and
  update `docs/gardenctl-notes.md`. Minimum supported version is
  `session.MinGardenctlVersion`.
- **Session safety**: only ever delete the session directories gtui created
  itself (`$TMPDIR/garden/sessions/<id>`). Never touch the user's own shell
  session or `KUBECONFIG`.
- **Untrusted text**: free text from shoots (annotations, messages, …) is
  passed through `cleanText` in `internal/gardener/types.go` to strip control
  characters before it reaches the terminal. Do the same for new fields.
- **Keys**: tool keys may not collide with the reserved UI keys in
  `internal/config/gtui.go` (`reservedKeys`). When adding a UI key, add it
  there, to the help overlay and to the key table in the README.
- **No color dependency**: status must stay readable without colors (symbols
  `✓ ⟳ ✗ ☾ ?`, `NO_COLOR`, the `mono` theme).

## Tests

- Unit tests live next to the code (`*_test.go`); prefer table-driven tests.
- UI tests use `teatest` with the fake client: start the app with
  `startApp(t, testOptions(fake.Landscape()))`, send keys with `send(...)`,
  wait for output with `waitFor`, and compare with `snapshot(t, tm)` against
  `internal/ui/testdata/<TestName>.golden`.
- After an intended UI change, run `go test ./internal/ui/ -update`, then
  **review the golden diff** before committing — never update golden files to
  hide an unintended change.
- Code that calls gardenctl is tested against the fake `gardenctl` script in
  `internal/testutil` (it supports `FAKE_RESTRICTED` and `FAKE_TARGET_ERROR`);
  extend it instead of requiring a real gardenctl or cluster.
- Tests must not need network access, a Gardener cluster or a real gardenctl.

## Documentation

User-visible behavior (flags, keys, config options, themes) is documented in
`README.md`. Update it in the same change. Update `docs/demo.tape` only if the
demo flow changes (the GIF is regenerated with `make demo-gif`, which needs
`vhs`).

## Pull requests

Follow `.github/pull_request_template.md`: describe what and why, how it was
tested (unit/golden tests, `gtui --demo`, real landscape), and tick the
checklist (`make all` passes, tests updated, README updated, SPDX headers).
Releases are drafted by Release Drafter from PR labels (`enhancement`, `bug`,
`documentation`, `dependencies`, `skip-changelog`); don't edit version numbers
or tags manually.
