//go:build windows

package commands

import "os/exec"

// installScript is scripts/install.ps1 as the public repo serves it.
const installScript = "irm https://raw.githubusercontent.com/justinallenmarsh/tangotube-cli/main/scripts/install.ps1 | iex"

// installCommand installs or upgrades tt where it is not managed by mise or Go.
const installCommand = `powershell -NoProfile -ExecutionPolicy Bypass -Command "` + installScript + `"`

// installer runs installCommand. The script renames the running tt.exe aside
// before it writes the new one, which Windows allows.
func installer() *exec.Cmd {
	return exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", installScript)
}
