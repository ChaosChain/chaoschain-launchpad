package registry

import (
	"sync"

	"github.com/NethermindEth/chaoschain-launchpad/core"
)

var (
	// Map of chainID -> Map of agentID -> Agent
	agentRegistry = make(map[string]map[string]core.Agent)
	agentMutex    sync.RWMutex
)

// RegisterAgent stores an agent in the registry
func RegisterAgent(chainID string, agent core.Agent) {
	agentMutex.Lock()
	defer agentMutex.Unlock()

	// Initialize map if it doesn't exist
	if _, exists := agentRegistry[chainID]; !exists {
		agentRegistry[chainID] = make(map[string]core.Agent)
	}

	// Store agent by ID
	agentRegistry[chainID][agent.ID] = agent
}

// LinkAgentToValidator updates agent info with validator address
func LinkAgentToValidator(chainID string, agentID string, validatorAddr string) bool {
	agentMutex.Lock()
	defer agentMutex.Unlock()

	// Check if agent exists
	agent, exists := agentRegistry[chainID][agentID]
	if !exists {
		return false
	}

	// Update agent with validator info
	agent.ValidatorAddress = validatorAddr
	agent.IsValidator = true
	agentRegistry[chainID][agentID] = agent

	return true
}

// GetAgentByValidator returns agent info for a validator address
func GetAgentByValidator(chainID string, validatorAddr string) (core.Agent, bool) {
	agentMutex.RLock()
	defer agentMutex.RUnlock()

	// Search for agent with matching validator address
	for _, agent := range agentRegistry[chainID] {
		if agent.ValidatorAddress == validatorAddr {
			return agent, true
		}
	}
	return core.Agent{}, false
}

// GetAllAgents returns all agents for a given chain
func GetAllAgents(chainID string) []core.Agent {
	agentMutex.RLock()
	defer agentMutex.RUnlock()

	agents := make([]core.Agent, 0)
	if chainAgents, exists := agentRegistry[chainID]; exists {
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

	if chainAgents, exists := agentRegistry[chainID]; exists {
		for valAddr, agent := range chainAgents {
			result[valAddr] = agent.ID
		}
	}

	return result
}
