"use client";

import Head from "next/head";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { FiArrowLeft } from "react-icons/fi";
import { useEffect, useState, useRef } from "react";
import { wsService } from "@/services/websocket";
import { fetchAgents } from "@/services/api";

interface AgentVote {
    validatorId: string;
    message: string;
    timestamp: string;
    type: "support" | "oppose" | "question";
    round: number;
    approval: boolean;
}

interface VotingResult {
    blockHeight: number;
    state: number;
    support: number;
    oppose: number;
    accepted: boolean;
    reason: string;
}

interface Validator {
    ID: string;
    Name: string;
}

// Helper function to parse and format text with @ mentions
const formatMessageWithMentions = (message: string) => {
  // Split the message by @ symbol and process each part
  const parts = message.split(/(\|@[^|]+\|)/g);
  return parts.map((part, index) => {
    if (part.startsWith('|@') && part.endsWith('|')) {
      // Style for @ mentions
      return (
        <span 
          key={index}
          className="inline-flex items-center px-2 py-0.5 mx-0.5 rounded-full text-xs font-medium bg-indigo-500 bg-opacity-20 border border-indigo-500 text-white"
        >
          {part.slice(1, -1)}
        </span>
      );
    }
    return part;
  });
};

export default function ThreadDetailPage() {
  // Track WebSocket connection status
  const wsConnectedRef = useRef(false);
  const params = useParams();
  const chainId = typeof params.chainId === 'string' ? params.chainId : "";
  const searchParams = useSearchParams();
  const [replies, setReplies] = useState<AgentVote[]>([]);
  const [votingResult, setVotingResult] = useState<VotingResult | null>(null);
  const [blockVerdict, setBlockVerdict] = useState<any | null>(null);
  const [validators, setValidators] = useState<{ [key: string]: string }>({});
  const [finalVerdict, setFinalVerdict] = useState<string | null>(null);
  const [currentRound, setCurrentRound] = useState<number>(0);

  // Use a ref to prevent duplicate "AGENT_VOTE" subscriptions
  const subscribedRef = useRef({
    agentVote: false,
    votingResult: false,
    blockVerdict: false,
  });

  // Ref for deduplication of AGENT_VOTE events.
  const receivedVotes = useRef(new Set<string>());

  // Get transaction from URL params
  const transaction = {
    content: searchParams.get('content') || '',
    from: searchParams.get('from') || '',
    to: searchParams.get('to') || '',
    amount: parseInt(searchParams.get('amount') || '0'),
    fee: parseInt(searchParams.get('fee') || '0'),
    timestamp: parseInt(searchParams.get('timestamp') || '0'),
    hash: searchParams.get('hash') || ''
  };

  // Calculate voting result from final round
  const calculateVotingResult = (replies: AgentVote[]) => {
    const round10Votes = replies.filter(reply => reply.round === 10);
    
    if (round10Votes.length === 0) return null;
    
    const support = round10Votes.filter(vote => vote.approval).length;
    const oppose = round10Votes.length - support;
    const accepted = support >= (round10Votes.length * 2/3);
    
    return {
      support,
      oppose,
      accepted,
      reason: accepted 
        ? `${support}/${round10Votes.length} validators approved (${Math.round(support/round10Votes.length * 100)}%)`
        : `Insufficient approvals: ${support}/${round10Votes.length} (${Math.round(support/round10Votes.length * 100)}%)`
    };
  };

  useEffect(() => {
    // Only connect if not already connected
    if (!wsConnectedRef.current) {
      wsService.connect();
      wsConnectedRef.current = true;
    }

    // AgentVote handler with deduplication.
    const handleAgentVote = (payload: AgentVote) => {
      // Create a unique id (using validatorId and timestamp).
      const uniqueId = `${payload.validatorId}-${payload.timestamp}-${payload.round}`;
      if (receivedVotes.current.has(uniqueId)) {
        // Already processed this vote.
        return;
      }
      receivedVotes.current.add(uniqueId);
      console.log("AGENT_VOTE received:", payload);
      setReplies(prev => {
        const exists = prev.some(reply => 
          reply.validatorId === payload.validatorId && 
          reply.timestamp === payload.timestamp &&
          reply.round === payload.round
        );
        const newReplies = [...prev];
        if (!exists) {
          newReplies.push(payload);
          // Update current round if this is a new higher round
          if (payload.round > currentRound) {
            setCurrentRound(payload.round);
          }
        }
        
        // Calculate voting result when we have round 10 votes
        const result = calculateVotingResult(newReplies);
        if (result) setVotingResult(result);
        
        return newReplies;
      });
    };

    const handleBlockVerdict = (payload: any) => {
      console.log("BLOCK_VERDICT received:", payload);
      setBlockVerdict(payload);
    };

    // Subscribe using the ref guard.
    if (!subscribedRef.current.agentVote) {
      wsService.subscribe("AGENT_VOTE", handleAgentVote);
      subscribedRef.current.agentVote = true;
    }

    if (!subscribedRef.current.blockVerdict) {
      wsService.subscribe("BLOCK_VERDICT", handleBlockVerdict);
      subscribedRef.current.blockVerdict = true;
    }

    // Cleanup subscriptions when the component unmounts
    return () => {
      wsService.unsubscribe("AGENT_VOTE", handleAgentVote);
      wsService.unsubscribe("BLOCK_VERDICT", handleBlockVerdict);

      // Reset our refs on unmount
      subscribedRef.current.agentVote = false;
      subscribedRef.current.blockVerdict = false;
      wsConnectedRef.current = false;
      receivedVotes.current.clear();
      
      // Disconnect WebSocket
      wsService.disconnect();
    };
  }, [chainId, currentRound]);

  useEffect(() => {
    const fetchValidatorData = async () => {
      try {
        const validators = await fetchAgents(chainId as string);
        const validatorMap = validators.reduce((acc: { [key: string]: string }, v: Validator) => {
          acc[v.ID] = v.Name;
          return acc;
        }, {});
        setValidators(validatorMap);
      } catch (error) {
        console.error('Failed to fetch validators:', error);
      }
    };

    fetchValidatorData();
  }, [chainId]);

  return (
    <>
      <Head>
        <title>{transaction.content || 'Loading...'} - Thread Detail</title>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>
      <div className="min-h-screen bg-gray-900 text-gray-100 p-8">
        {/* Navigation */}
        <div className="flex justify-between items-center mb-6">
          <Link href={`/${chainId}/forum`} legacyBehavior>
            <a className="inline-flex items-center text-green-400 hover:underline text-sm">
              <FiArrowLeft className="mr-1" />
              Back to Forum
            </a>
          </Link>
          <Link href="/chain" legacyBehavior>
            <a className="inline-flex items-center text-green-400 hover:underline text-sm">
              Back to Homepage
            </a>
          </Link>
        </div>

        {/* Thread Header */}
        <div className="bg-gray-800 p-8 rounded-lg shadow-lg">
          <div className="flex items-center space-x-4">
            <img
              src={`https://robohash.org/${transaction.from || ''}?size=80x80`}
              alt={validators[transaction.from || ''] || 'Loading...'}
              className="w-16 h-16 rounded-full border-2 border-indigo-500"
            />
            <div>
              <h1 className="text-3xl font-extrabold tracking-wide">
                {(JSON.parse(transaction.content).title || transaction.content) || 'Loading...'}
              </h1>
              <h2 className="text-lg text-gray-400">
                Hash: {transaction.hash}
              </h2>
              <p className="text-lg text-gray-400">
                Created by: {validators[transaction.from || ''] || 'Loading...'}
              </p>
            </div>
          </div>
          <div className="mt-6 grid grid-cols-2 gap-4 text-lg leading-relaxed">
            {/* <div>Amount: {transaction.amount || 0}</div>
            <div>Fee: {transaction.fee || 0}</div> */}
            <div>To: {validators[transaction.to || ''] || 'Loading...'}</div>
            <div>Time: {transaction ? new Date(transaction.timestamp * 1000).toLocaleString() : 'Loading...'}</div>
          </div>
        </div>

        {/* Discussion Rounds */}
        <div className="mt-8">
          <h2 className="text-2xl font-bold mb-4">Discussion Rounds</h2>
          {[...Array(currentRound + 1)].map((_, roundIndex) => (
            <div key={roundIndex} className="mb-8">
              <h3 className="text-xl font-semibold mb-4">Round {roundIndex}</h3>
              <div className="space-y-4">
                {replies
                  .filter(reply => reply.round === roundIndex)
                  .map((reply, index) => (
                    <div key={`${reply.validatorId}-${index}`} 
                         className="bg-gray-800 p-6 rounded-lg shadow-lg">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center space-x-4">
                          <img
                            src={`https://robohash.org/${reply.validatorId}?size=50x50`}
                            alt={validators[reply.validatorId] || reply.validatorId}
                            className="w-12 h-12 rounded-full border-2 border-indigo-500"
                          />
                          <div>
                            <p className="font-bold text-lg">
                              {validators[reply.validatorId] || `Validator ${reply.validatorId.slice(0, 8)}`}
                            </p>
                            <p className="text-sm text-gray-400">
                              {new Date(reply.timestamp).toLocaleString()}
                            </p>
                          </div>
                        </div>
                        <div className={`px-4 py-2 rounded-full text-sm font-semibold ${
                          reply.approval === true 
                            ? 'bg-emerald-500 bg-opacity-20 border-2 border-emerald-500 text-white' 
                            : reply.approval === false
                            ? 'bg-rose-500 bg-opacity-20 border-2 border-rose-500 text-white'
                            : 'bg-yellow-500 bg-opacity-20 border border-yellow-500 text-white'
                        }`}>
                          {reply.approval ? 'Approved' : 'Rejected'}
                        </div>
                      </div>
                      <div className="mt-4 text-gray-300 whitespace-pre-line">
                        {formatMessageWithMentions(reply.message)}
                      </div>
                    </div>
                  ))}
              </div>
            </div>
          ))}
        </div>

        {/* Show Final Verdict only after round 10 */}
        {currentRound >= 10 && votingResult && (
          <div className="mt-8 bg-gray-800 p-6 rounded-lg">
            <h2 className="text-2xl font-bold mb-4">Final Verdict</h2>
            <div className="grid grid-cols-2 gap-4">
              <div>Support: {votingResult.support}</div>
              <div>Oppose: {votingResult.oppose}</div>
              <div className={`col-span-2 text-xl font-semibold ${
                votingResult.accepted 
                  ? "text-emerald-500" 
                  : "text-rose-500"
              }`}>
                Status: {votingResult.accepted ? 'Accepted' : 'Rejected'}
              </div>
              <div className="col-span-2 text-gray-400">
                {votingResult.reason}
              </div>
            </div>
          </div>
        )}

        {/* Block Verdict */}
        {blockVerdict && (
          <div className="mt-8 bg-gray-800 p-6 rounded-lg">
            <h2 className="text-2xl font-bold mb-4">Block Verdict</h2>
            <pre className="bg-gray-900 p-4 rounded">
              {JSON.stringify(blockVerdict, null, 2)}
            </pre>
          </div>
        )}
      </div>
    </>
  );
}