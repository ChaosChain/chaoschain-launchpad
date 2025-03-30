package communication

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type WSEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	EventBlockVerdict                 = "BLOCK_VERDICT"
	EventAgentVote                    = "AGENT_VOTE"
	EventVotingResult                 = "VOTING_RESULT"
	EventAgentAlliance                = "AGENT_ALLIANCE"
	EventAgentRegistered              = "AGENT_REGISTERED"
	EventNewTransaction               = "NEW_TRANSACTION"
	EventChainCreated                 = "CHAIN_CREATED"
	EventTaskBreakdown                = "TASK_BREAKDOWN"
	EventTaskBreakdownFinal           = "TASK_BREAKDOWN_FINAL"
	EventTaskBreakdownRoundStart      = "TASK_BREAKDOWN_ROUND_START"
	EventTaskBreakdownRoundIteration  = "TASK_BREAKDOWN_ROUND_ITERATION"
	EventTaskDelegation               = "TASK_DELEGATION"
	EventTaskDelegationFinal          = "TASK_DELEGATION_FINAL"
	EventTaskDelegationRoundStart     = "TASK_DELEGATION_ROUND_START"
	EventTaskDelegationRoundIteration = "TASK_DELEGATION_ROUND_ITERATION"
	EventTaskAssignment               = "TASK_ASSIGNMENT"
	EventTaskBreakdownStarted         = "TASK_BREAKDOWN_STARTED"
	EventTaskBreakdownMessage         = "TASK_BREAKDOWN_MESSAGE"
	EventTaskBreakdownIteration       = "TASK_BREAKDOWN_ITERATION"
	EventTaskBreakdownCompleted       = "TASK_BREAKDOWN_COMPLETED"
	EventTaskDelegationStarted        = "TASK_DELEGATION_STARTED"
	EventTaskDelegationMessage        = "TASK_DELEGATION_MESSAGE"
	EventTaskDelegationCompleted      = "TASK_DELEGATION_COMPLETED"
	EventTaskDelegationIteration      = "TASK_DELEGATION_ITERATION"
)

type WebSocketManager struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan WSEvent
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

var (
	wsManager *WebSocketManager
	once      sync.Once
)

func GetWSManager() *WebSocketManager {
	once.Do(func() {
		wsManager = &WebSocketManager{
			clients:    make(map[*websocket.Conn]bool),
			broadcast:  make(chan WSEvent),
			register:   make(chan *websocket.Conn),
			unregister: make(chan *websocket.Conn),
		}
		go wsManager.run()
	})
	return wsManager
}

func (manager *WebSocketManager) run() {
	for {
		select {
		case client := <-manager.register:
			manager.mu.Lock()
			manager.clients[client] = true
			manager.mu.Unlock()

		case client := <-manager.unregister:
			manager.mu.Lock()
			if _, ok := manager.clients[client]; ok {
				delete(manager.clients, client)
				client.Close()
			}
			manager.mu.Unlock()

		case event := <-manager.broadcast:
			manager.mu.RLock()
			for client := range manager.clients {
				if err := client.WriteJSON(event); err != nil {
					log.Printf("WebSocket error: %v", err)
					client.Close()
					delete(manager.clients, client)
				}
			}
			manager.mu.RUnlock()
		}
	}
}

func BroadcastEvent(eventType string, payload interface{}) {
	event := WSEvent{
		Type:    eventType,
		Payload: payload,
	}
	GetWSManager().broadcast <- event
}

func (w *WebSocketManager) Register() chan<- *websocket.Conn {
	return w.register
}

func (w *WebSocketManager) Unregister() chan<- *websocket.Conn {
	return w.unregister
}
