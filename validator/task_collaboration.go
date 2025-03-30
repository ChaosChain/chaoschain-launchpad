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

// TaskBreakdownResults contains the final consolidated task breakdown
type TaskBreakdownResults struct {
	FinalSubtasks      []string       // The final, agreed-upon list of subtasks
	Discussion         TaskDiscussion // Complete discussion history
	ConsensusScore     float64        // The final consensus score
	BlockInfo          *core.Block    // The block that triggered this breakdown
	TransactionDetails string         // String representation of transaction details
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
	// We'll add more fields later when fully implementing the discussion approach
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

// DecisionStrategy represents an agent's strategy for final decision making
type DecisionStrategy struct {
	ValidatorID   string `json:"validatorId"`
	ValidatorName string `json:"validatorName"`
	Strategy      string `json:"strategy"` // e.g., "consensus", "majority", "expert", etc.
	Reasoning     string `json:"reasoning"`
	Timestamp     time.Time
}

const (
	InitialProposalRound = 1
	FeedbackRound        = 2
	FinalizationRound    = 3
	RoundDuration        = 5 * time.Second // Time per round
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

// DelegationMessage represents a message in the task delegation discussion
type TaskDelegationMessage struct {
	ValidatorID   string            `json:"validatorId"`
	ValidatorName string            `json:"validatorName"`
	MessageType   string            `json:"messageType"`
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
	}

	// Temporary hardcoded validators to avoid dependency issues
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

	// PHASE 1: Initial Proposals
	// Each validator independently proposes their initial breakdown
	log.Printf("Beginning initial proposal phase")

	initialProposals := make(map[string][]string) // validatorID -> subtasks
	var initialMessages []DiscussionMessage

	for _, v := range validators {
		log.Printf("\n🤔 [%s] Analyzing task and generating initial breakdown...", v.Name)

		// Generate initial proposal
		proposal := generateInitialProposal(v, results)

		// Add to tracking structures
		initialProposals[v.ID] = proposal.Subtasks

		log.Printf("📝 [%s] proposes %d subtasks:", v.Name, len(proposal.Subtasks))
		for i, subtask := range proposal.Subtasks {
			log.Printf("  %d. %s", i+1, subtask)
		}
		log.Printf("Reasoning: %s", truncateString(proposal.Reasoning, 200))

		// Create discussion message
		message := DiscussionMessage{
			ValidatorID:   v.ID,
			ValidatorName: v.Name,
			MessageType:   "proposal",
			Content: fmt.Sprintf("I propose breaking down this task into the following subtasks: %s\n\nReasoning: %s",
				formatSubtasksList(proposal.Subtasks), proposal.Reasoning),
			Proposal:  proposal.Subtasks,
			MessageID: uuid.New().String(),
			Timestamp: time.Now(),
		}

		initialMessages = append(initialMessages, message)

		// Broadcast the proposal
		communication.BroadcastEvent(communication.EventTaskBreakdownMessage, map[string]interface{}{
			"message":     message,
			"blockHeight": block.Height,
			"timestamp":   time.Now(),
		})

		// Short delay between validators to simulate natural conversation timing
		time.Sleep(100 * time.Millisecond)
	}

	// Add all initial messages to the discussion
	results.Discussion.Messages = append(results.Discussion.Messages, initialMessages...)

	// PHASE 2: Open Discussion
	// Validators discuss, critique, refine proposals until consensus emerges
	log.Printf("\n🗣️ Beginning open discussion phase")

	// Define consensus parameters
	maxIterations := 10
	consensusThreshold := 0.75 // At least 75% consensus needed
	log.Printf("Consensus threshold set to %.2f, maximum %d discussion iterations",
		consensusThreshold, maxIterations)

	// Track the best consensus so far
	var bestConsensus []string
	bestConsensusScore := 0.0

	// Discussion continues until consensus threshold is reached or max iterations
	for iteration := 0; iteration < maxIterations; iteration++ {
		log.Printf("\n📣 Discussion iteration %d started", iteration+1)

		// Build context of discussion so far
		discussionContext := formatDiscussionContext(results.Discussion)

		var iterationMessages []DiscussionMessage
		currentProposals := make(map[string][]string)

		log.Printf("Current discussion length: %d messages", len(results.Discussion.Messages))

		// Each validator considers the discussion and may contribute
		for _, v := range validators {
			// Validator decides whether to contribute based on the current discussion
			shouldContribute, contribution := generateContribution(v, discussionContext, results, iteration)

			if shouldContribute {
				log.Printf("\n💬 [%s] contributes to the discussion (%s):", v.Name, contribution.MessageType)
				log.Printf("%s", truncateString(contribution.Content, 300))

				if len(contribution.Proposal) > 0 {
					log.Printf("Proposes %d refined subtasks:", len(contribution.Proposal))
					for i, subtask := range contribution.Proposal {
						log.Printf("  %d. %s", i+1, subtask)
					}
				}

				// Add to tracking
				currentProposals[v.ID] = contribution.Proposal

				// Create message
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

				iterationMessages = append(iterationMessages, message)

				// Broadcast message
				communication.BroadcastEvent(communication.EventTaskBreakdownMessage, map[string]interface{}{
					"message":     message,
					"blockHeight": block.Height,
					"iteration":   iteration + 1,
					"timestamp":   time.Now(),
				})

				// Add slight delay between messages
				time.Sleep(150 * time.Millisecond)
			} else {
				log.Printf("😶 [%s] chooses to remain silent this round", v.Name)
			}
		}

		// Add messages to discussion history
		results.Discussion.Messages = append(results.Discussion.Messages, iterationMessages...)

		// If no one contributed in this round, it might mean we've reached a natural conclusion
		if len(iterationMessages) == 0 {
			log.Printf("No further contributions in iteration %d, discussion may have naturally concluded", iteration+1)
			// But continue for at least 3 iterations to ensure thorough discussion
			if iteration >= 3 {
				break
			}
		}

		// Extract current consensus proposal
		currentConsensus := extractConsensusProposal(results.Discussion)
		currentConsensusScore := calculateConsensusScore(currentProposals, currentConsensus)

		log.Printf("\n📊 Current consensus evaluation:")
		log.Printf("Consensus score: %.2f (threshold: %.2f)", currentConsensusScore, consensusThreshold)

		if len(currentConsensus) > 0 {
			log.Printf("Current consensus includes %d subtasks:", len(currentConsensus))
			for i, subtask := range currentConsensus {
				log.Printf("  %d. %s", i+1, subtask)
			}
		} else {
			log.Printf("No consensus subtasks identified yet")
		}

		// Broadcast iteration results
		communication.BroadcastEvent(communication.EventTaskBreakdownIteration, map[string]interface{}{
			"iteration":        iteration + 1,
			"consensusScore":   currentConsensusScore,
			"threshold":        consensusThreshold,
			"consensusReached": currentConsensusScore >= consensusThreshold,
			"blockHeight":      block.Height,
			"timestamp":        time.Now(),
		})

		// Track best consensus so far
		if currentConsensusScore > bestConsensusScore {
			bestConsensusScore = currentConsensusScore
			bestConsensus = currentConsensus
			log.Printf("✅ New best consensus identified (score: %.2f)", bestConsensusScore)
		}

		// Check if consensus threshold reached
		if currentConsensusScore >= consensusThreshold {
			log.Printf("🎉 Consensus threshold reached after %d iterations!", iteration+1)
			results.FinalSubtasks = currentConsensus
			results.ConsensusScore = currentConsensusScore
			break
		}

		// Brief pause between iterations
		time.Sleep(250 * time.Millisecond)
	}

	// If we didn't reach consensus threshold, use best consensus found
	if results.ConsensusScore < consensusThreshold {
		log.Printf("⚠️ Consensus threshold not reached after maximum iterations. Using best consensus (score: %.2f)", bestConsensusScore)
		results.FinalSubtasks = bestConsensus
		results.ConsensusScore = bestConsensusScore
	}

	// PHASE 3: Final consensus summary
	// A nominated validator (or system) summarizes the final consensus
	log.Printf("\n📝 Generating final consensus summary")
	summaryMessage := generateFinalSummary(results, validators)
	results.Discussion.Messages = append(results.Discussion.Messages, summaryMessage)

	log.Printf("\n======= FINAL TASK BREAKDOWN RESULTS =======")
	log.Printf("Final consensus score: %.2f", results.ConsensusScore)
	log.Printf("Final subtasks (%d):", len(results.FinalSubtasks))
	for i, subtask := range results.FinalSubtasks {
		log.Printf("  %d. %s", i+1, subtask)
	}
	log.Printf("===========================================")

	// Broadcast final consensus
	communication.BroadcastEvent(communication.EventTaskBreakdownCompleted, map[string]interface{}{
		"subtasks":       results.FinalSubtasks,
		"consensusScore": results.ConsensusScore,
		"blockHeight":    block.Height,
		"messageCount":   len(results.Discussion.Messages),
		"timestamp":      time.Now(),
	})

	log.Printf("Task breakdown complete with %d subtasks and consensus score of %.2f",
		len(results.FinalSubtasks), results.ConsensusScore)

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

	// Validator's probability of contributing decreases if they've recently spoken
	recentlySpokeCount := 0
	for i := len(results.Discussion.Messages) - 1; i >= 0 && i >= len(results.Discussion.Messages)-5; i-- {
		if results.Discussion.Messages[i].ValidatorID == v.ID {
			recentlySpokeCount++
		}
	}

	// Probability of speaking decreases if they've spoken recently
	// But increases in later rounds to help reach consensus
	chanceToSpeak := 0.8 - (float64(recentlySpokeCount) * 0.2) + (float64(iteration) * 0.05)
	if rand.Float64() > chanceToSpeak {
		return false, contribution
	}

	// Generate a prompt for the validator based on the discussion context
	prompt := fmt.Sprintf(`
You are %s, a validator with traits: %s.

You are participating in a collaborative task breakdown discussion. Below is the context of the discussion so far:

TRANSACTION DETAILS:
%s

DISCUSSION CONTEXT:
%s

Based on your personality and the discussion so far, choose ONE action:

1. PROPOSE a refined list of subtasks
2. CRITIQUE a specific proposal or aspect of the discussion
3. AGREE with a specific proposal or point, possibly adding minor refinements
4. ASK a clarifying question
5. SUMMARIZE the discussion and identify emerging consensus
6. STAY SILENT if you have nothing meaningful to add

Consider:
- What has already been said? Don't repeat others unnecessarily
- What expertise or perspective can you uniquely contribute?
- What would move the discussion toward consensus?
- Has the discussion already reached a good consensus?

Respond with a JSON object with these fields:
{
  "action": "PROPOSE|CRITIQUE|AGREE|ASK|SUMMARIZE|SILENT",
  "message": "Your actual contribution text",
  "replyToMessageID": "ID of message you're replying to (if applicable)",
  "subtasks": ["Only include if you're proposing a refined list of subtasks"]
}
`, v.Name, strings.Join(v.Traits, ", "), results.TransactionDetails, discussionContext)

	// Get LLM response
	response := ai.GenerateLLMResponse(prompt)

	// Parse response
	var result struct {
		Action           string   `json:"action"`
		Message          string   `json:"message"`
		ReplyToMessageID string   `json:"replyToMessageID"`
		Subtasks         []string `json:"subtasks"`
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
		"PROPOSE":   "proposal",
		"CRITIQUE":  "critique",
		"AGREE":     "agreement",
		"ASK":       "question",
		"SUMMARIZE": "summary",
	}

	contribution.MessageType = messageTypeMap[result.Action]
	contribution.Content = result.Message
	contribution.ReplyTo = result.ReplyToMessageID
	contribution.Proposal = result.Subtasks

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

// extractConsensusProposal analyzes the discussion to extract the current consensus on subtasks
func extractConsensusProposal(discussion TaskDiscussion) []string {
	// Subtask -> count
	subtaskMentions := make(map[string]int)

	// Track all proposed tasks by normalizing them
	allNormalizedTasks := make(map[string][]string) // normalized task -> original tasks

	// Track task frequencies
	for _, msg := range discussion.Messages {
		// Only consider proposals
		if len(msg.Proposal) > 0 {
			for _, subtask := range msg.Proposal {
				normalizedTask := normalizeTask(subtask)

				// Try to find similar existing task
				var foundSimilar bool
				for existingNormalized := range allNormalizedTasks {
					similarity := calculateTaskSimilarity(normalizedTask, existingNormalized)
					if similarity > 0.7 {
						// Found similar task, use the existing one
						subtaskMentions[existingNormalized]++
						// Also record this as a variant of the task
						allNormalizedTasks[existingNormalized] = append(
							allNormalizedTasks[existingNormalized], subtask)
						foundSimilar = true
						break
					}
				}

				// If no similar task found, add as new
				if !foundSimilar {
					subtaskMentions[normalizedTask] = 1
					allNormalizedTasks[normalizedTask] = []string{subtask}
				}
			}
		}
	}

	// Get total number of proposals from unique validators
	validatorProposalCount := 0
	validatorSeen := make(map[string]bool)

	for _, msg := range discussion.Messages {
		if len(msg.Proposal) > 0 && !validatorSeen[msg.ValidatorID] {
			validatorProposalCount++
			validatorSeen[msg.ValidatorID] = true
		}
	}

	// Calculate vote threshold (40% of validators need to mention a subtask)
	threshold := int(float64(validatorProposalCount) * 0.4)
	if threshold < 1 {
		threshold = 1 // At least 1 vote needed
	}

	// Collect consensus subtasks
	var consensusSubtasks []string

	// Debug log
	log.Printf("Extracting consensus from %d unique validator proposals (threshold: %d mentions)",
		validatorProposalCount, threshold)

	// Select subtasks with sufficient mentions
	for normalizedTask, count := range subtaskMentions {
		if count >= threshold {
			// Find the most common original form of this task
			variants := allNormalizedTasks[normalizedTask]

			// Use the best variant (first one for now, could be improved)
			// A better approach would use the most frequently seen variant
			consensusSubtasks = append(consensusSubtasks, variants[0])

			log.Printf("Consensus task: \"%s\" mentioned %d times (variants: %d)",
				variants[0], count, len(variants))
		}
	}

	return consensusSubtasks
}

// countUniqueValidators counts the number of distinct validators in a discussion
func countUniqueValidators(discussion TaskDiscussion, startIdx int) int {
	validators := make(map[string]bool)

	for i := startIdx; i < len(discussion.Messages); i++ {
		validators[discussion.Messages[i].ValidatorID] = true
	}

	return len(validators)
}

// calculateConsensusScore measures how much consensus exists across proposals
// Returns a value between 0 (no consensus) and 1 (perfect consensus)
func calculateConsensusScore(currentProposals map[string][]string, consensusSubtasks []string) float64 {
	if len(currentProposals) == 0 || len(consensusSubtasks) == 0 {
		return 0.0
	}

	// Track total agreement score
	var totalAgreementScore float64

	// For each validator, calculate how many of their tasks match consensus tasks
	for validatorID, validatorTasks := range currentProposals {
		// Skip validators with no tasks
		if len(validatorTasks) == 0 {
			continue
		}

		// Create efficient lookup for validator's tasks
		validatorTaskMap := make(map[string]bool)
		for _, task := range validatorTasks {
			validatorTaskMap[normalizeTask(task)] = true
		}

		// Count how many consensus tasks this validator included
		var taskMatches float64
		for _, consensusTask := range consensusSubtasks {
			normalizedConsensusTask := normalizeTask(consensusTask)
			// Look for exact match or high similarity
			if validatorTaskMap[normalizedConsensusTask] {
				taskMatches++
			} else {
				// Check for similar tasks (near match)
				for validatorTask := range validatorTaskMap {
					if calculateTaskSimilarity(normalizedConsensusTask, validatorTask) > 0.7 {
						taskMatches += 0.7 // Partial credit for similar tasks
						break
					}
				}
			}
		}

		// Calculate agreement as percentage of consensus tasks matched
		var agreementScore float64
		if len(consensusSubtasks) > 0 {
			agreementScore = taskMatches / float64(len(consensusSubtasks))
		}

		log.Printf("  - Validator %s agreement score: %.2f (matched %.1f of %d consensus tasks)",
			validatorID, agreementScore, taskMatches, len(consensusSubtasks))

		totalAgreementScore += agreementScore
	}

	// Calculate average agreement across all validators with proposals
	validatorCount := len(currentProposals)
	if validatorCount == 0 {
		return 0.0
	}

	return totalAgreementScore / float64(validatorCount)
}

// normalizeTask prepares a task string for comparison by removing whitespace and lowercasing
func normalizeTask(task string) string {
	// Convert to lowercase and trim spaces
	normalized := strings.ToLower(strings.TrimSpace(task))
	// Remove extra internal spaces
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// calculateTaskSimilarity measures how similar two tasks are (0.0 to 1.0)
func calculateTaskSimilarity(task1, task2 string) float64 {
	// For simple similarity, use Jaccard similarity between word sets
	words1 := strings.Fields(task1)
	words2 := strings.Fields(task2)

	// Create sets of words
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, word := range words1 {
		set1[word] = true
	}

	for _, word := range words2 {
		set2[word] = true
	}

	// Calculate intersection size
	var intersectionSize int
	for word := range set1 {
		if set2[word] {
			intersectionSize++
		}
	}

	// Calculate union size
	unionSize := len(set1) + len(set2) - intersectionSize

	if unionSize == 0 {
		return 0.0
	}

	return float64(intersectionSize) / float64(unionSize)
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
	}

	// Temporary hardcoded validators to avoid dependency issues
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
		"timestamp":   time.Now(),
	})

	// Track assignment votes
	assignmentVotes := make(map[string]map[string]int)

	// Initialize voting maps for each subtask
	for _, subtask := range taskBreakdown.FinalSubtasks {
		assignmentVotes[subtask] = make(map[string]int)
	}

	// Each validator proposes assignments
	for _, v := range validators {
		log.Printf("🤔 [%s] Generating task delegation proposal...", v.Name)

		// Generate a proposal for each subtask
		validatorAssignments := make(map[string]string)

		// For this simulation, we'll have each validator assign based on a simple algorithm
		// In a real implementation, this would involve LLM calls to the validator
		for i, subtask := range taskBreakdown.FinalSubtasks {
			// Simple round-robin assignment
			assigneeIndex := (i + len(validatorAssignments)) % len(validators)
			validatorAssignments[subtask] = validators[assigneeIndex].Name

			// Record this "vote"
			assignmentVotes[subtask][validators[assigneeIndex].Name]++

			log.Printf("  - [%s] proposes \"%s\" → %s", v.Name,
				truncateString(subtask, 40), validators[assigneeIndex].Name)
		}

		// Broadcast validator proposal
		communication.BroadcastEvent(communication.EventTaskDelegationMessage, map[string]interface{}{
			"validatorId":   v.ID,
			"validatorName": v.Name,
			"assignments":   validatorAssignments,
			"blockHeight":   taskBreakdown.BlockInfo.Height,
			"timestamp":     time.Now(),
		})

		// Short delay between validators
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("📊 Aggregating delegation proposals and determining consensus...")

	// Now determine the most popular assignment for each subtask
	for subtask, votes := range assignmentVotes {
		var maxVotes int
		var winner string

		// Log the votes for transparency
		log.Printf("Votes for subtask \"%s\":", truncateString(subtask, 40))

		for validator, voteCount := range votes {
			log.Printf("  - %s: %d votes", validator, voteCount)
			if voteCount > maxVotes {
				maxVotes = voteCount
				winner = validator
			}
		}

		if winner != "" {
			results.Assignments[subtask] = winner
			log.Printf("✅ Selected: %s (with %d votes)", winner, maxVotes)
		} else if len(validators) > 0 {
			// Fallback to first validator if no consensus
			results.Assignments[subtask] = validators[0].Name
			log.Printf("⚠️ No consensus reached, defaulting to: %s", validators[0].Name)
		}
	}

	// Create a summary message
	var summary strings.Builder
	summary.WriteString("Task delegation complete. Final assignments:\n\n")

	for subtask, assignee := range results.Assignments {
		summary.WriteString(fmt.Sprintf("• %s → %s\n", subtask, assignee))
	}

	// Create a visual summary of assignments grouped by validator
	log.Printf("\n======= FINAL TASK DELEGATION RESULTS =======")

	// Group tasks by validator for a cleaner summary
	validatorTasks := make(map[string][]string)
	for subtask, assignee := range results.Assignments {
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
		len(results.Assignments), len(validatorTasks))

	// Broadcast completion
	communication.BroadcastEvent(communication.EventTaskDelegationCompleted, map[string]interface{}{
		"assignments": results.Assignments,
		"summary":     summary.String(),
		"blockHeight": taskBreakdown.BlockInfo.Height,
		"timestamp":   time.Now(),
	})

	log.Printf("Task delegation process completed successfully")

	return results
}

// Helper function to format a list of subtasks for prompts
func formatSubtasksList(subtasks []string) string {
	var result strings.Builder
	for i, subtask := range subtasks {
		result.WriteString(fmt.Sprintf("%d. %s\n", i+1, subtask))
	}
	return result.String()
}

// Helper function to truncate long strings for logging
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
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
