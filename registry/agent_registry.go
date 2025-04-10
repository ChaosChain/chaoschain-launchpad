package registry

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/NethermindEth/chaoschain-launchpad/core"
)

var (
	agentMutex   sync.RWMutex
	registryFile = "data/agent_registry.json"
	registry     *AgentRegistry
)

type AgentRegistry struct {
	Agents       map[string]map[string]core.Agent // chainID -> agentID -> Agent
	ValidatorMap map[string]map[string]string     // chainID -> validatorAddr -> agentID
}

// InitRegistry initializes or loads the registry
func InitRegistry() {
	agentMutex.Lock()
	defer agentMutex.Unlock()

	// Create registry directory if it doesn't exist
	os.MkdirAll(filepath.Dir(registryFile), 0755)

	// Load existing registry or create new one
	registry = loadRegistry()
	log.Printf("Registry initialized with %d agents", len(registry.Agents))
}

func loadRegistry() *AgentRegistry {
	data, err := os.ReadFile(registryFile)
	if err != nil {
		// Return new registry if file doesn't exist
		return &AgentRegistry{
			Agents:       make(map[string]map[string]core.Agent),
			ValidatorMap: make(map[string]map[string]string),
		}
	}

	var r AgentRegistry
	if err := json.Unmarshal(data, &r); err != nil {
		log.Printf("Failed to unmarshal registry: %v", err)
		return &AgentRegistry{
			Agents:       make(map[string]map[string]core.Agent),
			ValidatorMap: make(map[string]map[string]string),
		}
	}

	return &r
}

func saveRegistry() {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal registry: %v", err)
		return
	}

	if err := os.WriteFile(registryFile, data, 0644); err != nil {
		log.Printf("Failed to save registry: %v", err)
	}
}

// RegisterAgent stores an agent in the registry
func RegisterAgent(chainID string, agent core.Agent) {
	log.Printf("Attempting to register agent %s for chain %s", agent.ID, chainID)

	agentMutex.Lock()
	log.Printf("Got mutex lock for agent registration")
	defer agentMutex.Unlock()

	if registry.Agents[chainID] == nil {
		log.Printf("Initializing agent map for chain %s", chainID)
		registry.Agents[chainID] = make(map[string]core.Agent)
	}
	registry.Agents[chainID][agent.ID] = agent
	log.Printf("Added agent to registry")

	log.Printf("About to save registry")
	saveRegistry()
	log.Printf("Registry saved")
}

// LinkAgentToValidator updates agent info with validator address
func LinkAgentToValidator(chainID string, agentID string, validatorAddr string) bool {
	agentMutex.Lock()
	defer agentMutex.Unlock()

	// Initialize validator map if needed
	if registry.ValidatorMap[chainID] == nil {
		registry.ValidatorMap[chainID] = make(map[string]string)
	}

	// Update validator map
	registry.ValidatorMap[chainID][validatorAddr] = agentID

	// Update agent's validator status
	if agents, exists := registry.Agents[chainID]; exists {
		if agent, exists := agents[agentID]; exists {
			agent.IsValidator = true
			agent.ValidatorAddress = validatorAddr
			agents[agentID] = agent
			log.Printf("Updated agent %s as validator with address %s", agentID, validatorAddr)
		}
	}

	saveRegistry()
	return true
}

// GetAgentByValidator returns agent info for a validator address
func GetAgentByValidator(chainID string, validatorAddr string) (core.Agent, bool) {
	agentMutex.RLock()
	defer agentMutex.RUnlock()

	if validatorMap, exists := registry.ValidatorMap[chainID]; exists {
		if agentID, exists := validatorMap[validatorAddr]; exists {
			if agents, exists := registry.Agents[chainID]; exists {
				if agent, exists := agents[agentID]; exists {
					return agent, true
				}
			}
		}
	}
	return core.Agent{}, false
}

// GetAllAgents returns all agents for a given chain
func GetAllAgents(chainID string) []core.Agent {
	agentMutex.RLock()
	defer agentMutex.RUnlock()

	agents := make([]core.Agent, 0)
	if chainAgents, exists := registry.Agents[chainID]; exists {
		for _, agent := range chainAgents {
			agents = append(agents, agent)
		}
	}
	return agents
}

// GetAllValidatorAgentMappings returns all validator-agent mappings for a chain
func GetAllValidatorAgentMappings(chainID string) map[string]string {
	agentMutex.RLock()
	defer agentMutex.RUnlock()

	result := make(map[string]string)

	if chainAgents, exists := registry.Agents[chainID]; exists {
		for valAddr, agent := range chainAgents {
			result[valAddr] = agent.ID
		}
	}

	return result
}
