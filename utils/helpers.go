package utils

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
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

func ensureDiscussionsDir() error {
	// Create discussions directory if it doesn't exist
	if err := os.MkdirAll("data/discussions", 0755); err != nil {
		return fmt.Errorf("failed to create discussions directory: %v", err)
	}
	return nil
}

func GetCurrentRound(chainID string) int {
	// Ensure directory exists first
	if err := ensureDiscussionsDir(); err != nil {
		log.Printf("Warning: %v", err)
		return 1
	}

	roundFile := fmt.Sprintf("data/discussions/%s_round.txt", chainID)
	data, err := os.ReadFile(roundFile)
	if err != nil {
		// Create file with initial round 1
		if err := os.WriteFile(roundFile, []byte("1"), 0644); err != nil {
			log.Printf("Warning: Failed to create round file: %v", err)
		}
		return 1
	}
	round, _ := strconv.Atoi(string(data))
	return round
}

func IncrementRound(chainID string) {
	if err := ensureDiscussionsDir(); err != nil {
		log.Printf("Warning: %v", err)
		return
	}

	current := GetCurrentRound(chainID)
	roundFile := fmt.Sprintf("data/discussions/%s_round.txt", chainID)
	if err := os.WriteFile(roundFile, []byte(fmt.Sprintf("%d", current+1)), 0644); err != nil {
		log.Printf("Warning: Failed to increment round: %v", err)
	}
}

func GetDiscussionLog(chainID string) string {
	if err := ensureDiscussionsDir(); err != nil {
		log.Printf("Warning: %v", err)
		return ""
	}

	logFile := fmt.Sprintf("data/discussions/%s.txt", chainID)
	data, err := os.ReadFile(logFile)
	if err != nil {
		// Create empty file if it doesn't exist
		if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
			log.Printf("Warning: Failed to create discussion log: %v", err)
		}
		return ""
	}
	return string(data)
}

func AppendDiscussionLog(chainID, message string) {
	if err := ensureDiscussionsDir(); err != nil {
		log.Printf("Warning: %v", err)
		return
	}

	logFile := fmt.Sprintf("data/discussions/%s.txt", chainID)
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Warning: Failed to open discussion log: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(message + "\n"); err != nil {
		log.Printf("Warning: Failed to append to discussion log: %v", err)
	}
}
