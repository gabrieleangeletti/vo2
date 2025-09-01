package internal

import (
	"fmt"
	"os/exec"
	"runtime"
)

const (
	fourYearsInMonths = 48
)

func OpenURLInBrowser(url string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "start"
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, url).Start()
}
