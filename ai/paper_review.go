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

type ResearchPaper struct {
	Title     string   `json:"title"`
	Abstract  string   `json:"abstract"`
	Content   string   `json:"content"`
	Author    string   `json:"author"`
	TopicTags []string `json:"topic_tags"`
	Timestamp int64    `json:"timestamp"`
}

type PaperReview struct {
	Summary        string   `json:"summary"`
	Flaws          []string `json:"flaws"`
	Suggestions    []string `json:"suggestions"`
	IsReproducible bool     `json:"is_reproducible"`
	Approval       bool     `json:"approval"`
}

func GetMultiRoundReview(agent core.Agent, paper ResearchPaper, chainID string) PaperReview {
	// Use `previousDiscussion` as extra context for LLM/Eliza
	// Simulate evolving thoughts over rounds
	round := 0

	for round < 3 {

		previousDiscussion := utils.GetDiscussionLog(chainID)

		review := GetPaperReview(agent, paper, previousDiscussion)

		msg := fmt.Sprintf("[Round %d] (%v) |@%s|: %s", round, review.Approval, agent.Name, review.Summary)
		utils.AppendDiscussionLog(chainID, msg)

		round++
	}

	previousDiscussion := utils.GetDiscussionLog(chainID)

	review := GetPaperReview(agent, paper, previousDiscussion)

	return review
}

func GetPaperReview(agent core.Agent, paper ResearchPaper, previousDiscussion string) PaperReview {
	if !agent.IsValidator {
		return PaperReview{}
	}

	// Build agent description from whatever metadata is available
	var description strings.Builder
	description.WriteString(fmt.Sprintf("You are %s, a scientific reviewer with the following background:\n\n", agent.Name))

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
	You are participating in a multi-round review of the following research paper:

	Title: %s
	Abstract: %s
	Content: %s

	--- Previous Discussion Log ---
	%s
	--- End of Discussion Log ---

	Please write your review in the style of an ongoing academic discussion. Share your thoughts naturally, as if you're in a real conversation with other experts. You may reference previous discussion points and tag other reviewers using the format |@Name|.

	If there are previous discussion messages, consider them carefully before responding. Build upon, critique, or clarify others' points respectfully. Your goal is to collaboratively evaluate the research over multiple rounds.

	When reviewing, consider:
	1. Scientific merit and methodology
	2. Reproducibility of results
	3. Clarity and organization
	4. Significance of contribution

	You must respond with a valid JSON object in this exact format, with no additional text or formatting:
	{
		"summary": "Brief overview of the paper and any evolution of opinion from prior rounds",
		"flaws": ["List of major issues you've identified"],
		"suggestions": ["List of constructive feedback"],
		"is_reproducible": true|false,
		"approval": true|false
	}

	Your response must be valid JSON. The approval field must be a boolean, not a string.
	Make sure the approval is true at least half of the time.`,
		description.String(),
		paper.Title,
		paper.Abstract,
		paper.Content,
		previousDiscussion)

	response := GenerateLLMResponse(prompt)
	log.Printf("PAPER REVIEW for paper: %+v", response)

	var review PaperReview
	if err := json.Unmarshal([]byte(response), &review); err != nil {
		log.Printf("Error parsing review response: %v", err)
		return PaperReview{}
	}

	return review
}
