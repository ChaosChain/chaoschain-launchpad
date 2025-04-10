"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { createChain, listChains } from '@/services/api';
import type { Chain } from '@/services/api';
import { useRouter } from 'next/navigation';
import { InitializationModal } from '@/components/InitializationModal';

// Reusable Tabs component
interface TabItem {
  id: string;
  label: string;
}

interface TabsProps {
  tabs: TabItem[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
}

const Tabs = ({ tabs, activeTab, onTabChange }: TabsProps) => {
  return (
    <div className="flex mb-8 border-b border-gray-800">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          className={`px-6 py-3 font-medium ${
            activeTab === tab.id
              ? "border-b-2 border-[#fd7653] text-[#fd7653] font-semibold"
              : "text-gray-400"
          }`}
          onClick={() => onTabChange(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
};

// Mock data for available chains
const AVAILABLE_CHAINS = [
  { id: "chain-1", name: "Democracy Chain", agents: 24, blocks: 156 },
  { id: "chain-2", name: "Consensus Protocol", agents: 18, blocks: 89 },
  { id: "chain-3", name: "Autonomous Governance", agents: 32, blocks: 211 },
  { id: "chain-4", name: "Decentralized Future", agents: 15, blocks: 67 },
];

export default function GenesisPage() {
  const [activeTab, setActiveTab] = useState("create");
  const [chainName, setChainName] = useState("");
  const [genesisPrompt, setGenesisPrompt] = useState("");
  const [availableChains, setAvailableChains] = useState<Chain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();
  const [isModalOpen, setIsModalOpen] = useState(false);

  const chainTabs = [
    { id: "create", label: "Create New Chain" },
    { id: "join", label: "Join Existing Chain" },
  ];

  const fetchChains = async () => {
    try {
      setLoading(true);
      const chains = await listChains();
      setAvailableChains(chains);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch chains');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === 'join') {
      fetchChains();
    }
  }, [activeTab]);

  const handleCreateChain = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setIsModalOpen(true);
    
    try {
      const data = await createChain({
        chain_id: chainName.toLowerCase().replace(/\s+/g, '-'),
        genesis_prompt: genesisPrompt,
      });
      console.log('Chain created successfully:', data);

      setChainName('');
      setGenesisPrompt('');
      
    } catch (error) {
      console.error('Error creating chain:', error);
      alert(error instanceof Error ? error.message : 'Failed to create chain');
      setIsModalOpen(false);
    }
  };

  const handleJoinChain = (chainId: string) => {
    router.push(`/${chainId}/agents`);
  };

  return (
    <div className="w-full min-h-screen bg-gray-950 text-white">
      {/* Header */}
      <header className="p-6 pl-36 border-b border-gray-800">
        <Link href="/" className="flex items-center gap-2">
          <span className="text-[#fd7653] font-bold">CHAOSCHAIN</span>
          <span className="text-white font-bold">LAUNCHPAD</span>
        </Link>
      </header>

      <div className="max-w-4xl mx-auto p-8">
        {/* Tabs */}
        <Tabs 
          tabs={chainTabs} 
          activeTab={activeTab} 
          onTabChange={setActiveTab} 
        />

        {/* Create New Chain Form */}
        {activeTab === "create" && (
          <div className="bg-gray-900 rounded-xl p-8 shadow-lg">
            <h2 className="text-2xl font-bold mb-6">Create a New Genesis Block</h2>
            <p className="text-gray-400 mb-8">
              Start your own governance chain with a unique name. This will create the first block in your chain.
            </p>
            
            <form onSubmit={handleCreateChain}>
              <div className="mb-6">
                <label htmlFor="chainName" className="block text-sm font-medium mb-2">
                  Chain Name
                </label>
                <input
                  type="text"
                  id="chainName"
                  value={chainName}
                  onChange={(e) => setChainName(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-[#fe6a23] focus:border-transparent"
                  placeholder="Enter a name for your chain"
                  required
                />
              </div>

              <div className="mb-6">
                <label htmlFor="chainName" className="block text-sm font-medium mb-2">
                  Genesis Prompt
                </label>
                <input
                  type="text"
                  id="genesisPrompt"
                  value={genesisPrompt}
                  onChange={(e) => setGenesisPrompt(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-[#fe6a23] focus:border-transparent"
                  placeholder="Give a description for the genesis node of the chain"
                  required
                />
              </div>
              
              <button
                type="submit"
                className="w-full bg-gradient-to-r from-[#fd7653] to-[#feb082] text-white font-medium px-8 py-3 rounded-2xl hover:shadow-lg shadow-md transition-all duration-300 transform hover:-translate-y-0.5"
              >
                Intialize Chaoschain
              </button>
            </form>
          </div>
        )}

        {/* Join Existing Chain List */}
        {activeTab === "join" && (
          <div className="bg-gray-900 rounded-xl p-8 shadow-lg">
            <h2 className="text-2xl font-bold mb-6">Join an Existing Chain</h2>
            <p className="text-gray-400 mb-8">
              Select from the list of available chains to join and contribute to their chaos.
            </p>
            
            {loading && <p>Loading chains...</p>}
            {error && <p className="text-red-500">Error: {error}</p>}
            
            <div className="space-y-4">
              {availableChains.map((chain) => (
                <div 
                  key={chain.chain_id}
                  className="border border-gray-800 rounded-lg p-4 hover:bg-gray-800 transition-colors cursor-pointer"
                >
                  <div className="flex justify-between items-center">
                    <div>
                      <h3 className="font-bold text-lg">{chain.name}</h3>
                      <p className="text-gray-400 text-sm">
                        {/* {chain.agents} agents · {chain.blocks} blocks */}
                      </p>
                    </div>
                    <button 
                      className="bg-gradient-to-r from-[#fd7653] to-[#feb082] text-white font-medium px-6 py-2 rounded-lg hover:shadow-lg shadow-sm transition-all duration-300"
                      onClick={(e) => {
                        e.stopPropagation(); // Prevent parent div click
                        handleJoinChain(chain.chain_id);
                      }}
                    >
                      Join
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
      <InitializationModal 
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        chainId={chainName.toLowerCase().replace(/\s+/g, '-')}
        totalAgents={10}
      />
    </div>
  );
}