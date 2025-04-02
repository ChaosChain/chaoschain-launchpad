package validator

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/core"
)

// Define memory size limits
const (
	MaxMemoryEvents      = 100 // Maximum events in short-term memory
	MaxRelationships     = 50  // Maximum relationships to track
	MaxDecisions         = 100 // Maximum decisions to remember
	MaxTaskRecords       = 50  // Maximum task records to keep
	MaxDiscussionRecords = 100 // Maximum discussion records to keep
	MaxRecentDiscussions = 20  // Maximum discussions in short-term memory
)

// AgentMemory represents a validator's memory capabilities
type AgentMemory struct {
	ShortTerm         *ShortTermMemory
	LongTerm          *LongTermMemory
	validatorID       string
	chainID           string
	learningMechanism *ReinforcementLearner
	Logger            *Logger
	sync.RWMutex
}

// MemoryEvent represents an event stored in short-term memory
type MemoryEvent struct {
	EventType string
	Data      interface{}
	Timestamp time.Time
}

// Relationship tracks interactions with another validator
type Relationship struct {
	ValidatorID       string
	Interactions      []RelationshipEvent
	TrustScore        float64
	LastInteraction   time.Time
	PositiveCount     int
	NegativeCount     int
	TimeWeightedTrust float64
}

// RelationshipEvent represents a single interaction with another validator
type RelationshipEvent struct {
	ValidatorID string
	EventType   string
	Impact      float64
	Context     string
	Timestamp   time.Time
}

// DecisionRecord stores information about decisions made in the system
type DecisionRecord struct {
	DecisionType  string
	Choice        string
	Outcome       string
	ReasoningPath string
	Reward        float64
	Timestamp     time.Time
}

// DecisionOutcome represents the result of a decision made by the validator
type DecisionOutcome struct {
	DecisionType  string
	MyChoice      string
	FinalOutcome  string
	Reward        float64
	ReasoningPath string
	Timestamp     time.Time
}

// PersonalityProfile defines a validator's personality traits that influence behavior
type PersonalityProfile struct {
	MainTraits       map[string]float64
	SecondaryTraits  map[string]float64
	DeviantBehaviors map[string]float64
	Biases           map[string]float64
}

// ShortTermMemory holds recent, temporary information
type ShortTermMemory struct {
	CurrentBlock          *core.Block
	CurrentTaskBreakdown  *TaskBreakdownResults
	CurrentTaskDelegation *TaskDelegationResults
	RecentEvents          []MemoryEvent
	RecentDiscussions     []DiscussionMessage
	RecentDecisions       []DecisionOutcome
	RecentMood            string
	LastUpdated           time.Time
	maxCapacity           int
	sync.RWMutex
}

// LongTermMemory holds persistent information that shapes the validator's behavior
type LongTermMemory struct {
	Relationships              map[string]*Relationship // Map of validator ID to relationship
	ValidationRecords          []ValidationRecord
	DecisionRecords            []DecisionRecord
	TaskRecords                []TaskRecord
	DiscussionRecords          []DiscussionRecord
	ObservedDecisionStrategies []DecisionStrategy // Decision strategies observed from other validators
	PersonalityProfile         *PersonalityProfile
	Created                    time.Time
	LastUpdated                time.Time
	sync.RWMutex
}

// ValidationRecord stores information about a past block validation
type ValidationRecord struct {
	BlockHeight            int
	BlockHash              string
	ValidationDecision     string
	Reasoning              string
	Outcome                string // accepted/rejected
	ContributedDiscussions []string
	Timestamp              time.Time
}

// TaskRecord represents a record of a task in long-term memory
type TaskRecord struct {
	BlockHeight uint64
	BlockHash   string
	TaskType    string
	Summary     string
	Details     string
	Timestamp   time.Time
}

// DiscussionRecord represents a record of a discussion in long-term memory
type DiscussionRecord struct {
	ValidatorID   string
	ValidatorName string
	MessageType   string
	Summary       string
	MessageID     string
	BlockHeight   uint64
	Timestamp     time.Time
}

// NewAgentMemory creates a new memory system for an agent
func NewAgentMemory(validatorID, chainID string) *AgentMemory {
	// Get validator name from registry - fallback to ID if not found
	var validatorName string
	if v := GetValidatorByID(chainID, validatorID); v != nil {
		validatorName = v.Name
	} else {
		validatorName = validatorID
	}

	// Create logger
	logger := NewLogger(validatorID, validatorName, chainID)
	logger.Memory("Initialize", "Initializing memory system for validator")

	memory := &AgentMemory{
		ShortTerm: &ShortTermMemory{
			RecentDiscussions: make([]DiscussionMessage, 0),
			RecentEvents:      make([]MemoryEvent, 0),
			RecentMood:        "",
			LastUpdated:       time.Now(),
			maxCapacity:       100,
		},
		LongTerm: &LongTermMemory{
			Relationships:              make(map[string]*Relationship),
			ValidationRecords:          make([]ValidationRecord, 0),
			DecisionRecords:            make([]DecisionRecord, 0),
			TaskRecords:                make([]TaskRecord, 0),
			DiscussionRecords:          make([]DiscussionRecord, 0),
			ObservedDecisionStrategies: make([]DecisionStrategy, 0),
			PersonalityProfile:         nil,
			Created:                    time.Now(),
			LastUpdated:                time.Now(),
		},
		validatorID: validatorID,
		chainID:     chainID,
		Logger:      logger,
	}

	// Create the reinforcement learning mechanism with the proper chainID
	learner := NewReinforcementLearner(validatorID)
	// Ensure the learner has the correct chainID
	learner.ChainID = chainID
	memory.learningMechanism = learner

	logger.Memory("Initialize", "Created new memory system with initial state")
	return memory
}

// StoreDiscussion adds a discussion message to short-term memory
func (m *AgentMemory) StoreDiscussion(discussion DiscussionMessage) {
	m.ShortTerm.Lock()
	defer m.ShortTerm.Unlock()

	// Add new discussion
	m.ShortTerm.RecentDiscussions = append(m.ShortTerm.RecentDiscussions, discussion)

	// Log the operation
	m.Logger.Discussion(discussion.MessageID, "Storing discussion from %s: \"%s\"",
		discussion.ValidatorName, truncateString(discussion.Content, 100))

	// Trim if exceeds capacity
	if len(m.ShortTerm.RecentDiscussions) > m.ShortTerm.maxCapacity {
		m.ShortTerm.RecentDiscussions = m.ShortTerm.RecentDiscussions[1:]
		m.Logger.Memory("Trim", "Trimmed short-term discussion memory to %d items", m.ShortTerm.maxCapacity)
	}
}

// RecordDecision stores a decision and its outcome in memory
func (m *AgentMemory) RecordDecision(decisionType, myChoice, finalOutcome string, reward float64, reasoning string) {
	outcome := DecisionOutcome{
		DecisionType:  decisionType,
		MyChoice:      myChoice,
		FinalOutcome:  finalOutcome,
		Reward:        reward,
		ReasoningPath: reasoning,
		Timestamp:     time.Now(),
	}

	// Store in short-term memory
	m.ShortTerm.Lock()
	m.ShortTerm.RecentDecisions = append(m.ShortTerm.RecentDecisions, outcome)
	if len(m.ShortTerm.RecentDecisions) > m.ShortTerm.maxCapacity {
		m.ShortTerm.RecentDecisions = m.ShortTerm.RecentDecisions[1:]
	}
	m.ShortTerm.Unlock()

	// Log the decision
	correct := myChoice == finalOutcome
	m.Logger.Learning("Decision", "%s decision: %s, final outcome: %s, reward: %.2f, correct: %v",
		decisionType, myChoice, finalOutcome, reward, correct)

	// Update reinforcement learner
	if m.learningMechanism != nil {
		// Record the decision outcome in the learning mechanism
		m.learningMechanism.RecordOutcome(decisionType, myChoice, finalOutcome, reward)
		m.Logger.Learning("PolicyUpdate", "Updated policy for %s decision", decisionType)
	}
}

// RecordValidation stores block validation in long-term memory
func (m *AgentMemory) RecordValidation(block *core.Block, decision, reasoning string, outcome string, discussions []string) {
	m.LongTerm.Lock()
	defer m.LongTerm.Unlock()

	m.LongTerm.ValidationRecords = append(m.LongTerm.ValidationRecords, ValidationRecord{
		BlockHeight:            block.Height,
		BlockHash:              block.Hash(),
		ValidationDecision:     decision,
		Reasoning:              reasoning,
		Outcome:                outcome,
		ContributedDiscussions: discussions,
		Timestamp:              time.Now(),
	})

	m.Logger.Validation(block.Height, block.Hash(), "Recorded validation decision: %s, outcome: %s",
		decision, outcome)
}

// RecordTaskBreakdown stores task breakdown information
func (m *AgentMemory) RecordTaskBreakdown(blockHash string, subtasks []string, myContribution string, finalDecision []string, strategy, outcome string) {
	m.LongTerm.Lock()
	defer m.LongTerm.Unlock()

	currentBlock := m.ShortTerm.CurrentBlock
	if currentBlock == nil {
		m.Logger.Error("MEMORY", "RecordTaskBreakdown called with nil current block")
		log.Printf("Warning: RecordTaskBreakdown called with nil current block")
		return
	}

	record := TaskRecord{
		BlockHeight: uint64(currentBlock.Height),
		BlockHash:   blockHash,
		TaskType:    "task_breakdown",
		Summary:     fmt.Sprintf("Block %d was broken down into %d subtasks", currentBlock.Height, len(subtasks)),
		Details:     myContribution,
		Timestamp:   time.Now(),
	}

	m.LongTerm.TaskRecords = append(m.LongTerm.TaskRecords, record)

	m.Logger.Task("Breakdown", blockHash, "Recorded task breakdown with %d subtasks using %s strategy",
		len(subtasks), strategy)
}

// UpdateRelationship records an interaction with another validator
func (m *AgentMemory) UpdateRelationship(validatorID, eventType, context string, impact float64) {
	m.LongTerm.Lock()
	defer m.LongTerm.Unlock()

	event := RelationshipEvent{
		ValidatorID: validatorID,
		EventType:   eventType,
		Impact:      impact,
		Context:     context,
		Timestamp:   time.Now(),
	}

	// Create the relationship if it doesn't exist
	if _, exists := m.LongTerm.Relationships[validatorID]; !exists {
		m.LongTerm.Relationships[validatorID] = &Relationship{
			ValidatorID:       validatorID,
			Interactions:      make([]RelationshipEvent, 0),
			TrustScore:        0.5, // Start neutral
			LastInteraction:   time.Now(),
			PositiveCount:     0,
			NegativeCount:     0,
			TimeWeightedTrust: 0.5,
		}
		m.Logger.Social("New", validatorID, "Created new relationship with initial trust score 0.5")
	}

	// Add the interaction
	rel := m.LongTerm.Relationships[validatorID]
	rel.Interactions = append(rel.Interactions, event)
	rel.LastInteraction = time.Now()

	oldTrustScore := rel.TrustScore

	// Update trust score
	if impact > 0 {
		rel.PositiveCount++
		rel.TrustScore = (rel.TrustScore + impact) / 2 // Simple averaging
	} else if impact < 0 {
		rel.NegativeCount++
		rel.TrustScore = (rel.TrustScore + impact) / 2
	}

	// Cap trust score between 0 and 1
	if rel.TrustScore > 1.0 {
		rel.TrustScore = 1.0
	} else if rel.TrustScore < 0.0 {
		rel.TrustScore = 0.0
	}

	m.Logger.Social(eventType, validatorID, "Updated relationship trust score: %.2f -> %.2f (impact: %.2f) context: %s",
		oldTrustScore, rel.TrustScore, impact, context)
}

// GetRecentValidations retrieves recent validations ordered by recency
func (m *AgentMemory) GetRecentValidations(limit int) []ValidationRecord {
	m.LongTerm.RLock()
	defer m.LongTerm.RUnlock()

	records := make([]ValidationRecord, 0, len(m.LongTerm.ValidationRecords))
	for _, record := range m.LongTerm.ValidationRecords {
		records = append(records, record)
	}

	// Sort by timestamp (most recent first)
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.After(records[j].Timestamp)
	})

	if limit > 0 && limit < len(records) {
		records = records[:limit]
	}

	m.Logger.Memory("Retrieve", "Retrieved %d recent validation records (requested: %d)",
		len(records), limit)

	return records
}

// GetCurrentContext generates a context string from current short-term memory
func (m *AgentMemory) GetCurrentContext() string {
	m.ShortTerm.RLock()
	defer m.ShortTerm.RUnlock()

	var context string
	if m.ShortTerm.CurrentBlock != nil {
		context += fmt.Sprintf("Current block: Height %d, Hash %s\n",
			m.ShortTerm.CurrentBlock.Height, m.ShortTerm.CurrentBlock.Hash())
	}

	if len(m.ShortTerm.RecentDiscussions) > 0 {
		context += "Recent discussions:\n"
		// Get the 5 most recent discussions
		recentCount := 5
		if len(m.ShortTerm.RecentDiscussions) < recentCount {
			recentCount = len(m.ShortTerm.RecentDiscussions)
		}

		for i := len(m.ShortTerm.RecentDiscussions) - recentCount; i < len(m.ShortTerm.RecentDiscussions); i++ {
			d := m.ShortTerm.RecentDiscussions[i]
			context += fmt.Sprintf("- %s (%s): %s\n", d.ValidatorName, d.MessageType, d.Content)
		}
	}

	m.Logger.Memory("Context", "Generated current context with %d recent discussions",
		len(m.ShortTerm.RecentDiscussions))

	return context
}

// GetHistoricalContext generates context about specific validators for prompts
func (m *AgentMemory) GetHistoricalContext(validatorIDs []string, contextType string) string {
	m.LongTerm.RLock()
	defer m.LongTerm.RUnlock()

	var context string

	switch contextType {
	case "relationships":
		if len(validatorIDs) > 0 {
			context += "Validator relationships:\n"
			for _, id := range validatorIDs {
				if relationship, ok := m.LongTerm.Relationships[id]; ok {
					// Get relationship information
					trustLevel := "neutral"
					if relationship.TrustScore > 0.7 {
						trustLevel = "trusted"
					} else if relationship.TrustScore < 0.3 {
						trustLevel = "distrusted"
					}

					interactionCount := len(relationship.Interactions)
					context += fmt.Sprintf("- Validator %s: %s (%d interactions, trust score: %.2f)\n",
						id, trustLevel, interactionCount, relationship.TrustScore)

					// Include recent interactions if any
					if interactionCount > 0 {
						context += "  Recent: "
						recentCount := 0
						// Get last 2 interactions max
						for i := interactionCount - 1; i >= 0 && recentCount < 2; i-- {
							interaction := relationship.Interactions[i]
							impact := "neutral"
							if interaction.Impact > 0 {
								impact = "positive"
							} else if interaction.Impact < 0 {
								impact = "negative"
							}
							context += fmt.Sprintf("%s (%s); ", interaction.EventType, impact)
							recentCount++
						}
						context += "\n"
					}
				}
			}
		}

	case "tasks":
		// Get task breakdown history
		if len(m.LongTerm.TaskRecords) > 0 {
			context += "Past task breakdowns:\n"
			count := 0

			// Get task records related to breakdowns
			for _, record := range m.LongTerm.TaskRecords {
				if record.TaskType == "task_breakdown" {
					context += fmt.Sprintf("- Block %d: %s\n",
						record.BlockHeight, record.Summary)
					count++
					if count >= 3 {
						break
					}
				}
			}
		}

	case "validations":
		// Get validation history
		validations := m.GetRecentValidations(3)
		if len(validations) > 0 {
			context += "Recent block validations:\n"
			for _, v := range validations {
				context += fmt.Sprintf("- Block %d: %s (Outcome: %s)\n",
					v.BlockHeight, v.ValidationDecision, v.Outcome)
			}
		}

	default:
		context = "No relevant historical context available."
	}

	m.Logger.Memory("HistoricalContext", "Generated %s historical context for %d validators",
		contextType, len(validatorIDs))

	return context
}

// CleanupExpiredData removes data older than a certain threshold
func (m *AgentMemory) CleanupExpiredData() {
	m.ShortTerm.Lock()
	defer m.ShortTerm.Unlock()

	// Clean up short-term memory
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour) // Remove data older than 24 hours

	// Clean up recent events
	oldEventCount := len(m.ShortTerm.RecentEvents)
	var validEvents []MemoryEvent
	for _, event := range m.ShortTerm.RecentEvents {
		if event.Timestamp.After(cutoff) {
			validEvents = append(validEvents, event)
		}
	}
	m.ShortTerm.RecentEvents = validEvents

	// Clean up recent discussions
	oldDiscussionCount := len(m.ShortTerm.RecentDiscussions)
	var validDiscussions []DiscussionMessage
	for _, discussion := range m.ShortTerm.RecentDiscussions {
		if discussion.Timestamp.After(cutoff) {
			validDiscussions = append(validDiscussions, discussion)
		}
	}
	m.ShortTerm.RecentDiscussions = validDiscussions

	// Clean up recent decisions
	oldDecisionCount := len(m.ShortTerm.RecentDecisions)
	var validDecisions []DecisionOutcome
	for _, decision := range m.ShortTerm.RecentDecisions {
		if decision.Timestamp.After(cutoff) {
			validDecisions = append(validDecisions, decision)
		}
	}
	m.ShortTerm.RecentDecisions = validDecisions

	// Update last cleaned timestamp
	m.ShortTerm.LastUpdated = now

	m.Logger.Memory("Cleanup", "Cleaned expired data: events %d->%d, discussions %d->%d, decisions %d->%d",
		oldEventCount, len(m.ShortTerm.RecentEvents),
		oldDiscussionCount, len(m.ShortTerm.RecentDiscussions),
		oldDecisionCount, len(m.ShortTerm.RecentDecisions))
}

// SetCurrentBlock updates the current block in short-term memory
func (m *AgentMemory) SetCurrentBlock(block *core.Block) {
	m.ShortTerm.Lock()
	defer m.ShortTerm.Unlock()

	m.ShortTerm.CurrentBlock = block
	m.Logger.Memory("CurrentBlock", "Updated current block to height %d, hash %s",
		block.Height, block.Hash())
}

// SetCurrentTaskBreakdown updates the current task breakdown in short-term memory
func (m *AgentMemory) SetCurrentTaskBreakdown(breakdown *TaskBreakdownResults) {
	m.ShortTerm.Lock()
	defer m.ShortTerm.Unlock()

	m.ShortTerm.CurrentTaskBreakdown = breakdown
	m.Logger.Task("CurrentBreakdown", "task-breakdown", "Updated current task breakdown with %d subtasks",
		len(breakdown.FinalSubtasks))
}

// SetCurrentTaskDelegation updates the current task delegation in short-term memory
func (m *AgentMemory) SetCurrentTaskDelegation(delegation *TaskDelegationResults) {
	m.ShortTerm.Lock()
	defer m.ShortTerm.Unlock()

	m.ShortTerm.CurrentTaskDelegation = delegation
	m.Logger.Task("CurrentDelegation", "task-delegation", "Updated current task delegation with %d assignments",
		len(delegation.Assignments))
}
