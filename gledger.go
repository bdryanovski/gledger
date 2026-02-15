package main

import "fmt"
import "os"

import "gledger/ui"
import tea "github.com/charmbracelet/bubbletea"

func main() {

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
