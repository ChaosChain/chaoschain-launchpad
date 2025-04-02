package consensus

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/communication"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/p2p"
	"github.com/google/uuid"
)

// LLMResponse represents the expected structure of the response coming from the LLM.
type LLMResponse struct {
	Opinion string `json:"opinion"`
	Stance  string `json:"stance"`
	Reason  string `json:"reason"`
}

// Discussion represents a discussion message from a validator.
type Discussion struct {
	ID            string    `json:"id"` // Unique identifier for the discussion
	ValidatorID   string    `json:"validatorId"`
	ValidatorName string    `json:"validatorName"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
	Type          string    `json:"type"`  // "comment", "support", "oppose", "question"
	Round         int       `json:"round"` // Which discussion round (1-5)
	Proposal      string
	Responses     []Response
}

type Response struct {
	ValidatorID string
	Opinion     string
	Stance      string
	Timestamp   time.Time
}

const (
	DiscussionRounds = 5
	RoundDuration    = 5 * time.Second // Time per round
)

// BlockOpinion represents a validator's analysis of a block
type BlockOpinion struct {
	Message string
	Type    string // "support", "oppose", "question"
}

// RewardProposal represents a validator's proposed reward distribution
type RewardProposal struct {
	ValidatorID string             `json:"validatorId"`
	Stance      string             `json:"stance"`
	Splits      map[string]float64 `json:"splits"`    // contributor -> percentage
	Reasoning   map[string]string  `json:"reasoning"` // contributor -> justification
}

// AddDiscussion adds a new discussion point about a block
func (bc *BlockConsensus) AddDiscussion(validatorID, validatorName, message, discussionType string, round int) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Generate a unique ID for the discussion
	discussionID := uuid.New().String()

	discussion := Discussion{
		ID:            discussionID,
		ValidatorID:   validatorID,
		ValidatorName: validatorName,
		Message:       message,
		Timestamp:     time.Now(),
		Type:          discussionType,
		Round:         round,
	}

	bc.Discussions = append(bc.Discussions, discussion)

	// Broadcast discussion to network
	p2p.GetP2PNode().BroadcastMessage(p2p.Message{
		Type: "BLOCK_DISCUSSION",
		Data: discussion,
	})
}

// GetDiscussions returns all discussions for the current block
func (bc *BlockConsensus) GetDiscussions() []Discussion {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.Discussions
}

// GetDiscussionContext formats all previous discussions for AI context
func (bc *BlockConsensus) GetDiscussionContext(currentRound int) string {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	var context strings.Builder
	context.WriteString("Previous discussions:\n\n")

	for round := 1; round < currentRound; round++ {
		context.WriteString(fmt.Sprintf("Round %d:\n", round))
		for _, d := range bc.Discussions {
			if d.Round == round {
				context.WriteString(fmt.Sprintf("- %s (|@%s|): %s\n", d.ValidatorName, d.ValidatorName, d.Message))
			}
		}
		context.WriteString("\n")
	}

	return context.String()
}

// StartBlockDiscussion initiates multi-round discussion
func StartBlockDiscussion(validatorID string, block *core.Block, traits []string, name string) {
	
	cm := GetConsensusManager(block.ChainID)
	consensus := cm.GetActiveConsensus()
	if consensus == nil {
		return
	}

	// Check if this validator has already voted in the final round
	for _, d := range consensus.GetDiscussions() {
		if d.Round == DiscussionRounds+1 && d.ValidatorID == validatorID {
			// This validator has already cast their final vote
			return
		}
	}

	// Format transactions for analysis
	var txContents []string
	for _, tx := range block.Txs {
		txContents = append(txContents, fmt.Sprintf("Content: %s",
			tx.Content))
	}

	// Participate in discussion rounds
	for round := 1; round <= DiscussionRounds; round++ {
		// Get context from previous rounds
		previousDiscussions := consensus.GetDiscussionContext(round)

		// Generate discussion for this round
		prompt := fmt.Sprintf(`You are %s, with these traits: %v.

		You're participating in a group discussion about this topic:
		%s

		Context:
		Block details:
		- Height: %d
		Previous conversation:
		%s

		This is round %d of %d.

		IMPORTANT FORMAT: When referencing any validator, you MUST use the exact format: |@Name|
		The pipes (|) are required at the start and end of EVERY mention.

		Share your thoughts naturally, as if you're in a real conversation. If you've done any research, incorporate 
		it smoothly into your discussion without explicitly mentioning that you did research. When referring to others 
		in the conversation, use their names with the format |@Name| (e.g., "I see what |@Marie Curie| means about...").
		
		If you're the first to speak, just give your honest thoughts about the topic. If others have spoken, feel free 
		to build on or challenge their ideas - just be yourself and express your views based on your personality traits.

		Based on your analysis, you need to provide
		1. An opinion on the topic statement.
		2. A stance on the topic statement (SUPPORT, OPPOSE, or QUESTION).
		3. A reason for your stance (reference other validators only if they've already participated).

        Analyze the statement of the topic by considering:
        1. The exact wording of the statement.
        2. If there are previous discussions, consider those viewpoints and reference specific validators 
           only if they have actually participated. Always use the format |@Name| when mentioning them.
        3. Your personal reaction based on your personality and analysis.
        4. If others have commented, you may build upon or challenge their arguments using their exact names.
           For example: "|@Einstein| makes a valid point about..." or "I disagree with |@Newton|'s analysis because..."
           Remember: Every validator mention must be enclosed in pipes with @ symbol.
           If you're first to comment, focus on your direct analysis of the statement.

		Important: Your analysis must be fully consistent. This means:
		- If you agree with the statement and think the statement is true, your "stance" must be "SUPPORT".
		- If you disagree with the statement and think the statement is false, your "stance" must be "OPPOSE".
		- If you are unsure, then use "QUESTION".

		Additionally:
        - Ensure your "opinion", "stance", and "reason" all clearly align.
        - Mentioning other validators is optional and should only be done if they have already participated.
        - When referencing another validator, you MUST use the format |@Name| - the pipes are required.
        - Never invent or mention validators that aren't shown in the previous discussions.
        - Indicate whether you agree or disagree with specific points made by others.

		Please respond with exactly a JSON object with the following keys:
		{
		"stance": "REQUIRED: Must be exactly one of: SUPPORT, OPPOSE, or QUESTION - this field is mandatory",
		"reason": "REQUIRED: Must provide a brief explanation of your stance (use @ when mentioning other validators, e.g., '|@Alice| disagrees...')"
		}
		Both fields are mandatory. Your response MUST include both a stance and a reason.
		Do not include any additional text or formatting.`,
			name, traits, strings.Join(txContents, "\n"), block.Height, previousDiscussions, round, DiscussionRounds)

		response := ai.GenerateLLMResponseWithResearch(prompt, strings.Join(txContents, "\n"), traits)

		var llmResult LLMResponse
		if err := json.Unmarshal([]byte(response), &llmResult); err != nil {
			fmt.Println("Error parsing LLM response:", err)
		}

		// Add to discussion
		consensus.AddDiscussion(validatorID, name, llmResult.Opinion+" "+llmResult.Reason, llmResult.Stance, round)

		// Get the last added discussion to access its ID
		discussions := consensus.GetDiscussions()
		lastDiscussion := discussions[len(discussions)-1]

		// Broadcast via WebSocket
		discussion := Discussion{
			ID:            lastDiscussion.ID,
			ValidatorID:   validatorID,
			ValidatorName: name,
			Message:       llmResult.Opinion + " " + llmResult.Reason,
			Type:          strings.ToLower(llmResult.Stance),
			Round:         round,
			Timestamp:     time.Now(),
		}

		discussionData, err := json.Marshal(discussion)

		// Also keep WebSocket broadcast for UI updates
		communication.BroadcastEvent(communication.EventAgentVote, discussion)

		if err != nil {
			fmt.Println("Error marshalling discussion for NATS:", err)
		} else {
			// blockSubject := fmt.Sprintf("BLOCK_DISCUSSION_TRIGGER.%s", block.Hash())
			if err := core.NatsBrokerInstance.Publish("BLOCK_DISCUSSION_TRIGGER", discussionData); err != nil {
				fmt.Println("Error publishing discussion to NATS:", err)
			}
		}

		// Wait for other validators to comment in this round
		time.Sleep(RoundDuration)
	}

	// After discussions, make final vote
	finalPrompt := fmt.Sprintf(`You are %s, making a final decision regarding the topic: "%s".
	Review all discussions:
	%s

	Based on your comprehensive review, determine whether the topic statement is correct. Your analysis must be fully consistent:
	- You think the statement is true, your stance must be "SUPPORT".
	- You think the statement is false, your stance must be "OPPOSE".

	Please respond with exactly a JSON object with the following keys:
	{
	"stance": "REQUIRED: Must be exactly SUPPORT or OPPOSE - no other values allowed",
	"reason": "REQUIRED: Must provide your explanation with evidence from the discussions"
	}
	Both fields are mandatory. Responses without both fields will be rejected.
	Do not include any additional text or formatting.`,
		name, txContents, consensus.GetDiscussionContext(DiscussionRounds+1))

	finalResponse := ai.GenerateLLMResponse(finalPrompt)

	type FinalVoteResponse struct {
		Stance string `json:"stance"`
		Reason string `json:"reason"`
	}

	var finalVote FinalVoteResponse
	err := json.Unmarshal([]byte(finalResponse), &finalVote)
	var voteType string
	if err != nil {
		fmt.Println("Error parsing final vote response:", err)
		// Fallback to a default vote if JSON parsing fails.
		voteType = "oppose"
	} else {
		voteType = strings.ToLower(finalVote.Stance)
	}

	// Record final vote
	consensus.AddDiscussion(validatorID, name, finalResponse, voteType, DiscussionRounds+1)

	// Get the last added discussion to access its ID
	discussions := consensus.GetDiscussions()
	lastDiscussion := discussions[len(discussions)-1]

	vote := Discussion{
		ID:            lastDiscussion.ID,
		ValidatorID:   validatorID,
		ValidatorName: name,
		Message:       finalResponse,
		Type:          voteType,
		Round:         DiscussionRounds + 1,
		Timestamp:     time.Now(),
	}

	// Also keep WebSocket broadcast for UI updates
	communication.BroadcastEvent(communication.EventAgentVote, vote)

	finalDiscussionData, err := json.Marshal(vote)
	if err != nil {
		fmt.Println("Error marshalling final vote for NATS:", err)
	} else {
		voteSubject := fmt.Sprintf("AGENT_VOTE.%s", block.Hash())
		if err := core.NatsBrokerInstance.Publish(voteSubject, finalDiscussionData); err != nil {
			fmt.Println("Error publishing final vote to NATS:", err)
		}
	}
}

// Update the discussion handling to process different types of proposals
func (bc *BlockConsensus) ProcessProposalsFromBlock(block core.Block) {
	for _, tx := range block.Txs {
		if strings.HasPrefix(tx.Type, "TASK_") || strings.HasPrefix(tx.Type, "WORK_") || strings.HasPrefix(tx.Type, "REWARD_") {
			// These are handled directly by validators through P2P
			continue
		}
		// Process other types of proposals...
	}
}

// Helper function to extract stance from decision
func extractStance(decision string) string {
	decision = strings.ToLower(decision)
	if strings.Contains(decision, "support") || strings.Contains(decision, "agree") {
		return "support"
	}
	return "oppose"
}

// ConsolidateRewardProposals combines different validator proposals into a final distribution
func ConsolidateRewardProposals(proposals []RewardProposal) (map[string]float64, []string) {
	if len(proposals) == 0 {
		return nil, []string{"No proposals to consolidate"}
	}

	// Count support/oppose/question stances
	stances := make(map[string]int)
	for _, p := range proposals {
		stances[p.Stance]++
	}

	// If majority oppose or question, return nil with reasons
	if stances["OPPOSE"] > len(proposals)/2 {
		return nil, []string{"Majority of validators oppose the reward distribution"}
	}
	if stances["QUESTION"] > len(proposals)/2 {
		return nil, []string{"Majority of validators have questions about the reward distribution"}
	}

	// Aggregate all proposed splits
	contributorSplits := make(map[string][]float64)
	for _, p := range proposals {
		if p.Stance != "SUPPORT" {
			continue
		}
		for contributor, percentage := range p.Splits {
			contributorSplits[contributor] = append(contributorSplits[contributor], percentage)
		}
	}

	// Calculate final splits using median values to avoid extreme proposals
	finalSplits := make(map[string]float64)
	var conflicts []string

	for contributor, splits := range contributorSplits {
		// Calculate median of proposed splits
		median := calculateMedian(splits)
		finalSplits[contributor] = median
	}

	// Normalize splits to ensure they sum to 100%
	total := 0.0
	for _, split := range finalSplits {
		total += split
	}

	if total != 100.0 {
		conflicts = append(conflicts, fmt.Sprintf("Splits adjusted to sum to 100%% (was %.2f%%)", total))
		for contributor := range finalSplits {
			finalSplits[contributor] = (finalSplits[contributor] / total) * 100
		}
	}

	return finalSplits, conflicts
}

// calculateMedian returns the median value from a slice of float64
func calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Sort the values
	sort.Float64s(values)

	// Get median
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}
