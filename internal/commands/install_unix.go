//go:build !windows

package commands

import "os/exec"

// installCommand installs or upgrades tt where it is not managed by mise or Go.
const installCommand = "curl -fsSL https://tangotube.tv/install-cli | bash"

// installer runs installCommand.
func installer() *exec.Cmd { return exec.Command("bash", "-c", installCommand) }
