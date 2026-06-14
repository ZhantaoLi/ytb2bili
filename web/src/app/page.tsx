"use client";

import { getApiBaseUrl, apiFetch } from '@/lib/api';
import { Plus, Youtube, Video, AlertCircle, CheckCircle, Upload, ListChecks, Clock, Puzzle, Link2, Twitter, LinkIcon, DownloadCloud, Lightbulb } from 'lucide-react';
import { useState, useRef } from 'react';

export default function HomePage() {
  // Segment 控制状态
  const [activeTab, setActiveTab] = useState<'url' | 'upload'>('url');

  // URL 提交状态
  const [videoUrl, setVideoUrl] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitMessage, setSubmitMessage] = useState('');
  const [messageType, setMessageType] = useState<'success' | 'error' | ''>('');

  // 本地视频上传状态
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploadMessage, setUploadMessage] = useState('');
  const [uploadMessageType, setUploadMessageType] = useState<'success' | 'error' | ''>('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 提交视频链接到后端
  const handleSubmitUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!videoUrl.trim()) {
      setMessageType('error');
      setSubmitMessage('请输入视频链接');
      return;
    }

    setIsSubmitting(true);
    setSubmitMessage('');
    setMessageType('');

    try {
      const response = await apiFetch('/submit', {
        method: 'POST',
        body: JSON.stringify({
          url: videoUrl,
          title: '', // 可以为空，后端会自动提取
          description: '',
          operationType: '1', // 默认操作类型
          subtitles: [],
          playlistId: '',
          timestamp: new Date().toISOString(),
          savedAt: new Date().toISOString(),
        }),
      });

      const result = await response.json();

      if (result.success) {
        setMessageType('success');
        setSubmitMessage(`视频链接已成功提交！${result.data?.isExisting ? '(更新了现有记录)' : ''}`);
        setVideoUrl(''); // 清空输入框
      } else {
        setMessageType('error');
        setSubmitMessage(result.message || '提交失败，请重试');
      }
    } catch (error) {
      console.error('提交失败:', error);
      setMessageType('error');
      setSubmitMessage('网络错误，请检查后端服务是否正常运行');
    } finally {
      setIsSubmitting(false);
    }
  };

  // 检测视频平台类型
  const detectPlatform = (url: string) => {
    if (url.includes('youtube.com') || url.includes('youtu.be')) return 'YouTube';
    if (url.includes('bilibili.com')) return 'Bilibili';
    if (url.includes('twitter.com') || url.includes('x.com')) return 'Twitter/X';
    if (url.includes('tiktok.com')) return 'TikTok';
    if (url.includes('instagram.com')) return 'Instagram';
    return '未知平台';
  };

  // 处理文件选择
  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      // 检查文件类型
      const validTypes = ['video/mp4', 'video/webm', 'video/ogg', 'video/quicktime', 'video/x-msvideo', 'video/x-matroska'];
      if (!validTypes.includes(file.type) && !file.name.match(/\.(mp4|webm|ogg|mov|avi|mkv|flv)$/i)) {
        setUploadMessageType('error');
        setUploadMessage('不支持的文件格式，请上传视频文件（mp4, webm, mov, avi, mkv等）');
        return;
      }

      // 检查文件大小（限制为2GB）
      const maxSize = 2 * 1024 * 1024 * 1024; // 2GB
      if (file.size > maxSize) {
        setUploadMessageType('error');
        setUploadMessage('文件太大，最大支持2GB的视频文件');
        return;
      }

      setSelectedFile(file);
      setUploadMessage('');
      setUploadMessageType('');
    }
  };

  // 上传本地视频
  const handleUploadVideo = async () => {
    if (!selectedFile) {
      setUploadMessageType('error');
      setUploadMessage('请先选择视频文件');
      return;
    }

    setIsUploading(true);
    setUploadProgress(0);
    setUploadMessage('');
    setUploadMessageType('');

    try {
      const formData = new FormData();
      formData.append('file', selectedFile);
      formData.append('title', selectedFile.name.replace(/\.[^/.]+$/, '')); // 使用文件名作为标题

      const apiBaseUrl = getApiBaseUrl();

      // 使用 XMLHttpRequest 以便跟踪上传进度
      const xhr = new XMLHttpRequest();

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          const progress = Math.round((e.loaded / e.total) * 100);
          setUploadProgress(progress);
        }
      });

      xhr.addEventListener('load', () => {
        if (xhr.status === 200) {
          const result = JSON.parse(xhr.responseText);
          setUploadMessageType('success');
          setUploadMessage(`视频上传成功！文件名: ${selectedFile.name}`);
          setSelectedFile(null);
          setUploadProgress(0);
          if (fileInputRef.current) {
            fileInputRef.current.value = '';
          }
        } else {
          const error = JSON.parse(xhr.responseText);
          setUploadMessageType('error');
          setUploadMessage(error.message || '上传失败，请重试');
        }
        setIsUploading(false);
      });

      xhr.addEventListener('error', () => {
        setUploadMessageType('error');
        setUploadMessage('网络错误，上传失败');
        setIsUploading(false);
      });

      xhr.open('POST', `${apiBaseUrl}/upload/video`);
      xhr.send(formData);
    } catch (error) {
      console.error('上传失败:', error);
      setUploadMessageType('error');
      setUploadMessage('上传出错，请稍后重试');
      setIsUploading(false);
    }
  };

  // 格式化文件大小
  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
  };

  return (
      <div className="max-w-5xl mx-auto space-y-4 md:space-y-8 px-4 md:px-0">
        {/* 主要功能区域 - Segment 切换面板 */}
        <div className="bg-gradient-to-br from-white dark:from-[#131722]/90 to-blue-50/30 dark:to-[#0f111a]/80 backdrop-blur-xl rounded-2xl shadow-xl dark:shadow-[0_8px_30px_rgb(0,0,0,0.4)] overflow-hidden border border-blue-100/50 dark:border-white/[0.08]">
          {/* Segment Control 标题栏 */}
          <div className="relative">
            {/* 装饰性渐变背景 */}
            <div className="absolute inset-0 bg-gradient-to-r from-blue-600 to-indigo-600 opacity-[0.02] dark:opacity-[0.05]"></div>

            <div className="relative px-4 md:px-8 pt-4 md:pt-6 pb-4 md:pb-6">
              {/* Segment Control 切换器 */}
              <div className="flex justify-center">
                <div className="inline-flex bg-gray-100 dark:bg-gray-700 rounded-lg p-1">
                  <button
                    onClick={() => setActiveTab('url')}
                    className={`px-4 md:px-6 py-2 md:py-2.5 rounded-md font-semibold transition-all ${
                      activeTab === 'url'
                        ? 'bg-white dark:bg-gray-800 dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 dark:text-gray-300 hover:text-gray-900 dark:text-white dark:hover:text-white'
                    }`}
                  >
                    <div className="flex items-center space-x-2">
                      <LinkIcon className="w-4 h-4" />
                      <span>在线链接</span>
                    </div>
                  </button>
                  <button
                    onClick={() => setActiveTab('upload')}
                    className={`px-4 md:px-6 py-2 md:py-2.5 rounded-md font-semibold transition-all ${
                      activeTab === 'upload'
                        ? 'bg-white dark:bg-gray-800 dark:bg-gray-800 text-blue-600 dark:text-blue-400 shadow-sm'
                        : 'text-gray-600 dark:text-gray-400 dark:text-gray-300 hover:text-gray-900 dark:text-white dark:hover:text-white'
                    }`}
                  >
                    <div className="flex items-center space-x-2">
                      <Upload className="w-4 h-4" />
                      <span>本地上传</span>
                    </div>
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* 内容面板 */}
          <div className="p-6">
            {/* URL 提交面板 */}
            {activeTab === 'url' && (
              <div className="space-y-6 animate-fade-in">
                <div className="max-w-3xl mx-auto">

                  <form onSubmit={handleSubmitUrl} className="space-y-6">
                    <div>
                      <label htmlFor="video-url" className="block text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
                        视频链接
                      </label>
                      <div className="relative group">
                        <div className="absolute inset-0 bg-gradient-to-r from-blue-500 to-indigo-500 rounded-xl opacity-0 group-hover:opacity-5 transition-opacity"></div>
                        <input
                          id="video-url"
                          type="url"
                          value={videoUrl}
                          onChange={(e) => setVideoUrl(e.target.value)}
                          placeholder="请输入视频链接，如：https://www.youtube.com/watch?v=..."
                          className="relative w-full px-4 sm:px-5 py-3 sm:py-4 pr-24 sm:pr-32 border-2 border-gray-200 dark:border-gray-700 rounded-xl focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all bg-white dark:bg-gray-800/50 backdrop-blur-sm text-sm sm:text-base"
                          disabled={isSubmitting}
                        />
                        <div className="absolute inset-y-0 right-0 flex items-center pr-4">
                          {videoUrl.trim() && (
                            <span className="text-xs font-medium text-blue-600 dark:text-blue-300 bg-blue-50 dark:bg-blue-900/30 px-3 py-1.5 rounded-lg border border-blue-200 dark:border-blue-800/50">
                              {detectPlatform(videoUrl)}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>

                    <button
                      type="submit"
                      disabled={isSubmitting || !videoUrl.trim()}
                      className="w-full flex items-center justify-center px-4 sm:px-6 py-3 sm:py-4 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl hover:from-blue-700 hover:to-indigo-700 disabled:from-gray-300 disabled:to-gray-300 dark:disabled:from-gray-700 dark:disabled:to-gray-700 dark:disabled:text-gray-400 disabled:cursor-not-allowed transition-all font-semibold text-sm sm:text-base shadow-lg shadow-blue-500/30 hover:shadow-xl hover:shadow-blue-500/40 disabled:shadow-none transform hover:scale-[1.02] active:scale-[0.98]"
                    >
                      {isSubmitting ? (
                        <>
                          <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
                          提交中...
                        </>
                      ) : (
                        <>
                          <Plus className="w-5 h-5 mr-2" />
                          提交下载
                        </>
                      )}
                    </button>
                  </form>

                  {/* 提交结果消息 */}
                  {submitMessage && (
                    <div className={`mt-6 p-5 rounded-xl flex items-center shadow-lg ${
                      messageType === 'success'
                        ? 'bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-900/30 dark:to-emerald-900/30 border-2 border-green-200 dark:border-green-800/50 text-green-800 dark:text-green-400'
                        : 'bg-gradient-to-r from-red-50 to-rose-50 dark:from-red-900/30 dark:to-rose-900/30 border-2 border-red-200 dark:border-red-800/50 text-red-800 dark:text-red-400'
                    }`}>
                      {messageType === 'success' ? (
                        <CheckCircle className="w-6 h-6 mr-3 text-green-600 flex-shrink-0" />
                      ) : (
                        <AlertCircle className="w-6 h-6 mr-3 text-red-600 flex-shrink-0" />
                      )}
                      <span className="font-medium">{submitMessage}</span>
                    </div>
                  )}
                </div>

                {/* 支持的平台展示 */}
                <div className="max-w-3xl mx-auto bg-gradient-to-br from-blue-50 dark:from-[#131722]/80 to-indigo-50 dark:to-[#0f111a]/60 border-2 border-blue-100 dark:border-white/[0.05] rounded-xl p-6 shadow-md backdrop-blur-lg">
                  <h4 className="text-sm font-bold text-gray-900 dark:text-white mb-4 flex items-center">
                    <div className="w-1.5 h-5 bg-gradient-to-b from-blue-500 to-indigo-500 rounded-full mr-2"></div>
                    支持的平台
                  </h4>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 md:gap-4">
                    <div className="flex items-center space-x-3 bg-white dark:bg-gray-800/60 backdrop-blur-sm px-4 py-3 rounded-lg hover:bg-white dark:hover:bg-gray-700/80 transition-colors">
                      <Youtube className="w-5 h-5 text-red-600" />
                      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">YouTube</span>
                    </div>
                    <div className="flex items-center space-x-3 bg-white dark:bg-gray-800/60 backdrop-blur-sm px-4 py-3 rounded-lg hover:bg-white dark:hover:bg-gray-700/80 transition-colors">
                      <Video className="w-5 h-5 text-blue-600" />
                      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Bilibili</span>
                    </div>
                    <div className="flex items-center space-x-3 bg-white dark:bg-gray-800/60 backdrop-blur-sm px-4 py-3 rounded-lg hover:bg-white dark:hover:bg-gray-700/80 transition-colors">
                      <Twitter className="w-5 h-5 text-blue-400" />
                      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Twitter/X</span>
                    </div>
                    <div className="flex items-center space-x-3 bg-white dark:bg-gray-800/60 backdrop-blur-sm px-4 py-3 rounded-lg hover:bg-white dark:hover:bg-gray-700/80 transition-colors">
                      <Video className="w-5 h-5 text-purple-600" />
                      <span className="text-sm font-medium text-gray-700 dark:text-gray-300">TikTok</span>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* 本地上传面板 */}
            {activeTab === 'upload' && (
              <div className="space-y-6 animate-fade-in">
                <div className="max-w-3xl mx-auto">

                  {/* 文件选择区域 */}
                  <div className="mb-6">
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept="video/*,.mkv,.avi,.flv"
                      onChange={handleFileSelect}
                      disabled={isUploading}
                      className="hidden"
                      id="video-file-input"
                    />
                    <label
                      htmlFor="video-file-input"
                      className={`relative flex flex-col items-center justify-center w-full h-72 border-3 border-dashed rounded-2xl cursor-pointer transition-all group overflow-hidden ${
                        isUploading
                          ? 'border-gray-300 dark:border-gray-600 bg-gray-50 dark:bg-gray-900 cursor-not-allowed'
                          : 'border-blue-300 dark:border-blue-600/50 hover:border-blue-500 dark:hover:border-blue-400 bg-gradient-to-br from-blue-50/50 dark:from-blue-900/10 to-indigo-50/50 dark:to-indigo-900/10 hover:from-blue-50 dark:hover:from-blue-900/20 hover:to-indigo-50 dark:hover:to-indigo-900/20'
                      }`}
                    >
                      <div className="absolute inset-0 bg-gradient-to-br from-blue-500/5 to-indigo-500/5 opacity-0 group-hover:opacity-100 transition-opacity"></div>

                      <div className="relative flex flex-col items-center justify-center py-8">
                        <div className="mb-6 transform group-hover:scale-110 transition-transform duration-300">
                          <div className="relative">
                            <div className="absolute inset-0 bg-blue-500 rounded-full blur-xl opacity-20 group-hover:opacity-30 transition-opacity"></div>
                            <Upload className="relative w-16 h-16 text-blue-500" />
                          </div>
                        </div>
                        {selectedFile ? (
                          <>
                            <p className="mb-3 text-lg md:text-xl font-bold text-blue-600">{selectedFile.name}</p>
                            <p className="text-base text-gray-600 dark:text-gray-400 font-medium">大小: {formatFileSize(selectedFile.size)}</p>
                          </>
                        ) : (
                          <>
                            <p className="mb-3 text-base font-bold text-gray-800 dark:text-gray-200">
                              点击选择文件或拖拽到此处
                            </p>
                            <p className="text-sm text-gray-600 dark:text-gray-400 font-medium mb-2">
                              支持格式: MP4, WebM, MOV, AVI, MKV, FLV
                            </p>
                            <p className="text-xs text-gray-500 dark:text-gray-400">
                              最大文件大小: 2GB
                            </p>
                          </>
                        )}
                      </div>
                    </label>
                  </div>

                  {/* 已选文件操作 */}
                  {selectedFile && !isUploading && (
                    <button
                      onClick={() => {
                        setSelectedFile(null);
                        if (fileInputRef.current) {
                          fileInputRef.current.value = '';
                        }
                      }}
                      className="mb-6 text-red-600 hover:text-red-800 text-sm font-semibold hover:underline transition-all"
                    >
                      × 移除文件
                    </button>
                  )}

                  {/* 上传进度条 */}
                  {isUploading && (
                    <div className="mb-6 space-y-3 bg-blue-50 dark:bg-blue-900/20 border-2 border-blue-200 dark:border-blue-800/50 rounded-xl p-5">
                      <div className="flex justify-between text-sm font-semibold text-gray-700 dark:text-gray-300">
                        <span>上传进度</span>
                        <span className="text-blue-600">{uploadProgress}%</span>
                      </div>
                      <div className="relative w-full bg-gray-200 rounded-full h-4 overflow-hidden">
                        <div
                          className="absolute inset-0 bg-gradient-to-r from-blue-500 to-indigo-500 h-4 rounded-full transition-all duration-300 shadow-lg"
                          style={{ width: `${uploadProgress}%` }}
                        >
                          <div className="absolute inset-0 bg-white dark:bg-gray-800/20 animate-pulse"></div>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* 上传按钮 */}
                  <button
                    onClick={handleUploadVideo}
                    disabled={!selectedFile || isUploading}
                    className="w-full flex items-center justify-center px-6 py-4 bg-gradient-to-r from-green-600 to-emerald-600 text-white rounded-xl hover:from-green-700 hover:to-emerald-700 disabled:from-gray-300 disabled:to-gray-300 dark:disabled:from-gray-700 dark:disabled:to-gray-700 dark:disabled:text-gray-400 disabled:cursor-not-allowed transition-all font-semibold text-base shadow-lg shadow-green-500/30 hover:shadow-xl hover:shadow-green-500/40 disabled:shadow-none transform hover:scale-[1.02] active:scale-[0.98]"
                  >
                    {isUploading ? (
                      <>
                        <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
                        上传中 ({uploadProgress}%)
                      </>
                    ) : (
                      <>
                        <Upload className="w-5 h-5 mr-2" />
                        开始上传
                      </>
                    )}
                  </button>

                  {/* 上传结果消息 */}
                  {uploadMessage && (
                    <div className={`mt-6 p-5 rounded-xl flex items-center shadow-lg ${
                      uploadMessageType === 'success'
                        ? 'bg-gradient-to-r from-green-50 to-emerald-50 dark:from-green-900/30 dark:to-emerald-900/30 border-2 border-green-200 dark:border-green-800/50 text-green-800 dark:text-green-400'
                        : 'bg-gradient-to-r from-red-50 to-rose-50 dark:from-red-900/30 dark:to-rose-900/30 border-2 border-red-200 dark:border-red-800/50 text-red-800 dark:text-red-400'
                    }`}>
                      {uploadMessageType === 'success' ? (
                        <CheckCircle className="w-6 h-6 mr-3 text-green-600 flex-shrink-0" />
                      ) : (
                        <AlertCircle className="w-6 h-6 mr-3 text-red-600 flex-shrink-0" />
                      )}
                      <span className="font-medium">{uploadMessage}</span>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* 支持的平台说明 */}
        <div className="bg-gradient-to-br from-blue-50 dark:from-[#131722]/80 via-indigo-50 dark:via-[#131722]/60 to-purple-50 dark:to-[#0f111a]/80 rounded-2xl border-2 border-blue-200/50 dark:border-white/[0.05] p-8 shadow-xl backdrop-blur-lg">
          <h3 className="text-lg md:text-xl font-bold text-gray-900 dark:text-white mb-6 flex items-center">
            <div className="flex items-center justify-center w-10 h-10 bg-gradient-to-br from-blue-500 to-indigo-600 rounded-xl mr-3 shadow-lg shadow-blue-500/30">
              <Lightbulb className="w-5 h-5 text-white" />
            </div>
            功能说明
          </h3>
          <div className="space-y-4">
            <div className="flex items-center space-x-3 bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl p-4 hover:bg-white dark:hover:bg-white/[0.05] transition-colors border border-blue-100 dark:border-white/[0.05]">
              <div className="flex-shrink-0 w-8 h-8 flex items-center justify-center bg-blue-100 dark:bg-white/10 rounded-lg">
                <LinkIcon className="w-4 h-4 text-blue-600 dark:text-blue-300" />
              </div>
              <div>
                <p className="text-sm text-gray-700 dark:text-gray-300"><strong className="text-blue-600 dark:text-blue-400">在线链接：</strong>基于 yt-dlp 技术，支持 1000+ 个视频网站</p>
              </div>
            </div>
            <div className="flex items-center space-x-3 bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl p-4 hover:bg-white dark:hover:bg-white/[0.05] transition-colors border border-blue-100 dark:border-white/[0.05]">
              <div className="flex-shrink-0 w-8 h-8 flex items-center justify-center bg-green-100 dark:bg-white/10 rounded-lg">
                <Upload className="w-4 h-4 text-green-600 dark:text-green-300" />
              </div>
              <div>
                <p className="text-sm text-gray-700 dark:text-gray-300"><strong className="text-green-600 dark:text-green-400">本地上传：</strong>直接上传本地视频文件，支持多种格式</p>
              </div>
            </div>
            <div className="flex items-center space-x-3 bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl p-4 hover:bg-white dark:hover:bg-white/[0.05] transition-colors border border-blue-100 dark:border-white/[0.05]">
              <div className="flex-shrink-0 w-8 h-8 flex items-center justify-center bg-purple-100 dark:bg-white/10 rounded-lg">
                <DownloadCloud className="w-4 h-4 text-purple-600 dark:text-purple-300" />
              </div>
              <div>
                <p className="text-sm text-gray-700 dark:text-gray-300">提交后系统将自动识别平台并开始下载处理</p>
              </div>
            </div>
          </div>
        </div>

        {/* 快捷导航 */}
        <div className="bg-white dark:bg-[#131722]/80 backdrop-blur-xl rounded-2xl shadow-xl p-8 border border-gray-100 dark:border-white/[0.05]">
          <h3 className="text-lg md:text-xl font-bold text-gray-900 dark:text-white mb-6 flex items-center">
            <div className="w-1.5 h-6 bg-gradient-to-b from-blue-500 to-indigo-500 rounded-full mr-3"></div>
            管理功能
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <a
              href="/dashboard"
              className="group flex items-center justify-start px-5 py-4 bg-gradient-to-br from-blue-50 to-blue-100/50 dark:from-white/[0.03] dark:to-white/[0.01] text-blue-700 dark:text-gray-200 rounded-2xl hover:from-blue-100 hover:to-blue-200 dark:hover:from-white/[0.08] dark:hover:to-white/[0.03] transition-all duration-300 shadow-sm dark:shadow-none hover:shadow-xl dark:hover:shadow-[0_0_20px_rgba(59,130,246,0.15)] transform hover:-translate-y-1 border border-blue-200 dark:border-white/[0.05] dark:hover:border-blue-500/30 backdrop-blur-md"
            >
              <div className="w-10 h-10 rounded-xl bg-white dark:bg-blue-500/10 shadow-sm border border-blue-100 dark:border-blue-500/20 flex items-center justify-center mr-3 group-hover:scale-110 group-hover:rotate-3 transition-transform duration-300">
                <ListChecks className="w-5 h-5 text-blue-600 dark:text-blue-400" />
              </div>
              <span className="font-semibold tracking-wide">任务队列</span>
            </a>
            <a
              href="/schedule"
              className="group flex items-center justify-start px-5 py-4 bg-gradient-to-br from-green-50 to-green-100/50 dark:from-white/[0.03] dark:to-white/[0.01] text-green-700 dark:text-gray-200 rounded-2xl hover:from-green-100 hover:to-green-200 dark:hover:from-white/[0.08] dark:hover:to-white/[0.03] transition-all duration-300 shadow-sm dark:shadow-none hover:shadow-xl dark:hover:shadow-[0_0_20px_rgba(34,197,94,0.15)] transform hover:-translate-y-1 border border-green-200 dark:border-white/[0.05] dark:hover:border-green-500/30 backdrop-blur-md"
            >
              <div className="w-10 h-10 rounded-xl bg-white dark:bg-green-500/10 shadow-sm border border-green-100 dark:border-green-500/20 flex items-center justify-center mr-3 group-hover:scale-110 group-hover:rotate-3 transition-transform duration-300">
                <Clock className="w-5 h-5 text-green-600 dark:text-green-400" />
              </div>
              <span className="font-semibold tracking-wide">定时上传</span>
            </a>
            <a
              href="/extension"
              className="group flex items-center justify-start px-5 py-4 bg-gradient-to-br from-purple-50 to-purple-100/50 dark:from-white/[0.03] dark:to-white/[0.01] text-purple-700 dark:text-gray-200 rounded-2xl hover:from-purple-100 hover:to-purple-200 dark:hover:from-white/[0.08] dark:hover:to-white/[0.03] transition-all duration-300 shadow-sm dark:shadow-none hover:shadow-xl dark:hover:shadow-[0_0_20px_rgba(168,85,247,0.15)] transform hover:-translate-y-1 border border-purple-200 dark:border-white/[0.05] dark:hover:border-purple-500/30 backdrop-blur-md"
            >
              <div className="w-10 h-10 rounded-xl bg-white dark:bg-purple-500/10 shadow-sm border border-purple-100 dark:border-purple-500/20 flex items-center justify-center mr-3 group-hover:scale-110 group-hover:rotate-3 transition-transform duration-300">
                <Puzzle className="w-5 h-5 text-purple-600 dark:text-purple-400" />
              </div>
              <span className="font-semibold tracking-wide">浏览器插件</span>
            </a>
            <a
              href="/accounts"
              className="group flex items-center justify-start px-5 py-4 bg-gradient-to-br from-gray-50 to-gray-100/50 dark:from-white/[0.03] dark:to-white/[0.01] text-gray-700 dark:text-gray-200 rounded-2xl hover:from-gray-100 hover:to-gray-200 dark:hover:from-white/[0.08] dark:hover:to-white/[0.03] transition-all duration-300 shadow-sm dark:shadow-none hover:shadow-xl dark:hover:shadow-[0_0_20px_rgba(156,163,175,0.15)] transform hover:-translate-y-1 border border-gray-200 dark:border-white/[0.05] dark:hover:border-gray-500/30 backdrop-blur-md"
            >
              <div className="w-10 h-10 rounded-xl bg-white dark:bg-gray-500/10 shadow-sm border border-gray-100 dark:border-gray-500/20 flex items-center justify-center mr-3 group-hover:scale-110 group-hover:rotate-3 transition-transform duration-300">
                <Link2 className="w-5 h-5 text-gray-600 dark:text-gray-400" />
              </div>
              <span className="font-semibold tracking-wide">账号绑定</span>
            </a>
          </div>
        </div>
      </div>
  );
}
