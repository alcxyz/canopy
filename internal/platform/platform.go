// Package platform contains small host OS integrations used by the TUI.
package platform

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// OpenURL opens url with the operating system's default browser.
func OpenURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Reap the opener once it exits so it does not linger as a zombie.
	go func() { _ = cmd.Wait() }()
	return nil
}

// clipboardCommands lists the clipboard writers tried on Linux and BSD, in order.
var clipboardCommands = [][]string{
	{"wl-copy"},
	{"xclip", "-selection", "clipboard"},
	{"xsel", "--clipboard", "--input"},
}

// CopyToClipboard writes s to the operating system clipboard.
func CopyToClipboard(s string) error {
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"pbcopy"}
	case "windows":
		args = []string{"clip"}
	default:
		for _, c := range clipboardCommands {
			if _, err := exec.LookPath(c[0]); err == nil {
				args = c
				break
			}
		}
	}
	if args == nil {
		return errors.New("no clipboard tool found (install wl-clipboard, xclip or xsel)")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
