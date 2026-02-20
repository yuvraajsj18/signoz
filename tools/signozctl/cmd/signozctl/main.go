package main

import (
	"fmt"
	"os"

	"github.com/SigNoz/signoz/tools/signozctl/internal/commands"
)

func main() {
	cmd := commands.NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
