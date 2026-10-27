package main

import (
	"os/exec"
	"runtime"
)

// openPath opens a file with the OS default handler: the config file lands
// in the user's editor, which is the interim Settings UI until the Wails
// hotkey tab (phase 3) ships.
func openPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
