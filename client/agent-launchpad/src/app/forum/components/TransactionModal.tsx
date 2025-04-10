import { useState, useEffect } from 'react';
import { FiX } from 'react-icons/fi';
import { fetchAgents } from "@/services/api";
import type { Validator } from "@/services/api";

interface TransactionModalProps {
    onClose: () => void;
    onSubmit: (transaction: any) => void;
    chainId: string;
}

export default function TransactionModal({ onClose, onSubmit, chainId }: TransactionModalProps) {
    const [agents, setAgents] = useState<Validator[]>([]);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [formData, setFormData] = useState({
        from: '',
        to: '',
        amount: 20, // set to something for now
        fee: 5, // set to something for now
        content: '',
        timestamp: Math.floor(Date.now() / 1000), // Convert to epoch seconds
        type: 'submit_paper' // Add default type
    });

    useEffect(() => {
        const loadValidators = async () => {
            try {
                const validators = await fetchAgents(chainId);
                setAgents(validators);
            } catch (error) {
                console.error('Failed to fetch validators:', error);
            }
        };
        loadValidators();
    }, [chainId]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsSubmitting(true);
        try {
            const submissionData = {
                ...formData,
                content: formData.content.replace(/\\/g, '')
            };
            
            await onSubmit(submissionData);
            onClose();
        } catch (error) {
            console.error('Transaction submission failed:', error);
            // Optionally show error message to user
        } finally {
            setIsSubmitting(false);
        }
    };

    return (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
            <div className="bg-gray-900 p-6 rounded-lg w-full max-w-md">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold">Propose Transaction</h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-white">
                        <FiX size={24} />
                    </button>
                </div>
                
                <form onSubmit={handleSubmit} className="space-y-4">
                    <div>
                        <label className="block text-sm font-medium mb-1">Type of Transaction</label>
                        <select
                            value={formData.type}
                            onChange={(e) => setFormData({...formData, type: e.target.value})}
                            className="w-full bg-gray-800 rounded p-2"
                            required
                        >
                            <option value="submit_paper">Submit Paper</option>
                            <option value="loan_request">Loan Request</option>
                            <option value="discuss_transaction">Discuss Transaction</option>
                        </select>
                    </div>
                    
                    <div>
                        <label className="block text-sm font-medium mb-1">From</label>
                        <select
                            value={formData.from}
                            onChange={(e) => setFormData({...formData, from: e.target.value})}
                            className="w-full bg-gray-800 rounded p-2"
                            required
                        >
                            <option value="">Select agent</option>
                            {agents.map(agent => (
                                <option key={agent.ID} value={agent.ID}>{agent.Name}</option>
                            ))}
                        </select>
                    </div>

                    <div>
                        <label className="block text-sm font-medium mb-1">To</label>
                        <select
                            value={formData.to}
                            onChange={(e) => setFormData({...formData, to: e.target.value})}
                            className="w-full bg-gray-800 rounded p-2"
                            required
                        >
                            <option value="">Select agent</option>
                            {agents.map(agent => (
                                <option key={agent.ID} value={agent.ID}>{agent.Name}</option>
                            ))}
                        </select>
                    </div>

                    {/* <div>
                        <label className="block text-sm font-medium mb-1">Amount</label>
                        <input
                            type="number"
                            value={formData.amount}
                            onChange={(e) => setFormData({...formData, amount: parseInt(e.target.value)})}
                            className="w-full bg-gray-800 rounded p-2"
                            required
                        />
                    </div>

                    <div>
                        <label className="block text-sm font-medium mb-1">Fee</label>
                        <input
                            type="number"
                            value={formData.fee}
                            onChange={(e) => setFormData({...formData, fee: parseInt(e.target.value)})}
                            className="w-full bg-gray-800 rounded p-2"
                            required
                        />
                    </div> */}

                    <div>
                        <label className="block text-sm font-medium mb-1">Timestamp</label>
                        <input
                            type="text"
                            value={formData.timestamp}
                            className="w-full bg-gray-800 rounded p-2"
                            disabled
                        />
                    </div>

                    <div>
                        <label className="block text-sm font-medium mb-1">Content</label>
                        <textarea
                            value={formData.content}
                            onChange={(e) => setFormData({...formData, content: e.target.value})}
                            className="w-full bg-gray-800 rounded p-2"
                            required
                            rows={3}
                        />
                    </div>

                    <button
                        type="submit"
                        className="w-full bg-gradient-to-r from-[#fd7653] to-[#feb082] hover:opacity-90 text-white font-bold py-2 px-4 rounded"
                        disabled={isSubmitting}
                    >
                        {isSubmitting ? (
                            <div className="flex items-center justify-center">
                                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white mr-2"></div>
                                Submitting...
                            </div>
                        ) : (
                            'Submit Transaction'
                        )}
                    </button>
                </form>
            </div>
        </div>
    );
} 