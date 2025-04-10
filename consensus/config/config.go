package config

import (
	"os"

	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
)

// DefaultConfig returns default config for the consensus node
func DefaultConfig(rootDir string) *cfg.Config {
	config := cfg.DefaultConfig()

	// Update paths
	config.BaseConfig.RootDir = rootDir
	config.BaseConfig.ProxyApp = "tcp://127.0.0.1:26658"

	// P2P Configuration
	config.P2P.ListenAddress = "tcp://0.0.0.0:26656"
	config.P2P.AllowDuplicateIP = true

	// Consensus Configuration
	config.Consensus.TimeoutCommit = 1000 // 1 second
	config.Consensus.SkipTimeoutCommit = false

	// RPC Configuration
	config.RPC.ListenAddress = "tcp://0.0.0.0:26657"

	return config
}

// InitFilesWithConfig initializes node configuration files
func InitFilesWithConfig(config *cfg.Config) error {
	// Create directories
	cfg.EnsureRoot(config.RootDir)

	// Generate validator key if not exists
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()

	if !tExists(privValKeyFile) {
		pv := privval.GenFilePV(privValKeyFile, privValStateFile)
		pv.Save()
	}

	// Generate node key if not exists
	nodeKeyFile := config.NodeKeyFile()
	_, err := p2p.LoadOrGenNodeKey(nodeKeyFile)
	if err != nil {
		return err
	}

	return nil
}

// tExists returns true if path exists
func tExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
