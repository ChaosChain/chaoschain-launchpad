package communication

// Event types - only those not defined in websocket.go
const (
	EventBlockProposed    = "BLOCK_PROPOSED"
	EventBlockValidated   = "BLOCK_VALIDATED"
	EventDecisionStrategy = "DECISION_STRATEGY"
	EventStrategyVote     = "STRATEGY_VOTE"
)
