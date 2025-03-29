package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

var (
	listChainID string
	listAPIURL  string
)

// ListCmd represents the list command
var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents",
	Long:  `List all agents in a chain.`,
	Run: func(cmd *cobra.Command, args []string) {
		if listChainID == "" {
			fmt.Println("Error: chain ID is required")
			os.Exit(1)
		}
		
		if listAPIURL == "" {
			listAPIURL = "http://localhost:3000"
		}
		
		listAgents()
	},
}

func init() {
	ListCmd.Flags().StringVar(&listChainID, "chain", "", "Chain ID")
	ListCmd.Flags().StringVar(&listAPIURL, "api-url", "", "API URL (default: http://localhost:3000)")
	
	ListCmd.MarkFlagRequired("chain")
}

// listAgents lists all agents in a chain
func listAgents() {
	// Send request
	req, err := http.NewRequest("GET", listAPIURL+"/api/validators", nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}
	
	req.Header.Set("X-Chain-ID", listChainID)
	
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
		fmt.Printf("Error listing agents: %s\n", body)
		os.Exit(1)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing response: %v\n", err)
		os.Exit(1)
	}
	
	validators, ok := response["validators"].([]interface{})
	if !ok {
		fmt.Println("No agents found.")
		return
	}
	
	fmt.Printf("Found %d agents in chain '%s':\n", len(validators), listChainID)
	for _, v := range validators {
		validator, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		
		fmt.Printf("- %s (ID: %s)\n", validator["Name"], validator["ID"])
		
		if traits, ok := validator["Traits"].([]interface{}); ok {
			fmt.Printf("  Traits: ")
			for i, t := range traits {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(t)
			}
			fmt.Println()
		}
		
		if style, ok := validator["Style"].(string); ok {
			fmt.Printf("  Style: %s\n", style)
		}
		
		if mood, ok := validator["Mood"].(string); ok {
			fmt.Printf("  Mood: %s\n", mood)
		}
		
		fmt.Println()
	}
} 