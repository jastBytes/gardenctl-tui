// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package state

import "os"

func writeFile(p, content string) error { return os.WriteFile(p, []byte(content), 0o600) }
