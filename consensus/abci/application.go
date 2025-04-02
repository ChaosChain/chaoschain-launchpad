package abci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/registry"
	"github.com/NethermindEth/chaoschain-launchpad/utils"
	types "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/privval"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

type Application struct {
	chainID           string
	mu                sync.RWMutex
	discussions       map[string]map[string]bool
	validators        []types.ValidatorUpdate // Persistent validator set
	pendingValUpdates []types.ValidatorUpdate // Diffs to return in EndBlock
}

func NewApplication(chainID string) types.Application {
	return &Application{
		chainID:           chainID,
		discussions:       make(map[string]map[string]bool),
		validators:        make([]types.ValidatorUpdate, 0),
		pendingValUpdates: make([]types.ValidatorUpdate, 0),
	}
}

// Required ABCI methods
func (app *Application) Info(req types.RequestInfo) types.ResponseInfo {
	return types.ResponseInfo{
		Data:             "ChaosChain L2",
		Version:          "1.0.0",
		AppVersion:       1,
		LastBlockHeight:  0,
		LastBlockAppHash: []byte{},
	}
}

func (app *Application) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	// Use the validators from the genesis file
	app.validators = req.Validators

	// Log the validators we're using
	for i, val := range app.validators {
		log.Printf("Using validator %d: %v", i, val)
	}

	// For PoA, we need to ensure we have at least one validator
	if len(app.validators) == 0 {
		log.Printf("WARNING: No validators in genesis, consensus may not work properly")
	}

	// Log validators to debug
	log.Printf("InitChain with %d validators from genesis", len(app.validators))

	return types.ResponseInitChain{
		Validators: app.validators, // Return the validators from genesis
		ConsensusParams: &tmproto.ConsensusParams{
			Block: &tmproto.BlockParams{
				MaxBytes: 22020096, // 21MB
				MaxGas:   -1,
			},
			Evidence: &tmproto.EvidenceParams{
				MaxAgeNumBlocks: 100000,
				MaxAgeDuration:  172800000000000, // 48 hours
				MaxBytes:        1048576,         // 1MB
			},
			Validator: &tmproto.ValidatorParams{
				PubKeyTypes: []string{"ed25519"},
			},
			// Add PoA specific parameters
			Version: &tmproto.VersionParams{
				App: 1,
			},
		},
	}
}

func (app *Application) Query(req types.RequestQuery) types.ResponseQuery {
	return types.ResponseQuery{}
}

func (app *Application) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	return types.ResponseCheckTx{Code: 0}
}

func (app *Application) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	log.Printf("DeliverTx received: %X", req.Tx)

	// Decode transaction
	var tx core.Transaction
	if err := json.Unmarshal(req.Tx, &tx); err != nil {
		log.Printf("Failed to unmarshal transaction: %v", err)
		return types.ResponseDeliverTx{
			Code: 1,
			Log:  fmt.Sprintf("Invalid transaction format: %v", err),
		}
	}

	log.Printf("Delivering transaction: %+v", tx)

	// Handle different transaction types
	switch tx.Type {
	case "register_validator":
		// This is a validator registration transaction
		if len(tx.Data) == 0 {
			return types.ResponseDeliverTx{
				Code: 1,
				Log:  "Missing validator public key",
			}
		}

		// Create public key from bytes
		pubKey := ed25519.PubKey(tx.Data)

		// Register the validator with voting power
		app.RegisterValidator(pubKey, 100) // Give it some voting power

		log.Printf("Registered validator %s with pubkey %X", tx.From, tx.Data)

		validatorAddr := fmt.Sprintf("%X", pubKey.Address())
		if ok := registry.LinkAgentToValidator(app.chainID, tx.From, validatorAddr); !ok {
			log.Printf("Warning: Failed to link agent %s to validator %s", tx.From, validatorAddr)
		}

		return types.ResponseDeliverTx{
			Code: 0,
			Log:  fmt.Sprintf("Validator %s registered successfully", tx.From),
		}

	case "discuss_transaction":
		// Accept all discussion transactions by default
		log.Printf("Accepted discussion from validator %s", tx.From)
		return types.ResponseDeliverTx{
			Code: 0,
			Log:  fmt.Sprintf("Discussion accepted from %s", tx.From),
		}

	default:
		return types.ResponseDeliverTx{Code: 0}
	}
}

func (app *Application) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	return types.ResponseBeginBlock{}
}

func (app *Application) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	app.mu.Lock()
	defer app.mu.Unlock()

	log.Printf("EndBlock at height %d — %d new validator updates", req.Height, len(app.pendingValUpdates))

	// Log validator updates
	for i, update := range app.pendingValUpdates {
		log.Printf("Validator update %d: pubkey=%X, power=%d",
			i, update.PubKey.GetEd25519(), update.Power)
	}

	updates := app.pendingValUpdates
	app.pendingValUpdates = nil // Clear for next block

	return types.ResponseEndBlock{
		ValidatorUpdates: updates,
	}
}

func (app *Application) Commit() types.ResponseCommit {
	return types.ResponseCommit{}
}

func (app *Application) ListSnapshots(req types.RequestListSnapshots) types.ResponseListSnapshots {
	return types.ResponseListSnapshots{}
}

func (app *Application) OfferSnapshot(req types.RequestOfferSnapshot) types.ResponseOfferSnapshot {
	return types.ResponseOfferSnapshot{}
}

func (app *Application) LoadSnapshotChunk(req types.RequestLoadSnapshotChunk) types.ResponseLoadSnapshotChunk {
	return types.ResponseLoadSnapshotChunk{}
}

func (app *Application) ApplySnapshotChunk(req types.RequestApplySnapshotChunk) types.ResponseApplySnapshotChunk {
	return types.ResponseApplySnapshotChunk{}
}

// PrepareProposal is called when this validator is the proposer
func (app *Application) PrepareProposal(req types.RequestPrepareProposal) types.ResponsePrepareProposal {
	log.Printf("PrepareProposal called with %d transactions", len(req.Txs))

	app.mu.Lock()
	defer app.mu.Unlock()

	var validTxs [][]byte
	for _, tx := range req.Txs {
		var transaction core.Transaction
		if err := json.Unmarshal(tx, &transaction); err != nil {
			log.Printf("Failed to unmarshal transaction: %v", err)
			continue
		}

		if transaction.Type == "register_validator" {
			log.Printf("Including validator registration tx from %s", transaction.From)
			validTxs = append(validTxs, tx)
			continue
		}

		if transaction.Type == "discuss_transaction" {
			// Accept any discussion transaction that has content
			if transaction.Content != "" {
				log.Printf("Including discussion tx from %s with content: %s",
					transaction.From, transaction.Content)
				validTxs = append(validTxs, tx)
			} else {
				log.Printf("Rejecting empty discussion tx from %s", transaction.From)
			}
		}
	}

	return types.ResponsePrepareProposal{Txs: validTxs}
}

// ProcessProposal is called on all other validators to validate the block proposal
func (app *Application) ProcessProposal(req types.RequestProcessProposal) types.ResponseProcessProposal {
	app.mu.Lock()
	defer app.mu.Unlock()

	log.Printf("ProcessProposal called with %d transactions at height %d",
		len(req.Txs), req.Height)

	// We need to get the current validator's address, not the proposer's
	// This requires accessing the private validator key
	privValKeyFile := fmt.Sprintf("./data/%s/%s/config/priv_validator_key.json",
		app.chainID, os.Getenv("AGENT_ID"))
	privValStateFile := fmt.Sprintf("./data/%s/%s/data/priv_validator_state.json",
		app.chainID, os.Getenv("AGENT_ID"))

	if utils.FileExists(privValKeyFile) && utils.FileExists(privValStateFile) {
		privVal := privval.LoadFilePV(privValKeyFile, privValStateFile)
		pubKey, err := privVal.GetPubKey()
		if err == nil {
			currentValidatorAddr := fmt.Sprintf("%X", pubKey.Address())
			log.Printf("Current validator address: %s", currentValidatorAddr)

			// Get current validator's agent info
			currentAgent, exists := registry.GetAgentByValidator(app.chainID, currentValidatorAddr)
			if exists {
				log.Printf("Found agent %s for current validator", currentAgent.Name)

				// Process transactions with this validator's agent
				for i, tx := range req.Txs {
					var transaction core.Transaction
					if err := json.Unmarshal(tx, &transaction); err != nil {
						log.Printf("Failed to unmarshal transaction %d: %v", i, err)
						continue
					}

					log.Printf("Processing transaction %d: Type=%s, From=%s",
						i, transaction.Type, transaction.From)

					if transaction.Type == "discuss_transaction" {
						// Get current validator's discussion response
						discussion := ai.GetValidatorDiscussion(currentAgent, transaction)
						log.Printf("Got discussion response from agent %s: Support=%v",
							currentAgent.Name, discussion.Support)

						// Log the discussion
						utils.LogDiscussion(currentAgent.Name, discussion.Message, app.chainID, false)

						// Accept proposal only if current validator supports it
						if !discussion.Support {
							log.Printf("Current validator %s does not support the discussion: %s",
								currentAgent.Name, discussion.Message)
							return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_REJECT}
						}

						log.Printf("Current validator %s supports the discussion: %s",
							currentAgent.Name, discussion.Message)
					}
				}
			} else {
				log.Printf("No agent found for current validator %s", currentValidatorAddr)

				// List all registered validator-agent mappings for debugging
				log.Printf("Registered validator-agent mappings for chain %s:", app.chainID)
				mappings := registry.GetAllValidatorAgentMappings(app.chainID)
				for valAddr, agentID := range mappings {
					log.Printf("  Validator %s -> Agent %s", valAddr, agentID)
				}
			}
		} else {
			log.Printf("Failed to get validator public key: %v", err)
		}
	} else {
		log.Printf("Validator key files not found: %s, %s", privValKeyFile, privValStateFile)
	}

	return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}
}

// RegisterValidator adds a new validator to the set
func (app *Application) RegisterValidator(pubKey crypto.PubKey, power int64) {
	app.mu.Lock()
	defer app.mu.Unlock()

	valUpdate := types.Ed25519ValidatorUpdate(pubKey.Bytes(), power)

	// Log the validator being registered
	log.Printf("Registering validator with address: %X, power: %d", pubKey.Address(), power)

	// Check if validator already exists
	for _, val := range app.validators {
		if bytes.Equal(val.PubKey.GetEd25519(), pubKey.Bytes()) {
			// Already exists, no update needed
			log.Printf("Validator already exists, not adding again")
			return
		}
	}

	// Add to persistent set
	app.validators = append(app.validators, valUpdate)
	log.Printf("Added validator to persistent set, now have %d validators", len(app.validators))

	// Also include in the updates for EndBlock
	app.pendingValUpdates = append(app.pendingValUpdates, valUpdate)
	log.Printf("Added validator to pending updates, now have %d pending updates", len(app.pendingValUpdates))
}
