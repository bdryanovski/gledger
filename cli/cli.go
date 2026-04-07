package cli

import (
	"doublebook/cli/commands"
	"fmt"
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
	Usage: doublebook [command] [options]
	`

	fmt.Println(help)

	return nil
}

func runVersion(args []string) error {
	version := "0.1.0"

	fmt.Printf("doublebook version %s\n", version)

	return nil
}
