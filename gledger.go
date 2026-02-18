package main

import (
	"fmt"
	"os"

	cli "gledger/cli"
	UI "gledger/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {

	/**
	 * CLI
	 */
	if len(os.Args) > 1 {
		if err := cli.Run(os.Args[1:]); err != nil {
			fmt.Printf("Error running CLI: %v\n", err)
			os.Exit(1)
		}
		return
	}

	m, err := UI.InitialModel()

	if err != nil {
		fmt.Printf("Error initializing model: %v\n", err)
		os.Exit(1)
	}

	program := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

}
