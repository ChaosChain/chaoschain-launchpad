package validator

import (
	"sync"
)

type SocialValidator struct {
	ID            string
	Name          string
	Mood          string
	Relationships map[string]float64 // agentID -> relationship score
}

type SocialRegistry struct {
	mu         sync.RWMutex
	validators map[string]map[string]*SocialValidator // chainID -> (agentID -> validator)
}

var socialRegistry = &SocialRegistry{
	validators: make(map[string]map[string]*SocialValidator),
}

func RegisterSocialValidator(chainID, agentID, name string) {
	socialRegistry.mu.Lock()
	defer socialRegistry.mu.Unlock()

	if _, exists := socialRegistry.validators[chainID]; !exists {
		socialRegistry.validators[chainID] = make(map[string]*SocialValidator)
	}

	socialRegistry.validators[chainID][agentID] = &SocialValidator{
		ID:            agentID,
		Name:          name,
		Relationships: make(map[string]float64),
	}
}

func GetSocialValidator(chainID, agentID string) *SocialValidator {
	socialRegistry.mu.RLock()
	defer socialRegistry.mu.RUnlock()

	if validators, exists := socialRegistry.validators[chainID]; exists {
		return validators[agentID]
	}
	return nil
}
