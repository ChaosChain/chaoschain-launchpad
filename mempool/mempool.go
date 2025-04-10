package mempool

import (
	"sync"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/core"
)

var (
	// Map of chainID -> Mempool
	mempools  = make(map[string]*Mempool)
	mempoolMu sync.RWMutex
)

// Mempool stores pending transactions before they are added to a block
type Mempool struct {
	mu                       sync.Mutex
	transactions             map[string]core.Transaction
	expirationSec            int64  // Transactions expire after X seconds
	chainID                  string // Add chainID to mempool
	EphemeralBlockHashes     []string
	EphemeralVotes           []EphemeralVote
	EphemeralAgentIdentities map[string]string
}

// EphemeralVote represents a temporary vote stored in the mempool
type EphemeralVote struct {
	ID           string `json:"id"` // Unique identifier for the vote
	AgentID      string `json:"agentId"`
	VoteDecision string `json:"voteDecision"`
	Timestamp    int64  `json:"timestamp"`
}

// Initialize mempool separately
func InitMempool(chainID string, timeout int64) *Mempool {
	mempoolMu.Lock()
	defer mempoolMu.Unlock()

	mp := &Mempool{
		transactions:             make(map[string]core.Transaction),
		expirationSec:            timeout,
		chainID:                  chainID,
		EphemeralBlockHashes:     []string{},
		EphemeralVotes:           []EphemeralVote{},
		EphemeralAgentIdentities: make(map[string]string),
	}
	mempools[chainID] = mp
	return mp
}

// GetMempool returns the default mempool instance
func GetMempool(chainID string) *Mempool {
	mempoolMu.RLock()
	defer mempoolMu.RUnlock()
	return mempools[chainID]
}

// AddTransaction adds a new transaction to the mempool if valid
func (mp *Mempool) AddTransaction(tx interface{}) bool {
	transaction, ok := tx.(core.Transaction)
	if !ok {
		return false
	}

	// Verify transaction belongs to this chain
	if transaction.ChainID != mp.chainID {
		return false
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Ensure transaction is valid before adding
	if !transaction.VerifyTransaction(transaction.From) {
		return false
	}

	mp.transactions[transaction.Signature] = transaction
	return true
}

// GetPendingTransactions returns all pending transactions
func (mp *Mempool) GetPendingTransactions() []core.Transaction {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	var txs []core.Transaction
	for _, tx := range mp.transactions {
		txs = append(txs, tx)
	}
	return txs
}

// RemoveTransaction removes a transaction once it's included in a block
func (mp *Mempool) RemoveTransaction(txID string) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	delete(mp.transactions, txID)
}

// CleanupExpiredTransactions removes old transactions
func (mp *Mempool) CleanupExpiredTransactions() {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	now := time.Now().Unix()
	for id, tx := range mp.transactions {
		if now-tx.Timestamp > mp.expirationSec {
			delete(mp.transactions, id)
		}
	}
}

// Size returns the number of transactions in the mempool
func (mp *Mempool) Size() int {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	return len(mp.transactions)
}

// NewMempool creates a new mempool instance
func NewMempool(chainID string) *Mempool {
	return &Mempool{
		transactions:             make(map[string]core.Transaction),
		expirationSec:            3600, // 1 hour default
		chainID:                  chainID,
		EphemeralBlockHashes:     []string{},
		EphemeralVotes:           []EphemeralVote{},
		EphemeralAgentIdentities: make(map[string]string),
	}
}

// ClearTemporaryData resets temporary data after block finalization
func (mp *Mempool) ClearTemporaryData() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.EphemeralBlockHashes = []string{}
	mp.EphemeralVotes = []EphemeralVote{}
	mp.EphemeralAgentIdentities = make(map[string]string)
}
