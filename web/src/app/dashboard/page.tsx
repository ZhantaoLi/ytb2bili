"use client";

import { useState } from 'react';
import TaskQueueStats from '@/components/dashboard/TaskQueueStats';
import { ListChecks } from 'lucide-react';

export default function DashboardPage() {
  // const { user, loading, handleLoginSuccess, handleRefreshStatus, handleLogout } = useAuth();
  const [selectedVideoId, setSelectedVideoId] = useState<string | null>(null);

  const handleVideoSelect = (videoId: string) => {
    setSelectedVideoId(videoId);
  };

  return (
      <div className="bg-white dark:bg-[#131722]/80 backdrop-blur-xl rounded-lg shadow-md border border-transparent dark:border-white/[0.05]">
        <div className="p-6 border-b border-gray-200 dark:border-white/[0.05]">
          <div className="flex items-center space-x-3">
            <ListChecks className="w-5 h-5 text-gray-600 dark:text-gray-400" />
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">任务队列</h2>
          </div>
        </div>
        
        <div className="p-6">
          <TaskQueueStats onVideoSelect={handleVideoSelect} />
        </div>
      </div>
  );
}