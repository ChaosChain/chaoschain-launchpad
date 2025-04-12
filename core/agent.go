package core

// Agent represents a generic AI-powered entity in ChaosChain (Producer or Validator)
type Agent struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Role             string                 `json:"role"` // "producer" or "validator"
	ValidatorAddress string                 `json:"validator_address,omitempty"`
	IsValidator      bool                   `json:"is_validator"`
	Metadata         map[string]interface{} `json:"metadata"` // Flexible metadata for external agents
}
