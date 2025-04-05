package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NethermindEth/chaoschain-launchpad/core"
)

func GenerateAgents(topic string) (string, error) {
	// Load example files
	physicsData, err := os.ReadFile("examples/physics.json")
	if err != nil {
		return "", fmt.Errorf("failed to read physics.json: %v", err)
	}

	biologyData, err := os.ReadFile("examples/biology.json")
	if err != nil {
		return "", fmt.Errorf("failed to read biology.json: %v", err)
	}

	prompt := fmt.Sprintf(`Create 10 unique AI agents as blockchain validators for a %s-focused discussion chain.
	Each agent should have:
	1. A unique name (preferably of a famous scientist/thinker in this field)
	2. 3-5 personality traits that influence their decision making
	3. The traits should create diverse perspectives and interesting discussions

	Return a JSON array where each agent has:
	- "ID": a UUID v4 string
	- "Name": their full name
	- "Traits": array of personality traits
	- "Role": must be "validator"
	- "Style": communication style description
	- "Influences": array of field-specific influences
	- "Mood": mood description

	Here are two example files showing the expected format:

	Physics example:
	%s

	Biology example:
	%s

	Follow these examples to create 10 agents for the %s field.
	Format the response as valid JSON only, no additional text.`, topic, string(physicsData), string(biologyData), topic)

	response := GenerateLLMResponse(prompt)

	// Parse the response to validate JSON
	var agents []core.Agent

	if err := json.Unmarshal([]byte(response), &agents); err != nil {
		return "", fmt.Errorf("invalid JSON response: %v", err)
	}

	// Create examples directory if it doesn't exist
	if err := os.MkdirAll("examples", 0755); err != nil {
		return "", fmt.Errorf("failed to create examples directory: %v", err)
	}

	// Replace spaces with underscores in the topic name
	sanitizedTopic := strings.ReplaceAll(topic, " ", "_")
	filename := fmt.Sprintf("examples/%s.json", sanitizedTopic)
	if err := os.WriteFile(filename, []byte(response), 0644); err != nil {
		return "", fmt.Errorf("failed to write agents file: %v", err)
	}

	return filepath.Base(filename), nil
}
