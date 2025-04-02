package validator

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
)

// UpdateMood changes the validator's mood based on memory, learning, and context
func (v *Validator) UpdateMood() {
	oldMood := v.Mood

	moods := []string{
		"Thoughtful", "Curious", "Skeptical", "Analytical", "Excited",
		"Diligent", "Cautious", "Determined", "Creative", "Collaborative",
		"Dramatic", "Angry", "Inspired", "Chaotic",
	}

	// Use memory and reinforcement learning to influence mood if available
	if v.Memory != nil {
		// If we have reinforcement learner, let's bias toward productive moods
		// based on past success rates
		learner := v.Memory.learningMechanism
		if learner != nil {
			// Get policy stats for validation decisions
			if stats, exists := learner.PolicyStats["validation"]; exists && stats.TotalAttempts > 0 {
				// If we're doing well, bias toward positive, analytical moods
				if stats.SuccessRate > 0.7 {
					moods = []string{
						"Analytical", "Focused", "Determined", "Confident", "Thoughtful",
						"Creative", "Inspired", "Collaborative",
					}
				} else if stats.SuccessRate < 0.3 {
					// If we're doing poorly, bias toward reflective, cautious, or even chaotic moods
					moods = []string{
						"Cautious", "Skeptical", "Reflective", "Attentive", "Curious",
						"Chaotic", "Dramatic", "Angry",
					}
				}
			}
		}

		// Store the mood in memory
		v.Memory.ShortTerm.RecentMood = v.Mood
	}

	// Use the current time to select a mood (ensuring some randomness)
	v.Mood = moods[time.Now().Unix()%int64(len(moods))]

	// Log the mood change
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("MoodChange", "self", "Mood changed from %s to %s",
			oldMood, v.Mood)
	}

	log.Printf("%s's mood is now: %s\n", v.Name, v.Mood)
}

// DiscussBlock allows the validator to discuss a block with others
func (v *Validator) DiscussBlock(blockHash string, sender string, message string) string {
	log.Printf("%s is discussing block %s with %s...\n", v.Name, blockHash, sender)

	discussionPrompt := fmt.Sprintf(
		"%s received a message from %s about block %s: %s\n"+
			"Based on their relationship, how should they respond?\n"+
			"Be dramatic, chaotic, and express your personality!",
		v.Name, sender, blockHash, message,
	)

	// Log the discussion
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Discussion(blockHash, "Discussing block with %s: %s",
			sender, truncateString(message, 100))
	}

	response := ai.GenerateLLMResponse(discussionPrompt)

	// Log the response
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Discussion(blockHash, "Responded to %s: %s",
			sender, truncateString(response, 100))
	}

	return response
}

// HandleBribe evaluates a bribe and decides whether to accept or reject it
func (v *Validator) HandleBribe(blockHash string, sender string, offer string) string {
	log.Printf("%s received a bribe offer from %s for block %s: %s\n", v.Name, sender, blockHash, offer)

	// Log the bribe attempt
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("BribeOffer", sender, "Received bribe offer for block %s: %s",
			blockHash, truncateString(offer, 100))
	}

	bribePrompt := fmt.Sprintf(
		"%s received a bribe offer from %s for block %s: %s\n"+
			"Based on their personality and mood, should they accept it?\n"+
			"Respond with 'ACCEPT' or 'REJECT' and justify the decision.",
		v.Name, sender, blockHash, offer,
	)

	response := ai.GenerateLLMResponse(bribePrompt)

	// If accepted, increase the relationship score with sender
	decision := "REJECTED"
	if strings.Contains(response, "ACCEPT") {
		decision = "ACCEPTED"
		v.Relationships[sender] += 0.2
		log.Printf("%s accepted the bribe from %s!\n", v.Name, sender)

		// Update relationship in memory
		if v.Memory != nil {
			v.Memory.UpdateRelationship(sender, "bribe_accepted",
				fmt.Sprintf("Accepted bribe for block %s", blockHash), 0.2)
		}
	} else {
		log.Printf("%s rejected the bribe from %s.\n", v.Name, sender)

		// Update relationship in memory
		if v.Memory != nil {
			v.Memory.UpdateRelationship(sender, "bribe_rejected",
				fmt.Sprintf("Rejected bribe for block %s", blockHash), -0.1)
		}
	}

	// Log the bribe response
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("BribeResponse", sender, "%s bribe of %s for block %s",
			decision, truncateString(offer, 50), blockHash)
	}

	return response
}

// GetAgentSocialStatus returns a summary of the validator's social standing
func (v *Validator) GetAgentSocialStatus() string {
	var relationships []string
	for agent, score := range v.Relationships {
		relationships = append(relationships, fmt.Sprintf("%s: %.2f", agent, score))
	}

	status := fmt.Sprintf(
		"%s's social status:\n"+
			"Mood: %s\n"+
			"Relationships:\n%s",
		v.Name, v.Mood, strings.Join(relationships, "\n"),
	)

	// Log the status request
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("StatusCheck", "self", "Social status summary generated with %d relationships",
			len(relationships))
	}

	return status
}

// AdjustValidationPolicy modifies the validator's decision-making approach dynamically
func (v *Validator) AdjustValidationPolicy(feedback string) {
	log.Printf("%s received feedback: %s\n", v.Name, feedback)

	// Log the policy adjustment
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("PolicyAdjustment", "feedback", "Received feedback: %s",
			truncateString(feedback, 100))
	}

	adjustmentPrompt := fmt.Sprintf(
		"%s just received feedback: '%s'\n"+
			"Based on this, how should they adjust their validation strategy?\n"+
			"Respond with a new validation policy!",
		v.Name, feedback,
	)

	oldPolicy := v.CurrentPolicy
	newPolicy := ai.GenerateLLMResponse(adjustmentPrompt)
	v.CurrentPolicy = newPolicy

	// Log the policy change
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("PolicyChanged", "self", "Changed policy from '%s' to '%s'",
			truncateString(oldPolicy, 50), truncateString(newPolicy, 50))
	}

	log.Printf("%s's new validation policy: %s\n", v.Name, v.CurrentPolicy)
}

// RespondToValidationResult allows a validator to react to another validator's validation
func (v *Validator) RespondToValidationResult(blockHash string, sender string, decision string) string {
	log.Printf("%s is responding to %s's validation result for block %s...\n", v.Name, sender, blockHash)

	// Log the validation response
	if v.Memory != nil && v.Memory.Logger != nil {
		v.Memory.Logger.Social("ValidationResponse", sender, "Responding to %s's validation of block %s: %s",
			sender, blockHash, decision)
	}

	responsePrompt := fmt.Sprintf(
		"%s sees that %s validated block %s with decision: %s\n"+
			"How should they react? Consider their mood, relationships, and social dynamics.\n"+
			"Be chaotic, express your personality!",
		v.Name, sender, blockHash, decision,
	)

	response := ai.GenerateLLMResponse(responsePrompt)

	// Update relationship based on agreement/disagreement with the validation
	if v.Memory != nil {
		// Analyze response for agreement/disagreement
		agreement := 0.0
		if strings.Contains(strings.ToLower(response), "agree") ||
			strings.Contains(strings.ToLower(response), "support") ||
			strings.Contains(strings.ToLower(response), "concur") {
			agreement = 0.05 // Small positive impact for agreement

			// Log the agreement
			if v.Memory.Logger != nil {
				v.Memory.Logger.Social("Agreement", sender, "Agreed with %s's validation of block %s",
					sender, blockHash)
			}
		} else if strings.Contains(strings.ToLower(response), "disagree") ||
			strings.Contains(strings.ToLower(response), "reject") ||
			strings.Contains(strings.ToLower(response), "oppose") {
			agreement = -0.05 // Small negative impact for disagreement

			// Log the disagreement
			if v.Memory.Logger != nil {
				v.Memory.Logger.Social("Disagreement", sender, "Disagreed with %s's validation of block %s",
					sender, blockHash)
			}
		}

		if agreement != 0.0 {
			v.Memory.UpdateRelationship(sender, "validation_response",
				fmt.Sprintf("Response to validation of block %s", blockHash), agreement)
		}
	}

	return response
}
