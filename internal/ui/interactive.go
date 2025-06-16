package ui

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"yapfs/pkg/utils"
)

// ConsoleUI implements simple console-based interactive UI
type ConsoleUI struct {}

// NewConsoleUI creates a new console-based interactive UI
func NewConsoleUI() *ConsoleUI {
	return &ConsoleUI{}
}

// ShowMessage displays a message to the user
func (c *ConsoleUI) ShowMessage(message string) {
	log.Printf("%s\n", message)
}

// InputCode prompts user to input an 8-character alphanumeric code with validation
func (c *ConsoleUI) InputCode() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("Enter code from sender: ")
		scanner.Scan()
		code := strings.TrimSpace(scanner.Text())

		if utils.IsValidCode(code) {
			return code, nil
		}

		fmt.Printf("Invalid code. Please enter again.\n")
	}
}
