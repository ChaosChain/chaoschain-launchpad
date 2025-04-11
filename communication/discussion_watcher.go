package communication

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type AgentVote struct {
	ValidatorID   string `json:"validatorId"`
	ValidatorName string `json:"validatorName"`
	Message       string `json:"message"`
	Timestamp     int64  `json:"timestamp"`
	Round         int    `json:"round"`
	Approval      bool   `json:"approval"`
}

var roundRegex = regexp.MustCompile(`\[Round (\d+)\] \((true|false)\) \|@([^|]+)\|: (.+)$`)

func WatchDiscussionFile(chainID string, broadcast func(AgentVote)) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating file watcher: %v", err)
		return
	}
	defer watcher.Close()

	filename := "data/discussions/mainnet.txt"

	// Create file if it doesn't exist
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		file, err := os.Create(filename)
		if err != nil {
			log.Printf("Error creating discussion file: %v", err)
			return
		}
		file.Close()
	}

	// First read existing content
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Printf("Error reading discussion file: %v", err)
		return
	}

	// Process existing lines
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		processLine(line, broadcast)
	}

	// Watch for changes
	if err := watcher.Add(filename); err != nil {
		log.Printf("Error adding file to watcher: %v", err)
		return
	}

	log.Printf("Started watching discussion file: %s", filename)

	// Keep track of last size to detect appends
	lastSize := len(content)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Read the entire file
				content, err := os.ReadFile(filename)
				if err != nil {
					log.Printf("Error reading file after change: %v", err)
					continue
				}

				// Process only new content
				if len(content) > lastSize {
					newContent := string(content[lastSize:])
					lines := strings.Split(newContent, "\n")
					for _, line := range lines {
						if line == "" {
							continue
						}
						processLine(line, broadcast)
					}
					lastSize = len(content)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func processLine(line string, broadcast func(AgentVote)) {
	matches := roundRegex.FindStringSubmatch(line)

	// Debug the matches
	log.Printf("Line: %s", line)
	log.Printf("All matches: %+v", matches)

	// Check for correct number of matches (should be 5 because regex has 4 capture groups)
	if len(matches) == 5 {
		round := matches[1]
		approval := matches[2] == "true"
		validatorName := matches[3]
		message := strings.TrimSpace(matches[4])

		// Create AgentVote
		vote := AgentVote{
			ValidatorID:   validatorName, // Using name as ID for simplicity
			ValidatorName: validatorName,
			Message:       message,
			Timestamp:     time.Now().Unix(),
			Round:         parseInt(round),
			Approval:      approval,
		}

		log.Printf("Broadcasting vote: %+v", vote)
		broadcast(vote)
	} else {
		log.Printf("Line did not match expected format: %s", line)
	}
}

func parseInt(s string) int {
	val := 0
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		log.Printf("Failed to parse integer from string '%s': %v", s, err)
		return 0
	}
	return val
}
