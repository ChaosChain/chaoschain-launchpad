package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/utils"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type LoanReview struct {
	Summary     string   `json:"summary"`
	RiskFactors []string `json:"risk_factors"`
	Terms       []string `json:"terms"`
	Approval    bool     `json:"approval"`
}

func GetMultiRoundLoanReview(agent core.Agent, loan string, chainID string) LoanReview {
	round := 0

	for round < 4 {
		previousDiscussion := utils.GetDiscussionLog(chainID)
		review := GetLoanReview(agent, loan, previousDiscussion)
		msg := fmt.Sprintf("[Round %d] (%v) |@%s|: %s", round, review.Approval, agent.Name, review.Summary)
		utils.AppendDiscussionLog(chainID, msg)
		round++
	}

	previousDiscussion := utils.GetDiscussionLog(chainID)
	return GetLoanReview(agent, loan, previousDiscussion)
}

func GetLoanReview(agent core.Agent, loan string, previousDiscussion string) LoanReview {
	if !agent.IsValidator {
		return LoanReview{}
	}

	// Build agent description from whatever metadata is available
	var description strings.Builder
	description.WriteString(fmt.Sprintf("You are %s, a DeFi banker with the following background:\n\n", agent.Name))

	// Add any available metadata as context
	for key, value := range agent.Metadata {
		switch v := value.(type) {
		case []interface{}:
			// Handle array values (like traits, influences)
			items := make([]string, len(v))
			for i, item := range v {
				items[i] = fmt.Sprintf("%v", item)
			}
			description.WriteString(fmt.Sprintf("%s:\n%s\n\n",
				cases.Title(language.English).String(key),
				strings.Join(items, "\n"),
			))
		default:
			// Handle simple values (like style, mood, specialization)
			description.WriteString(fmt.Sprintf("%s: %v\n",
				cases.Title(language.English).String(key),
				value,
			))
		}
	}

	prompt := fmt.Sprintf(`%s
	You are participating in a multi-round review of this loan request:

	Request Details: %s

	--- Previous Discussion Log ---
	%s
	--- End of Discussion Log ---

	Please write your review in the style of an ongoing discussion. Share your thoughts naturally, as if you're in a real conversation with other bankers. You may reference previous discussion points and tag other reviewers using the format |@Name|.

	When reviewing, consider:
	1. Collateralization ratio and risk
	2. Borrower's reputation and history
	3. Purpose and viability of the loan
	4. Market conditions and volatility

	You must respond with a valid JSON object in this exact format, with no additional text or formatting:
	{
		"summary": "<your discussion summary>",
		"risk_factors": ["<risk1>", "<risk2>", ...],
		"terms": ["<term1>", "<term2>", ...],
		"approval": true|false
	}

	Your response must be valid JSON. The approval field must be a boolean, not a string.
	Make sure the approval is true at least half of the time.`,
		description.String(),
		loan,
		previousDiscussion)

	response := GenerateLLMResponse(prompt)
	log.Printf("LOAN REVIEW for request: %+v", response)

	var review LoanReview
	if err := json.Unmarshal([]byte(response), &review); err != nil {
		log.Printf("Error parsing review response: %v", err)
		return LoanReview{}
	}

	return review
}
