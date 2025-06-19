package utils

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

func AskForCode(ctx context.Context) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Create a channel to receive the input
		inputCh := make(chan string, 1)
		defer close(inputCh)

		fmt.Printf("Enter code from sender: ")
		go func() {
			if scanner.Scan() {
				inputCh <- strings.TrimSpace(scanner.Text())
			}
		}()

		// Wait for either input or context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case code := <-inputCh:
			if IsValidCode(code) {
				return code, nil
			}
			fmt.Printf("Invalid code. Please enter again.\n")
		}
	}
}
