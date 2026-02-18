package commands

import "fmt"

func ListCommand(args []string) error {
	fmt.Println("Running list command with args:", args)
	return nil
}
