// Simple test component for debugging the TaskBreakdownPanel
'use client';

import React, { useState } from 'react';
import { TaskBreakdownPanel } from './TaskBreakdownPanel';

export default function SimpleBreakdownTest() {
  const [showPanel, setShowPanel] = useState(false);
  
  return (
    <div className="p-8 bg-gray-900 min-h-screen text-white">
      <h1 className="text-2xl font-bold mb-4">TaskBreakdownPanel Test</h1>
      
      <button 
        onClick={() => setShowPanel(!showPanel)}
        className="px-4 py-2 bg-blue-500 text-white rounded mb-4"
      >
        {showPanel ? 'Hide Panel' : 'Show Panel'}
      </button>
      
      {showPanel && (
        <div className="mt-4 p-4 border border-gray-700 rounded">
          <TaskBreakdownPanel chainId="mainnet" blockHeight={1} />
        </div>
      )}
    </div>
  );
} 