package git

import (
	"os/exec"
	"strings"
)

func Remotes() ([]string, error) {
	remoteCmd := exec.Command("git", "remote", "-v")
	remoteCmd.Stderr = nil
	output, err := remoteCmd.Output()
	return outputLines(output), err
}

func outputLines(output []byte) []string {
	lines := strings.TrimSuffix(string(output), "\n")
	if lines == "" {
		return []string{}
	}
	return strings.Split(lines, "\n")
}