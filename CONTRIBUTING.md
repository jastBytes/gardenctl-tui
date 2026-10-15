# Contributing

Thanks for your interest in gtui! Bug reports, ideas and pull requests are
welcome.

## Reporting bugs and asking for features

Open an issue and include the gtui version (`gtui --version`), the gardenctl
version, your OS and terminal, and the steps to reproduce. Please remove
garden, project and shoot names or kubeconfig details you do not want to make
public. Security issues go through [SECURITY.md](SECURITY.md) instead.

## Pull requests

1. Discuss larger changes in an issue first.
2. Keep changes focused; add or update tests (`go test ./internal/ui/ -update`
   refreshes the golden files) and the README where behavior changes.
3. Run `make all` (build, vet, test, lint) before pushing; CI runs the same
   checks plus `go mod tidy` and govulncheck.
4. New Go files start with the license header:

   ```go
   // SPDX-FileCopyrightText: 2026 Jan Steffen
   // SPDX-License-Identifier: Apache-2.0
   ```

`gtui --demo` starts the UI with a fake landscape, so most UI work does not
need a Gardener cluster.

## License

By contributing you agree that your contributions are licensed under the
[Apache License 2.0](LICENSE), as stated in section 5 of the license.
