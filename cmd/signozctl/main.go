package main

import (
	"os"
	"os/exec"
)

func main() {
	args := append([]string{"run", "./cmd/signozctl"}, os.Args[1:]...)
	cmd := exec.Command("go", args...)
	cmd.Dir = "tools/signozctl"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
