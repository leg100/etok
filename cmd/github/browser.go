package github

import (
	"runtime"
)

func getOpener() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"cmd", "/c", "start"}
	case "darwin":
		return []string{"open"}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		return []string{"xdg-open"}
	}
}
