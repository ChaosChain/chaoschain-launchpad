package p2p

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
)

// Peer represents a node in the P2P network
type Peer struct {
	Address string
	Conn    net.Conn
}

// ChainConfig represents the configuration for a specific chain
type ChainConfig struct {
	ChainID    string
	P2PPort    int
	APIPort    int
	NetworkKey string // Optional: Could be used to further isolate networks
}

// Node manages peer connections and message handling
type Node struct {
	ChainID     string
	Peers       map[string]*Peer
	mu          sync.Mutex
	listener    net.Listener
	subscribers map[string][]func([]byte)
	port        int
}

var defaultNode = NewNode(ChainConfig{ChainID: "main", P2PPort: 8080})

// GetP2PNode returns the default P2P node instance
func GetP2PNode() *Node {
	return defaultNode
}

// NewNode initializes a new P2P network node
func NewNode(config ChainConfig) *Node {
	return &Node{
		ChainID:     config.ChainID,
		Peers:       make(map[string]*Peer),
		subscribers: make(map[string][]func([]byte)),
		port:        config.P2PPort,
	}
}

// StartServer starts listening for new connections
func (n *Node) StartServer(port int) {
	n.port = port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	n.listener = listener
	log.Printf("P2P server started on port %d\n", port)

	// Server is ready to accept connections
	go n.acceptConnections()
}

func (n *Node) acceptConnections() {
	for {
		conn, err := n.listener.Accept()
		if err != nil {
			log.Printf("Connection failed: %v", err)
			continue
		}
		go n.handleConnection(conn)
	}
}

// Add these constants
const (
	MAX_PEERS = 10 // Maximum number of peer connections
	MIN_PEERS = 3  // Minimum desired peer connections
)

// Add handshake struct at package level
type handshakeMsg struct {
	ChainID string `json:"chain_id"`
	Address string `json:"address"`
}

// ConnectToPeer connects to a peer at a given address
func (n *Node) ConnectToPeer(address string) {
	myAddr := fmt.Sprintf("localhost:%d", n.port)

	// Don't connect to self
	if address == myAddr {
		return
	}

	// Don't connect if we already have this peer
	n.mu.Lock()
	if _, exists := n.Peers[address]; exists {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	log.Printf("Node %s attempting to connect to peer at %s", myAddr, address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Failed to connect to peer %s: %v", address, err)
		return
	}

	// Send handshake
	handshake := handshakeMsg{
		ChainID: n.ChainID,
		Address: myAddr,
	}

	handshakeData, _ := json.Marshal(handshake)
	if _, err := conn.Write(handshakeData); err != nil {
		conn.Close()
		return
	}

	// Wait for handshake response
	buffer := make([]byte, 1024)
	bytesRead, err := conn.Read(buffer)
	if err != nil {
		conn.Close()
		return
	}

	var response handshakeMsg
	if err := json.Unmarshal(buffer[:bytesRead], &response); err != nil {
		conn.Close()
		return
	}

	// Verify chain ID
	if response.ChainID != n.ChainID {
		conn.Close()
		return
	}

	peer := &Peer{Address: address, Conn: conn}
	n.mu.Lock()
	n.Peers[address] = peer
	n.mu.Unlock()

	go n.listenToPeer(peer)
	log.Printf("Node %s connected to peer: %s\n", myAddr, address)
}

// handleConnection handles incoming peer connections
func (n *Node) handleConnection(conn net.Conn) {
	// Read initial handshake
	buffer := make([]byte, 1024)
	bytesRead, err := conn.Read(buffer)
	if err != nil {
		conn.Close()
		return
	}

	var handshake handshakeMsg
	if err := json.Unmarshal(buffer[:bytesRead], &handshake); err != nil {
		conn.Close()
		return
	}

	// Verify chain ID
	if handshake.ChainID != n.ChainID {
		log.Printf("Rejecting peer from different chain: %s", handshake.ChainID)
		conn.Close()
		return
	}

	myAddr := fmt.Sprintf("localhost:%d", n.port)

	// Use the address sent in handshake
	peerAddr := handshake.Address

	// Only accept connection if we don't have this peer and it's not ourselves
	n.mu.Lock()
	if _, exists := n.Peers[peerAddr]; exists || peerAddr == myAddr {
		n.mu.Unlock()
		conn.Close()
		return
	}

	peer := &Peer{Address: peerAddr, Conn: conn}
	n.Peers[peerAddr] = peer
	n.mu.Unlock()

	// Send handshake response
	response := handshakeMsg{
		ChainID: n.ChainID,
		Address: myAddr,
	}
	handshakeData, _ := json.Marshal(response)
	if _, err := conn.Write(handshakeData); err != nil {
		log.Printf("Failed to send handshake response: %v", err)
		conn.Close()
		return
	}

	go n.listenToPeer(peer)
	log.Printf("Node %s accepted connection from: %s\n", myAddr, peerAddr)
}

// listenToPeer listens for messages from a peer
func (p *Node) listenToPeer(peer *Peer) {
	defer peer.Conn.Close()

	for {
		buffer := make([]byte, 4096)
		n, err := peer.Conn.Read(buffer)
		if err != nil {
			log.Printf("Connection lost with %s", peer.Address)
			p.mu.Lock()
			delete(p.Peers, peer.Address)
			p.mu.Unlock()
			return
		}

		var msg Message
		err = json.Unmarshal(buffer[:n], &msg)
		log.Printf("Received message: %s", msg)
		if err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		go p.handleMessage(msg, peer)
	}
}

// handleMessage processes incoming messages
func (n *Node) handleMessage(msg Message, peer *Peer) {
	log.Printf("Received message from %s: %s", peer.Address, msg.Type)

	switch msg.Type {
	case "GET_PEERS":
		// Send our peer list
		n.mu.Lock()
		peerList := make([]string, 0)
		for addr := range n.Peers {
			peerList = append(peerList, addr)
		}
		n.mu.Unlock()

		n.BroadcastMessage(Message{
			Type: "PEER_LIST",
			Data: peerList,
		})

	case "PEER_LIST":
		// Connect to some new peers from the list
		if peerList, ok := msg.Data.([]string); ok {
			n.mu.Lock()
			currentPeerCount := len(n.Peers)
			n.mu.Unlock()

			// Connect to more peers if we're below minimum
			if currentPeerCount < MIN_PEERS {
				for _, addr := range peerList {
					n.ConnectToPeer(addr)
					if len(n.Peers) >= MAX_PEERS {
						break
					}
				}
			}
		}
		// case "TRANSACTION":
		// 	// Process incoming transaction
		// 	log.Println("Transaction received:", msg.Data)
		// case "BLOCK":
		// 	// Process new block
		// 	log.Println("New block received:", msg.Data)
		// case "VALIDATION":
		// 	// Process validation result
		// 	log.Println("Validation received:", msg.Data)
	}
}

// BroadcastMessage sends a message to all peers
func (p *Node) BroadcastMessage(msg Message) {
	p.mu.Lock()
	defer p.mu.Unlock()

	msgBytes, _ := json.Marshal(msg)
	for _, peer := range p.Peers {
		_, err := peer.Conn.Write(msgBytes)
		if err != nil {
			log.Printf("Failed to send message to %s: %v", peer.Address, err)
		}
	}
}

// Subscribe registers a callback for a specific message type
func (p *Node) Subscribe(msgType string, callback func([]byte)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscribers[msgType] = append(p.subscribers[msgType], callback)
}

// Publish sends a message to all subscribers of a specific type
func (p *Node) Publish(msgType string, data []byte) {
	p.mu.Lock()
	callbacks := p.subscribers[msgType]
	p.mu.Unlock()

	for _, callback := range callbacks {
		go callback(data)
	}
}

func (n *Node) GetPort() int {
	return n.port
}

func SetDefaultNode(n *Node) {
	defaultNode = n
}

func (n *Node) GetPeerCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.Peers)
}
