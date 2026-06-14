"use client";

import { useState } from 'react';
import { 
  Download, 
  ExternalLink, 
  Chrome, 
  CheckCircle, 
  AlertCircle,
  FileText,
  Settings,
  Puzzle
} from 'lucide-react';

export default function ExtensionPage() {
  const [isDownloading, setIsDownloading] = useState(false);

  const handleDownloadExtension = async () => {
    setIsDownloading(true);
    try {
      const response = await fetch('https://api.github.com/repos/difyz9/ytb2bili_extension/releases/latest');
      const release = await response.json();
      
      if (release.assets && release.assets.length > 0) {
        const zipAsset = release.assets.find((asset: any) => asset.name.endsWith('.zip'));
        if (zipAsset) {
          const link = document.createElement('a');
          link.href = zipAsset.browser_download_url;
          link.download = zipAsset.name;
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
        } else {
          window.open('https://github.com/difyz9/ytb2bili_extension/releases/latest', '_blank');
        }
      } else {
        window.open('https://github.com/difyz9/ytb2bili_extension/releases/latest', '_blank');
      }
    } catch (error) {
      console.error('下载失败:', error);
      window.open('https://github.com/difyz9/ytb2bili_extension/releases/latest', '_blank');
    } finally {
      setIsDownloading(false);
    }
  };

  return (
      <div className="space-y-6">
        {/* 插件介绍 */}
        <div className="bg-white dark:bg-[#131722]/80 backdrop-blur-xl rounded-xl shadow-md border border-transparent dark:border-white/[0.05] p-4 md:p-6">
          <div className="flex items-start space-x-4">
            <div className="flex-shrink-0">
              <Puzzle className="w-8 h-8 text-blue-600 dark:text-blue-400" />
            </div>
            <div className="flex-1">
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
                YTB2BILI 浏览器插件
              </h2>
              <p className="text-gray-600 dark:text-gray-400 mb-4">
                通过安装我们的浏览器插件，您可以更方便地使用 YTB2BILI 平台的各项功能。
              </p>
              <div className="flex flex-col sm:flex-row gap-3">
                <button
                  onClick={handleDownloadExtension}
                  disabled={isDownloading}
                  className="flex items-center justify-center px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-sm"
                >
                  {isDownloading ? (
                    <>
                      <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
                      下载中...
                    </>
                  ) : (
                    <>
                      <Download className="w-4 h-4 mr-2" />
                      下载最新版本
                    </>
                  )}
                </button>
                <a
                  href="https://github.com/difyz9/ytb2bili_extension"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center justify-center px-6 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-white/[0.05] transition-colors"
                >
                  <ExternalLink className="w-4 h-4 mr-2" />
                  GitHub 项目
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* 功能特性 */}
        <div className="bg-white dark:bg-[#131722]/80 backdrop-blur-xl rounded-xl shadow-md border border-transparent dark:border-white/[0.05]">
          <div className="p-4 md:p-6 border-b border-gray-200 dark:border-white/[0.05]">
            <h3 className="text-base md:text-lg font-medium text-gray-900 dark:text-white">插件功能</h3>
          </div>
          <div className="p-4 md:p-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 md:p-6">
              <div className="flex items-start space-x-2 md:space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white">自动获取视频信息</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">在 B 站视频页面自动提取标题、描述、封面等信息</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-2 md:space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white">快速导入视频</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">一键将当前浏览的视频添加到上传队列</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-2 md:space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white">批量操作</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">支持批量导入收藏夹或播放列表中的视频</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-2 md:space-x-3">
                <CheckCircle className="w-5 h-5 text-green-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white">同步管理</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">与 YTB2BILI Web 平台实时同步数据</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* 安装教程 */}
        <div className="bg-white dark:bg-[#131722]/80 backdrop-blur-xl rounded-xl shadow-md border border-transparent dark:border-white/[0.05]">
          <div className="p-4 md:p-6 border-b border-gray-200 dark:border-white/[0.05]">
            <h3 className="text-base md:text-lg font-medium text-gray-900 dark:text-white">安装教程</h3>
          </div>
          <div className="p-4 md:p-6">
            <div className="space-y-3 md:space-y-4">
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-500/[0.15] text-blue-600 dark:text-blue-400 border border-transparent dark:border-blue-500/[0.2] rounded-full flex items-center justify-center text-sm font-medium">
                  1
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-1">下载插件文件</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">点击上方的&ldquo;下载最新版本&rdquo;按钮，下载插件压缩包到本地</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-500/[0.15] text-blue-600 dark:text-blue-400 border border-transparent dark:border-blue-500/[0.2] rounded-full flex items-center justify-center text-sm font-medium">
                  2
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-1">解压文件</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">将下载的 zip 文件解压到一个文件夹中</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-500/[0.15] text-blue-600 dark:text-blue-400 border border-transparent dark:border-blue-500/[0.2] rounded-full flex items-center justify-center text-sm font-medium">
                  3
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-1">打开扩展管理页面</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">在 Chrome 浏览器中访问 <code className="bg-gray-100 dark:bg-white/[0.05] border border-transparent dark:border-white/[0.1] px-1.5 py-0.5 rounded text-gray-800 dark:text-gray-300">chrome://extensions/</code></p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-500/[0.15] text-blue-600 dark:text-blue-400 border border-transparent dark:border-blue-500/[0.2] rounded-full flex items-center justify-center text-sm font-medium">
                  4
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-1">启用开发者模式</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">在扩展页面右上角打开&ldquo;开发者模式&rdquo;开关</p>
                </div>
              </div>
              
              <div className="flex items-start space-x-4">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-500/[0.15] text-blue-600 dark:text-blue-400 border border-transparent dark:border-blue-500/[0.2] rounded-full flex items-center justify-center text-sm font-medium">
                  5
                </div>
                <div>
                  <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-1">加载插件</h4>
                  <p className="text-sm text-gray-600 dark:text-gray-400">点击&ldquo;加载已解压的扩展程序&rdquo;，选择刚才解压的文件夹</p>
                </div>
              </div>
            </div>

            <div className="mt-6 p-4 bg-gradient-to-br from-amber-50 dark:from-amber-500/[0.08] to-orange-50 dark:to-orange-500/[0.03] border border-amber-100 dark:border-white/[0.05] shadow-sm dark:shadow-none backdrop-blur-md rounded-xl">
              <div className="flex items-start space-x-2">
                <AlertCircle className="w-5 h-5 text-amber-600 dark:text-amber-500 mt-0.5 flex-shrink-0" />
                <div>
                  <h4 className="text-sm font-medium text-amber-800 dark:text-amber-400">注意事项</h4>
                  <p className="text-sm text-amber-700 dark:text-amber-500/90 mt-1">
                    由于插件还未上架应用商店，需要手动安装。如果遇到问题，请查看 GitHub 项目页面的详细说明。
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
  );
}