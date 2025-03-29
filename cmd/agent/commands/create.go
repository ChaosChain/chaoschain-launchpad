package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/NethermindEth/chaoschain-launchpad/cmd/agent/templates"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/spf13/cobra"
)

var (
	createChainID      string
	createTemplateName string
	createAgentName    string
	createTraits       string
	createStyle        string
	createRole         string
	createAPIURL       string
)

// CreateCmd represents the create command
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent",
	Long:  `Create a new agent from a template or with custom parameters.`,
	Run: func(cmd *cobra.Command, args []string) {
		if createChainID == "" {
			fmt.Println("Error: chain ID is required")
			os.Exit(1)
		}
		
		if createAPIURL == "" {
			createAPIURL = "http://localhost:3000"
		}
		
		if createTemplateName != "" {
			createAgentFromTemplate()
		} else {
			createCustomAgent()
		}
	},
}

func init() {
	CreateCmd.Flags().StringVar(&createChainID, "chain", "", "Chain ID to register the agent with")
	CreateCmd.Flags().StringVar(&createTemplateName, "template", "", "Template name to use")
	CreateCmd.Flags().StringVar(&createAgentName, "name", "", "Custom name for the agent")
	CreateCmd.Flags().StringVar(&createTraits, "traits", "", "Comma-separated list of traits")
	CreateCmd.Flags().StringVar(&createStyle, "style", "", "Agent style")
	CreateCmd.Flags().StringVar(&createRole, "role", "validator", "Agent role (validator or producer)")
	CreateCmd.Flags().StringVar(&createAPIURL, "api-url", "", "API URL (default: http://localhost:3000)")
	
	CreateCmd.MarkFlagRequired("chain")
}

// createAgentFromTemplate creates a new agent from a template
func createAgentFromTemplate() {
	// Get template
	registry := templates.NewTemplateRegistry()
	template, err := registry.GetTemplate(createTemplateName)
	if err != nil {
		fmt.Printf("Error loading template: %v\n", err)
		os.Exit(1)
	}
	
	// Override template values if provided
	if createAgentName != "" {
		template.Name = createAgentName
	}
	
	if createTraits != "" {
		template.Traits = strings.Split(createTraits, ",")
	}
	
	if createStyle != "" {
		template.Style = createStyle
	}
	
	if createRole != "" {
		template.Role = createRole
	}
	
	// Convert template to core.Agent struct
	agent := template.ToAgentStruct()
	
	// Create agent using API
	createAgent(agent)
}

// createCustomAgent creates a new agent with custom parameters
func createCustomAgent() {
	if createAgentName == "" {
		fmt.Println("Error: agent name is required")
		os.Exit(1)
	}
	
	if createTraits == "" {
		fmt.Println("Error: traits are required")
		os.Exit(1)
	}
	
	if createStyle == "" {
		fmt.Println("Error: style is required")
		os.Exit(1)
	}
	
	// Create agent struct
	agent := core.Agent{
		Name:       createAgentName,
		Role:       createRole,
		Traits:     strings.Split(createTraits, ","),
		Style:      createStyle,
		Influences: []string{},
		Mood:       "neutral", // Default mood
	}
	
	createAgent(agent)
}

// createAgent sends the API request to create an agent
func createAgent(agent core.Agent) {
	requestJSON, err := json.Marshal(agent)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}
	
	// Send request
	req, err := http.NewRequest("POST", createAPIURL+"/api/register", bytes.NewBuffer(requestJSON))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Chain-ID", createChainID)
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		os.Exit(1)
	}
	
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Error creating agent: %s\n", body)
		os.Exit(1)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Agent created successfully!\n")
	fmt.Printf("Agent ID: %s\n", response["agentID"])
	fmt.Printf("P2P Port: %v\n", response["p2pPort"])
	fmt.Printf("API Port: %v\n", response["apiPort"])
} 