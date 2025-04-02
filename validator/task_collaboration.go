package validator

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/communication"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/google/uuid"
)

// TaskMessage represents a task request for validators
type TaskMessage struct {
	Content     string    `json:"content"`
	Timestamp   time.Time `json:"timestamp"`
	InitiatorID string    `json:"initiatorId"`
}

// TaskBreakdownRound represents a single round of task breakdown discussion
type TaskBreakdownRound struct {
	Round     int
	Proposals map[string]TaskBreakdownProposal // validatorID -> proposal
}

// TaskBreakdownProposal represents a validator's proposed task breakdown
type TaskBreakdownProposal struct {
	ValidatorID   string   `json:"validatorId"`
	ValidatorName string   `json:"validatorName"`
	Subtasks      []string `json:"subtasks"`
	Reasoning     string   `json:"reasoning"`
	Timestamp     time.Time
}

// DecisionStrategy represents an agent's strategy for final decision making
type DecisionStrategy struct {
	ValidatorID   string `json:"validatorId"`
	ValidatorName string `json:"validatorName"`
	Name          string `json:"name"`        // Name of the strategy (e.g., "consensus", "leader", "expert")
	Description   string `json:"description"` // Detailed description of how the strategy works
	Reasoning     string `json:"reasoning"`   // Why this strategy is proposed
	Timestamp     time.Time
}

// StrategyVote represents a validator's vote for a decision strategy
type StrategyVote struct {
	ValidatorID   string `json:"validatorId"`
	ValidatorName string `json:"validatorName"`
	StrategyName  string `json:"strategyName"`
	Reasoning     string `json:"reasoning"`
	Timestamp     time.Time
}

// StrategyDiscussion represents a discussion message about strategy selection
type StrategyDiscussion struct {
	ValidatorID   string            `json:"validatorId"`
	ValidatorName string            `json:"validatorName"`
	MessageType   string            `json:"messageType"` // "propose", "support", "question", "refine"
	Content       string            `json:"content"`
	Strategy      *DecisionStrategy `json:"strategy,omitempty"`
	Timestamp     time.Time
}

// TaskBreakdownResults contains the final consolidated task breakdown
type TaskBreakdownResults struct {
	FinalSubtasks      []string             // The final, agreed-upon list of subtasks
	Discussion         TaskDiscussion       // Complete discussion history
	ConsensusScore     float64              // The final consensus score
	BlockInfo          *core.Block          // The block that triggered this breakdown
	TransactionDetails string               // String representation of transaction details
	SelectedStrategy   *DecisionStrategy    // The selected decision strategy
	StrategyDiscussion []StrategyDiscussion // Discussion about strategy selection
}

// TaskDelegationRound represents a single round of task delegation discussion
type TaskDelegationRound struct {
	Round     int
	Proposals map[string]TaskDelegationProposal // validatorID -> proposal
}

// TaskDelegationProposal represents a validator's proposed task delegation
type TaskDelegationProposal struct {
	ValidatorID   string            `json:"validatorId"`
	ValidatorName string            `json:"validatorName"`
	Assignments   map[string]string `json:"assignments"` // subtask -> validator name
	Reasoning     string            `json:"reasoning"`
	Timestamp     time.Time
}

// TaskDelegationResults contains the final consolidated task delegations
type TaskDelegationResults struct {
	Assignments map[string]string // subtask -> validator name
	BlockInfo   *core.Block       // The block that triggered this delegation
	Subtasks    []string          // The subtasks being delegated
	Discussion  TaskDelegationDiscussion
	Strategy    *DecisionStrategy // The selected decision strategy
}

// AgentFeedback represents feedback from an agent on a proposal
type AgentFeedback struct {
	ValidatorID     string   `json:"validatorId"`
	ValidatorName   string   `json:"validatorName"`
	FeedbackType    string   `json:"feedbackType"`              // "support", "critique", "refine"
	Message         string   `json:"message"`                   // Detailed feedback message
	RefinedSubtasks []string `json:"refinedSubtasks,omitempty"` // Only present for "refine" type
	Timestamp       time.Time
}

const (
	InitialProposalRound = 1
	FeedbackRound        = 2
	FinalizationRound    = 3
	DiscussionRounds     = 5               // Total number of discussion rounds
	FinalProposalRound   = 6               // New round for final proposals
	RoundDuration        = 5 * time.Second // Time per round
)

// Event types for task delegation
const (
	EventTaskDelegationVote = "TASK_DELEGATION_VOTE"
)

var (
	taskBreakdownMutex  sync.Mutex
	taskDelegationMutex sync.Mutex
)

// Replace TaskBreakdownRound with a more flexible Discussion-based structure
type TaskDiscussion struct {
	Messages []DiscussionMessage // Chronological list of all messages
}

type DiscussionMessage struct {
	ValidatorID   string    `json:"validatorId"`
	ValidatorName string    `json:"validatorName"`
	MessageType   string    `json:"messageType"`        // "proposal", "critique", "refinement", "agreement", "question", etc.
	Content       string    `json:"content"`            // The actual message text
	Proposal      []string  `json:"proposal,omitempty"` // Optional: if proposing subtasks
	ReplyTo       string    `json:"replyTo,omitempty"`  // Optional: ID of message being replied to
	MessageID     string    `json:"messageId"`          // Unique ID for this message
	Timestamp     time.Time `json:"timestamp"`
}

// Replace TaskDelegationRound with a more flexible Discussion-based structure
type TaskDelegationDiscussion struct {
	Messages []TaskDelegationMessage // Chronological list of all messages
}

// TaskDelegationMessage represents a message in the task delegation discussion
type TaskDelegationMessage struct {
	ValidatorID   string            `json:"validatorId"`
	ValidatorName string            `json:"validatorName"`
	MessageType   string            `json:"messageType"` // "proposal", "agreement", "suggestion", "question", "summary"
	Content       string            `json:"content"`
	Assignments   map[string]string `json:"assignments,omitempty"`
	MessageID     string            `json:"messageId"`
	Timestamp     time.Time         `json:"timestamp"`
}

// Validator represents a node that can validate transactions and perform tasks
type TaskValidator struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Traits []string `json:"traits,omitempty"`
}

// ProposalVote represents a validator's vote on a specific proposal
type ProposalVote struct {
	ValidatorID   string  `json:"validatorId"`
	ValidatorName string  `json:"validatorName"`
	ProposalIndex int     `json:"proposalIndex"`
	Score         float64 `json:"score"` // 0.0 to 1.0
	Reasoning     string  `json:"reasoning"`
	Timestamp     time.Time
}

// StartCollaborativeTaskBreakdown initiates a fluid discussion-based task breakdown process
func StartCollaborativeTaskBreakdown(chainID string, block *core.Block, transactionDetails string) *TaskBreakdownResults {
	log.Printf("Starting collaborative task breakdown for transactions in block %d", block.Height)

	// Initialize results
	results := &TaskBreakdownResults{
		FinalSubtasks:      []string{},
		Discussion:         TaskDiscussion{Messages: []DiscussionMessage{}},
		ConsensusScore:     0.0,
		BlockInfo:          block,
		TransactionDetails: transactionDetails,
		SelectedStrategy:   nil,
		StrategyDiscussion: []StrategyDiscussion{},
	}

	// Get validators for this chain
	validators := GetAllValidators(chainID)
	if len(validators) == 0 {
		log.Printf("No validators found for chain %s", chainID)
		return results
	}

	log.Printf("Found %d validators for task breakdown discussion", len(validators))

	// Create a communication thread for this breakdown session
	threadID := fmt.Sprintf("task-breakdown-%s", block.Hash())
	log.Printf("Created discussion thread with ID: %s", threadID)

	communication.BroadcastEvent(communication.EventTaskBreakdownStarted, map[string]interface{}{
		"blockHeight": block.Height,
		"threadId":    threadID,
		"timestamp":   time.Now(),
	})

	// PHASE 1: Strategy Selection
	log.Printf("Beginning strategy selection phase")

	// Each validator proposes and discusses strategies
	var proposedStrategies []*DecisionStrategy
	for _, v := range validators {
		// Generate strategy proposal
		strategy := generateStrategyProposal(v, results)
		if strategy != nil {
			proposedStrategies = append(proposedStrategies, strategy)

			// Add to discussion
			discussion := StrategyDiscussion{
				ValidatorID:   v.ID,
				ValidatorName: v.Name,
				MessageType:   "propose",
				Content:       strategy.Description,
				Strategy:      strategy,
				Timestamp:     time.Now(),
			}
			results.StrategyDiscussion = append(results.StrategyDiscussion, discussion)

			// Broadcast strategy proposal
			communication.BroadcastEvent(communication.EventDecisionStrategy, map[string]interface{}{
				"validatorId":   v.ID,
				"validatorName": v.Name,
				"strategy":      strategy,
				"blockHeight":   block.Height,
				"timestamp":     time.Now(),
			})

			// Add delay between strategy proposals for better UI visibility
			time.Sleep(2 * time.Second)
		}
	}

	// Add delay to ensure all strategy proposals are visible
	time.Sleep(3 * time.Second)

	// Allow validators to discuss and vote on strategies
	strategyVotes := conductStrategyVoting(validators, proposedStrategies, results)

	// Select winning strategy
	selectedStrategy := selectWinningStrategy(strategyVotes, proposedStrategies)
	results.SelectedStrategy = selectedStrategy

	log.Printf("Selected decision strategy: %s", selectedStrategy.Name)

	// Broadcast selected strategy to all validators
	communication.BroadcastEvent(communication.EventStrategySelected, map[string]interface{}{
		"strategy":    selectedStrategy,
		"blockHeight": block.Height,
		"timestamp":   time.Now(),
	})

	// Add delay to ensure strategy selection is visible before proceeding
	time.Sleep(3 * time.Second)

	// PHASE 2: Initial Proposals and Refinements
	log.Printf("Beginning initial proposal and refinement phase using %s strategy", selectedStrategy.Name)

	// Initialize tracking for validators who have contributed
	hasContributed := make(map[string]bool)

	// Each validator decides whether to propose new ideas or refine existing ones
	for _, v := range validators {
		if hasContributed[v.ID] {
			continue // Skip if already contributed
		}

		log.Printf("🤔 [%s] Considering contribution to task breakdown...", v.Name)

		// Convert to TaskValidator for compatibility
		taskValidator := validatorToTaskValidator(v)

		// Generate contribution based on current state
		shouldContribute, contribution := generateContribution(taskValidator, formatDiscussionContext(results.Discussion), results, 1)
		if !shouldContribute {
			log.Printf("💭 [%s] Chose to observe rather than contribute at this stage", v.Name)
			continue
		}

		message := DiscussionMessage{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			MessageType:   contribution.MessageType,
			Content:       contribution.Content,
			Proposal:      contribution.Proposal,
			ReplyTo:       contribution.ReplyTo,
			MessageID:     uuid.New().String(),
			Timestamp:     time.Now(),
		}

		results.Discussion.Messages = append(results.Discussion.Messages, message)
		hasContributed[v.ID] = true

		// Broadcast validator's contribution
		communication.BroadcastEvent(communication.EventTaskBreakdownMessage, map[string]interface{}{
			"validatorId":   v.ID,
			"validatorName": v.Name,
			"messageType":   message.MessageType,
			"content":       message.Content,
			"proposal":      message.Proposal,
			"messageId":     message.MessageID,
			"blockHeight":   block.Height,
			"timestamp":     time.Now(),
		})

		// Store in validator's memory if available
		if v.Memory != nil {
			v.Memory.StoreDiscussion(message)
		}

		// Add short delay between validators to simulate thinking time
		time.Sleep(100 * time.Millisecond)
	}

	// Add new phase for final proposals
	log.Printf("Beginning final proposal round")

	// Initialize tracking for final proposals
	finalProposals := make(map[string]TaskBreakdownProposal)

	// Each validator submits their final proposal
	for _, v := range validators {
		// Format discussion context for final decision
		var discussionContext strings.Builder
		discussionContext.WriteString("Previous discussion and proposals:\n\n")

		for _, msg := range results.Discussion.Messages {
			if msg.MessageType == "proposal" || msg.MessageType == "refinement" {
				discussionContext.WriteString(fmt.Sprintf("From %s (%s):\n", msg.ValidatorName, msg.MessageType))
				if len(msg.Proposal) > 0 {
					for i, task := range msg.Proposal {
						discussionContext.WriteString(fmt.Sprintf("%d. %s\n", i+1, task))
					}
				}
				discussionContext.WriteString(fmt.Sprintf("Reasoning: %s\n\n", msg.Content))
			}
		}

		prompt := fmt.Sprintf(`You are %s, with traits: %v.
		After participating in the discussion about task breakdown, it's time to submit your FINAL proposal.
		
		Discussion Context:
		%s
		
		You can either:
		1. Submit your own refined version of the task breakdown
		2. Support and adopt another validator's proposal with minor refinements
		3. Create a merged proposal combining the best elements from multiple proposals
		
		Consider:
		- The feedback and critiques from the discussion
		- The strengths of each proposal
		- The overall effectiveness and completeness
		
		Respond with a JSON object:
		{
			"subtasks": ["task1", "task2", ...],
			"reasoning": "Explain your final choice and any refinements made",
			"basedOn": "If adopting/refining another's proposal, mention their name"
		}`, v.Name, v.Traits, discussionContext.String())

		response := ai.GenerateLLMResponse(prompt)

		var finalProposalData struct {
			Subtasks  []string `json:"subtasks"`
			Reasoning string   `json:"reasoning"`
			BasedOn   string   `json:"basedOn"`
		}

		if err := json.Unmarshal([]byte(response), &finalProposalData); err != nil {
			log.Printf("Error parsing final proposal from %s: %v", v.Name, err)
			continue
		}

		// Create final proposal
		finalProposal := TaskBreakdownProposal{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			Subtasks:      finalProposalData.Subtasks,
			Reasoning:     finalProposalData.Reasoning,
			Timestamp:     time.Now(),
		}

		finalProposals[v.ID] = finalProposal

		// Add to discussion
		message := DiscussionMessage{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			MessageType:   "final_proposal",
			Content: fmt.Sprintf("Final Proposal%s\n\nSubtasks:\n%s\n\nReasoning: %s",
				func() string {
					if finalProposalData.BasedOn != "" {
						return fmt.Sprintf(" (based on %s's proposal)", finalProposalData.BasedOn)
					}
					return ""
				}(),
				func() string {
					var subtasksList strings.Builder
					for i, task := range finalProposalData.Subtasks {
						subtasksList.WriteString(fmt.Sprintf("%d. %s\n", i+1, task))
					}
					return subtasksList.String()
				}(),
				finalProposalData.Reasoning),
			Proposal:  finalProposalData.Subtasks,
			MessageID: uuid.New().String(),
			Timestamp: time.Now(),
		}

		results.Discussion.Messages = append(results.Discussion.Messages, message)

		// Broadcast final proposal
		communication.BroadcastEvent(communication.EventTaskBreakdownMessage, map[string]interface{}{
			"validatorId":   v.ID,
			"validatorName": v.Name,
			"messageType":   "final_proposal",
			"content":       message.Content,
			"proposal":      finalProposalData.Subtasks,
			"messageId":     message.MessageID,
			"blockHeight":   block.Height,
			"round":         FinalProposalRound,
			"timestamp":     time.Now(),
		})

		// Add delay between validators
		time.Sleep(500 * time.Millisecond)
	}

	// Add delay before moving to final decision
	time.Sleep(2 * time.Second)

	// PHASE 3: Final Decision Making using coordinator agent
	log.Printf("Beginning final decision making using coordinator agent with %s strategy", selectedStrategy.Name)

	// Convert final proposals to array for coordinator
	var allProposals []TaskBreakdownProposal
	for _, proposal := range finalProposals {
		allProposals = append(allProposals, proposal)
	}

	// Use the coordinator agent to determine final subtasks
	finalSubtasks := coordinateDecision(allProposals, results.Discussion.Messages, selectedStrategy)

	// If the coordinator failed to produce results, fall back to consensus
	if len(finalSubtasks) == 0 {
		log.Printf("Coordinator produced no results, falling back to consensus")
		finalSubtasks = extractConsensusProposal(results.Discussion)
	}

	// Calculate consensus score based on agreement with final decision
	consensusScore := calculateConsensusScore(results.Discussion, finalSubtasks)
	results.ConsensusScore = consensusScore

	// Generate a summary message from a validator
	taskValidators := convertValidators(validators)
	summaryMessage := generateFinalSummary(results, taskValidators)
	results.Discussion.Messages = append(results.Discussion.Messages, summaryMessage)

	// Set final subtasks in results
	results.FinalSubtasks = finalSubtasks

	// Broadcast final subtasks
	communication.BroadcastEvent(communication.EventTaskBreakdownCompleted, map[string]interface{}{
		"subtasks":         results.FinalSubtasks,
		"consensusScore":   results.ConsensusScore,
		"decisionStrategy": selectedStrategy.Name,
		"blockHeight":      block.Height,
		"summary":          summaryMessage.Content,
		"timestamp":        time.Now(),
	})

	// Update validator memories with the outcome
	for _, v := range validators {
		if v.Memory != nil {
			// Find this validator's proposal
			var myProposal string
			for _, msg := range results.Discussion.Messages {
				if msg.ValidatorID == v.ID && msg.MessageType == "proposal" {
					myProposal = msg.Content
					break
				}
			}

			v.Memory.RecordTaskBreakdown(
				block.Hash(),
				finalSubtasks,
				myProposal,
				finalSubtasks,
				selectedStrategy.Name,
				"completed",
			)

			// Record decision for reinforcement learning
			// If validator's proposed tasks were included in the final list, reward them
			reward := 0.0
			myLastProposal := []string{}
			for i := len(results.Discussion.Messages) - 1; i >= 0; i-- {
				msg := results.Discussion.Messages[i]
				if msg.ValidatorID == v.ID && len(msg.Proposal) > 0 {
					myLastProposal = msg.Proposal
					break
				}
			}

			// Calculate overlap between validator's proposal and final subtasks
			if len(myLastProposal) > 0 {
				overlap := 0
				for _, task := range myLastProposal {
					for _, finalTask := range finalSubtasks {
						if calculateTaskSimilarity(task, finalTask) > 0.7 {
							overlap++
							break
						}
					}
				}
				reward = float64(overlap) / float64(len(myLastProposal))
			}

			v.Memory.RecordDecision(
				"task_breakdown",
				strings.Join(myLastProposal, ","),
				strings.Join(finalSubtasks, ","),
				reward,
				"Collaborative task breakdown",
			)
		}
	}

	log.Printf("\n======= TASK BREAKDOWN RESULTS =======")
	log.Printf("Consensus Score: %.2f", results.ConsensusScore)
	log.Printf("Decision Strategy: %s", selectedStrategy.Name)
	log.Printf("Final Subtasks:")
	for i, task := range results.FinalSubtasks {
		log.Printf("%d. %s", i+1, task)
	}
	log.Printf("===================================\n")

	return results
}

// generateInitialProposal creates an initial task breakdown proposal from a validator
func generateInitialProposal(v *TaskValidator, results *TaskBreakdownResults) TaskBreakdownProposal {
	prompt := fmt.Sprintf(`You are %s, with traits: %v.

You are participating in a collaborative task breakdown process. Your task is to provide an INITIAL BREAKDOWN 
of this request into clear, manageable subtasks.

The following task needs to be broken down:
%s

Block Information:
- Height: %d
- Hash: %s
- Proposer: %s
- Timestamp: %d

Please provide a comprehensive, logical breakdown that addresses all aspects of the task.
Focus on creating subtasks that are:
1. Clear and specific
2. Manageable and implementable 
3. Comprehensive (covering all aspects of the work)
4. Logically organized

Please respond with a JSON object containing:
{
  "subtasks": ["Subtask 1 description", "Subtask 2 description", ...],
  "reasoning": "Your explanation of why you chose this breakdown and your approach to analyzing the task"
}

Make sure your subtasks are detailed enough to guide implementation but not so granular that they become micromanagement.`,
		v.Name, v.Traits, results.TransactionDetails,
		results.BlockInfo.Height, results.BlockInfo.Hash(),
		results.BlockInfo.Proposer, results.BlockInfo.Timestamp)

	response := ai.GenerateLLMResponse(prompt)

	// Parse the response
	var proposalData struct {
		Subtasks  []string `json:"subtasks"`
		Reasoning string   `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &proposalData); err != nil {
		log.Printf("Error parsing initial task breakdown proposal from %s: %v", v.Name, err)
		// Fall back to a simple structure if parsing fails
		proposalData.Subtasks = []string{"Error parsing response"}
		proposalData.Reasoning = "Error parsing AI response"
	}

	return TaskBreakdownProposal{
		ValidatorID:   v.ID,
		ValidatorName: v.Name,
		Subtasks:      proposalData.Subtasks,
		Reasoning:     proposalData.Reasoning,
		Timestamp:     time.Now(),
	}
}

// generateContribution determines if and how a validator should contribute to the discussion
func generateContribution(v *TaskValidator, discussionContext string, results *TaskBreakdownResults, iteration int) (bool, struct {
	MessageType string
	Content     string
	Proposal    []string
	ReplyTo     string
}) {
	// Default return value
	contribution := struct {
		MessageType string
		Content     string
		Proposal    []string
		ReplyTo     string
	}{
		MessageType: "",
		Content:     "",
		Proposal:    nil,
		ReplyTo:     "",
	}

	// Generate a prompt for the validator based on the discussion context
	prompt := fmt.Sprintf(`
You are %s, a validator with traits: %s.

You are participating in a collaborative task breakdown discussion. Below is the context of the discussion so far:

TRANSACTION DETAILS:
%s

DISCUSSION CONTEXT:
%s

Based on your personality and the discussion so far, decide how you want to contribute. You can:

1. PROPOSE NEW IDEAS: Suggest a completely new breakdown of subtasks if you think current proposals miss important aspects
2. MERGE AND IMPROVE: Take existing proposals and combine/refine them into a better solution
3. CRITIQUE: Point out specific issues or concerns with existing proposals
4. AGREE AND ENHANCE: Support a proposal while suggesting minor improvements
5. ASK: Request clarification about specific aspects of proposals
6. SUMMARIZE: Synthesize the discussion and identify emerging consensus
7. STAY SILENT: If you feel others' contributions already cover what you would say

Consider:
- Are there valuable ideas in existing proposals that could be combined or improved?
- Do you see gaps or issues that others haven't addressed?
- Can you enhance someone else's proposal rather than starting from scratch?
- Would a completely new proposal add value, or is it better to build on existing ideas?

Respond with a JSON object:
{
  "action": "PROPOSE_NEW|MERGE_IMPROVE|CRITIQUE|AGREE_ENHANCE|ASK|SUMMARIZE|SILENT",
  "message": "Your detailed contribution",
  "replyToMessageID": "ID of message you're building upon (if applicable)",
  "subtasks": ["Include if you're proposing or refining tasks"],
  "mergedFrom": ["IDs of messages whose ideas you're incorporating (if merging)"]
}`, v.Name, strings.Join(v.Traits, ", "), results.TransactionDetails, discussionContext)

	// Get LLM response
	response := ai.GenerateLLMResponse(prompt)

	// Parse response
	var result struct {
		Action           string   `json:"action"`
		Message          string   `json:"message"`
		ReplyToMessageID string   `json:"replyToMessageID"`
		Subtasks         []string `json:"subtasks"`
		MergedFrom       []string `json:"mergedFrom"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Printf("Error parsing validator contribution: %v", err)
		return false, contribution
	}

	// Stay silent if that's the chosen action
	if result.Action == "SILENT" {
		return false, contribution
	}

	// Map action to message type
	messageTypeMap := map[string]string{
		"PROPOSE_NEW":   "proposal",
		"MERGE_IMPROVE": "refinement",
		"CRITIQUE":      "critique",
		"AGREE_ENHANCE": "agreement",
		"ASK":           "question",
		"SUMMARIZE":     "summary",
	}

	contribution.MessageType = messageTypeMap[result.Action]
	contribution.Content = result.Message
	contribution.ReplyTo = result.ReplyToMessageID
	contribution.Proposal = result.Subtasks

	// If this is a merged proposal, include reference to source messages in content
	if len(result.MergedFrom) > 0 {
		mergeInfo := "\n\nThis proposal merges and improves ideas from messages: " + strings.Join(result.MergedFrom, ", ")
		contribution.Content += mergeInfo
	}

	return true, contribution
}

// formatDiscussionContext creates a readable context of the discussion so far
func formatDiscussionContext(discussion TaskDiscussion) string {
	var result strings.Builder

	result.WriteString("--- DISCUSSION HISTORY ---\n\n")

	for i, msg := range discussion.Messages {
		result.WriteString(fmt.Sprintf("[%s] %s (%s):\n%s\n\n",
			msg.Timestamp.Format("15:04:05"),
			msg.ValidatorName,
			msg.MessageType,
			msg.Content))

		// If this message has a proposal, format it clearly
		if len(msg.Proposal) > 0 {
			result.WriteString("Proposed subtasks:\n")
			for j, subtask := range msg.Proposal {
				result.WriteString(fmt.Sprintf("%d. %s\n", j+1, subtask))
			}
			result.WriteString("\n")
		}

		// To avoid context getting too long, only include the last 15 messages
		if i >= len(discussion.Messages)-15 {
			break
		}
	}

	return result.String()
}

// extractConsensusProposal analyzes the discussion to extract the final proposal
func extractConsensusProposal(discussion TaskDiscussion) []string {
	// First try to find a summary message from the validator who proposed the chosen strategy
	var finalSummary *DiscussionMessage
	for i := len(discussion.Messages) - 1; i >= 0; i-- {
		msg := discussion.Messages[i]
		if msg.MessageType == "summary" && len(msg.Proposal) > 0 {
			finalSummary = &msg
			break
		}
	}

	// If we have a summary with a proposal, use that
	if finalSummary != nil {
		return finalSummary.Proposal
	}

	// Otherwise, find the last proposal from any validator
	for i := len(discussion.Messages) - 1; i >= 0; i-- {
		if len(discussion.Messages[i].Proposal) > 0 {
			return discussion.Messages[i].Proposal
		}
	}

	// If no proposals found, return empty list
	return []string{}
}

// calculateConsensusScore measures how much agreement exists for the final proposal
func calculateConsensusScore(discussion TaskDiscussion, finalSubtasks []string) float64 {
	if len(finalSubtasks) == 0 {
		return 0.0
	}

	// Find messages that express agreement with the final proposal
	agreements := 0
	totalResponses := 0

	for _, msg := range discussion.Messages {
		if msg.MessageType == "agreement" {
			agreements++
		}
		totalResponses++
	}

	if totalResponses == 0 {
		return 0.0
	}

	return float64(agreements) / float64(totalResponses)
}

// generateFinalSummary creates a final summary message of the discussion outcome
func generateFinalSummary(results *TaskBreakdownResults, validators []*TaskValidator) DiscussionMessage {
	// Select a validator with leadership traits to summarize
	var summarizer *TaskValidator

	for _, v := range validators {
		// Look for leadership traits
		for _, trait := range v.Traits {
			if strings.Contains(strings.ToLower(trait), "leader") ||
				strings.Contains(strings.ToLower(trait), "organiz") ||
				strings.Contains(strings.ToLower(trait), "systemat") {
				summarizer = v
				break
			}
		}
		if summarizer != nil {
			break
		}
	}

	// If no leader found, pick the first validator
	if summarizer == nil && len(validators) > 0 {
		summarizer = validators[0]
	}

	// Create summary content
	subtasksFormatted := formatSubtasksList(results.FinalSubtasks)
	summaryContent := fmt.Sprintf(`
I'd like to summarize our discussion on task breakdown. After our collaborative analysis, we've reached a consensus (score: %.2f) on the following subtasks:

%s

This breakdown represents our collective wisdom and addresses the key components of the task at hand. Thank you all for your contributions to this discussion.
`, results.ConsensusScore, subtasksFormatted)

	// Create message
	return DiscussionMessage{
		ValidatorID:   summarizer.ID,
		ValidatorName: summarizer.Name,
		MessageType:   "summary",
		Content:       summaryContent,
		Proposal:      results.FinalSubtasks,
		MessageID:     uuid.New().String(),
		Timestamp:     time.Now(),
	}
}

// coordinateTaskDelegation uses the coordinator agent to make final task assignments
func coordinateTaskDelegation(proposals []TaskDelegationProposal, discussions []TaskDelegationMessage, selectedStrategy *DecisionStrategy) map[string]string {
	if selectedStrategy == nil || len(proposals) == 0 {
		return make(map[string]string)
	}

	// Get all validators
	validators := GetAllValidators(selectedStrategy.ValidatorID)

	switch strings.ToUpper(selectedStrategy.Name) {
	case "CONSENSUS", "COLLABORATIVE":
		// ROUND 1: Final Proposals
		var finalProposals []TaskDelegationProposal

		// Format existing proposals for context
		var proposalsContext strings.Builder
		for i, p := range proposals {
			proposalsContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
			proposalsContext.WriteString("Assignments:\n")
			for subtask, assignee := range p.Assignments {
				proposalsContext.WriteString(fmt.Sprintf("- %s -> %s\n", subtask, assignee))
			}
			proposalsContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
		}

		// Each validator creates their final proposal
		for _, v := range validators {
			prompt := fmt.Sprintf(`You are %s, with traits: %v.
			Based on all previous delegation proposals and discussions:
			%s

			Create your FINAL proposal for task delegation. Consider:
			1. The strengths of each existing proposal
			2. The feedback and discussions
			3. Each validator's expertise and traits
			4. Task dependencies and efficiency

			Respond with a JSON object:
			{
				"assignments": {"subtask1": "validator name", "subtask2": "validator name", ...},
				"reasoning": "Explain why this is the optimal delegation"
			}`, v.Name, v.Traits, proposalsContext.String())

			response := ai.GenerateLLMResponse(prompt)

			var proposalData struct {
				Assignments map[string]string `json:"assignments"`
				Reasoning   string            `json:"reasoning"`
			}

			if err := json.Unmarshal([]byte(response), &proposalData); err != nil {
				log.Printf("Error parsing final delegation proposal from %s: %v", v.Name, err)
				continue
			}

			finalProposal := TaskDelegationProposal{
				ValidatorID:   v.ID,
				ValidatorName: v.Name,
				Assignments:   proposalData.Assignments,
				Reasoning:     proposalData.Reasoning,
				Timestamp:     time.Now(),
			}

			finalProposals = append(finalProposals, finalProposal)

			// Broadcast final proposal
			communication.BroadcastEvent(communication.EventTaskDelegationMessage, map[string]interface{}{
				"validatorId":   v.ID,
				"validatorName": v.Name,
				"messageType":   "final_proposal",
				"content":       proposalData.Reasoning,
				"assignments":   proposalData.Assignments,
				"timestamp":     time.Now(),
			})
		}

		// ROUND 2: Voting
		type DelegationVote struct {
			ProposalIndex int
			Score         float64 // 0.0 to 1.0
			Reasoning     string
		}

		proposalVotes := make(map[int][]DelegationVote)

		// Each validator votes on all final proposals
		for _, v := range validators {
			var votingContext strings.Builder
			for i, p := range finalProposals {
				votingContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
				votingContext.WriteString("Assignments:\n")
				for subtask, assignee := range p.Assignments {
					votingContext.WriteString(fmt.Sprintf("- %s -> %s\n", subtask, assignee))
				}
				votingContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
			}

			prompt := fmt.Sprintf(`You are %s, with traits: %v.
			Review these FINAL task delegation proposals:
			%s

			Vote on EACH proposal with:
			1. A score from 0.0 to 1.0 (where 1.0 means full support)
			2. Brief reasoning for your score

			Consider:
			- Appropriate matching of skills to tasks
			- Workload balance
			- Task dependencies
			- Overall efficiency

			Respond with a JSON array of votes:
			{
				"votes": [
					{"proposalIndex": 1, "score": 0.8, "reasoning": "Well-balanced distribution..."},
					{"proposalIndex": 2, "score": 0.4, "reasoning": "Suboptimal skill matching..."},
					...
				]
			}`, v.Name, v.Traits, votingContext.String())

			response := ai.GenerateLLMResponse(prompt)

			var result struct {
				Votes []DelegationVote `json:"votes"`
			}

			if err := json.Unmarshal([]byte(response), &result); err != nil {
				log.Printf("Error parsing delegation votes from %s: %v", v.Name, err)
				continue
			}

			// Record votes
			for _, vote := range result.Votes {
				proposalVotes[vote.ProposalIndex] = append(proposalVotes[vote.ProposalIndex], vote)

				// Broadcast vote
				communication.BroadcastEvent(EventTaskDelegationVote, map[string]interface{}{
					"validatorId":   v.ID,
					"validatorName": v.Name,
					"proposalIndex": vote.ProposalIndex,
					"score":         vote.Score,
					"reasoning":     vote.Reasoning,
					"timestamp":     time.Now(),
				})
			}
		}

		// Calculate average scores and find winning proposal
		var highestScore float64
		var winningIndex int

		for idx, votes := range proposalVotes {
			if len(votes) == 0 {
				continue
			}

			total := 0.0
			for _, vote := range votes {
				total += vote.Score
			}
			avgScore := total / float64(len(votes))

			if avgScore > highestScore {
				highestScore = avgScore
				winningIndex = idx
			}
		}

		// Return winning proposal's assignments
		if winningIndex > 0 && winningIndex <= len(finalProposals) {
			return finalProposals[winningIndex-1].Assignments
		}

		// Fallback to first proposal if no clear winner
		if len(finalProposals) > 0 {
			return finalProposals[0].Assignments
		}
	}

	// Fallback to first proposal if strategy not handled
	if len(proposals) > 0 {
		return proposals[0].Assignments
	}

	return make(map[string]string)
}

// StartCollaborativeTaskDelegation starts the collaborative task delegation process
func StartCollaborativeTaskDelegation(chainID string, taskBreakdown *TaskBreakdownResults) *TaskDelegationResults {
	if taskBreakdown == nil || len(taskBreakdown.FinalSubtasks) == 0 {
		log.Printf("Cannot start task delegation with empty subtasks")
		return nil
	}

	log.Printf("Starting collaborative task delegation for %d subtasks", len(taskBreakdown.FinalSubtasks))

	// Initialize results
	results := &TaskDelegationResults{
		Assignments: make(map[string]string),
		BlockInfo:   taskBreakdown.BlockInfo,
		Subtasks:    taskBreakdown.FinalSubtasks,
		Discussion:  TaskDelegationDiscussion{Messages: []TaskDelegationMessage{}},
		Strategy:    taskBreakdown.SelectedStrategy,
	}

	// Get validators for this chain
	validators := GetAllValidators(chainID)
	if len(validators) == 0 {
		log.Printf("No validators found for chain %s", chainID)
		return results
	}

	log.Printf("Found %d validators for task delegation", len(validators))

	// Create a communication thread for this delegation session
	threadID := fmt.Sprintf("task-delegation-%s", taskBreakdown.BlockInfo.Hash())
	log.Printf("Created delegation thread with ID: %s", threadID)

	// Broadcast start of task delegation
	communication.BroadcastEvent(communication.EventTaskDelegationStarted, map[string]interface{}{
		"blockHeight": taskBreakdown.BlockInfo.Height,
		"threadId":    threadID,
		"subtasks":    taskBreakdown.FinalSubtasks,
		"strategy":    results.Strategy.Name,
		"timestamp":   time.Now(),
	})

	// PHASE 1: Initial Delegation Proposals with Chain of Thought
	log.Printf("Beginning initial delegation proposals with chain of thought reasoning")

	// Update validators' memory with current task
	for _, v := range validators {
		if v.Memory != nil {
			v.Memory.SetCurrentBlock(taskBreakdown.BlockInfo)
			v.Memory.SetCurrentTaskBreakdown(taskBreakdown)
		}
	}

	// Collect delegation proposals, using chain of thought reasoning
	var delegationProposals []TaskDelegationProposal

	// Build context for delegation
	subtasksContext := formatSubtasksList(taskBreakdown.FinalSubtasks)
	breakdownStrategy := taskBreakdown.SelectedStrategy.Name

	// Each validator proposes task assignments with reasoning
	for _, v := range validators {
		log.Printf("🤔 [%s] Generating task delegation proposal...", v.Name)

		// Get historical context for this validator
		var historicalContext string
		if v.Memory != nil {
			// Get relevant validators to include in context
			relevantValidators := make([]string, 0, len(validators))
			for _, other := range validators {
				relevantValidators = append(relevantValidators, other.ID)
			}

			historicalContext = v.Memory.GetHistoricalContext(relevantValidators, "tasks")
		}

		// Prepare validator expertise mapping
		var validatorExpertise strings.Builder
		validatorExpertise.WriteString("Validator expertise information:\n")
		for _, validator := range validators {
			validatorExpertise.WriteString(fmt.Sprintf("- %s: Traits: %s\n",
				validator.Name, strings.Join(validator.Traits, ", ")))
		}

		// Generate delegation proposal with chain of thought reasoning
		delegationPrompt := fmt.Sprintf(
			"Genesis Context: %s\n\n"+
				"You are %s, a blockchain validator with these traits: %s.\n"+
				"Task: Delegate %d subtasks from Block %d to the available validators\n\n"+
				"Subtasks to delegate:\n%s\n\n"+
				"%s\n\n"+
				"Historical Context:\n%s\n\n"+
				"Task breakdown was done using the '%s' strategy.\n\n"+
				"I want you to think step by step about the optimal task delegation. Consider:\n\n"+
				"1. Each validator's expertise based on their traits\n"+
				"2. Your past experiences with these validators\n"+
				"3. The nature of each subtask and which skills it requires\n"+
				"4. Potential dependencies between subtasks\n"+
				"5. How to optimize for successful completion\n\n"+
				"After your chain of thought reasoning, respond with a JSON object containing:\n"+
				"{\n"+
				"  \"assignments\": {\"subtask1\": \"validator name\", \"subtask2\": \"validator name\", ...},\n"+
				"  \"reasoning\": \"Your complete chain of thought reasoning process\"\n"+
				"}",
			v.GenesisPrompt, v.Name, strings.Join(v.Traits, ", "),
			len(taskBreakdown.FinalSubtasks), taskBreakdown.BlockInfo.Height,
			subtasksContext, validatorExpertise.String(), historicalContext, breakdownStrategy,
		)

		// Get delegation proposal through LLM
		response := ai.GenerateLLMResponse(delegationPrompt)

		// Parse the response
		var result struct {
			Assignments map[string]string `json:"assignments"`
			Reasoning   string            `json:"reasoning"`
		}

		if err := json.Unmarshal([]byte(response), &result); err != nil {
			log.Printf("Error parsing delegation proposal from %s: %v", v.Name, err)
			continue
		}

		// Create formal proposal
		proposal := TaskDelegationProposal{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			Assignments:   result.Assignments,
			Reasoning:     result.Reasoning,
			Timestamp:     time.Now(),
		}

		delegationProposals = append(delegationProposals, proposal)

		// Create discussion message
		message := TaskDelegationMessage{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			MessageType:   "proposal",
			Content:       result.Reasoning,
			Assignments:   result.Assignments,
			MessageID:     uuid.New().String(),
			Timestamp:     time.Now(),
		}

		results.Discussion.Messages = append(results.Discussion.Messages, message)

		// Broadcast proposal
		communication.BroadcastEvent(communication.EventTaskDelegationMessage, map[string]interface{}{
			"validatorId":   v.ID,
			"validatorName": v.Name,
			"messageType":   "proposal",
			"content":       truncateString(result.Reasoning, 500),
			"assignments":   result.Assignments,
			"messageId":     message.MessageID,
			"blockHeight":   taskBreakdown.BlockInfo.Height,
			"timestamp":     time.Now(),
		})

		// Store in validator's memory
		if v.Memory != nil {
			v.Memory.StoreDiscussion(DiscussionMessage{
				ValidatorID:   v.ID,
				ValidatorName: v.Name,
				MessageType:   "delegation_proposal",
				Content:       result.Reasoning,
				MessageID:     message.MessageID,
				Timestamp:     time.Now(),
			})
		}

		// Short delay between validators
		time.Sleep(100 * time.Millisecond)
	}

	// PHASE 2: Discussion and Refinement
	log.Printf("Beginning delegation discussion and refinement")

	// Discussion round for validators to comment on each other's proposals
	discussionRounds := 2

	for round := 1; round <= discussionRounds; round++ {
		log.Printf("Starting delegation discussion round %d", round)

		for _, v := range validators {
			// Build context from all proposals and discussions so far
			var discussionContext strings.Builder
			discussionContext.WriteString("Current delegation proposals and discussions:\n\n")

			for _, proposal := range delegationProposals {
				discussionContext.WriteString(fmt.Sprintf("From %s:\n", proposal.ValidatorName))
				discussionContext.WriteString(fmt.Sprintf("Reasoning: %s\n", proposal.Reasoning))
				for subtask, assignee := range proposal.Assignments {
					discussionContext.WriteString(fmt.Sprintf("- %s -> %s\n", subtask, assignee))
				}
				discussionContext.WriteString("\n")
			}

			var prompt string
			if round == 1 {
				// First round: Initial reactions and suggestions
				prompt = fmt.Sprintf(
					"You are %s, reviewing task delegation proposals.\n\n"+
						"The subtasks are:\n%s\n\n"+
						"%s\n\n"+
						"Based on your expertise as %s and your traits (%s), analyze these proposals.\n"+
						"Consider:\n"+
						"1. Which assignments make sense and why?\n"+
						"2. What potential issues do you see?\n"+
						"3. What alternative assignments might work better?\n\n"+
						"Respond with a JSON object:\n"+
						"{\n"+
						"  \"messageType\": \"critique\" or \"support\" or \"question\",\n"+
						"  \"content\": \"Your detailed analysis\",\n"+
						"  \"refinedAssignments\": {optional map of subtask to validator, only include if suggesting specific changes}\n"+
						"}",
					v.Name, subtasksContext, discussionContext.String(),
					v.Name, strings.Join(v.Traits, ", "),
				)
			} else {
				// Second round: Focus on merging and improving ideas
				prompt = fmt.Sprintf(
					"You are %s, participating in the final round of task delegation discussion.\n\n"+
						"The subtasks are:\n%s\n\n"+
						"%s\n\n"+
						"This is the final round. Your goal is to help reach the best possible assignments.\n"+
						"Consider:\n"+
						"1. Can you combine good ideas from different proposals?\n"+
						"2. Are there any remaining issues that need to be addressed?\n"+
						"3. What would be the optimal final assignments based on all discussion?\n\n"+
						"If you see opportunity to improve assignments, propose a refined version.\n"+
						"If you think current assignments are optimal, express your support.\n\n"+
						"Respond with a JSON object:\n"+
						"{\n"+
						"  \"messageType\": \"merge\" or \"refine\" or \"support\",\n"+
						"  \"content\": \"Your detailed contribution explaining your thinking\",\n"+
						"  \"refinedAssignments\": {map of subtask to validator, include if proposing merged/refined assignments}\n"+
						"}",
					v.Name, subtasksContext, discussionContext.String(),
					v.Name, strings.Join(v.Traits, ", "),
				)
			}

			response := ai.GenerateLLMResponse(prompt)

			// Parse response
			var feedback struct {
				MessageType        string            `json:"messageType"`
				Content            string            `json:"content"`
				RefinedAssignments map[string]string `json:"refinedAssignments"`
			}

			if err := json.Unmarshal([]byte(response), &feedback); err != nil {
				log.Printf("Error parsing feedback from %s: %v", v.Name, err)
				continue
			}

			// Create discussion message
			message := TaskDelegationMessage{
				ValidatorID:   v.ID,
				ValidatorName: v.Name,
				MessageType:   feedback.MessageType,
				Content:       feedback.Content,
				Assignments:   feedback.RefinedAssignments,
				MessageID:     uuid.New().String(),
				Timestamp:     time.Now(),
			}

			results.Discussion.Messages = append(results.Discussion.Messages, message)

			// Broadcast message
			communication.BroadcastEvent(communication.EventTaskDelegationMessage, map[string]interface{}{
				"validatorId":   v.ID,
				"validatorName": v.Name,
				"messageType":   feedback.MessageType,
				"content":       feedback.Content,
				"assignments":   feedback.RefinedAssignments,
				"messageId":     message.MessageID,
				"blockHeight":   taskBreakdown.BlockInfo.Height,
				"timestamp":     time.Now(),
			})

			log.Printf("💬 [%s] provided %s contribution in round %d", v.Name, feedback.MessageType, round)

			// If validator proposes merged/refined assignments, add to proposals
			if len(feedback.RefinedAssignments) > 0 {
				refinedProposal := TaskDelegationProposal{
					ValidatorID:   v.ID,
					ValidatorName: v.Name,
					Assignments:   feedback.RefinedAssignments,
					Reasoning:     feedback.Content,
					Timestamp:     time.Now(),
				}
				delegationProposals = append(delegationProposals, refinedProposal)
			}

			// Small delay between validators
			time.Sleep(50 * time.Millisecond)
		}

		// Short pause between rounds
		time.Sleep(100 * time.Millisecond)
	}

	// PHASE 3: Final Decision Making using coordinator agent
	log.Printf("Making final delegation decisions using coordinator agent with %s strategy", taskBreakdown.SelectedStrategy.Name)

	// Use the coordinator agent to determine final assignments
	finalAssignments := coordinateTaskDelegation(delegationProposals, results.Discussion.Messages, taskBreakdown.SelectedStrategy)

	// Set final assignments in results
	results.Assignments = finalAssignments
	results.Strategy = taskBreakdown.SelectedStrategy

	// Create final summary message
	var summary strings.Builder
	summary.WriteString("Task delegation complete. Final assignments:\n\n")

	for subtask, assignee := range finalAssignments {
		summary.WriteString(fmt.Sprintf("• %s → %s\n", subtask, assignee))
	}

	// Add summary message to discussion
	summaryMessage := TaskDelegationMessage{
		ValidatorID:   "system",
		ValidatorName: "System",
		MessageType:   "summary",
		Content:       summary.String(),
		Assignments:   finalAssignments,
		MessageID:     uuid.New().String(),
		Timestamp:     time.Now(),
	}

	results.Discussion.Messages = append(results.Discussion.Messages, summaryMessage)

	// Broadcast completion
	communication.BroadcastEvent(communication.EventTaskDelegationCompleted, map[string]interface{}{
		"assignments": results.Assignments,
		"summary":     summary.String(),
		"blockHeight": taskBreakdown.BlockInfo.Height,
		"strategy":    results.Strategy.Name,
		"timestamp":   time.Now(),
	})

	// Update validator memories with the outcome
	for _, v := range validators {
		if v.Memory != nil {
			// Update memory with task delegation
			v.Memory.SetCurrentTaskDelegation(results)

			// Record decisions for reinforcement learning
			// Calculate reward based on how many of validator's suggestions were used
			reward := 0.0
			for _, proposal := range delegationProposals {
				if proposal.ValidatorID == v.ID {
					// Count how many of this validator's assignments were used
					matches := 0
					for subtask, assignee := range proposal.Assignments {
						if finalAssignee, ok := finalAssignments[subtask]; ok && finalAssignee == assignee {
							matches++
						}
					}

					if len(proposal.Assignments) > 0 {
						reward = float64(matches) / float64(len(proposal.Assignments))
					}
					break
				}
			}

			// Record the decision
			v.Memory.RecordDecision(
				"task_delegation",
				fmt.Sprintf("delegate-%s", taskBreakdown.BlockInfo.Hash()),
				"delegation_complete",
				reward,
				fmt.Sprintf("Task delegation for block %d", taskBreakdown.BlockInfo.Height),
			)

			// Update relationships with assigned validators
			for subtask, assignee := range finalAssignments {
				// Find the validator ID for this assignee name
				var assigneeID string
				for _, other := range validators {
					if other.Name == assignee {
						assigneeID = other.ID
						break
					}
				}

				if assigneeID != "" && assigneeID != v.ID {
					// Small positive relationship impact for delegating task
					v.Memory.UpdateRelationship(
						assigneeID,
						"task_delegation",
						fmt.Sprintf("Delegated subtask: %s", subtask),
						0.05,
					)
				}
			}
		}
	}

	// Visual summary of assignments grouped by validator
	log.Printf("\n======= FINAL TASK DELEGATION RESULTS =======")

	// Group tasks by validator for a cleaner summary
	validatorTasks := make(map[string][]string)
	for subtask, assignee := range finalAssignments {
		validatorTasks[assignee] = append(validatorTasks[assignee], subtask)
	}

	// Display tasks organized by validator
	for validator, tasks := range validatorTasks {
		log.Printf("\n👤 %s will handle:", validator)
		for i, task := range tasks {
			log.Printf("  %d. %s", i+1, task)
		}
	}

	log.Printf("\n=== Total: %d subtasks assigned to %d validators ===\n",
		len(finalAssignments), len(validatorTasks))

	return results
}

// Helper function to format a list of subtasks for prompts
func formatSubtasksList(subtasks []string) string {
	var result strings.Builder
	for i, task := range subtasks {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, task))
	}
	return result.String()
}

// NotifyAssignedValidators notifies validators of their assigned tasks
func NotifyAssignedValidators(chainID string, delegationResults *TaskDelegationResults) {
	if delegationResults == nil || len(delegationResults.Assignments) == 0 {
		log.Printf("No assignments to notify validators about")
		return
	}

	log.Printf("======= STARTING VALIDATOR TASK NOTIFICATIONS =======")
	log.Printf("Chain ID: %s", chainID)
	log.Printf("Block Height: %d", delegationResults.BlockInfo.Height)
	log.Printf("Block Hash: %s", delegationResults.BlockInfo.Hash())
	log.Printf("Total Assignments: %d", len(delegationResults.Assignments))
	log.Printf("---------------------------------------------------")

	// Get tempporary hardcoded validators
	validators := []*TaskValidator{
		{
			ID:     "validator-1",
			Name:   "Validator 1",
			Traits: []string{"technical", "detail-oriented", "problem-solver"},
		},
		{
			ID:     "validator-2",
			Name:   "Validator 2",
			Traits: []string{"creative", "big-picture thinker", "strategist"},
		},
		{
			ID:     "validator-3",
			Name:   "Validator 3",
			Traits: []string{"organized", "leadership", "communicator"},
		},
	}
	log.Printf("Found %d validators for this chain", len(validators))

	validatorMap := make(map[string]*TaskValidator)
	for _, v := range validators {
		validatorMap[v.Name] = v
		log.Printf("Validator mapped: %s (ID: %s)", v.Name, v.ID)
	}

	// Group tasks by validator
	validatorTasks := make(map[string][]string)
	log.Printf("Assignment details:")
	for subtask, validatorName := range delegationResults.Assignments {
		validatorTasks[validatorName] = append(validatorTasks[validatorName], subtask)
		log.Printf("- Subtask: \"%s\" → Assigned to: %s", subtask, validatorName)
	}
	log.Printf("---------------------------------------------------")

	// Notify each validator of their assigned tasks
	log.Printf("Sending notifications to validators:")
	for validatorName, tasks := range validatorTasks {
		validator, exists := validatorMap[validatorName]
		if !exists {
			log.Printf("❌ ERROR: Cannot notify validator %s: not found in validator map", validatorName)
			continue
		}

		log.Printf("🔔 Notifying validator: %s (ID: %s)", validatorName, validator.ID)
		log.Printf("  Assigned tasks (%d):", len(tasks))
		for i, task := range tasks {
			log.Printf("  %d. %s", i+1, task)
		}

		// Create task notification payload
		taskNotification := map[string]interface{}{
			"validatorId":   validator.ID,
			"validatorName": validator.Name,
			"subtasks":      tasks,
			"blockHeight":   delegationResults.BlockInfo.Height,
			"blockHash":     delegationResults.BlockInfo.Hash(),
			"timestamp":     time.Now(),
		}

		// Broadcast task assignment event
		communication.BroadcastEvent(communication.EventTaskAssignment, taskNotification)
		log.Printf("  ✅ Assignment notification sent successfully via EventTaskAssignment")
	}

	log.Printf("======= VALIDATOR TASK NOTIFICATIONS COMPLETE =======")
}

// Using TaskValidator struct for compatibility with existing functions
func validatorToTaskValidator(v *Validator) *TaskValidator {
	return &TaskValidator{
		ID:     v.ID,
		Name:   v.Name,
		Traits: v.Traits,
	}
}

// Converts a slice of Validators to TaskValidators
func convertValidators(validators []*Validator) []*TaskValidator {
	taskValidators := make([]*TaskValidator, len(validators))
	for i, v := range validators {
		taskValidators[i] = validatorToTaskValidator(v)
	}
	return taskValidators
}

// normalizeTask converts a task description to lowercase and removes extra whitespace
func normalizeTask(task string) string {
	return strings.TrimSpace(strings.ToLower(task))
}

// calculateTaskSimilarity returns a similarity score between 0 and 1 for two tasks
func calculateTaskSimilarity(task1, task2 string) float64 {
	// Normalize both tasks
	norm1 := normalizeTask(task1)
	norm2 := normalizeTask(task2)

	// If exact match, return 1.0
	if norm1 == norm2 {
		return 1.0
	}

	// Use simple string comparison - if they share a prefix
	minLen := math.Min(float64(len(norm1)), float64(len(norm2)))
	if minLen == 0 {
		return 0.0
	}

	// Count matching characters from start
	matchingChars := 0
	for i := 0; i < int(minLen); i++ {
		if norm1[i] != norm2[i] {
			break
		}
		matchingChars++
	}

	// Return similarity score based on matching prefix length
	maxLen := math.Max(float64(len(norm1)), float64(len(norm2)))
	return float64(matchingChars) / maxLen
}

// generateStrategyProposal creates a new decision strategy proposal from a validator
func generateStrategyProposal(v *Validator, results *TaskBreakdownResults) *DecisionStrategy {
	// Define the three available strategies
	strategies := []struct {
		Name        string
		Description string
		BestFor     string
	}{
		{
			Name:        "CONSENSUS",
			Description: "All validators have equal voting power. Each validator reviews and votes on proposals. The proposal with the highest average score wins.",
			BestFor:     "Tasks that benefit from collective wisdom and require broad agreement.",
		},
		{
			Name:        "LEADER",
			Description: "A validator with strong leadership traits guides the decision process. Other validators provide input, but the leader makes the final decision.",
			BestFor:     "Complex tasks needing clear direction and quick decisions.",
		},
		{
			Name:        "AUCTION",
			Description: "Validators bid on tasks based on their expertise and capacity. Tasks are assigned to those best positioned to complete them.",
			BestFor:     "Tasks where specific expertise and resource availability are crucial.",
		},
	}

	// Generate prompt for strategy selection
	prompt := fmt.Sprintf(`You are %s, with traits: %v.
	You need to select a decision-making strategy for this task:
	%s

	Available strategies:

	1. CONSENSUS:
	   - %s
	   - Best for: %s

	2. LEADER:
	   - %s
	   - Best for: %s

	3. AUCTION:
	   - %s
	   - Best for: %s

	Based on:
	1. Your traits and past experience
	2. The nature and complexity of the current task
	3. The need for efficient decision-making
	4. The importance of validator participation

	Choose ONE of these three strategies.

	Respond with a JSON object:
	{
		"selectedStrategy": "REQUIRED: One of: CONSENSUS | LEADER | AUCTION",
		"reasoning": "REQUIRED: Why this strategy is most appropriate for this task"
	}`, v.Name, v.Traits, results.TransactionDetails,
		strategies[0].Description, strategies[0].BestFor,
		strategies[1].Description, strategies[1].BestFor,
		strategies[2].Description, strategies[2].BestFor)

	response := ai.GenerateLLMResponse(prompt)

	var proposalData struct {
		SelectedStrategy string `json:"selectedStrategy"`
		Reasoning        string `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(response), &proposalData); err != nil {
		log.Printf("Error parsing strategy proposal from %s: %v", v.Name, err)
		return nil
	}

	// Validate selected strategy
	validStrategy := false
	var selectedStrategyDesc string
	for _, s := range strategies {
		if strings.ToUpper(proposalData.SelectedStrategy) == s.Name {
			validStrategy = true
			selectedStrategyDesc = s.Description
			break
		}
	}

	if !validStrategy {
		log.Printf("Invalid strategy selected by %s: %s", v.Name, proposalData.SelectedStrategy)
		// Default to consensus if invalid strategy selected
		proposalData.SelectedStrategy = "CONSENSUS"
		selectedStrategyDesc = strategies[0].Description
		proposalData.Reasoning += " (Defaulted to consensus due to invalid selection)"
	}

	// Create the strategy
	strategy := &DecisionStrategy{
		ValidatorID:   v.ID,
		ValidatorName: v.Name,
		Name:          proposalData.SelectedStrategy,
		Description:   selectedStrategyDesc,
		Reasoning:     proposalData.Reasoning,
		Timestamp:     time.Now(),
	}

	return strategy
}

// collectConsensusVotes collects votes from all validators on all proposals
func collectConsensusVotes(validators []*Validator, proposals []TaskBreakdownProposal) []ProposalVote {
	var votes []ProposalVote

	for _, v := range validators {
		// Format proposals for voting
		var proposalsContext strings.Builder
		for i, p := range proposals {
			proposalsContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
			for j, task := range p.Subtasks {
				proposalsContext.WriteString(fmt.Sprintf("%d.%d. %s\n", i+1, j+1, task))
			}
			proposalsContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
		}

		prompt := fmt.Sprintf(`You are %s, with traits: %v.
		Review these task breakdown proposals:
		%s

		For each proposal, provide:
		1. A score from 0.0 to 1.0 (where 1.0 means full support)
		2. Brief reasoning for your score

		Consider:
		- Clarity and completeness of subtasks
		- Feasibility of implementation
		- Coverage of requirements
		- Logical organization

		Respond with a JSON array of votes:
		{
			"votes": [
				{"proposalIndex": 1, "score": 0.8, "reasoning": "Clear and comprehensive..."},
				{"proposalIndex": 2, "score": 0.4, "reasoning": "Missing key aspects..."},
				...
			]
		}`, v.Name, v.Traits, proposalsContext.String())

		response := ai.GenerateLLMResponse(prompt)

		var result struct {
			Votes []struct {
				ProposalIndex int     `json:"proposalIndex"`
				Score         float64 `json:"score"`
				Reasoning     string  `json:"reasoning"`
			} `json:"votes"`
		}

		if err := json.Unmarshal([]byte(response), &result); err != nil {
			log.Printf("Error parsing votes from %s: %v", v.Name, err)
			continue
		}

		// Add votes to the collection
		for _, vote := range result.Votes {
			votes = append(votes, ProposalVote{
				ValidatorID:   v.ID,
				ValidatorName: v.Name,
				ProposalIndex: vote.ProposalIndex,
				Score:         vote.Score,
				Reasoning:     vote.Reasoning,
				Timestamp:     time.Now(),
			})
		}
	}

	return votes
}

// selectProposalByConsensus selects the proposal with the highest consensus score
func selectProposalByConsensus(votes []ProposalVote, proposals []TaskBreakdownProposal) []string {
	if len(proposals) == 0 {
		return nil
	}

	// Calculate average score for each proposal
	scores := make(map[int]float64)
	voteCount := make(map[int]int)

	for _, vote := range votes {
		scores[vote.ProposalIndex] += vote.Score
		voteCount[vote.ProposalIndex]++
	}

	// Find proposal with highest average score
	var highestScore float64
	var selectedIndex int

	for idx, totalScore := range scores {
		if count := voteCount[idx]; count > 0 {
			avgScore := totalScore / float64(count)
			if avgScore > highestScore {
				highestScore = avgScore
				selectedIndex = idx
			}
		}
	}

	// Return the winning proposal's subtasks
	if selectedIndex > 0 && selectedIndex <= len(proposals) {
		return proposals[selectedIndex-1].Subtasks
	}

	return nil
}

// coordinateDecision uses a coordinator agent to facilitate decision making based on the selected strategy
func coordinateDecision(proposals []TaskBreakdownProposal, discussions []DiscussionMessage, selectedStrategy *DecisionStrategy) []string {
	log.Printf("Coordinating decision using %s strategy", selectedStrategy.Name)

	// Get all validators
	validators := GetAllValidators(selectedStrategy.ValidatorID)

	switch strings.ToUpper(selectedStrategy.Name) {
	case "CONSENSUS":
		// ROUND 1: Final Proposals
		var finalProposals []TaskBreakdownProposal
		for _, v := range validators {
			// Format previous proposals for context
			var proposalsContext strings.Builder
			for i, p := range proposals {
				proposalsContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
				for j, task := range p.Subtasks {
					proposalsContext.WriteString(fmt.Sprintf("%d.%d. %s\n", i+1, j+1, task))
				}
				proposalsContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
			}

			prompt := fmt.Sprintf(`You are %s, with traits: %v.
			Based on all previous proposals and discussions:
			%s

			Create your FINAL proposal for task breakdown. Consider:
			1. The strengths of each existing proposal
			2. The feedback and concerns raised in discussions
			3. Your own expertise and judgment

			Respond with a JSON object:
			{
				"subtasks": ["task1", "task2", ...],
				"reasoning": "Explain why this is the best breakdown"
			}`, v.Name, v.Traits, proposalsContext.String())

			response := ai.GenerateLLMResponse(prompt)

			var proposalData struct {
				Subtasks  []string `json:"subtasks"`
				Reasoning string   `json:"reasoning"`
			}

			if err := json.Unmarshal([]byte(response), &proposalData); err != nil {
				log.Printf("Error parsing final proposal from %s: %v", v.Name, err)
				continue
			}

			finalProposal := TaskBreakdownProposal{
				ValidatorID:   v.ID,
				ValidatorName: v.Name,
				Subtasks:      proposalData.Subtasks,
				Reasoning:     proposalData.Reasoning,
				Timestamp:     time.Now(),
			}

			finalProposals = append(finalProposals, finalProposal)

			// Broadcast final proposal
			communication.BroadcastEvent(communication.EventTaskBreakdownMessage, map[string]interface{}{
				"validatorId":   v.ID,
				"validatorName": v.Name,
				"messageType":   "final_proposal",
				"content":       proposalData.Reasoning,
				"proposal":      proposalData.Subtasks,
				"timestamp":     time.Now(),
			})
		}

		// ROUND 2: Voting
		type ProposalVote struct {
			ProposalIndex int
			Score         float64 // 0.0 to 1.0
			Reasoning     string
		}

		proposalVotes := make(map[int][]ProposalVote)

		for _, v := range validators {
			// Format final proposals for voting
			var votingContext strings.Builder
			for i, p := range finalProposals {
				votingContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
				for j, task := range p.Subtasks {
					votingContext.WriteString(fmt.Sprintf("%d.%d. %s\n", i+1, j+1, task))
				}
				votingContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
			}

			prompt := fmt.Sprintf(`You are %s, with traits: %v.
			Review these FINAL task breakdown proposals:
			%s

			Vote on EACH proposal with:
			1. A score from 0.0 to 1.0 (where 1.0 means full support)
			2. Brief reasoning for your score

			Consider:
			- Clarity and completeness
			- Feasibility of implementation
			- Coverage of requirements
			- Logical organization

			Respond with a JSON array of votes:
			{
				"votes": [
					{"proposalIndex": 1, "score": 0.8, "reasoning": "Clear and comprehensive..."},
					{"proposalIndex": 2, "score": 0.4, "reasoning": "Missing key aspects..."},
					...
				]
			}`, v.Name, v.Traits, votingContext.String())

			response := ai.GenerateLLMResponse(prompt)

			var result struct {
				Votes []ProposalVote `json:"votes"`
			}

			if err := json.Unmarshal([]byte(response), &result); err != nil {
				log.Printf("Error parsing votes from %s: %v", v.Name, err)
				continue
			}

			// Record votes
			for _, vote := range result.Votes {
				proposalVotes[vote.ProposalIndex] = append(proposalVotes[vote.ProposalIndex], vote)

				// Broadcast vote
				communication.BroadcastEvent(EventTaskDelegationVote, map[string]interface{}{
					"validatorId":   v.ID,
					"validatorName": v.Name,
					"proposalIndex": vote.ProposalIndex,
					"score":         vote.Score,
					"reasoning":     vote.Reasoning,
					"timestamp":     time.Now(),
				})
			}
		}

		// Calculate average scores and find winning proposal
		var highestScore float64
		var winningIndex int

		for idx, votes := range proposalVotes {
			if len(votes) == 0 {
				continue
			}

			total := 0.0
			for _, vote := range votes {
				total += vote.Score
			}
			avgScore := total / float64(len(votes))

			if avgScore > highestScore {
				highestScore = avgScore
				winningIndex = idx
			}
		}

		// Return winning proposal's subtasks
		if winningIndex > 0 && winningIndex <= len(finalProposals) {
			return finalProposals[winningIndex-1].Subtasks
		}

		// Fallback to first proposal if no clear winner
		if len(finalProposals) > 0 {
			return finalProposals[0].Subtasks
		}

	case "LEADER":
		// Find the leader (validator who proposed the strategy)
		var leader *Validator
		for _, v := range validators {
			if v.ID == selectedStrategy.ValidatorID {
				leader = v
				break
			}
		}

		if leader == nil {
			log.Printf("Leader not found, falling back to consensus")
			return extractConsensusProposal(TaskDiscussion{Messages: discussions})
		}

		// Format proposals for leader's review
		var proposalsContext strings.Builder
		for i, p := range proposals {
			proposalsContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
			for j, task := range p.Subtasks {
				proposalsContext.WriteString(fmt.Sprintf("%d.%d. %s\n", i+1, j+1, task))
			}
			proposalsContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
		}

		// Ask leader to make final decision
		prompt := fmt.Sprintf(`As the designated leader %s, review these proposals:
%s

Choose the best proposal or create a consolidated version.
Consider:
- Team alignment and buy-in
- Clear direction and coordination
- Efficient execution path

Respond with a JSON object:
{
    "selectedProposal": 1, // Index of chosen proposal, or 0 for consolidated
    "consolidatedTasks": ["task1", "task2", ...], // If creating consolidated version
    "reasoning": "Explain your decision process"
}`, leader.Name, proposalsContext.String())

		response := ai.GenerateLLMResponse(prompt)

		var result struct {
			SelectedProposal  int      `json:"selectedProposal"`
			ConsolidatedTasks []string `json:"consolidatedTasks"`
			Reasoning         string   `json:"reasoning"`
		}

		if err := json.Unmarshal([]byte(response), &result); err != nil {
			log.Printf("Error parsing leader decision: %v", err)
			return extractConsensusProposal(TaskDiscussion{Messages: discussions})
		}

		if result.SelectedProposal > 0 && result.SelectedProposal <= len(proposals) {
			return proposals[result.SelectedProposal-1].Subtasks
		} else if len(result.ConsolidatedTasks) > 0 {
			return result.ConsolidatedTasks
		}

	case "AUCTION":
		// ROUND 1: Validators bid on proposals
		type Bid struct {
			ValidatorID   string
			ValidatorName string
			ProposalIndex int
			Confidence    float64 // 0.0 to 1.0
			Expertise     float64 // 0.0 to 1.0
			Reasoning     string
		}

		var bids []Bid

		// Format proposals for bidding
		var proposalsContext strings.Builder
		for i, p := range proposals {
			proposalsContext.WriteString(fmt.Sprintf("\nProposal %d (from %s):\n", i+1, p.ValidatorName))
			for j, task := range p.Subtasks {
				proposalsContext.WriteString(fmt.Sprintf("%d.%d. %s\n", i+1, j+1, task))
			}
			proposalsContext.WriteString(fmt.Sprintf("Reasoning: %s\n", p.Reasoning))
		}

		// Each validator bids on proposals
		for _, v := range validators {
			prompt := fmt.Sprintf(`You are %s, with traits: %v.
			Review these task breakdown proposals:
			%s

			For each proposal, evaluate:
			1. Your confidence in implementing this breakdown (0.0 to 1.0)
			2. Your expertise relevant to this approach (0.0 to 1.0)
			3. Why you believe you're well-suited for this approach

			Respond with a JSON array of bids:
			{
				"bids": [
					{
						"proposalIndex": 1,
						"confidence": 0.8,
						"expertise": 0.9,
						"reasoning": "My technical expertise aligns well..."
					},
					...
				]
			}`, v.Name, v.Traits, proposalsContext.String())

			response := ai.GenerateLLMResponse(prompt)

			var result struct {
				Bids []struct {
					ProposalIndex int     `json:"proposalIndex"`
					Confidence    float64 `json:"confidence"`
					Expertise     float64 `json:"expertise"`
					Reasoning     string  `json:"reasoning"`
				} `json:"bids"`
			}

			if err := json.Unmarshal([]byte(response), &result); err != nil {
				log.Printf("Error parsing bids from %s: %v", v.Name, err)
				continue
			}

			// Record bids
			for _, bid := range result.Bids {
				bids = append(bids, Bid{
					ValidatorID:   v.ID,
					ValidatorName: v.Name,
					ProposalIndex: bid.ProposalIndex,
					Confidence:    bid.Confidence,
					Expertise:     bid.Expertise,
					Reasoning:     bid.Reasoning,
				})

				// Broadcast bid
				communication.BroadcastEvent(communication.EventTaskBreakdownMessage, map[string]interface{}{
					"validatorId":   v.ID,
					"validatorName": v.Name,
					"messageType":   "bid",
					"proposalIndex": bid.ProposalIndex,
					"confidence":    bid.Confidence,
					"expertise":     bid.Expertise,
					"reasoning":     bid.Reasoning,
					"timestamp":     time.Now(),
				})
			}
		}

		// Calculate weighted scores and find winning proposal
		scores := make(map[int]float64)
		bidCounts := make(map[int]int)

		for _, bid := range bids {
			// Weight = 0.6 * expertise + 0.4 * confidence
			weight := 0.6*bid.Expertise + 0.4*bid.Confidence
			scores[bid.ProposalIndex] += weight
			bidCounts[bid.ProposalIndex]++
		}

		var highestScore float64
		var winningIndex int

		for idx, score := range scores {
			if count := bidCounts[idx]; count > 0 {
				avgScore := score / float64(count)
				if avgScore > highestScore {
					highestScore = avgScore
					winningIndex = idx
				}
			}
		}

		// Return winning proposal's subtasks
		if winningIndex > 0 && winningIndex <= len(proposals) {
			return proposals[winningIndex-1].Subtasks
		}

		// Fallback to first proposal if no clear winner
		if len(proposals) > 0 {
			return proposals[0].Subtasks
		}
	}

	// Fallback to consensus proposal if strategy not handled
	log.Printf("Strategy %s not fully implemented, falling back to consensus", selectedStrategy.Name)
	return extractConsensusProposal(TaskDiscussion{Messages: discussions})
}

// conductStrategyVoting manages the voting process for strategy selection
func conductStrategyVoting(validators []*Validator, strategies []*DecisionStrategy, results *TaskBreakdownResults) []StrategyVote {
	var votes []StrategyVote

	// Add delay for UI to show strategy proposals first
	time.Sleep(2 * time.Second)

	for _, v := range validators {
		// Build context of all proposed strategies
		var strategyContext strings.Builder
		for _, s := range strategies {
			strategyContext.WriteString(fmt.Sprintf("\nStrategy: %s\nProposed by: %s\nDescription: %s\nReasoning: %s\n\n",
				s.Name, s.ValidatorName, s.Description, s.Reasoning))
		}

		prompt := fmt.Sprintf(`You are %s, with traits: %v.
		Review these proposed decision-making strategies:
		%s

		Based on your expertise and the task requirements:
		1. Which strategy do you think is best?
		2. Why do you support this strategy?

		Respond with a JSON object:
		{
			"selectedStrategy": "Exact name of the strategy you're voting for",
			"reasoning": "Your detailed reasoning for this choice"
		}`, v.Name, v.Traits, strategyContext.String())

		response := ai.GenerateLLMResponse(prompt)

		var voteData struct {
			SelectedStrategy string `json:"selectedStrategy"`
			Reasoning        string `json:"reasoning"`
		}

		if err := json.Unmarshal([]byte(response), &voteData); err != nil {
			log.Printf("Error parsing vote from %s: %v", v.Name, err)
			continue
		}

		vote := StrategyVote{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			StrategyName:  voteData.SelectedStrategy,
			Reasoning:     voteData.Reasoning,
			Timestamp:     time.Now(),
		}

		votes = append(votes, vote)

		// Add vote to discussion
		discussion := StrategyDiscussion{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			MessageType:   "vote",
			Content:       voteData.Reasoning,
			Timestamp:     time.Now(),
		}
		results.StrategyDiscussion = append(results.StrategyDiscussion, discussion)

		// Find the full strategy details
		var votedStrategy *DecisionStrategy
		for _, s := range strategies {
			if strings.EqualFold(s.Name, voteData.SelectedStrategy) {
				votedStrategy = s
				break
			}
		}

		// Broadcast vote with complete information
		communication.BroadcastEvent(communication.EventStrategyVote, map[string]interface{}{
			"validatorId":   v.ID,
			"validatorName": v.Name,
			"strategyName":  voteData.SelectedStrategy,
			"strategyDescription": func() string {
				if votedStrategy != nil {
					return votedStrategy.Description
				}
				return fmt.Sprintf("Vote for %s strategy", voteData.SelectedStrategy)
			}(),
			"reasoning":   fmt.Sprintf("%s's reasoning: %s", v.Name, voteData.Reasoning),
			"blockHeight": results.BlockInfo.Height,
			"timestamp":   time.Now(),
		})

		// Add small delay between votes for better UI visualization
		time.Sleep(500 * time.Millisecond)
	}

	// Add delay before final selection for UI to show all votes
	time.Sleep(2 * time.Second)

	return votes
}

// selectWinningStrategy determines the winning strategy based on votes
func selectWinningStrategy(votes []StrategyVote, strategies []*DecisionStrategy) *DecisionStrategy {
	// Count votes for each strategy
	voteCount := make(map[string]int)
	for _, vote := range votes {
		voteCount[vote.StrategyName]++
	}

	// Find strategy with most votes
	var winningStrategy *DecisionStrategy
	maxVotes := 0
	for strategyName, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
			// Find the strategy object
			for _, s := range strategies {
				if s.Name == strategyName {
					winningStrategy = s
					break
				}
			}
		}
	}

	// If no clear winner, use first proposed strategy
	if winningStrategy == nil && len(strategies) > 0 {
		winningStrategy = strategies[0]
	}

	return winningStrategy
}
