package utils

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// fileExists checks if a file exists
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// FindAvailableAPIPort finds an available port for the API server
func FindAvailableAPIPort() int {
	port := 8080
	for {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			listener.Close()
			return port
		}
		port++
	}
}

func LogDiscussion(agentName, message, chainID string, isProposer bool) {
	role := "Validator"
	if isProposer {
		role = "Proposer"
	}

	logEntry := fmt.Sprintf("[%s] %s (%s): %s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		agentName,
		role,
		message)

	// Ensure directory exists
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
		return
	}

	// Append to discussions log file
	filename := fmt.Sprintf("logs/discussions_%s.log", chainID)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(logEntry); err != nil {
		log.Printf("Failed to write to log file: %v", err)
	}
}
