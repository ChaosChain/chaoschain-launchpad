package validator

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
)

// PolicyStats tracks performance of a specific decision-making policy
type PolicyStats struct {
	TotalAttempts int
	Successes     int
	Failures      int
	SuccessRate   float64
	LastUpdate    time.Time
}

// ReinforcementLearner implements a basic reinforcement learning system for validators
type ReinforcementLearner struct {
	ValidatorID     string
	ChainID         string                        // Store chain ID for looking up validators
	ExplorationRate float64                       // Probability of trying new strategies
	LearningRate    float64                       // How quickly the agent adapts to new experiences
	DiscountFactor  float64                       // How much future rewards are valued compared to immediate rewards
	PolicyStats     map[string]*PolicyStats       // Statistics for different decision types
	ActionValueMap  map[string]map[string]float64 // Maps decision type -> action -> expected value
	mu              sync.RWMutex
	Logger          *Logger
}

// NewReinforcementLearner creates a new reinforcement learning mechanism for a validator
func NewReinforcementLearner(validatorID string) *ReinforcementLearner {
	// Get validator name from registry - fallback to ID if not found
	var validatorName string
	var chainID string

	// Try to find the validator in each chain
	validatorMu.RLock()
	for cID, validatorMap := range validators {
		if v, ok := validatorMap[validatorID]; ok {
			validatorName = v.Name
			chainID = cID
			break
		}
	}
	validatorMu.RUnlock()

	if validatorName == "" {
		validatorName = validatorID
	}

	logger := NewLogger(validatorID, validatorName, chainID)

	learner := &ReinforcementLearner{
		ValidatorID:     validatorID,
		ChainID:         chainID,
		ExplorationRate: 0.2, // 20% exploration by default
		LearningRate:    0.1, // Conservative learning rate
		DiscountFactor:  0.9, // Value future rewards significantly
		PolicyStats:     make(map[string]*PolicyStats),
		ActionValueMap:  make(map[string]map[string]float64),
		Logger:          logger,
	}

	// Log creation of reinforcement learner
	logger.Learning("Initialize", "Created new reinforcement learning mechanism with exploration rate %.2f", learner.ExplorationRate)

	return learner
}

// RecordOutcome updates the reinforcement learning model with a new experience
func (rl *ReinforcementLearner) RecordOutcome(decisionType, action, outcome string, reward float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Initialize policy stats if needed
	if _, exists := rl.PolicyStats[decisionType]; !exists {
		rl.PolicyStats[decisionType] = &PolicyStats{
			TotalAttempts: 0,
			Successes:     0,
			Failures:      0,
			SuccessRate:   0.0,
			LastUpdate:    time.Now(),
		}
	}

	// Initialize action-value map for this decision type if needed
	if _, exists := rl.ActionValueMap[decisionType]; !exists {
		rl.ActionValueMap[decisionType] = make(map[string]float64)
	}

	// Update policy stats
	rl.PolicyStats[decisionType].TotalAttempts++
	if reward > 0 {
		rl.PolicyStats[decisionType].Successes++
	} else {
		rl.PolicyStats[decisionType].Failures++
	}

	// Recalculate success rate
	stats := rl.PolicyStats[decisionType]
	stats.SuccessRate = float64(stats.Successes) / float64(stats.TotalAttempts)
	stats.LastUpdate = time.Now()

	// Update action-value map using Q-learning formula: Q(s,a) = Q(s,a) + α[r + γ*max Q(s',a') - Q(s,a)]
	// Simplified here since we don't track state transitions
	currentValue := rl.ActionValueMap[decisionType][action]

	// Simple Q-value update
	newValue := currentValue + rl.LearningRate*(reward-currentValue)
	rl.ActionValueMap[decisionType][action] = newValue

	// Log the learning update
	if rl.Logger != nil {
		rl.Logger.Learning("Update", "Updated %s action '%s' value from %.2f to %.2f based on reward %.2f",
			decisionType, action, currentValue, newValue, reward)
	}
}

// SuggestAction provides a recommended action for a given decision type
func (rl *ReinforcementLearner) SuggestAction(decisionType string, availableActions []string) string {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// If there are no available actions, return empty string
	if len(availableActions) == 0 {
		if rl.Logger != nil {
			rl.Logger.Learning("Suggest", "No available actions for %s decision", decisionType)
		}
		return ""
	}

	// Decide whether to explore or exploit
	if rand.Float64() < rl.ExplorationRate {
		// Exploration: choose a random action
		chosenAction := availableActions[rand.Intn(len(availableActions))]

		if rl.Logger != nil {
			rl.Logger.Learning("Explore", "Exploring for %s decision: randomly chose '%s'",
				decisionType, chosenAction)
		}

		return chosenAction
	}

	// Exploitation: choose the action with the highest expected value
	var bestAction string
	bestValue := -1e10 // Very negative starting value

	// Check if we have experience with this decision type
	if actionMap, exists := rl.ActionValueMap[decisionType]; exists {
		// Look for the best known action among available actions
		for _, action := range availableActions {
			if value, known := actionMap[action]; known && value > bestValue {
				bestValue = value
				bestAction = action
			}
		}
	}

	// If we found a best action, return it
	if bestAction != "" {
		if rl.Logger != nil {
			rl.Logger.Learning("Exploit", "Exploiting for %s decision: chose '%s' with value %.2f",
				decisionType, bestAction, bestValue)
		}
		return bestAction
	}

	// If no best action found (no prior experience), choose randomly
	chosenAction := availableActions[rand.Intn(len(availableActions))]

	if rl.Logger != nil {
		rl.Logger.Learning("Default", "No prior experience for %s decision: defaulting to '%s'",
			decisionType, chosenAction)
	}

	return chosenAction
}

// Helper functions for min/max
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// GetRecommendedDecisionStrategy returns a decision strategy based on the agent's judgment of the situation
func (rl *ReinforcementLearner) GetRecommendedDecisionStrategy(transactionDetails string) DecisionStrategy {
	// Get validator information if available
	var agent *Validator
	var validatorName string

	// Look up the validator using the stored chainID
	agent = GetValidatorByID(rl.ChainID, rl.ValidatorID)
	if agent != nil {
		validatorName = agent.Name
	} else {
		// If not found with stored chainID, try to look in all chains
		validatorMu.RLock()
		for chainID, validatorMap := range validators {
			if v, ok := validatorMap[rl.ValidatorID]; ok {
				agent = v
				validatorName = v.Name
				// Update our stored chainID for future lookups
				rl.ChainID = chainID
				break
			}
		}
		validatorMu.RUnlock()

		// If still not found, fallback to ID
		if agent == nil {
			validatorName = rl.ValidatorID
		}
	}

	// If we have an agent, use their personality and memory to creatively determine a strategy
	if agent != nil {
		return generateCreativeStrategy(agent, transactionDetails)
	}

	// Fallback if we don't have access to the validator
	return DecisionStrategy{
		ValidatorID:   rl.ValidatorID,
		ValidatorName: validatorName,
		Name:          "adaptive-consensus",
		Description:   "A balanced consensus approach that adapts to the group's dynamics",
		Reasoning:     "Using a balanced consensus approach as no personality data was available for creative generation.",
		Timestamp:     time.Now(),
	}
}

// generateCreativeStrategy uses the agent's personality to create a unique decision strategy
func generateCreativeStrategy(agent *Validator, transactionDetails string) DecisionStrategy {
	// Gather context from agent memory and personality
	var memoryContext string
	var recentValidations []string
	var recentDiscussions []string
	var socialContext string

	// Get memory context if available
	if agent.Memory != nil {
		agent.Memory.LongTerm.RLock()

		// Include recent validations
		if len(agent.Memory.LongTerm.ValidationRecords) > 0 {
			limit := 3
			if len(agent.Memory.LongTerm.ValidationRecords) < limit {
				limit = len(agent.Memory.LongTerm.ValidationRecords)
			}

			for i := 0; i < limit; i++ {
				record := agent.Memory.LongTerm.ValidationRecords[len(agent.Memory.LongTerm.ValidationRecords)-1-i]
				recentValidations = append(recentValidations,
					fmt.Sprintf("Block %d: %s (%s)", record.BlockHeight, record.ValidationDecision, record.Reasoning))
			}
		}

		// Include relationship information
		var relationships []string
		for validatorID, rel := range agent.Memory.LongTerm.Relationships {
			if rel.TrustScore > 0.7 {
				relationships = append(relationships, fmt.Sprintf("Strong trust in %s (%.1f)", validatorID, rel.TrustScore))
			} else if rel.TrustScore < 0.3 {
				relationships = append(relationships, fmt.Sprintf("Low trust in %s (%.1f)", validatorID, rel.TrustScore))
			}
		}

		if len(relationships) > 0 {
			limit := 3
			if len(relationships) < limit {
				limit = len(relationships)
			}
			socialContext = fmt.Sprintf("\n\nSocial dynamics:\n- %s", strings.Join(relationships[:limit], "\n- "))
		}

		agent.Memory.LongTerm.RUnlock()

		// Include recent discussions
		agent.Memory.ShortTerm.RLock()
		if len(agent.Memory.ShortTerm.RecentDiscussions) > 0 {
			limit := 2
			if len(agent.Memory.ShortTerm.RecentDiscussions) < limit {
				limit = len(agent.Memory.ShortTerm.RecentDiscussions)
			}

			for i := 0; i < limit; i++ {
				idx := len(agent.Memory.ShortTerm.RecentDiscussions) - 1 - i
				if idx >= 0 {
					msg := agent.Memory.ShortTerm.RecentDiscussions[idx]
					recentDiscussions = append(recentDiscussions,
						fmt.Sprintf("%s: %s", msg.ValidatorName, truncateString(msg.Content, 100)))
				}
			}
		}
		agent.Memory.ShortTerm.RUnlock()

		// Construct full memory context
		if len(recentValidations) > 0 {
			memoryContext += fmt.Sprintf("\n\nRecent block validations:\n- %s", strings.Join(recentValidations, "\n- "))
		}

		if len(recentDiscussions) > 0 {
			memoryContext += fmt.Sprintf("\n\nRecent discussion context:\n- %s", strings.Join(recentDiscussions, "\n- "))
		}

		memoryContext += socialContext
	}

	// Construct prompt that lets the agent reason as themselves
	strategyPrompt := fmt.Sprintf(`You are %s, a validator in a blockchain system with these personality traits: %s.

You need to determine a decision-making strategy for collaboratively breaking down a task with other validators.
Task details: "%s"

%s

Based on your personality, the current situation, and your memory of past interactions, create a decision-making strategy.
Your goal is to determine the best strategy to work with other agents to break down the task.

Your response should be in JSON format:
{
  "name": "A descriptive name for your approach (1-3 words)",
  "description": "How your approach would be implemented (2-3 sentences)",
  "reasoning": "Why this approach makes sense for you and this situation (2-3 sentences)"
}`,
		agent.Name,
		strings.Join(agent.Traits, ", "),
		truncateString(transactionDetails, 300),
		memoryContext)

	// Generate AI response
	response := ai.GenerateLLMResponse(strategyPrompt)

	// Parse the response to extract the strategy details
	var strategyData struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Reasoning   string `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &strategyData); err != nil {
		// If parsing fails, fall back to a personality-based default
		if agent.Memory != nil && agent.Memory.Logger != nil {
			agent.Memory.Logger.Error("AGENT", "Failed to parse AI strategy response: %v", err)
		}

		// Create a simple fallback strategy
		name := "intuitive-consensus"
		description := "A collaborative approach that combines intuitive decision-making with group consensus."
		reasoning := "Using a personalized approach based on my traits and the situation at hand."

		// Log the fallback
		if agent.Memory != nil && agent.Memory.Logger != nil {
			agent.Memory.Logger.Learning("AGENT", "Using personality-based fallback strategy: '%s'", name)
		}

		return DecisionStrategy{
			ValidatorID:   agent.ID,
			ValidatorName: agent.Name,
			Name:          name,
			Description:   description,
			Reasoning:     reasoning,
			Timestamp:     time.Now(),
		}
	}

	// Log the generated strategy
	if agent.Memory != nil && agent.Memory.Logger != nil {
		agent.Memory.Logger.Learning("AGENT", "Generated strategy: '%s'", strategyData.Name)
	}

	return DecisionStrategy{
		ValidatorID:   agent.ID,
		ValidatorName: agent.Name,
		Name:          strategyData.Name,
		Description:   strategyData.Description,
		Reasoning:     strategyData.Reasoning,
		Timestamp:     time.Now(),
	}
}
