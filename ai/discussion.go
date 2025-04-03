package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/google/uuid"
)

type Discussion struct {
	ID            string    `json:"id"` // Unique identifier for the discussion
	ValidatorID   string    `json:"validatorId"`
	ValidatorName string    `json:"validatorName"`
	Message       string    `json:"message"`
	Support       bool      `json:"support"`
	Oppose        bool      `json:"oppose"`
	Question      bool      `json:"question"`
	Timestamp     time.Time `json:"timestamp"`
	Round         int       `json:"round"` // Which discussion round (1-5)
}

func GetValidatorDiscussion(agent core.Agent, tx core.Transaction) Discussion {
	if !agent.IsValidator {
		return Discussion{}
	}

	prompt := fmt.Sprintf(`You are %s, with these traits: %v.

		You're participating in a group discussion about this topic:
		%s

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

		Your response MUST be a JSON object with exactly these fields:
		{
			"id": "",                     // Leave empty, will be filled in
			"validatorId": "",           // Leave empty, will be filled in
			"validatorName": "",         // Leave empty, will be filled in
			"message": "Your detailed discussion message here. Must reference other validators using |@Name| format",
			"support": false | true,            // Should be true if you support the statement, false otherwise
			"oppose": false | true,             // Should be true if you oppose the statement, false otherwise
			"question": false | true,           // Should be true if you are unsure, false otherwise
			"round": 0,                  // Leave as 0, will be filled in
			"timestamp": ""              // Leave empty, will be filled in
		}

		Requirements:
		1. The message should express your thoughts based on your personality traits
		2. The support, oppose, and question fields must be true or false, not null, and only one of them should be true
		3. When mentioning other validators, you MUST use |@Name| format
		4. Never invent or mention validators that aren't in the previous discussions
		5. Your response must be ONLY the JSON object - no other text before or after
		6. Leave id, validatorId, validatorName, round, and timestamp empty - they will be filled in later

		Do not include any additional text or formatting.`,
		agent.Name, strings.Join(agent.Traits, ", "), tx.Content)

	response := GenerateLLMResponseWithResearch(prompt, tx.Content, agent.Traits)

	var discussion Discussion
	if err := json.Unmarshal([]byte(response), &discussion); err != nil {
		fmt.Println("Error parsing LLM response:", err)
		return Discussion{}
	}

	// Fill in the required fields
	discussion.ID = uuid.New().String()
	discussion.ValidatorID = agent.ID
	discussion.ValidatorName = agent.Name
	discussion.Round = 1 // Initial discussion is always round 1
	discussion.Timestamp = time.Now()

	return discussion
}
