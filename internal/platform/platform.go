// Package platform contains small host OS integrations used by the TUI.
package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OpenURL opens url with the operating system's default browser. It waits for
// the opener (xdg-open, open, …), which normally exits once it has handed the
// URL to the browser, so callers should run it off the UI goroutine.
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
	if out, err := cmd.CombinedOutput(); err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, firstLine(msg))
		}
		return err
	}
	return nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
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
