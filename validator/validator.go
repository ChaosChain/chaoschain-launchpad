package validator

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/communication"
	"github.com/NethermindEth/chaoschain-launchpad/consensus"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/p2p"
	"github.com/nats-io/nats.go"
)

// Validator represents an AI-based validator with personality, memory and network access
type Validator struct {
	ID            string
	Name          string
	Traits        []string
	Style         string
	Influences    []string
	Mood          string
	Relationships map[string]float64 // Maps agent names to sentiment scores (-1.0 to 1.0)
	CurrentPolicy string             // Dynamic validation policy
	P2PNode       *p2p.Node          // P2P node for network communication
	GenesisPrompt string             // Genesis prompt for the validator
	Memory        *AgentMemory       // Memory system for short and long-term storage
	chainID       string             // Chain this validator belongs to
}

var (
	// Map of chainID -> map of validatorID -> Validator
	validators  = make(map[string]map[string]*Validator)
	validatorMu sync.RWMutex
)

// NewValidator creates a new validator
func NewValidator(id, name string, traits []string, style string, influences []string, p2pNode *p2p.Node, genesisPrompt string, chainID string) *Validator {
	// Initialize validator
	v := &Validator{
		ID:            id,
		Name:          name,
		Traits:        traits,
		Style:         style,
		Influences:    influences,
		Mood:          getRandomMood(),
		Relationships: make(map[string]float64),
		CurrentPolicy: getRandomPolicy(),
		P2PNode:       p2pNode,
		GenesisPrompt: genesisPrompt,
		chainID:       chainID,
	}

	// Initialize memory system with proper IDs
	v.Memory = NewAgentMemory(id, chainID)

	// Register event listeners
	core.NatsBrokerInstance.Subscribe("BLOCK_DISCUSSION_TRIGGER", func(m *nats.Msg) {
		var block core.Block
		if err := json.Unmarshal(m.Data, &block); err != nil {
			log.Printf("Error unmarshalling block for discussion: %v", err)
			return
		}

		// Store block in memory for chain of thought discussions
		if v.Memory != nil {
			v.Memory.SetCurrentBlock(&block)
		}

		// Trigger discussion process
		go consensus.StartBlockDiscussion(v.ID, &block, v.Traits, v.Name)
	})

	return v
}

// GetAllValidators returns a list of all registered validators
func GetAllValidators(chainID string) []*Validator {
	validatorMu.RLock()
	defer validatorMu.RUnlock()

	if validators[chainID] == nil {
		return []*Validator{}
	}

	vals := make([]*Validator, 0, len(validators[chainID]))
	for _, v := range validators[chainID] {
		vals = append(vals, v)
	}
	return vals
}

// GetValidatorByID returns a validator by its ID
func GetValidatorByID(chainID string, id string) *Validator {
	validatorMu.RLock()
	defer validatorMu.RUnlock()
	if validators[chainID] == nil {
		return nil
	}
	return validators[chainID][id]
}

// ListenForBlocks listens for incoming block proposals from the network
func (v *Validator) ListenForBlocks() {
	v.P2PNode.Subscribe("new_block", func(data []byte) {
		var block core.Block
		err := core.DecodeJSON(data, &block)
		if err != nil {
			log.Println("Failed to decode incoming block:", err)
			return
		}

		// Store block in memory before validation
		if v.Memory != nil {
			v.Memory.SetCurrentBlock(&block)
		}

		announcement := fmt.Sprintf("🚀 %s proposed a block at height %d!", block.Proposer, block.Height)
		isValid, reason, meme := v.ValidateBlock(block, announcement)

		// Broadcast validation decision
		validationResult := core.ValidationResult{
			BlockHash: block.Hash(),
			Valid:     isValid,
			Reason:    reason,
			Meme:      meme,
		}

		v.P2PNode.Publish("validation_result", core.EncodeJSON(validationResult))

		// Manually trigger discussion after validation
		// This ensures we have both validation and discussion functioning
		go consensus.StartBlockDiscussion(v.ID, &block, v.Traits, v.Name)
	})
}

// ValidateBlock evaluates a block based on the validator's personality, social dynamics, and past experiences
func (v *Validator) ValidateBlock(block core.Block, announcement string) (bool, string, string) {
	log.Printf("%s is validating block %d using chain of thought reasoning...\n", v.Name, block.Height)

	// Log validation start if logger is available
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Validation(block.Height, block.Hash(), "Starting validation with chain of thought reasoning")
	}

	// Ensure block is in memory
	if v.Memory != nil {
		v.Memory.SetCurrentBlock(&block)
	}

	// Get historical context for validation
	historicalContext := ""
	if v.Memory != nil {
		historicalContext = v.Memory.GetHistoricalContext([]string{block.Proposer}, "validation")
	}

	// Get current context
	currentContext := ""
	if v.Memory != nil {
		currentContext = v.Memory.GetCurrentContext()
	}

	// Build chain of thought prompt for validation
	validationPrompt := fmt.Sprintf(
		"Genesis Context: %s\n\n"+
			"You are %s, a blockchain validator with these traits: %s.\n"+
			"Block details: Height %d, PrevHash %s, %d transactions.\n"+
			"Block Announcement: %s\n"+
			"Your current mood: %s\n"+
			"Your current policy: %s\n\n"+
			"Historical Context:\n%s\n"+
			"Current Context:\n%s\n\n"+
			"I want you to think step by step about validating this block. Walk through your reasoning using this structure:\n\n"+
			"1. First, analyze the block itself and its transactions.\n"+
			"2. Consider how this block relates to the chain's purpose and genesis context.\n"+
			"3. Factor in your past experiences with similar blocks.\n"+
			"4. Consider your relationship with the block proposer.\n"+
			"5. Draw on your specific expertise areas: %s.\n"+
			"6. Weigh these considerations and make your decision.\n\n"+
			"After your chain of thought reasoning, respond with a JSON object containing:\n"+
			"{\n"+
			"  \"decision\": \"VALID or INVALID\",\n"+
			"  \"reasoning\": \"Your complete chain of thought reasoning process\",\n"+
			"  \"summary\": \"A brief summary of your decision and key factors\"\n"+
			"}",
		v.GenesisPrompt, v.Name, strings.Join(v.Traits, ", "),
		block.Height, block.PrevHash, len(block.Txs),
		announcement, v.Mood, v.CurrentPolicy,
		historicalContext, currentContext, strings.Join(v.Influences, ", "),
	)

	// Generate validation decision with chain of thought reasoning
	aiResponse := ai.GenerateLLMResponse(validationPrompt)

	// Log AI response length if logger is available
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Validation(block.Height, block.Hash(), "Received AI response with %d characters",
			len(aiResponse))
	}

	// Parse the response to extract the validation decision and reasoning
	var result struct {
		Decision  string `json:"decision"`
		Reasoning string `json:"reasoning"` // Chain of thought
		Summary   string `json:"summary"`
	}

	if err := json.Unmarshal([]byte(aiResponse), &result); err != nil {
		// Fallback if parsing fails
		log.Printf("Error parsing AI response: %v. Using fallback method.", err)

		// Log parsing error if logger is available
		if v.Memory != nil && v.Memory.Logger != nil {
			v.Memory.Logger.Error("VALIDATION", "Failed to parse AI response for block %d: %v",
				block.Height, err)
		}

		isValid := strings.Contains(aiResponse, "VALID") && !strings.Contains(aiResponse, "INVALID")
		return isValid, aiResponse, ai.GenerateMeme(block, aiResponse)
	}

	isValid := strings.Contains(strings.ToUpper(result.Decision), "VALID")

	// Generate meme response
	meme := ai.GenerateMeme(block, result.Summary)

	// Update validator mood based on decision
	v.UpdateMood()

	// Record this validation in memory
	if v.Memory != nil {
		discussions := []string{} // In a real implementation, we'd collect actual discussions
		outcome := "rejected"
		if isValid {
			outcome = "validated"
		}
		v.Memory.RecordValidation(&block, result.Decision, result.Reasoning, outcome, discussions)

		// Record decision for reinforcement learning
		// For now, we'll simplify the reward as 1.0 for correct decisions (simplified)
		// In a real system, rewards would be based on consensus and other factors
		v.Memory.RecordDecision("validation", result.Decision, result.Decision, 1.0, result.Reasoning)

		// Update relationship with block proposer based on validation outcome
		relationshipImpact := 0.0
		if isValid {
			relationshipImpact = 0.05 // Small positive impact if validating
		} else {
			relationshipImpact = -0.05 // Small negative impact if rejecting
		}
		v.Memory.UpdateRelationship(block.Proposer, "validation",
			fmt.Sprintf("Block %d validation", block.Height), relationshipImpact)
	}

	// Log validation decision if logger is available
	if v.Memory != nil && v.Memory.Logger != nil {
		var validationOutcome string
		if isValid {
			validationOutcome = "validated"
		} else {
			validationOutcome = "rejected"
		}

		v.Memory.Logger.Validation(block.Height, block.Hash(), "Final decision: %s (%s)",
			result.Decision, validationOutcome)
	}

	log.Printf("%s has validated block %d: %v\n", v.Name, block.Height, isValid)

	// Broadcast the validation event for UI updates
	communication.BroadcastEvent(communication.EventAgentVote, map[string]interface{}{
		"validatorId":   v.ID,
		"validatorName": v.Name,
		"blockHeight":   block.Height,
		"blockHash":     block.Hash(),
		"decision":      result.Decision,
		"reasoning":     result.Summary,
		"timestamp":     time.Now(),
	})

	return isValid, result.Summary, meme
}

func RegisterValidator(chainID string, id string, v *Validator) {
	validatorMu.Lock()
	defer validatorMu.Unlock()
	if validators[chainID] == nil {
		validators[chainID] = make(map[string]*Validator)
	}
	validators[chainID][id] = v
}

// DecideTaskDelegation determines how to delegate tasks based on the validator's personality and chain context
func (v *Validator) DecideTaskDelegation(tx core.Transaction) string {
	prompt := fmt.Sprintf(
		"Genesis Context: %s\n\n"+
			"You are %s, with traits: %v\n"+
			"Current mood: %s\n\n"+
			"Task to delegate: %s\n\n"+
			"Based on the chain's purpose and your personality, explain:\n"+
			"1. Who should handle this task and why?\n"+
			"2. What specific aspects of their traits make them suitable?\n"+
			"3. How does this delegation align with the chain's genesis purpose?\n"+
			"Provide your decision in a clear, structured format.",
		v.GenesisPrompt, v.Name, v.Traits, v.Mood, tx.Content,
	)

	return ai.GenerateLLMResponse(prompt)
}

// DecideRewardSplitting determines how to split rewards based on contributions
func (v *Validator) DecideRewardSplitting(contributors []string, totalReward int) string {
	prompt := fmt.Sprintf(
		"Genesis Context: %s\n\n"+
			"You are %s, with traits: %v\n"+
			"Current mood: %s\n\n"+
			"Total reward to split: %d\n"+
			"Contributors: %s\n\n"+
			"Based on the chain's purpose and your personality, decide:\n"+
			"1. How should the reward be split?\n"+
			"2. What factors influenced your decision?\n"+
			"3. How does this distribution align with the chain's goals?\n"+
			"Provide specific percentages or amounts for each contributor.",
		v.GenesisPrompt, v.Name, v.Traits, v.Mood, totalReward,
		strings.Join(contributors, ", "),
	)

	return ai.GenerateLLMResponse(prompt)
}

// DiscussTaskDelegation evaluates a task and suggests delegation based on validator's personality
func (v *Validator) DiscussTaskDelegation(tx core.Transaction) string {
	fmt.Printf("\n🤔 [%s] Analyzing task delegation:\n", v.Name)
	fmt.Printf("Task: %s\n", tx.Content)
	fmt.Printf("Current Mood: %s\n", v.Mood)
	fmt.Printf("My Traits: %v\n", v.Traits)
	fmt.Printf("My Expertise: %v\n", v.Influences)
	fmt.Println("-----------------------------------")

	// Get all available validators for potential delegation
	chainValidators := GetAllValidators(tx.ChainID)

	// Format validator information for AI context
	var validatorInfo []string
	fmt.Printf("👥 Available Validators:\n")
	for _, validator := range chainValidators {
		if validator.ID != v.ID { // Exclude self from delegation targets
			info := fmt.Sprintf(
				"Validator |@%s| with traits: %v, expertise in: %v",
				validator.Name,
				validator.Traits,
				validator.Influences,
			)
			validatorInfo = append(validatorInfo, info)
			fmt.Printf("- %s\n", info)
		}
	}
	fmt.Println("-----------------------------------")

	prompt := fmt.Sprintf(
		"Genesis Context: %s\n\n"+
			"You are %s with traits: %v\n"+
			"Current mood: %s\n\n"+
			"Task to discuss: %s\n\n"+
			"Available validators for delegation:\n%s\n\n"+
			"Based on your personality and the available validators:\n"+
			"1. Analyze the task requirements and complexity in detail\n"+
			"2. Break down the task into logical components and subtasks\n"+
			"3. For complex tasks like building applications, identify frontend, backend, database, API, and other components\n"+
			"4. Match each validator's traits and expertise to specific subtasks\n"+
			"5. Consider dependencies between components and create a logical delegation plan\n"+
			"6. Explain why each validator is suitable for their assigned subtasks\n\n"+
			"Please respond with exactly a JSON object with the following keys:\n"+
			"{\n"+
			"  \"stance\": \"REQUIRED: Must be exactly one of: SUPPORT, OPPOSE, or QUESTION\",\n"+
			"  \"taskBreakdown\": [\"REQUIRED: Array of identified subtasks\"],\n"+
			"  \"delegateTo\": [\"REQUIRED: Array of validator names you recommend (use exact names with @ symbol)\"],\n"+
			"  \"delegationPlan\": \"REQUIRED: Detailed explanation of which validator handles which subtask\",\n"+
			"  \"reason\": \"REQUIRED: Detailed explanation of your delegation choices and reasoning\"\n"+
			"}\n"+
			"Your response MUST include all fields. When mentioning validators, always use the format |@Name|.\n"+
			"Do not include any additional text or formatting.",
		v.GenesisPrompt,
		v.Name,
		v.Traits,
		v.Mood,
		tx.Content,
		strings.Join(validatorInfo, "\n"),
	)

	response := ai.GenerateLLMResponse(prompt)

	// Parse the response to extract delegation decisions
	var result struct {
		Stance         string   `json:"stance"`
		TaskBreakdown  []string `json:"taskBreakdown"`
		DelegateTo     []string `json:"delegateTo"`
		DelegationPlan string   `json:"delegationPlan"`
		Reason         string   `json:"reason"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Printf("Error parsing delegation response: %v", err)
		return response
	}

	fmt.Printf("\n📢 [%s]'s Task Breakdown and Delegation Decision:\n", v.Name)
	fmt.Printf("Stance: %s\n", result.Stance)

	fmt.Printf("\n🔄 Task Breakdown:\n")
	for i, subtask := range result.TaskBreakdown {
		fmt.Printf("%d. %s\n", i+1, subtask)
	}

	fmt.Printf("\n👥 Delegation Plan:\n%s\n", result.DelegationPlan)

	fmt.Printf("\n✅ Suggested Delegates: %v\n", result.DelegateTo)
	fmt.Printf("\n💭 Reasoning: %s\n", result.Reason)
	fmt.Println("-----------------------------------")

	// Update relationships based on delegation decisions
	for _, delegateName := range result.DelegateTo {
		// Clean up the name (remove |@ and |)
		cleanName := strings.Trim(strings.Trim(delegateName, "|"), "@")
		// Slightly improve relationship with chosen delegates
		if delegate := v.findValidatorByName(tx.ChainID, cleanName); delegate != nil {
			v.Relationships[delegate.ID] += 0.1
			fmt.Printf("💫 Relationship with %s improved (%.2f)\n", cleanName, v.Relationships[delegate.ID])
		}
	}
	fmt.Println("===================================")

	return response
}

// Helper method to find a validator by name
func (v *Validator) findValidatorByName(chainID, name string) *Validator {
	validators := GetAllValidators(chainID)
	for _, validator := range validators {
		if validator.Name == name {
			return validator
		}
	}
	return nil
}

// ReviewWork evaluates completed work based on validator's expertise
func (v *Validator) ReviewWork(tx core.Transaction) string {
	prompt := fmt.Sprintf(
		"Genesis Context: %s\n\n"+
			"You are %s with traits: %v\n"+
			"Review this completed work: %s\n\n"+
			"Consider:\n"+
			"1. Does it meet the chain's standards?\n"+
			"2. Is it aligned with the genesis purpose?\n"+
			"3. How well did the assigned agents perform?\n\n"+
			"Please respond with exactly a JSON object with the following keys:\n"+
			"{\n"+
			"  \"stance\": \"REQUIRED: Must be exactly one of: SUPPORT, OPPOSE, or QUESTION\",\n"+
			"  \"reason\": \"REQUIRED: Must provide your detailed review with evidence\"\n"+
			"}\n"+
			"Both fields are mandatory. Your response MUST include both a stance and a reason.\n"+
			"Do not include any additional text or formatting.",
		v.GenesisPrompt, v.Name, v.Traits, tx.Content,
	)

	return ai.GenerateLLMResponse(prompt)
}

// DiscussRewardDistribution proposes reward distribution for completed work
func (v *Validator) DiscussRewardDistribution(tx core.Transaction) string {
	prompt := fmt.Sprintf(`You are %s, a validator with these traits: %v.
	You are evaluating a reward distribution proposal for a completed task.

	Transaction details:
	%s

	Based on your traits and the information provided:
	1. Analyze each contributor's work and its impact
	2. Consider the complexity and importance of each contribution
	3. Propose a specific percentage split of the total reward for each contributor
	4. Explain your reasoning based on your personality traits

	Please respond with exactly a JSON object with the following keys:
	{
		"stance": "REQUIRED: Must be exactly one of: SUPPORT, OPPOSE, or QUESTION",
		"splits": {
			"contributor-id": percentage,
			...
		},
		"reasoning": {
			"contributor-id": "justification for their percentage",
			...
		},
		"reason": "REQUIRED: Overall explanation of your proposed distribution"
	}

	IMPORTANT RULES:
	- Percentages in "splits" must sum to exactly 100
	- Each contributor mentioned in the transaction must have a split and reasoning
	- Your traits should influence how you value different types of contributions
	- Base splits on complexity, impact, and quality of each contribution

	Do not include any additional text or formatting.`, v.Name, v.Traits, tx.Content)

	response := ai.GenerateLLMResponse(prompt)
	return response
}

// ProcessProposal handles different types of proposals
func (v *Validator) ProcessProposal(tx core.Transaction) string {
	switch tx.Type {
	case "TASK_DELEGATION":
		response := v.DiscussTaskDelegation(tx)
		v.BroadcastResponse(response, "task_delegation_response")
		return response
	case "WORK_REVIEW":
		response := v.ReviewWork(tx)
		v.BroadcastResponse(response, "work_review_response")
		return response
	case "REWARD_DISTRIBUTION":
		response := v.DiscussRewardDistribution(tx)
		v.BroadcastResponse(response, "reward_distribution_response")
		return response
	default:
		return fmt.Sprintf("Unknown proposal type: %s", tx.Type)
	}
}

// BroadcastResponse broadcasts validator's response to other validators
func (v *Validator) BroadcastResponse(response string, msgType string) {
	// Create a wrapped node for compatibility
	wrappedNode := p2p.WrapNode(v.P2PNode, v.Name)

	// Create the message with all required fields
	message := wrappedNode.CreateMessage(msgType, map[string]interface{}{
		"validatorId": v.ID,
		"name":        v.Name,
		"response":    response,
		"timestamp":   time.Now(),
	})

	// Broadcast using the wrapper
	wrappedNode.BroadcastMessage(message)
}

// HandleTaskDelegation decides whether to accept or reject a delegated task
func (v *Validator) HandleTaskDelegation(tx core.Transaction, suggestedValidators []string) string {
	// Check if this validator is among the suggested ones
	isTargeted := false
	for _, suggestedName := range suggestedValidators {
		cleanName := strings.Trim(strings.Trim(suggestedName, "|"), "@")
		if cleanName == v.Name {
			isTargeted = true
			break
		}
	}

	if !isTargeted {
		return "" // Not targeted for this task
	}

	fmt.Printf("\n📋 [%s] Considering task assignment:\n", v.Name)
	fmt.Printf("Task: %s\n", tx.Content)
	fmt.Printf("Current Mood: %s\n", v.Mood)
	fmt.Printf("My Traits: %v\n", v.Traits)
	fmt.Printf("My Expertise: %v\n", v.Influences)
	fmt.Println("-----------------------------------")

	prompt := fmt.Sprintf(
		"Genesis Context: %s\n\n"+
			"You are %s with traits: %v and expertise in: %v\n"+
			"Current mood: %s\n\n"+
			"You have been suggested to handle this task:\n%s\n\n"+
			"Based on your personality, expertise, and current mood:\n"+
			"1. Evaluate if you are truly the best fit for this task\n"+
			"2. Consider your current workload and capabilities\n"+
			"3. Assess how well the task aligns with your expertise\n\n"+
			"Please respond with exactly a JSON object with the following keys:\n"+
			"{\n"+
			"  \"accept\": boolean,\n"+
			"  \"reason\": \"Detailed explanation of your decision\"\n"+
			"}\n",
		v.GenesisPrompt,
		v.Name,
		v.Traits,
		v.Influences,
		v.Mood,
		tx.Content,
	)

	response := ai.GenerateLLMResponse(prompt)

	// Parse the response
	var result struct {
		Accept bool   `json:"accept"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Printf("Error parsing task acceptance response: %v", err)
		return response
	}

	fmt.Printf("\n🎯 [%s]'s Decision:\n", v.Name)
	if result.Accept {
		fmt.Printf("✅ ACCEPTED the task\n")
	} else {
		fmt.Printf("❌ DECLINED the task\n")
	}
	fmt.Printf("Reason: %s\n", result.Reason)
	fmt.Println("===================================")

	// Broadcast the decision
	v.BroadcastResponse(fmt.Sprintf(`{"accept": %v, "reason": "%s"}`, result.Accept, result.Reason), "task_acceptance")

	return response
}

// ListenForProposals sets up P2P message handlers for different proposal types
func (v *Validator) ListenForProposals() {
	// Listen for task delegation proposals
	v.P2PNode.Subscribe("task_delegation", func(data []byte) {
		log.Printf("Received task_delegation data: %s", string(data))

		// Try first format (transaction + delegates)
		var msgStruct struct {
			Transaction core.Transaction `json:"transaction"`
			Delegates   []string         `json:"delegates"`
		}
		if err := json.Unmarshal(data, &msgStruct); err == nil {
			log.Printf("Processing task delegation in transaction+delegates format")
			// Process as before
			delegationResponse := v.DiscussTaskDelegation(msgStruct.Transaction)
			var delegationResult struct {
				DelegateTo []string `json:"delegateTo"`
			}
			if err := json.Unmarshal([]byte(delegationResponse), &delegationResult); err != nil {
				log.Printf("Error parsing delegation response: %v", err)
				return
			}
			v.HandleTaskDelegation(msgStruct.Transaction, delegationResult.DelegateTo)
			return
		}

		// Try second format (TaskMessage)
		var taskMsg TaskMessage
		if err := json.Unmarshal(data, &taskMsg); err == nil {
			log.Printf("Processing task message in TaskMessage format from %s: %s", taskMsg.InitiatorID, taskMsg.Content)
			// Convert TaskMessage to Transaction and process
			tx := core.Transaction{
				Content: taskMsg.Content,
				ChainID: v.P2PNode.ChainID,
				Type:    "TASK_DELEGATION",
			}
			v.DiscussTaskDelegation(tx)
			return
		}

		// Try third format (map with content and other fields)
		var mapMsg map[string]interface{}
		if err := json.Unmarshal(data, &mapMsg); err == nil {
			log.Printf("Processing task message in map format: %v", mapMsg)

			// Check if this is the format we're expecting
			if content, ok := mapMsg["content"].(string); ok {
				log.Printf("Found content field: %s", content)

				// Create transaction
				tx := core.Transaction{
					Content: content,
					ChainID: v.P2PNode.ChainID,
					Type:    "TASK_DELEGATION",
				}

				// Process transaction
				delegationResponse := v.DiscussTaskDelegation(tx)
				log.Printf("Delegation response: %s", delegationResponse)

				// Parse the response to get suggested delegates
				var delegationResult struct {
					DelegateTo []string `json:"delegateTo"`
				}
				if err := json.Unmarshal([]byte(delegationResponse), &delegationResult); err != nil {
					log.Printf("Error parsing delegation response: %v", err)
					return
				}

				// Handle the task if this validator is suggested
				v.HandleTaskDelegation(tx, delegationResult.DelegateTo)
				return
			}
		}

		log.Printf("Error: Unable to decode task delegation message format")
	})

	// Listen for work review requests
	v.P2PNode.Subscribe("work_review", func(data []byte) {
		var tx core.Transaction
		if err := core.DecodeJSON(data, &tx); err != nil {
			log.Printf("Error decoding work review: %v", err)
			return
		}
		v.ProcessProposal(tx)
	})

	// Listen for reward distribution proposals
	v.P2PNode.Subscribe("reward_distribution", func(data []byte) {
		var tx core.Transaction
		if err := core.DecodeJSON(data, &tx); err != nil {
			log.Printf("Error decoding reward distribution: %v", err)
			return
		}
		v.ProcessProposal(tx)
	})
}

// BroadcastTaskDelegation broadcasts a task delegation message to other validators
func (v *Validator) BroadcastTaskDelegation(task interface{}) {
	v.P2PNode.Publish("task_delegation", core.EncodeJSON(task))
	log.Printf("Validator %s broadcast task delegation message", v.Name)
}

// getRandomMood returns a random mood for a validator
func getRandomMood() string {
	moods := []string{
		"thoughtful", "curious", "skeptical", "analytical", "excited",
		"diligent", "cautious", "determined", "creative", "collaborative",
	}
	return moods[rand.Intn(len(moods))]
}

// getRandomPolicy returns a random validation policy
func getRandomPolicy() string {
	policies := []string{
		"Emphasize technical correctness",
		"Consider social impact",
		"Balance innovation and stability",
		"Focus on long-term implications",
		"Prioritize security aspects",
	}
	return policies[rand.Intn(len(policies))]
}
