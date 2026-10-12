package launcher

import "os"

func writeExec(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o700) // #nosec G306 -- test script
}
