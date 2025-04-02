// Add these event types for task breakdown and decisions
export interface DecisionStrategy {
  validatorId: string;
  validatorName: string;
  name: string;
  description: string;
  reasoning: string;
  timestamp: string;
  nominatedLeader?: {
    validatorId: string;
    validatorName: string;
    reasoning: string;
  };
}

export interface DecisionStrategyEvent {
  blockHeight: number;
  strategy: DecisionStrategy;
  timestamp: string;
  validatorId: string;
  validatorName: string;
}

export interface TaskBreakdownMessage {
  validatorId: string;
  validatorName: string;
  messageType: string;
  content: string;
  proposal?: string[];
  replyTo?: string;
  messageId: string;
  blockHeight: number;
  timestamp: string;
}

export interface TaskBreakdownCompleted {
  subtasks: string[];
  consensusScore: number;
  decisionStrategy: string;
  blockHeight: number;
  summary: string;
  timestamp: string;
}

// Add task delegation interfaces
export interface TaskDelegationMessage {
  validatorId: string;
  validatorName: string;
  messageType: string;
  content: string;
  assignments?: { [key: string]: string };
  messageId: string;
  timestamp: string;
}

export interface TaskDelegationCompleted {
  assignments: { [key: string]: string };
  summary: string;
  blockHeight: number;
  strategy: DecisionStrategy;
  timestamp: string;
}

export interface StrategySelectedEvent {
  blockHeight: number;
  strategy: DecisionStrategy;
  timestamp: string;
}

export interface StrategyVote {
  validatorId: string;
  validatorName: string;
  strategyName: string;
  strategyDescription: string;
  reasoning: string;
  blockHeight: number;
  timestamp: string;
  supportNominatedLeader?: boolean;
  leaderVoteReasoning?: string;
}

export interface TaskAssignment {
  validatorId: string;
  validatorName: string;
  subtasks: string[];
  blockHeight: number;
  blockHash: string;
  timestamp: string;
}

export type WebSocketEvent = 
  | { type: "DECISION_STRATEGY"; payload: DecisionStrategyEvent }
  | { type: "STRATEGY_SELECTED"; payload: StrategySelectedEvent }
  | { type: "STRATEGY_VOTE"; payload: StrategyVote }
  | { type: "TASK_BREAKDOWN_MESSAGE"; payload: TaskBreakdownMessage }
  | { type: "TASK_BREAKDOWN_COMPLETED"; payload: TaskBreakdownCompleted }
  | { type: "TASK_DELEGATION_MESSAGE"; payload: TaskDelegationMessage }
  | { type: "TASK_DELEGATION_COMPLETED"; payload: TaskDelegationCompleted }
  | { type: "TASK_ASSIGNMENT"; payload: TaskAssignment };

type WebSocketCallback = (event: any) => void;

class WebSocketService {
    private ws: WebSocket | null = null;
    private subscribers: Map<string, Set<Function>> = new Map();
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;

    connect() {
        if (this.ws?.readyState === WebSocket.OPEN) {
            return; // Already connected
        }
        
        this.ws = new WebSocket(process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:3000/ws');
        
        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.reconnectAttempts = 0;
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                console.log('🔄 WebSocket message received:', {
                    type: data.type,
                    payload: data.payload,
                    time: new Date().toISOString()
                });
                const subscribers = this.subscribers.get(data.type);
                if (subscribers && subscribers.size > 0) {
                    console.log(`✅ Found ${subscribers.size} subscribers for event "${data.type}"`);
                    subscribers.forEach(callback => callback(data.payload));
                } else {
                    console.log(`⚠️ No subscribers found for event "${data.type}"`);
                }
            } catch (error) {
                console.error('❌ Error processing WebSocket message:', error);
            }
        };

        this.ws.onclose = () => {
            if (this.reconnectAttempts < this.maxReconnectAttempts) {
                this.reconnectAttempts++;
                setTimeout(() => this.connect(), 1000 * this.reconnectAttempts);
            }
        };
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
            this.reconnectAttempts = 0;
        }
    }

    subscribe(eventType: string, callback: WebSocketCallback) {
        if (!this.subscribers.has(eventType)) {
            this.subscribers.set(eventType, new Set());
        }
        this.subscribers.get(eventType)?.add(callback);
        console.log(`➕ Subscribed to "${eventType}" event, total subscribers: ${this.subscribers.get(eventType)?.size}`);
    }

    unsubscribe(eventType: string, callback: WebSocketCallback) {
        const subscribers = this.subscribers.get(eventType);
        if (subscribers) {
            subscribers.delete(callback);
            console.log(`➖ Unsubscribed from "${eventType}" event, remaining subscribers: ${subscribers.size}`);
        }
    }

    // Debug methods
    getSubscribedEvents() {
        const events: Record<string, number> = {};
        this.subscribers.forEach((subscribers, event) => {
            events[event] = subscribers.size;
        });
        return events;
    }

    debugConnection() {
        return {
            isConnected: this.ws?.readyState === WebSocket.OPEN,
            readyState: this.ws?.readyState,
            subscribedEvents: this.getSubscribedEvents()
        };
    }
}

export const wsService = new WebSocketService(); 