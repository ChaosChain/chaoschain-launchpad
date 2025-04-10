import { useState, useEffect } from "react";
import { registerAgent, ApiError, fetchAgents } from "@/services/api";

// Shared Agent interface
export interface Agent {
  id: string;
  name: string;
  role: "producer" | "validator";
  traits: string[];
  style: string;
  influences: string[];
  mood: string;
}

export function useAgentsManager(chainId: string) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch existing agents when component mounts
  useEffect(() => {
    const loadExistingAgents = async () => {
      try {
        setIsLoading(true);
        const validators = await fetchAgents(chainId);
        console.log(validators);
        setAgents(validators.map(v => ({
          id: v.ID,
          name: v.Name,
          role: "validator",
          traits: v.Traits,
          style: v.Style,
          influences: v.Influences,
          mood: v.Mood
        })));
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch agents');
      } finally {
        setIsLoading(false);
      }
    };
    
    loadExistingAgents();
  }, [chainId]);

  const addAgent = async (agent: Agent) => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await registerAgent(agent, chainId);
      setAgents(prev => [...prev, { ...agent, id: response.agentID }]);
      return response;
    } catch (err) {
      const errorMessage = err instanceof ApiError 
        ? err.message 
        : 'Failed to register agent';
      setError(errorMessage);
      throw err;
    } finally {
      setIsLoading(false);
    }
  };

  const openModal = () => setIsModalOpen(true);
  const closeModal = () => setIsModalOpen(false);

  const requiredAgents = 3;
  const progressPercentage = Math.min((agents.length / requiredAgents) * 100, 100);

  return {
    agents,
    isModalOpen,
    isLoading,
    error,
    addAgent,
    openModal,
    closeModal,
    requiredAgents,
    progressPercentage,
  };
} 