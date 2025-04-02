import React, { useState, useEffect, useRef } from 'react';
import { wsService, DecisionStrategy, TaskBreakdownMessage, TaskBreakdownCompleted, TaskDelegationMessage, TaskDelegationCompleted, StrategyVote, StrategySelectedEvent } from '@/services/websocket';

interface TaskBreakdownPanelProps {
  chainId: string;
  blockHeight?: number;
}

export const TaskBreakdownPanel: React.FC<TaskBreakdownPanelProps> = ({ chainId, blockHeight }) => {
  const [strategies, setStrategies] = useState<DecisionStrategy[]>([]);
  const [strategyVotes, setStrategyVotes] = useState<StrategyVote[]>([]);
  const [selectedStrategy, setSelectedStrategy] = useState<DecisionStrategy | null>(null);
  const [messages, setMessages] = useState<TaskBreakdownMessage[]>([]);
  const [completedBreakdown, setCompletedBreakdown] = useState<TaskBreakdownCompleted | null>(null);
  const [delegationMessages, setDelegationMessages] = useState<TaskDelegationMessage[]>([]);
  const [completedDelegation, setCompletedDelegation] = useState<TaskDelegationCompleted | null>(null);

  useEffect(() => {
    console.log(`🔵 TaskBreakdownPanel mounted - chainId: ${chainId}, blockHeight: ${blockHeight}`);
    
    wsService.connect();
    
    const handleDecisionStrategy = (payload: any) => {
      console.log("🔵 Decision Strategy received:", payload);
      const strategy = payload.strategy || payload;
      setStrategies(prev => [...prev, {
        validatorId: strategy.validatorId,
        validatorName: strategy.validatorName,
        name: strategy.name,
        description: strategy.description,
        reasoning: strategy.reasoning,
        timestamp: strategy.timestamp || payload.timestamp
      }]);
    };

    const handleStrategyVote = (payload: StrategyVote) => {
      console.log("🔵 Strategy Vote received:", payload);
      setStrategyVotes(prev => [...prev, {
        validatorId: payload.validatorId,
        validatorName: payload.validatorName,
        strategyName: payload.strategyName,
        strategyDescription: payload.strategyDescription,
        reasoning: payload.reasoning,
        blockHeight: payload.blockHeight,
        timestamp: payload.timestamp
      }]);
    };

    const handleStrategySelected = (payload: any) => {
      console.log("🔵 Strategy Selected received:", payload);
      const strategy = payload.strategy || payload;
      setSelectedStrategy({
        validatorId: strategy.validatorId,
        validatorName: strategy.validatorName,
        name: strategy.name,
        description: strategy.description,
        reasoning: strategy.reasoning,
        timestamp: strategy.timestamp || payload.timestamp
      });
    };

    const handleTaskBreakdownMessage = (payload: TaskBreakdownMessage) => {
      console.log("🔵 Task Breakdown Message received:", payload);
      setMessages(prev => [...prev, payload]);
    };

    const handleTaskBreakdownCompleted = (payload: TaskBreakdownCompleted) => {
      console.log("🔵 Task Breakdown Completed received:", payload);
      setCompletedBreakdown(payload);
    };

    const handleTaskDelegationMessage = (payload: TaskDelegationMessage) => {
      console.log("🔵 Task Delegation Message received:", payload);
      setDelegationMessages(prev => [...prev, payload]);
    };

    const handleTaskDelegationCompleted = (payload: TaskDelegationCompleted) => {
      console.log("🔵 Task Delegation Completed received:", payload);
      setCompletedDelegation(payload);
    };
    
    wsService.subscribe("DECISION_STRATEGY", handleDecisionStrategy);
    wsService.subscribe("STRATEGY_VOTE", handleStrategyVote);
    wsService.subscribe("STRATEGY_SELECTED", handleStrategySelected);
    wsService.subscribe("TASK_BREAKDOWN_MESSAGE", handleTaskBreakdownMessage);
    wsService.subscribe("TASK_BREAKDOWN_COMPLETED", handleTaskBreakdownCompleted);
    wsService.subscribe("TASK_DELEGATION_MESSAGE", handleTaskDelegationMessage);
    wsService.subscribe("TASK_DELEGATION_COMPLETED", handleTaskDelegationCompleted);

    return () => {
      wsService.unsubscribe("DECISION_STRATEGY", handleDecisionStrategy);
      wsService.unsubscribe("STRATEGY_VOTE", handleStrategyVote);
      wsService.unsubscribe("STRATEGY_SELECTED", handleStrategySelected);
      wsService.unsubscribe("TASK_BREAKDOWN_MESSAGE", handleTaskBreakdownMessage);
      wsService.unsubscribe("TASK_BREAKDOWN_COMPLETED", handleTaskBreakdownCompleted);
      wsService.unsubscribe("TASK_DELEGATION_MESSAGE", handleTaskDelegationMessage);
      wsService.unsubscribe("TASK_DELEGATION_COMPLETED", handleTaskDelegationCompleted);
    };
  }, [chainId, blockHeight]);

  // Always render something for debugging
  return (
    <div className="bg-gray-900 rounded-lg p-6 mb-6">
      {/* <h2 className="text-xl font-bold mb-4 text-green-400">Block {blockHeight} Processing</h2> */}
      
      {/* Block Proposal and Voting Results */}
      <div className="mb-8">
        {/* <h3 className="text-lg font-bold mb-4 text-white">Block Proposal & Voting</h3> */}
        
        {/* Block Details Card */}
        {/* <div className="bg-gray-800 p-4 rounded-lg mb-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <span className="text-gray-400">Block Height:</span>
              <span className="ml-2 text-white">{blockHeight}</span>
            </div>
            <div>
              <span className="text-gray-400">Chain ID:</span>
              <span className="ml-2 text-white">{chainId}</span>
            </div>
          </div>
        </div> */}

        {/* Voting Results Card */}
        {/* <div className="bg-gray-800 p-4 rounded-lg">
          <div className="flex flex-col space-y-3">
            <h4 className="text-white font-semibold">Block Voting Results</h4>
            <div className="flex space-x-4">
              <span className="px-3 py-1 bg-green-900 text-green-200 rounded-full text-sm">
                Support: {strategyVotes.filter(v => v.strategyName === "SUPPORT").length}
              </span>
              <span className="px-3 py-1 bg-red-900 text-red-200 rounded-full text-sm">
                Oppose: {strategyVotes.filter(v => v.strategyName === "OPPOSE").length}
              </span>
            </div>
            <div className="text-sm text-gray-400">
              {strategyVotes.length > 0 ? 
                `${strategyVotes.length} validators have voted` : 
                'Waiting for validator votes...'}
            </div>
          </div>
        </div> */}

        {/* Recent Votes */}
        {/* {strategyVotes.length > 0 && (
          <div className="mt-4 bg-gray-800 p-4 rounded-lg">
            <h4 className="text-white font-semibold mb-3">Recent Votes</h4>
            <div className="space-y-2">
              {strategyVotes.slice(-5).reverse().map((vote, index) => (
                <div key={index} className="bg-gray-700 p-2 rounded flex items-center justify-between">
                  <div>
                    <span className="text-[#fd7653]">{vote.validatorName}</span>
                    <span className="text-sm text-gray-400 ml-2">voted</span>
                    <span className={`ml-2 px-2 py-0.5 rounded text-xs ${
                      vote.strategyName === "SUPPORT" ? 'bg-green-800 text-green-200' : 'bg-red-800 text-red-200'
                    }`}>
                      {vote.strategyName}
                    </span>
                  </div>
                  <div className="text-sm text-gray-400">
                    {new Date(vote.timestamp).toLocaleTimeString()}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )} */}
      </div>

      {/* Task Processing Section - Only show if block is accepted */}
      {completedBreakdown && (
        <div className="mt-12 pt-8 border-t-2 border-gray-800">
          <h3 className="text-lg font-bold mb-6 text-white">Task Processing</h3>
          
          {/* Phase Indicator */}
          <div className="flex items-center mb-8 space-x-4">
            <div className={`flex items-center ${!selectedStrategy ? 'text-[#fd7653]' : 'text-gray-500'}`}>
              <div className={`w-8 h-8 rounded-full flex items-center justify-center border-2 ${!selectedStrategy ? 'border-[#fd7653]' : 'border-gray-500'}`}>1</div>
              <span className="ml-2">Strategy Selection</span>
            </div>
            <div className="h-px w-8 bg-gray-700" />
            <div className={`flex items-center ${selectedStrategy && !completedBreakdown ? 'text-[#fd7653]' : 'text-gray-500'}`}>
              <div className={`w-8 h-8 rounded-full flex items-center justify-center border-2 ${selectedStrategy && !completedBreakdown ? 'border-[#fd7653]' : 'border-gray-500'}`}>2</div>
              <span className="ml-2">Task Breakdown</span>
            </div>
            <div className="h-px w-8 bg-gray-700" />
            <div className={`flex items-center ${completedBreakdown && !completedDelegation ? 'text-[#fd7653]' : 'text-gray-500'}`}>
              <div className={`w-8 h-8 rounded-full flex items-center justify-center border-2 ${completedBreakdown && !completedDelegation ? 'border-[#fd7653]' : 'border-gray-500'}`}>3</div>
              <span className="ml-2">Task Delegation</span>
            </div>
          </div>

          {/* Decision Strategy Section */}
          <div className="mb-12">
            <h3 className="text-lg font-bold mb-4 text-white flex items-center">
              <span className="w-6 h-6 rounded-full bg-gray-700 flex items-center justify-center text-sm mr-2">1</span>
              Decision Strategy Selection
            </h3>
            
            {strategies.length > 0 && (
              <div className="space-y-4">
                {strategies.map((strategy, index) => {
                  const strategyVoteCount = strategyVotes.filter(v => v.strategyName === strategy.name).length;
                  const recentVotes = strategyVotes
                    .filter(v => v.strategyName === strategy.name)
                    .slice(-3)
                    .reverse();

                  return (
                    <div key={index} className={`p-4 ${selectedStrategy?.name === strategy.name ? 'bg-green-900' : 'bg-gray-800'} rounded-lg`}>
                      <div className="flex items-center justify-between mb-3">
                        <div className="flex items-center">
                          <span className="text-lg font-semibold text-[#fd7653]">{strategy.validatorName}</span>
                          {selectedStrategy?.name === strategy.name && (
                            <span className="ml-2 px-2 py-1 bg-green-700 text-xs rounded">Selected Strategy</span>
                          )}
                        </div>
                        <div className="bg-gray-700 px-3 py-1 rounded">
                          <span className="text-gray-400">{strategyVoteCount} votes</span>
                        </div>
                      </div>

                      <div className="mb-3 space-y-2">
                        <div>
                          <span className="text-gray-400">Strategy:</span>{' '}
                          <span className="text-white font-medium">{strategy.name || 'Not specified'}</span>
                        </div>
                        {strategy.description && (
                          <div className="text-gray-300 text-sm">{strategy.description}</div>
                        )}
                        {strategy.reasoning && (
                          <div className="text-gray-400 text-sm italic">{strategy.reasoning}</div>
                        )}
                      </div>

                      {strategy.name === 'LEADER' && strategy.nominatedLeader && (
                        <div className="mb-3 bg-gray-700 p-3 rounded">
                          <div className="text-gray-300 font-medium mb-1">Nominated Leader</div>
                          <div className="text-[#fd7653]">{strategy.nominatedLeader.validatorName}</div>
                          <div className="text-sm text-gray-400 mt-1">{strategy.nominatedLeader.reasoning}</div>
                        </div>
                      )}

                      {recentVotes.length > 0 && (
                        <div className="mt-3 space-y-2">
                          <div className="text-gray-400 font-medium">Recent Votes</div>
                          {recentVotes.map((vote, voteIndex) => (
                            <div key={voteIndex} className="bg-gray-700 p-2 rounded">
                              <div className="flex items-center justify-between">
                                <span className="text-[#fd7653]">{vote.validatorName}</span>
                                {strategy.name === 'LEADER' && vote.supportNominatedLeader !== undefined && (
                                  <span className={`text-xs px-2 py-1 rounded ${vote.supportNominatedLeader ? 'bg-green-800 text-green-200' : 'bg-red-800 text-red-200'}`}>
                                    {vote.supportNominatedLeader ? 'Supports Leader' : 'Opposes Leader'}
                                  </span>
                                )}
                              </div>
                              <div className="text-sm text-gray-300 mt-1">{vote.reasoning}</div>
                              {strategy.name === 'LEADER' && vote.leaderVoteReasoning && (
                                <div className="text-sm text-gray-400 mt-1">
                                  Leader Vote: {vote.leaderVoteReasoning}
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Task Breakdown Section */}
          <div className="mb-12">
            <h3 className="text-lg font-bold mb-4 text-white flex items-center">
              <span className="w-6 h-6 rounded-full bg-gray-700 flex items-center justify-center text-sm mr-2">2</span>
              Task Breakdown
              {selectedStrategy && (
                <span className="ml-2 text-sm font-normal text-gray-400">
                  using {selectedStrategy.name} strategy
                </span>
              )}
            </h3>

            {messages.length > 0 && (
              <div className="space-y-3">
                {messages.map((message, index) => (
                  <div key={index} className="bg-gray-800 p-4 rounded-lg">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-[#fd7653] font-medium">{message.validatorName}</span>
                      <span className="px-2 py-1 bg-gray-700 text-xs rounded">{message.messageType}</span>
                    </div>
                    <div className="text-gray-300">{message.content}</div>
                    {message.proposal && message.proposal.length > 0 && (
                      <div className="mt-3 pl-3 border-l-2 border-gray-700">
                        <div className="text-gray-400 font-medium mb-2">Proposed Tasks:</div>
                        <ul className="space-y-1">
                          {message.proposal.map((task, taskIndex) => (
                            <li key={taskIndex} className="text-gray-300">{task}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {completedBreakdown && (
              <div className="mt-4 bg-green-900 p-4 rounded-lg">
                <div className="text-lg font-semibold text-white mb-3">Final Breakdown</div>
                <div className="space-y-2">
                  <div className="flex items-center">
                    <span className="text-gray-300">Consensus Score:</span>
                    <span className="ml-2 px-2 py-1 bg-green-800 rounded text-green-200">
                      {(completedBreakdown.consensusScore * 100).toFixed(1)}%
                    </span>
                  </div>
                  <div className="mt-3">
                    <div className="text-gray-300 mb-2">Approved Subtasks:</div>
                    <ul className="space-y-1 pl-4">
                      {completedBreakdown.subtasks.map((task, index) => (
                        <li key={index} className="text-gray-200">{task}</li>
                      ))}
                    </ul>
                  </div>
                  <div className="mt-3 text-gray-300">{completedBreakdown.summary}</div>
                </div>
              </div>
            )}
          </div>

          {/* Task Delegation Section */}
          <div className="mb-8">
            <h3 className="text-lg font-bold mb-4 text-white flex items-center">
              <span className="w-6 h-6 rounded-full bg-gray-700 flex items-center justify-center text-sm mr-2">3</span>
              Task Delegation
            </h3>

            {delegationMessages.length > 0 && (
              <div className="space-y-3">
                {delegationMessages.map((message, index) => (
                  <div key={index} className="bg-gray-800 p-4 rounded-lg">
                    <div className="flex items-center justify-between mb-3">
                      <span className="text-[#fd7653] font-medium">{message.validatorName}</span>
                      <span className="px-2 py-1 bg-gray-700 text-xs rounded">{message.messageType}</span>
                    </div>
                    <div className="text-gray-300 mb-3">{message.content}</div>
                    {message.assignments && (
                      <div className="mt-3 pl-3 border-l-2 border-gray-700">
                        <div className="text-gray-400 font-medium mb-2">Proposed Assignments:</div>
                        <div className="space-y-1">
                          {Object.entries(message.assignments).map(([task, assignee], i) => (
                            <div key={i} className="flex">
                              <span className="text-gray-300">{task}</span>
                              <span className="mx-2 text-gray-500">→</span>
                              <span className="text-[#fd7653]">{assignee}</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {completedDelegation && (
              <div className="mt-4 bg-green-900 p-4 rounded-lg">
                <div className="text-lg font-semibold text-white mb-3">Final Delegation</div>
                <div className="space-y-3">
                  <div className="text-gray-300">{completedDelegation.summary}</div>
                  <div className="mt-3">
                    <div className="text-gray-300 mb-2">Task Assignments:</div>
                    <div className="space-y-2 pl-4">
                      {Object.entries(completedDelegation.assignments || {}).map(([task, validator], index) => (
                        <div key={index} className="flex items-center">
                          <span className="text-gray-200">{task}</span>
                          <span className="mx-2 text-gray-400">→</span>
                          <span className="text-[#fd7653]">{validator}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}; 