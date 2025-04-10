package core

// Agent represents a generic AI-powered entity in ChaosChain (Producer or Validator)
type Agent struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Role             string   `json:"role"` // "producer" or "validator"
	Traits           []string `json:"traits"`
	Style            string   `json:"style"`
	Influences       []string `json:"influences"`
	Mood             string   `json:"mood"`
	APIKey           string   `json:"api_key"`
	Endpoint         string   `json:"endpoint"`
	ValidatorAddress string   `json:"validator_address,omitempty"`
	IsValidator      bool     `json:"is_validator"`
}
