// SPDX-FileCopyrightText: 2026 Jan Steffen
// SPDX-License-Identifier: Apache-2.0

package launcher

import "os"

func writeExec(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o700) // #nosec G306 -- test script
}
