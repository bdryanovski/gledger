package cli

import (
	"fmt"
	"gledger/cli/commands"
)

func Run(args []string) error {
	if len(args) == 0 {
		return nil
	}

	command := args[0]
	commandArgs := args[1:]

	switch command {
	case "add":
		return commands.AddCommand(commandArgs)
	case "list", "ls":
		return commands.ListCommand(commandArgs)
	case "help", "-h", "--help":
		return runHelp(commandArgs)
	case "version", "-v", "--version":
		return runVersion(commandArgs)
	default:
		return fmt.Errorf("Unknown command: %s", command)
	}
}

func runHelp(args []string) error {
	help := `
	Usage: gledger [command] [options]
	`

	fmt.Println(help)

	return nil
}

func runVersion(args []string) error {
	version := "0.1.0"

	fmt.Printf("gledger version %s\n", version)

	return nil
}
