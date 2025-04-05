package abci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/registry"
	"github.com/NethermindEth/chaoschain-launchpad/utils"
	types "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

type Application struct {
	chainID           string
	mu                sync.RWMutex
	discussions       map[string]map[string]bool
	selfValidatorAddr string
	validators        []types.ValidatorUpdate // Persistent validator set
	pendingValUpdates []types.ValidatorUpdate // Diffs to return in EndBlock
}

func NewApplication(chainID string, selfValidatorAddr string) types.Application {
	return &Application{
		chainID:           chainID,
		discussions:       make(map[string]map[string]bool),
		selfValidatorAddr: selfValidatorAddr,
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
	app.mu.Lock()
	defer app.mu.Unlock()

	// Check if this is the first time initializing the chain
	// isFirstInit := len(req.Validators) == 0

	log.Printf("the number of validators coming from the genesis is %d", len(req.Validators))

	// For subsequent nodes, get validators from the network
	client, err := rpchttp.New("tcp://localhost:26657", "/websocket")
	if err != nil {
		log.Printf("Failed to connect to network: %v", err)
		// Fallback to genesis validators if we can't connect
		app.validators = req.Validators
	} else {
		// Get current validator set from the network
		result, err := client.Validators(context.Background(), nil, nil, nil)
		if err != nil {
			log.Printf("Failed to get validators from network: %v", err)
			app.validators = req.Validators
		} else {
			// Convert network validators to ABCI validators
			app.validators = make([]types.ValidatorUpdate, len(result.Validators))
			for i, val := range result.Validators {
				app.validators[i] = types.Ed25519ValidatorUpdate(
					val.PubKey.Bytes(),
					val.VotingPower,
				)
			}
			log.Printf("Initialized with %d validators from network", len(app.validators))
		}
	}

	return types.ResponseInitChain{
		Validators: app.validators,
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

	if len(app.pendingValUpdates) > 0 {
		log.Printf("EndBlock at height %d - applying %d validator updates",
			req.Height, len(app.pendingValUpdates))

		// Add validators to the persistent set
		for _, update := range app.pendingValUpdates {
			app.validators = append(app.validators, update)
			log.Printf("Added validator to persistent set: %X", update.PubKey.GetEd25519())
		}

		updates := app.pendingValUpdates
		app.pendingValUpdates = nil // Clear for next block

		return types.ResponseEndBlock{
			ValidatorUpdates: updates,
		}
	}

	return types.ResponseEndBlock{}
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

	log.Printf("Processing transaction received: %X", req.Txs)

	utils.LogDiscussion("Validator", app.selfValidatorAddr, app.chainID, false)

	currentAgent, exists := registry.GetAgentByValidator(app.chainID, app.selfValidatorAddr)
	if !exists {
		log.Printf("No agent found for current validator %s", app.selfValidatorAddr)
		return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}
	}

	utils.LogDiscussion("Agent", currentAgent.Name, app.chainID, false)

	shouldReject := false

	for i, tx := range req.Txs {
		var transaction core.Transaction
		if err := json.Unmarshal(tx, &transaction); err != nil {
			log.Printf("Failed to unmarshal transaction %d: %v", i, err)
			continue
		}

		log.Printf("Processing transaction %d: Type=%s, From=%s",
			i, transaction.Type, transaction.From)

		if transaction.Type == "discuss_transaction" {
			log.Printf("Calling ai function for the following topic: %s %s %v %v",
				transaction.Content, currentAgent.Name, currentAgent.Traits, currentAgent.IsValidator)

			discussion := ai.GetValidatorDiscussion(currentAgent, transaction)

			utils.LogDiscussion(currentAgent.Name, discussion.Message, app.chainID, false)

			if !discussion.Support {
				log.Printf("Validator %s rejected discussion: %s", currentAgent.Name, transaction.Content)
				shouldReject = true
			}
		}
	}

	if shouldReject {
		return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_REJECT}
	}
	return types.ResponseProcessProposal{Status: types.ResponseProcessProposal_ACCEPT}
}

// RegisterValidator adds a new validator to the set
func (app *Application) RegisterValidator(pubKey crypto.PubKey, power int64) {
	app.mu.Lock()
	defer app.mu.Unlock()

	valUpdate := types.Ed25519ValidatorUpdate(pubKey.Bytes(), power)

	log.Printf("Registering validator with address: %X, power: %d", pubKey.Address(), power)

	// Check if validator already exists
	for _, val := range app.validators {
		if bytes.Equal(val.PubKey.GetEd25519(), pubKey.Bytes()) {
			log.Printf("Validator already exists, not adding again")
			return
		}
	}

	// Only add to pendingValUpdates first
	// The validator will be added to app.validators after the update is committed
	app.pendingValUpdates = append(app.pendingValUpdates, valUpdate)
	log.Printf("Added validator to pending updates, will be active in next block. Now have %d pending updates",
		len(app.pendingValUpdates))
}
