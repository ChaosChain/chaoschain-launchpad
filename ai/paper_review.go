package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/utils"
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

	prompt := fmt.Sprintf(`You are %s, a scientific reviewer with the following traits: %v.

	You are participating in a multi-round review of the following research paper:

	Title: %s
	Abstract: %s
	Content: %s

	--- Previous Discussion Log ---
	%s
	--- End of Discussion Log ---

	Please write your review in the style of an ongoing academic discussion. Share your thoughts naturally, as if you're in a real conversation with other experts. You may reference previous discussion points and tag other reviewers using the format |@Name|, for example: "I agree with |@Marie Curie| on the reproducibility concern..."

	If there are previous discussion messages, consider them carefully before responding. Build upon, critique, or clarify others' points respectfully. Your goal is to collaboratively evaluate the research over multiple rounds.

	When reviewing, consider:
	1. Scientific merit and methodology
	2. Reproducibility of results
	3. Clarity and organization
	4. Significance of contribution

	You may also bring your personality traits into your tone and reasoning — let them influence how you communicate and what you prioritize.

	Your 'approval' should be true if you think the paper is good, and false if you think it is bad.

	Respond ONLY with a JSON object in this format:
	{
	"summary": "Brief overview of the paper and any evolution of opinion from prior rounds",
	"flaws": "List of major issues you've identified or agree with from others (should be a list of strings)",
	"suggestions": "List of constructive feedback, possibly referencing or extending others' ideas (should be a list of strings)",
	"is_reproducible": true or false,
	"approval": true or false
	}

	Make sure the approval is true atleast half of the time.

	Do not include any additional text or formatting.`,
		agent.Name, strings.Join(agent.Traits, ", "),
		paper.Title, paper.Abstract, paper.Content, previousDiscussion)

	response := GenerateLLMResponse(prompt)

	log.Println("OPENAI PROMPT: ", prompt)

	log.Printf("OPEN AI REVIEW of the paper: %+v, for the paper %+v", response, paper)

	var review PaperReview
	if err := json.Unmarshal([]byte(response), &review); err != nil {
		log.Printf("Error parsing review response: %v", err)
		return PaperReview{}
	}

	return review
}
