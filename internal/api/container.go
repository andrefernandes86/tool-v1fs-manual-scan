package api

import (
	"os"
	"runtime"
)

// runningInContainer is best-effort (Docker Linux). Used only for user-facing hints.
func runningInContainer() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := os.Stat("/.dockerenv")
	return err == nil
}
