package input

import (
	"bufio"
	"os"
)

// ConsoleInputProvider provides user input from console
type ConsoleInputProvider struct {
	scanner *bufio.Scanner
}

// NewConsoleInputProvider creates a new console input provider
func NewConsoleInputProvider() *ConsoleInputProvider {
	return &ConsoleInputProvider{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// GetInput reads a line of input from the console
func (c *ConsoleInputProvider) GetInput() (string, bool) {
	if !c.scanner.Scan() {
		return "", false
	}
	return c.scanner.Text(), true
}
