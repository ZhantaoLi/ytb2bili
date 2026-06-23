"use client";

import { useState, useEffect } from 'react';
import { Settings } from 'lucide-react';
import { apiFetch } from '@/lib/api';

export default function SettingsPage() {
  const [autoUpload, setAutoUpload] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // 初始化：从后端读取自动上传开关真实状态
  useEffect(() => {
    (async () => {
      try {
        const res = await apiFetch('/config/auto-upload');
        const data = await res.json();
        if (data.code === 200 && data.data) {
          setAutoUpload(!!data.data.enabled);
        } else {
          setError('读取配置失败');
        }
      } catch {
        setError('读取配置失败，请检查后端服务');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  // 切换开关：写入后端并热生效；失败则回滚 UI
  const handleToggle = async (next: boolean) => {
    setSaving(true);
    setError(null);
    const prev = autoUpload;
    setAutoUpload(next); // 乐观更新
    try {
      const res = await apiFetch('/config/auto-upload', {
        method: 'PUT',
        body: JSON.stringify({ enabled: next }),
      });
      const data = await res.json();
      if (data.code !== 200) {
        setAutoUpload(prev);
        setError(data.message || '保存失败');
      }
    } catch {
      setAutoUpload(prev);
      setError('保存失败，请检查后端服务');
    } finally {
      setSaving(false);
    }
  };

  return (
      <div className="bg-white dark:bg-[#131722]/80 backdrop-blur-xl rounded-lg shadow-md border border-transparent dark:border-white/[0.05]">
        <div className="p-4 md:p-6 border-b border-gray-200 dark:border-white/[0.05]">
          <div className="flex items-center space-x-2 md:space-x-3">
            <Settings className="w-5 h-5 text-gray-600 dark:text-gray-400" />
            <h2 className="text-base md:text-lg font-medium text-gray-900 dark:text-white">设置</h2>
          </div>
        </div>

        <div className="p-4 md:p-6">
          <div className="space-y-3 md:space-y-4">
            <label className="flex items-center justify-between bg-gray-50 dark:bg-white/[0.02] p-4 rounded-md border border-transparent dark:border-white/[0.05] backdrop-blur-md">
              <div>
                <div className="text-sm font-medium">自动上传</div>
                <div className="text-xs text-gray-500 dark:text-gray-400">
                  开启后，准备就绪的视频将由调度器每小时自动上传到 Bilibili；关闭则停在就绪态，需在任务管理中手动上传。
                </div>
              </div>
              <input
                type="checkbox"
                checked={autoUpload}
                disabled={loading || saving}
                onChange={(e) => handleToggle(e.target.checked)}
                className="w-5 h-5 disabled:opacity-50"
              />
            </label>

            {error && (
              <div className="bg-gradient-to-br from-red-50 dark:from-red-500/[0.08] to-rose-50 dark:to-red-500/[0.03] p-4 rounded-md text-sm text-red-700 dark:text-red-400 border border-red-100 dark:border-white/[0.05] backdrop-blur-md">{error}</div>
            )}

            <div className="bg-gradient-to-br from-blue-50 dark:from-blue-500/[0.08] to-indigo-50 dark:to-blue-500/[0.03] p-4 rounded-md border border-blue-100 dark:border-white/[0.05] backdrop-blur-md">
              <div className="text-sm text-blue-800 dark:text-blue-300">
                <strong>提示：</strong> 该开关实时生效，无需重启服务。手动上传不受此开关影响。
              </div>
            </div>
          </div>
        </div>
      </div>
  );
}
