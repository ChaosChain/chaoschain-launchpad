package da

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/consensus"
)

// Constants for local storage
const (
	MASTER_INDEX_FILE = "eigenda_master_index.json"
	CONFIG_DIR        = ".chaoschain"
)

// MasterIndexConfig stores the configuration for the master index
type MasterIndexConfig struct {
	MasterIndexID string `json:"masterIndexId"`
	LastUpdated   int64  `json:"lastUpdated"`
}

// OffchainData represents the off-chain data stored in EigenDA for a specific chain.
type OffchainData struct {
	ChainID         string                 `json:"chainId"`
	BlockHash       string                 `json:"blockHash"`   // Block hash (used as thread ID)
	BlockHeight     int                    `json:"blockHeight"` // Block height
	Discussions     []consensus.Discussion `json:"discussions"`
	Votes           []Vote                 `json:"votes"`
	Outcome         string                 `json:"outcome"`
	AgentIdentities map[string]string      `json:"agentIdentities"`
	Timestamp       int64                  `json:"timestamp"` // When the data was created
}

// Vote represents an agent's vote off-chain.
type Vote struct {
	AgentID      string `json:"agentId"`
	VoteDecision string `json:"voteDecision"`
	Timestamp    int64  `json:"timestamp"`
}

// BlobReference stores the mapping between EigenDA blob ID, chain ID, and block information
type BlobReference struct {
	BlobID      string `json:"blobId"`      // EigenDA blob ID
	ChainID     string `json:"chainId"`     // Chain ID
	BlockHash   string `json:"blockHash"`   // Block hash (used as thread ID)
	BlockHeight int    `json:"blockHeight"` // Block height
	Timestamp   int64  `json:"timestamp"`   // When the blob was stored
	Outcome     string `json:"outcome"`     // Outcome of the consensus (accepted/rejected)
}

// MasterIndex represents the master index of all blob references
type MasterIndex struct {
	ChainIndices map[string]ChainIndex `json:"chainIndices"` // chainID -> ChainIndex
	LastUpdated  int64                 `json:"lastUpdated"`  // Timestamp of last update
}

// ChainIndex represents the index of blob references for a specific chain
type ChainIndex struct {
	BlobReferences map[string]BlobReference `json:"blobReferences"` // blockHash -> BlobReference
	LastUpdated    int64                    `json:"lastUpdated"`    // Timestamp of last update
}

// Global map to store blob references in memory
// In a production system, this would be stored in a database
var (
	blobReferencesLock sync.RWMutex
	blobReferences     = make(map[string]map[string]BlobReference) // chainID -> blockHash -> BlobReference
)

// Global variables for master index
var (
	masterIndexLock sync.RWMutex
	masterIndex     MasterIndex
	masterIndexID   string // The EigenDA blob ID for the master index
)

// StoreBlobReference stores a reference to an EigenDA blob
func StoreBlobReference(ref BlobReference) error {
	// Update in-memory map
	blobReferencesLock.Lock()

	// Initialize the map if it doesn't exist
	if _, ok := blobReferences[ref.ChainID]; !ok {
		blobReferences[ref.ChainID] = make(map[string]BlobReference)
	}

	// Add the reference, using blockHash as the key
	blobReferences[ref.ChainID][ref.BlockHash] = ref
	blobReferencesLock.Unlock()

	// Update master index
	masterIndexLock.Lock()
	defer masterIndexLock.Unlock()

	// Initialize chain index if it doesn't exist
	if _, ok := masterIndex.ChainIndices[ref.ChainID]; !ok {
		masterIndex.ChainIndices[ref.ChainID] = ChainIndex{
			BlobReferences: make(map[string]BlobReference),
			LastUpdated:    time.Now().Unix(),
		}
	}

	// Add the reference to the chain index
	chainIndex := masterIndex.ChainIndices[ref.ChainID]
	chainIndex.BlobReferences[ref.BlockHash] = ref
	chainIndex.LastUpdated = time.Now().Unix()
	masterIndex.ChainIndices[ref.ChainID] = chainIndex

	// Save the updated master index to EigenDA
	return saveMasterIndex()
}

// GetBlobReferencesForChain returns all blob references for a specific chain
func GetBlobReferencesForChain(chainID string) []BlobReference {
	masterIndexLock.RLock()
	defer masterIndexLock.RUnlock()

	var refs []BlobReference
	if chainIndex, ok := masterIndex.ChainIndices[chainID]; ok {
		for _, ref := range chainIndex.BlobReferences {
			refs = append(refs, ref)
		}
	}

	// Sort by block height (descending)
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].BlockHeight > refs[j].BlockHeight
	})

	return refs
}

// GetBlobReferenceByBlockHash returns the blob reference for a specific block hash
func GetBlobReferenceByBlockHash(chainID, blockHash string) (BlobReference, bool) {
	masterIndexLock.RLock()
	defer masterIndexLock.RUnlock()

	if chainIndex, ok := masterIndex.ChainIndices[chainID]; ok {
		if ref, ok := chainIndex.BlobReferences[blockHash]; ok {
			return ref, true
		}
	}
	return BlobReference{}, false
}

// GetBlobReferenceByHeight returns the blob reference for a specific block height
func GetBlobReferenceByHeight(chainID string, height int) (BlobReference, bool) {
	masterIndexLock.RLock()
	defer masterIndexLock.RUnlock()

	if chainIndex, ok := masterIndex.ChainIndices[chainID]; ok {
		for _, ref := range chainIndex.BlobReferences {
			if ref.BlockHeight == height {
				return ref, true
			}
		}
	}
	return BlobReference{}, false
}

// GetBlobReferenceByBlobID returns the blob reference for a specific blob ID
func GetBlobReferenceByBlobID(blobID string) (BlobReference, bool) {
	masterIndexLock.RLock()
	defer masterIndexLock.RUnlock()

	for _, chainIndex := range masterIndex.ChainIndices {
		for _, ref := range chainIndex.BlobReferences {
			if ref.BlobID == blobID {
				return ref, true
			}
		}
	}
	return BlobReference{}, false
}

// SaveOffchainData stores off-chain data into EigenDA using the global DataAvailabilityService.
// It marshals the off-chain data into a map and then stores it via StoreData.
func SaveOffchainData(data OffchainData) (string, error) {
	// Get the global DA service
	svc := GetGlobalDAService()
	if svc == nil {
		return "", fmt.Errorf("global DA service not initialized")
	}

	// Update the timestamp if needed
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().Unix()
	}

	// Ensure we have valid data to store
	if len(data.Discussions) == 0 && len(data.Votes) == 0 {
		return "", fmt.Errorf("no discussions or votes to store")
	}

	// Convert to a map for storage
	dataMap := map[string]interface{}{
		"chainId":         data.ChainID,
		"blockHash":       data.BlockHash,
		"blockHeight":     data.BlockHeight,
		"discussions":     data.Discussions,
		"votes":           data.Votes,
		"outcome":         data.Outcome,
		"agentIdentities": data.AgentIdentities,
		"timestamp":       data.Timestamp,
		"type":            "offchainData", // Add a type field to identify this as offchain data
	}

	// Store the data in EigenDA
	blobID, err := svc.StoreData(dataMap)
	if err != nil {
		return "", err
	}

	// Store the blob reference
	ref := BlobReference{
		BlobID:      blobID,
		ChainID:     data.ChainID,
		BlockHash:   data.BlockHash,
		BlockHeight: data.BlockHeight,
		Timestamp:   data.Timestamp,
		Outcome:     data.Outcome,
	}

	if err := StoreBlobReference(ref); err != nil {
		return blobID, fmt.Errorf("data stored but failed to update master index: %w", err)
	}

	return blobID, nil
}

// GetOffchainData retrieves off-chain data from EigenDA using the global DataAvailabilityService.
// It takes a dataID and returns the corresponding OffchainData.
func GetOffchainData(dataID string) (*OffchainData, error) {
	// Get the global DA service
	svc := GetGlobalDAService()
	if svc == nil {
		return nil, fmt.Errorf("global DA service not initialized")
	}

	// Retrieve the data from EigenDA
	dataMap, err := svc.RetrieveData(dataID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve offchain data: %w", err)
	}

	// Convert the map back to OffchainData
	jsonData, err := json.Marshal(dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retrieved data: %w", err)
	}

	var offchainData OffchainData
	if err := json.Unmarshal(jsonData, &offchainData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal offchain data: %w", err)
	}

	return &offchainData, nil
}

// ListOffchainData lists all off-chain data for a specific chain.
// This is a placeholder function that would need to be implemented with a proper
// indexing mechanism, as EigenDA doesn't provide a native way to list or query data.
func ListOffchainData(chainID string) ([]string, error) {
	// Get the global DA service
	svc := GetGlobalDAService()
	if svc == nil {
		return nil, fmt.Errorf("global DA service not initialized")
	}

	// In a real implementation, you would need to maintain an index of dataIDs
	// for each chain, possibly in a database or another storage mechanism.
	return nil, fmt.Errorf("listing offchain data is not implemented")
}

// InitializeMasterIndex loads the master index from EigenDA or creates a new one
func InitializeMasterIndex() error {
	masterIndexLock.Lock()
	defer masterIndexLock.Unlock()

	// Initialize empty master index
	masterIndex = MasterIndex{
		ChainIndices: make(map[string]ChainIndex),
		LastUpdated:  time.Now().Unix(),
	}

	// Try to load master index ID from local file
	config, err := loadMasterIndexConfig()
	if err == nil && config.MasterIndexID != "" {
		masterIndexID = config.MasterIndexID
		fmt.Printf("Loaded master index ID from local file: %s\n", masterIndexID)
	} else {
		fmt.Printf("No master index ID found in local file, will create a new one\n")
	}

	// Try to load existing master index from EigenDA
	if masterIndexID != "" {
		loadedIndex, err := loadMasterIndex(masterIndexID)
		if err == nil {
			masterIndex = *loadedIndex
			return nil
		}
		// Log the error but continue with a new master index
		fmt.Printf("Failed to load master index: %v, creating new one\n", err)
	}

	// Save the new master index to EigenDA
	if err := saveMasterIndex(); err != nil {
		return fmt.Errorf("failed to save master index: %w", err)
	}

	// Save the master index ID to local file
	return saveMasterIndexConfig()
}

// loadMasterIndex loads the master index from EigenDA
func loadMasterIndex(dataID string) (*MasterIndex, error) {
	// Get the global DA service
	svc := GetGlobalDAService()
	if svc == nil {
		return nil, fmt.Errorf("global DA service not initialized")
	}

	// Retrieve the data from EigenDA
	dataMap, err := svc.RetrieveData(dataID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve master index: %w", err)
	}

	// Convert the map back to MasterIndex
	jsonData, err := json.Marshal(dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal retrieved data: %w", err)
	}

	var index MasterIndex
	if err := json.Unmarshal(jsonData, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal master index: %w", err)
	}

	return &index, nil
}

// saveMasterIndex saves the master index to EigenDA
func saveMasterIndex() error {
	// Get the global DA service
	svc := GetGlobalDAService()
	if svc == nil {
		return fmt.Errorf("global DA service not initialized")
	}

	// Update the timestamp
	masterIndex.LastUpdated = time.Now().Unix()

	// Convert to a map for storage
	dataMap := map[string]interface{}{
		"chainIndices": masterIndex.ChainIndices,
		"lastUpdated":  masterIndex.LastUpdated,
		"type":         "masterIndex", // Add a type field to identify this as a master index
	}

	// Store the data in EigenDA
	blobID, err := svc.StoreData(dataMap)
	if err != nil {
		return fmt.Errorf("failed to store master index: %w", err)
	}

	// Update the master index ID
	masterIndexID = blobID

	// Save the master index ID to local file
	if err := saveMasterIndexConfig(); err != nil {
		return fmt.Errorf("failed to save master index config: %w", err)
	}

	return nil
}

// saveMasterIndexConfig saves the master index ID to a local file
func saveMasterIndexConfig() error {
	// Create config directory if it doesn't exist
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create config file
	configFile := filepath.Join(configDir, MASTER_INDEX_FILE)
	config := MasterIndexConfig{
		MasterIndexID: masterIndexID,
		LastUpdated:   time.Now().Unix(),
	}

	// Marshal config to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config to file
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// loadMasterIndexConfig loads the master index ID from a local file
func loadMasterIndexConfig() (*MasterIndexConfig, error) {
	configFile := filepath.Join(getConfigDir(), MASTER_INDEX_FILE)

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist")
	}

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal config from JSON
	var config MasterIndexConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// getConfigDir returns the path to the config directory
func getConfigDir() string {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return CONFIG_DIR
	}

	return filepath.Join(homeDir, CONFIG_DIR)
}
