package node

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/NethermindEth/chaoschain-launchpad/consensus/abci"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"

	tmlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/types"
)

type Node struct {
	cometCfg *cfg.Config
	node     *node.Node
	chainId  string
}

func NewNode(config *cfg.Config, chainId string, selfValidatorAddr string) (*Node, error) {
	// Initialize config files and keys
	cfg.EnsureRoot(config.RootDir) // This function returns void, no need to check error

	// Create ABCI application
	app := abci.NewApplication(chainId, selfValidatorAddr)

	genDoc, err := types.GenesisDocFromFile(config.GenesisFile())
	log.Printf("Genesis doc: %+v", genDoc)
	log.Printf("Genesis validators count: %d", len(genDoc.Validators))
	for i, v := range genDoc.Validators {
		log.Printf("Genesis validator %d: Address=%s, PubKey=%s, Power=%d, Name=%s",
			i, v.Address, v.PubKey, v.Power, v.Name)
	}
	if err != nil {
		log.Printf("Error reading genesis file: %v", err)
	}

	// Create node key
	nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load node key: %v", err)
	}

	// Create validator
	privValidator := privval.LoadOrGenFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	// Create genesis doc provider
	genDocProvider := func() (*types.GenesisDoc, error) {
		return types.GenesisDocFromFile(config.GenesisFile())
	}

	// Create node
	node, err := node.NewNode(
		config,
		privValidator,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		genDocProvider,
		node.DefaultDBProvider,
		node.DefaultMetricsProvider(config.Instrumentation),
		tmlog.NewTMLogger(tmlog.NewSyncWriter(os.Stdout)),
	)
	if err != nil {
		log.Fatalf("failed to create node NODE: %v", err)
		return nil, fmt.Errorf("failed to create node: %v", err)
	}

	return &Node{
		cometCfg: config,
		node:     node,
		chainId:  chainId,
	}, nil
}

func (n *Node) Start(ctx context.Context) error {
	if err := n.node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %v", err)
	}
	return nil
}

func (n *Node) Stop(ctx context.Context) error {
	return n.node.Stop()
}

func (n *Node) NodeInfo() p2p.NodeInfo {
	return n.node.NodeInfo().(p2p.DefaultNodeInfo)
}

func (n *Node) Config() *cfg.Config {
	return n.cometCfg
}
