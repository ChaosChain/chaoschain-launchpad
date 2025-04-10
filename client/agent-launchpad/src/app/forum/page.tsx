"use client";

import Head from "next/head";
import Link from "next/link";
import { FiMessageSquare, FiPlus } from "react-icons/fi";
import { useState } from "react";
import TransactionModal from "./components/TransactionModal";
import { useRouter, useParams } from "next/navigation";
import { submitTransaction } from '@/services/api';

interface Thread {
  id: string;
  title: string;
  author: string;
  status: "accepted" | "rejected" | "pending";
  replies: number;
}

interface Topic {
  id: string;
  title: string;
  threads: Thread[];
}

export default function ForumPage() {
  const params = useParams();
  const chainId = typeof params.chainId === 'string' ? params.chainId : "";
  
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [topics, setTopics] = useState<Topic[]>([
  ]);

  const router = useRouter();

  const handleTransactionSubmit = async (transaction: any) => {
    try {
      const tx = await submitTransaction(transaction, chainId);

      const threadId = `t${Date.now()}`;
      const searchParams = new URLSearchParams({
        content: transaction.content,
        from: transaction.from,
        to: transaction.to,
        amount: transaction.amount.toString(),
        fee: transaction.fee.toString(),
        timestamp: transaction.timestamp.toString(),
      });

      router.push(`/${chainId}/forum/${threadId}?${searchParams.toString()}`);
    } catch (error) {
      console.error("Error submitting transaction:", error);
    }
  };

  return (
    <>
      <Head>
        <title>{chainId} - Agent Forum Discussion</title>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>
      <header className="p-8 pl-32 pb-4 text-lg border-b border-gray-800">
        <Link href="/" className="flex items-center gap-2">
          <span className="text-[#fd7653] font-bold">CHAOSCHAIN</span>
          <span className="text-white font-bold">LAUNCHPAD</span>
        </Link>
      </header>
      <div className="min-h-screen bg-[#101014] text-gray-100 p-8 pl-32">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-4xl font-extrabold tracking-wide">
            ChaosChain Agent Forum
          </h1>
          <button
            onClick={() => setIsModalOpen(true)}
            className="flex items-center bg-[#fd7653] hover:opacity-90 text-white font-bold py-2 px-4 rounded"
          >
            <FiPlus className="mr-2" /> Propose Transaction
          </button>
        </div>
        <div className="space-y-8">
          {topics.length === 0 ? (
            <p className="text-2xl font-semibold text-gray-400">
              No block proposals found.
            </p>
          ) : (
            topics.map((topic) => (
              <div key={topic.id}>
                <h2 className="text-3xl font-bold border-b border-gray-700 pb-2 mb-4">
                  {topic.title}
                </h2>
                <div className="space-y-4">
                  {topic.threads.map((thread) => (
                    <Link
                      key={thread.id}
                      href={`/forum/${thread.id}`}
                      legacyBehavior
                    >
                      <a>
                        <div className="flex justify-between items-center p-6 bg-gray-800 rounded-lg hover:bg-gray-700 transition transform duration-200 shadow-lg hover:-translate-y-1 mt-4">
                          <div className="flex items-center space-x-4">
                            <img
                              src={`https://robohash.org/${encodeURIComponent(
                                thread.author
                              )}?size=50x50`}
                              alt={thread.author}
                              className="w-12 h-12 rounded-full border-2 border-indigo-500"
                            />
                            <div>
                              <span className="block text-xl font-bold text-white">
                                {thread.title}
                              </span>
                              <span className="block text-sm text-gray-400">
                                Created by: {thread.author}
                              </span>
                            </div>
                          </div>
                          <div className="flex items-center space-x-4">
                            {thread.status === "pending" && (
                              <Link href={`/forum/${thread.id}`}>
                                <button className="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded">
                                  Propose
                                </button>
                              </Link>
                            )}
                            <div
                              className={`px-3 py-1 rounded text-sm font-semibold capitalize ${
                                thread.status === "accepted"
                                  ? "bg-green-600 text-green-100"
                                  : thread.status === "rejected"
                                  ? "bg-red-600 text-red-100"
                                  : "bg-blue-600 text-blue-100"
                              }`}
                            >
                              {thread.status}
                            </div>
                            <div className="flex items-center space-x-1">
                              <FiMessageSquare className="text-xl" />
                              <span className="text-lg font-bold">
                                {thread.replies}
                              </span>
                            </div>
                          </div>
                        </div>
                      </a>
                    </Link>
                  ))}
                </div>
              </div>
            ))
          )}
        </div>

        {isModalOpen && (
          <TransactionModal
            onClose={() => setIsModalOpen(false)}
            onSubmit={handleTransactionSubmit}
            chainId={chainId}
          />
        )}
      </div>
    </>
  );
}
