package main

import (
	"fmt"
	"os"
	"strconv"
)

// ADDR remains available for existing development scripts. Containers use PORT.
func listenAddress() (string, error) {
	if address := os.Getenv("ADDR"); address != "" {
		return address, nil
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("PORT must be an integer between 1 and 65535")
	}
	return fmt.Sprintf("0.0.0.0:%d", n), nil
}
