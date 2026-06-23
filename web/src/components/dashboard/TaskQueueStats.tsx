'use client';

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { RefreshCw, Clock, Play, CheckCircle, Upload, AlertCircle, Trash2, X } from 'lucide-react';
import { apiFetch } from '@/lib/api';

interface Video {
  id: number;
  video_id: string;
  title: string;
  status: string;
  created_at: string;
  updated_at: string;
  task_steps?: TaskStep[];
  bili_bvid?: string;
}

interface TaskStep {
  step_name: string;
  step_order: number;
  status: string;
  start_time: string;
  end_time: string;
  error_msg: string;
  can_retry: boolean;
}

type TabType = 'all' | 'processing' | 'uploading' | 'uploaded' | 'completed' | 'failed';

interface TaskQueueStatsProps {
  onVideoSelect?: (videoId: string) => void;
}

export default function TaskQueueStats({ onVideoSelect }: TaskQueueStatsProps) {
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabType>('all');
  const [refreshing, setRefreshing] = useState(false);
  const [expandedVideoId, setExpandedVideoId] = useState<number | null>(null);
  const [detailedVideo, setDetailedVideo] = useState<Video | null>(null);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [deleteError, setDeleteError] = useState('');
  const [mounted, setMounted] = useState(false);
  const itemsPerPage = 10;

  useEffect(() => {
    setMounted(true);
  }, []);

  // 删除相关状态
  const [confirmTarget, setConfirmTarget] = useState<Video | null>(null); // 待确认删除的任务
  const [isDeleting, setIsDeleting] = useState(false); // 删除请求进行中

  // 运行中状态禁止删除：002 处理中 / 201 上传视频中 / 301 上传字幕中
  const isRunningStatus = (status: string) => ['002', '201', '301'].includes(status);

  // 删除核心：走 batch-delete，单条删除即 ids 只含一个。
  // 所见即所删——删哪条由前端按显式 ID 决定，后端只负责跳过运行中任务。
  const executeDelete = async (target: Video) => {
    setIsDeleting(true);
    setDeleteError('');
    try {
      const response = await apiFetch('/videos/batch-delete', {
        method: 'POST',
        body: JSON.stringify({ ids: [target.id] }),
      });
      const data = await response.json();
      if (data.code === 200 || data.code === 0) {
        const skipped = data.data?.skipped?.length || 0;
        setConfirmTarget(null);
        if (expandedVideoId === target.id) {
          setExpandedVideoId(null);
          setDetailedVideo(null);
        }
        await fetchVideos();
        if (skipped > 0) alert(data.message);
      } else {
        setDeleteError(`删除失败: ${data.message || '未知错误'}`);
      }
    } catch (error) {
      console.error('删除任务失败:', error);
      setDeleteError('删除请求失败，请检查后端服务');
    } finally {
      setIsDeleting(false);
    }
  };

  const handleDelete = () => {
    if (confirmTarget) executeDelete(confirmTarget);
  };

  useEffect(() => {
    fetchVideos();
    const interval = setInterval(fetchVideos, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchVideos = async () => {
    try {
      setRefreshing(true);
      const response = await apiFetch('/videos?page=1&limit=1000');
      const data = await response.json();
      
      console.log('视频数据响应:', data); // 调试日志
      
      if ((data.code === 0 || data.code === 200) && data.data) {
        const videos = data.data.videos || [];
        setVideos(videos);
        console.log('成功加载视频:', videos.length); // 调试日志
      } else {
        // 如果没有数据，设置为空数组
        setVideos([]);
        console.log('没有视频数据，设置为空数组');
      }
    } catch (error) {
      console.error('获取视频列表失败:', error);
      setVideos([]); // 出错时也设置为空数组
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const handleToggleDetails = async (videoId: number) => {
    if (expandedVideoId === videoId) {
      setExpandedVideoId(null);
      setDetailedVideo(null);
    } else {
      setExpandedVideoId(videoId);
      setIsDetailLoading(true);
      try {
        const response = await apiFetch(`/videos/${videoId}`);
        const data = await response.json();
        if (data.code === 200 || data.code === 0) {
          setDetailedVideo(data.data);
        } else {
          console.error('Failed to fetch video details:', data.message);
        }
      } catch (error) {
        console.error('Error fetching video details:', error);
      } finally {
        setIsDetailLoading(false);
      }
    }
  };

  const handleRetryStep = async (videoId: number, stepName: string) => {
    try {
      const response = await apiFetch(`/videos/${videoId}/steps/${stepName}/retry`, {
        method: 'POST',
      });
      const data = await response.json();
      if (data.code === 200 || data.code === 0) {
        // 刷新详情
        handleToggleDetails(videoId);
      } else {
        console.error('Failed to retry step:', data.message);
      }
    } catch (error) {
      console.error('Error retrying step:', error);
    }
  };
  
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-400';
      case 'failed':
        return 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-400';
      case 'running':
        return 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-400';
      case 'pending':
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300';
    }
  };

  // 状态映射
  const getStatusInfo = (status: string) => {
    const statusMap: { [key: string]: { label: string; color: string; icon: any; category: TabType } } = {
      '001': { label: '待处理', color: 'bg-gray-100 text-gray-700 dark:text-gray-300', icon: Clock, category: 'processing' },
      '002': { label: '处理中', color: 'bg-blue-100 text-blue-700', icon: Play, category: 'processing' },
      '200': { label: '准备就绪', color: 'bg-green-100 text-green-700', icon: CheckCircle, category: 'processing' },
      '250': { label: '已准备（不上传）', color: 'bg-emerald-100 text-emerald-700', icon: CheckCircle, category: 'completed' },
      '201': { label: '上传视频中', color: 'bg-purple-100 text-purple-700', icon: Upload, category: 'uploading' },
      '299': { label: '上传失败', color: 'bg-red-100 text-red-700', icon: AlertCircle, category: 'failed' },
      '300': { label: '视频已上传', color: 'bg-cyan-100 text-cyan-700', icon: CheckCircle, category: 'uploaded' },
      '301': { label: '上传字幕中', color: 'bg-indigo-100 text-indigo-700', icon: Upload, category: 'uploading' },
      '399': { label: '字幕上传失败', color: 'bg-orange-100 text-orange-700', icon: AlertCircle, category: 'failed' },
      '400': { label: '全部完成', color: 'bg-emerald-100 text-emerald-700', icon: CheckCircle, category: 'completed' },
      '999': { label: '任务失败', color: 'bg-red-100 text-red-700', icon: AlertCircle, category: 'failed' },
    };
    return statusMap[status] || { label: '未知', color: 'bg-gray-100 text-gray-700 dark:text-gray-300', icon: AlertCircle, category: 'all' };
  };

  // 获取当前阶段描述
  const getStageDescription = (status: string) => {
    const stageMap: { [key: string]: string } = {
      '001': '等待开始处理',
      '002': '正在执行准备任务链（下载视频→生成字幕→翻译字幕→生成元数据）',
      '200': '准备阶段完成，等待视频上传（每小时上传1个）',
      '250': '准备阶段完成，但按要求保留在本地，不进入自动上传队列',
      '201': '正在上传视频到Bilibili',
      '299': '视频上传失败，需要重试',
      '300': '视频已上传，等待1小时后上传字幕',
      '301': '正在上传字幕到Bilibili',
      '399': '字幕上传失败，需要重试',
      '400': '所有任务已完成',
      '999': '准备阶段失败，需要检查任务步骤',
    };
    return stageMap[status] || '未知状态';
  };

  // 分类视频
  const categorizeVideos = () => {
    return {
      processing: videos.filter(v => ['001', '002', '200'].includes(v.status)),
      uploading: videos.filter(v => ['201', '301'].includes(v.status)),
      uploaded: videos.filter(v => v.status === '300'),
      completed: videos.filter(v => ['250', '400'].includes(v.status)),
      failed: videos.filter(v => ['299', '399', '999'].includes(v.status)),
    };
  };

  const categories = categorizeVideos();
  const filteredVideos = activeTab === 'all' ? videos : categories[activeTab as keyof typeof categories];

  // 分页逻辑
  const totalItems = filteredVideos.length;
  const totalPages = Math.ceil(totalItems / itemsPerPage);
  const paginatedVideos = filteredVideos.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );

  const handlePageChange = (page: number) => {
    if (page >= 1 && page <= totalPages) {
      setCurrentPage(page);
    }
  };

  const tabs = [
    { key: 'all', label: '全部', count: videos.length },
    { key: 'processing', label: '处理中', count: categories.processing.length },
    { key: 'uploading', label: '上传中', count: categories.uploading.length },
    { key: 'uploaded', label: '已上传', count: categories.uploaded.length },
    { key: 'completed', label: '已完成', count: categories.completed.length },
    { key: 'failed', label: '失败', count: categories.failed.length },
  ];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4"></div>
          <p className="text-gray-600 dark:text-gray-400">加载任务数据...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 标题栏 */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">任务管理</h2>
        <div className="flex items-center space-x-2">
          <button
            onClick={fetchVideos}
            disabled={refreshing}
            className="flex items-center space-x-2 px-3 py-1.5 md:px-4 md:py-2 text-xs md:text-sm text-gray-600 dark:text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            <span>刷新</span>
          </button>
        </div>
      </div>

      {/* 标签页 */}
      <div className="border-b border-gray-200 dark:border-white/[0.05]">
        <div className="flex space-x-4 md:space-x-8 overflow-x-auto whitespace-nowrap scrollbar-hide pb-1">
          {tabs.map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key as TabType)}
              className={`pb-3 px-1 text-xs md:text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.key
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:text-gray-300 hover:border-gray-300 dark:border-gray-600'
              }`}
            >
              {tab.label}
              {tab.count > 0 && (
                <span className={`ml-2 text-xs px-2 py-0.5 rounded ${
                  activeTab === tab.key ? 'bg-blue-100' : 'bg-gray-100'
                }`}>
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* 视频处理任务链 */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-base md:text-lg font-semibold text-gray-900 dark:text-white">视频处理任务链</h3>
        </div>
        <div className="bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-lg border border-gray-200 dark:border-white/[0.05] shadow-sm dark:shadow-none">
          {paginatedVideos.length === 0 ? (
            <div className="p-12 text-center text-gray-500 dark:text-gray-400">
              暂无任务数据
            </div>
          ) : (
            <div className="flex flex-col space-y-1 py-2">
              {paginatedVideos.map(video => {
                const statusInfo = getStatusInfo(video.status);
                const Icon = statusInfo.icon;
                return (
                  <div key={video.id}>
                    <div
                      className="p-4 hover:bg-gray-50 dark:hover:bg-white/[0.03] transition-colors cursor-pointer mx-2 rounded-xl"
                      onClick={() => handleToggleDetails(video.id)}
                    >
                      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
                        <div className="flex-1 min-w-0">
                          <div className="flex flex-wrap items-center gap-2 mb-2">
                            <h4 className="font-medium text-gray-900 dark:text-white truncate">
                              {video.title || video.video_id}
                            </h4>
                            <span className={`flex-shrink-0 flex items-center space-x-1 text-[10px] sm:text-xs px-2 py-1 rounded ${statusInfo.color}`}>
                              <Icon className="w-3 h-3" />
                              <span>{statusInfo.label}</span>
                            </span>
                            {video.bili_bvid && (
                              <a
                                href={`https://www.bilibili.com/video/${video.bili_bvid}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-xs text-blue-600 hover:underline"
                              >
                                {video.bili_bvid}
                              </a>
                            )}
                          </div>
                          <p className="text-xs md:text-sm text-gray-600 dark:text-gray-400 mb-3 line-clamp-2">
                            {getStageDescription(video.status)}
                          </p>
                          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                            <span className="truncate">视频ID: {video.video_id}</span>
                            <span>创建: {new Date(video.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                            <span className="hidden sm:inline">更新: {new Date(video.updated_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                          </div>
                        </div>
                        <div className="flex items-center justify-end space-x-2 sm:ml-4 flex-shrink-0">
                          <button
                            className="px-3 py-1.5 md:px-4 md:py-2 text-xs md:text-sm text-blue-600 hover:bg-blue-50 rounded transition-colors"
                          >
                            {expandedVideoId === video.id ? '收起详情' : '查看详情'}
                          </button>
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              if (isRunningStatus(video.status)) return;
                              setConfirmTarget(video);
                            }}
                            disabled={isRunningStatus(video.status) || isDeleting}
                            title={isRunningStatus(video.status) ? '运行中任务不能删除' : '删除该任务及本地文件'}
                            className="p-2 text-xs md:text-sm text-red-600 hover:bg-red-50 rounded transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                    {expandedVideoId === video.id && (
                      <div className="p-4 border-t border-gray-200 dark:border-white/[0.05] bg-gray-50 dark:bg-white/[0.02]">
                        {isDetailLoading ? (
                          <div className="text-center text-gray-500 dark:text-gray-400">加载任务步骤...</div>
                        ) : detailedVideo && detailedVideo.task_steps ? (
                          <TaskStepDetail 
                            steps={detailedVideo.task_steps} 
                            onRetry={(stepName) => handleRetryStep(video.id, stepName)}
                          />
                        ) : (
                          <div className="text-center text-gray-500 dark:text-gray-400">无任务步骤信息</div>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* 分页控件 */}
      {totalPages > 1 && (
        <div className="flex justify-center items-center space-x-2 mt-6">
          <button
            onClick={() => handlePageChange(currentPage - 1)}
            disabled={currentPage === 1}
            className="px-3 py-1 text-xs md:text-sm text-gray-600 dark:text-gray-400 bg-white dark:bg-white/[0.02] border border-gray-300 dark:border-white/[0.05] rounded-md hover:bg-gray-50 dark:hover:bg-white/[0.05] disabled:opacity-50"
          >
            上一页
          </button>
          <span className="text-xs md:text-sm text-gray-700 dark:text-gray-300">
            第 {currentPage} 页 / 共 {totalPages} 页
          </span>
          <button
            onClick={() => handlePageChange(currentPage + 1)}
            disabled={currentPage === totalPages}
            className="px-3 py-1 text-xs md:text-sm text-gray-600 dark:text-gray-400 bg-white dark:bg-white/[0.02] border border-gray-300 dark:border-white/[0.05] rounded-md hover:bg-gray-50 dark:hover:bg-white/[0.05] disabled:opacity-50"
          >
            下一页
          </button>
        </div>
      )}

      {/* 自动化调度说明 */}
      <div>
        <h3 className="text-base md:text-lg font-semibold text-gray-900 dark:text-white mb-4">自动化调度策略</h3>
        <div className="bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05]">
          <div className="p-6 space-y-3 md:space-y-4">
            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0 w-8 h-8 bg-blue-100 rounded-lg flex items-center justify-center">
                <Play className="w-4 h-4 text-blue-600" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-gray-900 dark:text-white mb-1">准备阶段任务链</h4>
                <p className="text-xs md:text-sm text-gray-600 dark:text-gray-400">
                  每5秒检查一次待处理任务，依次执行：下载视频 → 生成字幕 → 翻译字幕 → 生成元数据
                </p>
                <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  当前: {categories.processing.length} 个
                </div>
              </div>
            </div>

            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0 w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center">
                <Upload className="w-4 h-4 text-purple-600" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-gray-900 dark:text-white mb-1">视频上传调度</h4>
                <p className="text-xs md:text-sm text-gray-600 dark:text-gray-400">
                  每5分钟检查一次，每小时自动上传1个准备就绪的视频到Bilibili
                </p>
                <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  上传中: {categories.uploading.length} 个
                </div>
              </div>
            </div>

            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0 w-8 h-8 bg-indigo-100 rounded-lg flex items-center justify-center">
                <Upload className="w-4 h-4 text-indigo-600" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-gray-900 dark:text-white mb-1">字幕上传调度</h4>
                <p className="text-xs md:text-sm text-gray-600 dark:text-gray-400">
                  视频上传完成1小时后，自动上传对应的字幕文件
                </p>
                <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  等待上传字幕: {videos.filter(v => v.status === '300').length} 个
                </div>
              </div>
            </div>

            {categories.failed.length > 0 && (
              <div className="flex items-start space-x-3 p-3 bg-red-50 rounded-lg">
                <div className="flex-shrink-0 w-8 h-8 bg-red-100 rounded-lg flex items-center justify-center">
                  <AlertCircle className="w-4 h-4 text-red-600" />
                </div>
                <div className="flex-1">
                  <h4 className="font-medium text-red-900 mb-1">失败任务提醒</h4>
                  <p className="text-xs md:text-sm text-red-700">
                    当前有 {categories.failed.length} 个任务失败，请查看详情并手动重试
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 统计汇总 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05] p-4 shadow-sm dark:shadow-none">
          <div className="text-xs md:text-sm text-gray-600 dark:text-gray-400 mb-1">总任务数</div>
          <div className="text-2xl font-bold text-gray-900 dark:text-white">{videos.length}</div>
        </div>
        <div className="bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05] p-4 shadow-sm dark:shadow-none">
          <div className="text-xs md:text-sm text-gray-600 dark:text-gray-400 mb-1">进行中</div>
          <div className="text-2xl font-bold text-blue-600">
            {categories.processing.length + categories.uploading.length + categories.uploaded.length}
          </div>
        </div>
        <div className="bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05] p-4 shadow-sm dark:shadow-none">
          <div className="text-xs md:text-sm text-gray-600 dark:text-gray-400 mb-1">已完成</div>
          <div className="text-2xl font-bold text-green-600">{categories.completed.length}</div>
        </div>
        <div className="bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05] p-4 shadow-sm dark:shadow-none">
          <div className="text-xs md:text-sm text-gray-600 dark:text-gray-400 mb-1">失败</div>
          <div className="text-2xl font-bold text-red-600">{categories.failed.length}</div>
        </div>
      </div>

      {/* 删除确认弹窗：使用 createPortal 渲染到 body，解决 z-index 和 backdrop-blur 遮挡及居中问题 */}
      {mounted && confirmTarget && createPortal(
        <div className="fixed inset-0 z-[100] flex items-center justify-center pb-[10vh] bg-black/40 p-4">
          <div className="bg-white dark:bg-[#131722]/95 backdrop-blur-xl rounded-xl shadow-[0_8px_30px_rgb(0,0,0,0.12)] dark:shadow-[0_8px_30px_rgb(0,0,0,0.5)] w-full max-w-md border border-gray-200 dark:border-white/[0.1] transform transition-all">
            <div className="relative flex items-center justify-between p-5 border-b border-gray-100 dark:border-white/[0.05]">
              <h3 className="text-base md:text-lg font-semibold text-red-600 dark:text-red-400">确认删除</h3>
              <button
                onClick={() => !isDeleting && setConfirmTarget(null)}
                className="text-gray-400 hover:text-gray-600 dark:text-gray-400 dark:hover:text-gray-200 transition-colors p-1 rounded-full hover:bg-gray-100 dark:hover:bg-white/[0.05]"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            
            <div className="p-5 space-y-4">
              <p className="text-sm md:text-base text-gray-700 dark:text-gray-300">
                此操作将同时删除记录和本地文件，不可恢复。
              </p>
              <div className="bg-gray-50 dark:bg-[#1C2130] rounded-lg p-3 text-xs md:text-sm text-gray-600 dark:text-gray-400 flex items-center justify-between border border-gray-100 dark:border-white/[0.02]">
                <span className="truncate mr-2 font-medium">{confirmTarget.title || confirmTarget.video_id}</span>
                <span className={`shrink-0 text-xs px-2 py-0.5 rounded ${getStatusInfo(confirmTarget.status).color}`}>
                  {getStatusInfo(confirmTarget.status).label}
                </span>
              </div>
              {deleteError && (
                <div className="text-xs md:text-sm text-red-500 bg-red-50 dark:bg-red-500/10 p-3 rounded-lg flex items-start space-x-2 border border-red-200 dark:border-red-500/20">
                  <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <span>{deleteError}</span>
                </div>
              )}
            </div>

            <div className="relative p-5 flex justify-end space-x-3 border-t border-gray-100 dark:border-white/[0.05]">
              <button
                onClick={() => setConfirmTarget(null)}
                disabled={isDeleting}
                className="px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-white/[0.05] hover:bg-gray-200 dark:hover:bg-white/[0.1] rounded-lg transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleDelete}
                disabled={isDeleting}
                className="px-4 py-2 text-sm font-medium text-white bg-red-500 hover:bg-red-600 active:bg-red-700 dark:bg-red-600 dark:hover:bg-red-500 dark:active:bg-red-700 rounded-lg transition-colors flex items-center space-x-2 shadow-sm shadow-red-500/20"
              >
                {isDeleting ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin shrink-0" />
                    <span>删除中...</span>
                  </>
                ) : (
                  <>
                    <Trash2 className="w-4 h-4 shrink-0" />
                    <span>确认删除</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>,
        document.body
      )}
    </div>
  );
}

const TaskStepDetail = ({ steps, onRetry }: { steps: TaskStep[], onRetry: (stepName: string) => void }) => {
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800 dark:bg-green-900/50 dark:text-green-400';
      case 'failed':
        return 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-400';
      case 'running':
        return 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-400';
      case 'pending':
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-white/[0.05] dark:text-gray-300';
    }
  };

  return (
    <div className="space-y-3">
      <h5 className="font-semibold text-gray-800 dark:text-gray-200">任务步骤</h5>
      <ul className="space-y-2">
        {steps.sort((a, b) => a.step_order - b.step_order).map(step => (
          <li key={step.step_name} className="p-3 bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05]">
            <div className="flex items-center justify-between">
              <div className="flex-1">
                <div className="flex items-center space-x-2">
                  <span className="font-medium text-gray-700 dark:text-gray-300">{step.step_name}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${getStatusColor(step.status)}`}>
                    {step.status}
                  </span>
                </div>
                {step.error_msg && (
                  <p className="text-xs text-red-600 mt-1">错误: {step.error_msg}</p>
                )}
              </div>
              {step.can_retry && (
                <button
                  onClick={() => onRetry(step.step_name)}
                  className="px-3 py-1 text-xs text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900/50 hover:bg-blue-200 dark:hover:bg-blue-800 rounded"
                >
                  重试
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
};
