"use client";

import { useState, useEffect, useCallback } from 'react';
import Image from 'next/image';
import { useAuth } from '@/hooks/useAuth';
import { getApiBaseUrl, apiFetch } from '@/lib/api';
import { CheckCircle, XCircle, Link2, ExternalLink, AlertCircle, Loader2, Clock, Info, ShieldCheck, Unlink, Star } from 'lucide-react';
import BindingDialog from '@/components/BindingDialog';

interface AccountStatus {
  platform: string;
  name: string;
  connected: boolean;
  username?: string;
  avatar?: string;
  connectedAt?: string;
  icon: string;
  color: string;
  bgColor: string;
  description: string;
  isSupported: boolean;
  isPrimary?: boolean;
  id?: number;
}

interface BindingData {
  id: number;
  platform: string;
  platform_uid: string;
  username: string;
  avatar: string;
  status: string;
  is_primary: boolean;
  is_active: boolean;
  create_time: number;
  last_used_at?: number;
}

export default function AccountsPage() {
  const { user, loading, handleLoginSuccess, handleRefreshStatus, handleLogout } = useAuth();
  const [showBindingDialog, setShowBindingDialog] = useState(false);
  const [selectedPlatform, setSelectedPlatform] = useState<{ key: string; name: string } | null>(null);
  const [accounts, setAccounts] = useState<AccountStatus[]>([
    {
      platform: 'bilibili',
      name: 'B站',
      connected: false,
      icon: '📺',
      color: 'bg-pink-500',
      bgColor: 'from-pink-500 to-pink-600',
      description: '绑定B站账号，自动发布视频到B站',
      isSupported: true
    },
    {
      platform: 'youtube',
      name: 'YouTube',
      connected: false,
      icon: '▶️',
      color: 'bg-red-600',
      bgColor: 'from-red-500 to-red-600',
      description: '绑定YouTube账号，同步管理国际平台',
      isSupported: true
    },
    {
      platform: 'douyin',
      name: '抖音',
      connected: false,
      icon: '🎵',
      color: 'bg-black',
      bgColor: 'from-black to-gray-800',
      description: '绑定抖音账号，自动发布短视频到抖音',
      isSupported: false
    },
    {
      platform: 'kuaishou',
      name: '快手',
      connected: false,
      icon: '⚡',
      color: 'bg-orange-500',
      bgColor: 'from-orange-500 to-orange-600',
      description: '绑定快手账号，覆盖更多用户群体',
      isSupported: false
    },
    {
      platform: 'wechat_channels',
      name: '微信视频号',
      connected: false,
      icon: '💬',
      color: 'bg-green-500',
      bgColor: 'from-green-500 to-green-600',
      description: '绑定微信视频号账号，拓展视频分发渠道',
      isSupported: false
    }
  ]);
  const [isChecking, setIsChecking] = useState(true);

  const checkAccountStatus = useCallback(async () => {
    if (!user?.id) return;
    
    setIsChecking(true);
    try {
      // 使用新的 API: /api/v1/accounts/list
      const response = await apiFetch(`/accounts/list?user_id=${user.id}`, {
        method: 'GET',
      });

      const data = await response.json();
      if (data.code === 200 && data.data) {
        const bindings: BindingData[] = data.data;
        
        // 更新账号状态
        setAccounts(prev => prev.map(account => {
          const binding = bindings.find((b: BindingData) => b.platform === account.platform);
          if (binding) {
            return {
              ...account,
              connected: true,
              username: binding.username,
              avatar: binding.avatar,
              connectedAt: new Date(binding.create_time * 1000).toISOString(),
              isPrimary: binding.is_primary,
              id: binding.id
            };
          }
          return {
            ...account,
            connected: false,
            username: undefined,
            avatar: undefined,
            connectedAt: undefined,
            isPrimary: false,
            id: undefined
          };
        }));
      }
    } catch (error) {
      console.error('检查账号状态失败:', error);
    } finally {
      setIsChecking(false);
    }
  }, [user?.id]);

  useEffect(() => {
    if (user?.id) {
      checkAccountStatus();
    } else {
      setIsChecking(false);
    }
  }, [user?.id, checkAccountStatus]);

  const handleConnect = async (platform: string, platformName: string) => {
    if (platform === 'bilibili') {
      // B站使用二维码绑定
      setSelectedPlatform({ key: platform, name: platformName });
      setShowBindingDialog(true);
      return;
    }

    // 其他平台暂时使用旧的OAuth流程
    try {
      const apiBaseUrl = getApiBaseUrl();
      
      // 打开OAuth授权窗口
      const authUrl = `${apiBaseUrl}/auth/${platform}/authorize`;
      const width = 600;
      const height = 700;
      const left = window.screen.width / 2 - width / 2;
      const top = window.screen.height / 2 - height / 2;
      
      window.open(
        authUrl,
        `${platform}_auth`,
        `width=${width},height=${height},left=${left},top=${top}`
      );

      // 监听授权成功消息
      window.addEventListener('message', (event) => {
        if (event.data.type === 'auth_success' && event.data.platform === platform) {
          checkAccountStatus();
        }
      });
    } catch (error) {
      console.error('连接账号失败:', error);
      alert('连接失败，请重试');
    }
  };

  const handleBindingSuccess = () => {
    // 绑定成功后刷新账号列表
    checkAccountStatus();
  };

  const handleDisconnect = async (account: AccountStatus) => {
    if (!confirm(`确定要解绑${account.name}账号吗？`)) {
      return;
    }

    if (!account.id) {
      alert('账号ID不存在');
      return;
    }

    try {
      const response = await apiFetch(`/accounts/${account.id}`, {
        method: 'DELETE',
      });

      const data = await response.json();
      if (data.code === 200) {
        checkAccountStatus();
      } else {
        alert(data.message || '解绑失败');
      }
    } catch (error) {
      console.error('解绑账号失败:', error);
      alert('解绑失败，请重试');
    }
  };

  return (
    <>
      <div className="max-w-6xl mx-auto space-y-6">
        {/* 页面标题 */}
        <div>
          <h2 className="text-3xl font-bold tracking-tight text-gray-900 dark:text-white">账号绑定管理</h2>
          <p className="text-gray-600 dark:text-gray-400 mt-2">绑定多个平台账号，实现视频多平台分发</p>
        </div>

        {/* 已绑定账号列表 */}
        <div className="space-y-3 md:space-y-4">
          <h3 className="text-xl font-semibold flex items-center gap-2">
            <ShieldCheck className="h-5 w-5 text-green-600" />
            已绑定账号
          </h3>
          <div className="min-h-[200px]">
            {isChecking ? (
              <div className="text-center py-12 bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-lg border border-transparent dark:border-white/[0.05] shadow-sm">
                <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent text-gray-400 mb-2" />
                <p className="text-gray-600 dark:text-gray-400 text-sm">加载中...</p>
              </div>
            ) : accounts.filter(a => a.connected).length === 0 ? (
              <div className="text-center py-12 bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-lg border border-dashed border-gray-200 dark:border-white/[0.05] shadow-sm">
                <Link2 className="h-12 w-12 text-gray-300 mx-auto mb-3" />
                <p className="text-gray-600 dark:text-gray-400 mb-1">暂无绑定账号</p>
                <p className="text-xs text-gray-400">请在下方选择平台进行绑定</p>
              </div>
            ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 md:p-6">
              {accounts.filter(a => a.connected).map((account) => (
                <div key={account.platform} className="group relative bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05] hover:border-blue-300 dark:hover:border-blue-500/50 shadow-sm hover:shadow-lg transition-all duration-300 flex flex-col overflow-hidden">
                  {/* 背景装饰 */}
                  <div className={`absolute -top-10 -right-10 w-32 h-32 rounded-full ${account.color} opacity-5 blur-3xl group-hover:opacity-10 transition-opacity pointer-events-none`}></div>
                  
                  <div className="p-5 flex-1 z-10">
                    {/* 头部：图标和名称 */}
                    <div className="flex justify-between items-start mb-6">
                      <div className="flex items-center gap-3">
                        <div className={`w-10 h-10 ${account.color} rounded-lg flex items-center justify-center text-white shadow-md transform group-hover:scale-105 transition-transform duration-300`}>
                          {account.icon}
                        </div>
                        <div>
                          <div className="flex items-center gap-1.5">
                            <h3 className="font-bold text-gray-900 dark:text-white leading-tight">{account.name}</h3>
                            {account.isPrimary && (
                              <span title="主账号" className="inline-flex">
                                <Star className="w-3.5 h-3.5 text-yellow-400 fill-yellow-400" />
                              </span>
                            )}
                          </div>
                          <span className="text-[10px] text-gray-500 dark:text-gray-400">已授权连接</span>
                        </div>
                      </div>
                      <div className="px-2.5 py-1 rounded-full text-xs font-medium bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400 border border-green-100 dark:border-green-700 flex items-center gap-1.5 shadow-sm">
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
                        </span>
                        已连接
                      </div>
                    </div>

                    {/* 中部：头像和用户信息 */}
                    <div className="flex flex-col items-center justify-center py-2 space-y-3">
                      <div className="relative group/avatar">
                        <div className="absolute -inset-0.5 bg-gradient-to-br from-blue-100 to-purple-100 rounded-full opacity-70 group-hover/avatar:opacity-100 transition duration-500 blur-sm"></div>
                        {account.avatar ? (
                          <div className="relative p-1 bg-white dark:bg-gray-800 rounded-full">
                             <Image
                               src={account.avatar}
                               alt={account.username || `${account.name} 头像`}
                               width={64}
                               height={64}
                               unoptimized
                               className="w-16 h-16 rounded-full object-cover shadow-sm border border-gray-100" 
                               onError={(e) => {
                                 e.currentTarget.style.display = 'none';
                                 const nextDiv = e.currentTarget.nextElementSibling as HTMLElement;
                                 if (nextDiv) nextDiv.classList.remove('hidden');
                               }}
                             />
                             {/* 备用头像占位 */}
                             <div className={`w-16 h-16 ${account.color} rounded-full flex items-center justify-center text-2xl text-white shadow-inner hidden`}>
                               {account.username ? account.username.charAt(0).toUpperCase() : '?'}
                             </div>
                          </div>
                        ) : (
                          <div className="relative p-1 bg-white dark:bg-gray-800 rounded-full">
                            <div className={`w-16 h-16 ${account.color} rounded-full flex items-center justify-center text-2xl text-white shadow-inner`}>
                               {account.icon}
                            </div>
                          </div>
                        )}
                        <div className="absolute bottom-0 right-0 bg-white dark:bg-gray-800 rounded-full p-1 shadow-md border border-gray-50 dark:border-gray-700">
                          <CheckCircle className="h-4 w-4 text-green-500" />
                        </div>
                      </div>
                      
                      <div className="text-center w-full px-2">
                        <h4 className="font-bold text-gray-900 dark:text-white truncate text-base md:text-lg" title={account.username}>{account.username}</h4>
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 font-medium bg-gray-50 dark:bg-gray-900 inline-block px-2 py-0.5 rounded-full border border-gray-100 dark:border-gray-700">
                          绑定于 {account.connectedAt ? new Date(account.connectedAt).toLocaleDateString('zh-CN') : '刚刚'}
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* 底部操作栏 */}
                  <div className="bg-gray-50/80 dark:bg-gray-900/80 backdrop-blur-sm border-t border-gray-100 dark:border-gray-700 px-5 py-3 flex items-center justify-between mt-auto">
                    <div className="flex flex-col">
                       <span className="text-[10px] uppercase text-gray-400 font-bold tracking-wider">Status</span>
                       <span className="text-xs font-bold text-green-600 flex items-center gap-1">
                         <ShieldCheck className="w-3 h-3" /> 正常同步
                       </span>
                    </div>
                    
                    <button 
                      onClick={() => handleDisconnect(account)} 
                      className="group/btn relative overflow-hidden bg-white dark:bg-gray-800 hover:bg-red-50 dark:hover:bg-red-900/30 text-gray-500 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 border border-gray-200 dark:border-gray-700 hover:border-red-200 dark:hover:border-red-800/50 px-3 py-1.5 rounded-lg transition-all duration-300 flex items-center gap-1.5 shadow-sm hover:shadow"
                      title="解绑账号"
                    >
                       <Unlink className="w-3.5 h-3.5 transition-transform group-hover/btn:rotate-45" />
                       <span className="text-xs font-medium">解除绑定</span>
                    </button>
                  </div>
                </div>
              ))}
            </div>
            )}
          </div>
        </div>

        {/* 可绑定平台列表 */}
        <div className="space-y-3 md:space-y-4">
          <h3 className="text-xl font-semibold flex items-center gap-2">
            <Link2 className="h-5 w-5 text-blue-600" />
            添加新平台
          </h3>
          
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 md:p-6">
            {accounts.map((account) => {
              const isBound = account.connected;
              return (
                <div
                  key={account.platform}
                  className={`relative group bg-white dark:bg-white/[0.02] backdrop-blur-md rounded-xl border border-gray-200 dark:border-white/[0.05] p-6 transition-all duration-300 ${
                    !account.isSupported 
                      ? 'opacity-70 grayscale-[0.5] hover:opacity-100 hover:grayscale-0' 
                      : 'hover:border-blue-400 hover:shadow-lg hover:-translate-y-1'
                  }`}
                >
                  <div className="flex flex-col items-center text-center space-y-3 md:space-y-4">
                    <div
                      className={`w-16 h-16 ${account.color} rounded-2xl rotate-3 group-hover:rotate-0 transition-transform duration-300 flex items-center justify-center text-3xl text-white shadow-lg`}
                    >
                      {account.icon}
                    </div>
                    <div className="flex-1 w-full">
                      <div className="flex items-center justify-center gap-2 mb-2">
                        <h3 className="font-bold text-base md:text-lg text-gray-900 dark:text-white">{account.name}</h3>
                        {!account.isSupported && (
                          <span className="text-[10px] bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 px-2 py-0.5 rounded-full border border-gray-200 dark:border-gray-600">
                            开发中
                          </span>
                        )}
                        {account.platform === 'bilibili' && (
                          <span className="text-[10px] bg-pink-50 dark:bg-pink-900/30 text-pink-600 dark:text-pink-400 px-2 py-0.5 rounded-full border border-pink-100 dark:border-pink-800/50">
                            热门
                          </span>
                        )}
                      </div>
                      <p className="text-sm text-gray-500 dark:text-gray-400 mb-6 min-h-[40px] leading-relaxed">{account.description}</p>
                      
                      <button
                        onClick={() => handleConnect(account.platform, account.name)}
                        disabled={isBound || !account.isSupported}
                        className={`w-full rounded-lg h-10 font-medium transition-all ${
                          isBound
                            ? 'bg-green-50 dark:bg-green-900/30 text-green-600 dark:text-green-400 border border-green-200 dark:border-green-700 hover:bg-green-50 dark:hover:bg-green-900/40 cursor-default'
                            : !account.isSupported
                            ? 'bg-gray-100 text-gray-400 border border-gray-200 dark:border-gray-700 cursor-not-allowed'
                            : `bg-gradient-to-r ${account.bgColor} text-white hover:opacity-90 shadow-md hover:shadow-lg`
                        }`}
                      >
                        {isBound ? (
                          <span className="flex items-center justify-center">
                            <CheckCircle className="w-4 h-4 mr-1.5" /> 已绑定
                          </span>
                        ) : !account.isSupported ? (
                          <span className="flex items-center justify-center">
                            <Clock className="w-4 h-4 mr-1.5" /> 敬请期待
                          </span>
                        ) : (
                          <span className="flex items-center justify-center">
                            <ExternalLink className="w-4 h-4 mr-1.5" /> 立即绑定
                          </span>
                        )}
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* 帮助与提示 - 双栏布局 */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 md:p-6">
          <div className="bg-gradient-to-br from-blue-50 dark:from-white/[0.03] to-indigo-50 dark:to-white/[0.01] rounded-xl p-6 border border-blue-100 dark:border-white/[0.05] shadow-sm dark:shadow-none backdrop-blur-md">
            <h4 className="font-semibold text-blue-900 dark:text-gray-200 flex items-center gap-2 mb-4">
              <Info className="h-5 w-5 text-blue-600 dark:text-blue-400" />
              快速指南
            </h4>
            <ul className="space-y-3">
              <li className="flex items-start text-sm text-blue-800/80 dark:text-gray-400">
                <span className="flex-shrink-0 w-5 h-5 rounded-full bg-blue-100 dark:bg-white/10 text-blue-600 dark:text-blue-300 flex items-center justify-center text-xs font-bold mr-3 mt-0.5">1</span>
                <span>选择您想要分发视频的目标平台，点击&ldquo;立即绑定&rdquo;</span>
              </li>
              <li className="flex items-start text-sm text-blue-800/80 dark:text-gray-400">
                <span className="flex-shrink-0 w-5 h-5 rounded-full bg-blue-100 dark:bg-white/10 text-blue-600 dark:text-blue-300 flex items-center justify-center text-xs font-bold mr-3 mt-0.5">2</span>
                <span>按照弹窗指引完成扫码或授权登录（YouTube需科学上网）</span>
              </li>
              <li className="flex items-start text-sm text-blue-800/80 dark:text-gray-400">
                <span className="flex-shrink-0 w-5 h-5 rounded-full bg-blue-100 dark:bg-white/10 text-blue-600 dark:text-blue-300 flex items-center justify-center text-xs font-bold mr-3 mt-0.5">3</span>
                <span>绑定成功后，即可在视频列表页选择一键发布</span>
              </li>
            </ul>
          </div>

          <div className="bg-gradient-to-br from-amber-50 dark:from-amber-500/[0.08] to-orange-50 dark:to-orange-500/[0.03] rounded-xl p-6 border border-amber-100 dark:border-white/[0.05] shadow-sm dark:shadow-none backdrop-blur-md">
            <h4 className="font-semibold text-amber-900 dark:text-amber-400 flex items-center gap-2 mb-4">
              <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-500" />
              注意事项
            </h4>
            <ul className="space-y-2.5">
              <li className="flex items-start text-sm text-amber-800/80 dark:text-amber-500/90">
                <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 dark:bg-amber-500 flex-shrink-0"></span>
                <span>B站二维码有效期为5分钟，请尽快完成扫码</span>
              </li>
              <li className="flex items-start text-sm text-amber-800/80 dark:text-amber-500/90">
                <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 dark:bg-amber-500 flex-shrink-0"></span>
                <span>YouTube授权仅请求必要的发布权限，保障账号安全</span>
              </li>
              <li className="flex items-start text-sm text-amber-800/80 dark:text-amber-500/90">
                <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 dark:bg-amber-500 flex-shrink-0"></span>
                <span>不同平台的Cookie有效期不同，失效后需重新绑定</span>
              </li>
              <li className="flex items-start text-sm text-amber-800/80 dark:text-amber-500/90">
                <span className="mr-2 mt-1.5 w-1.5 h-1.5 rounded-full bg-amber-400 dark:bg-amber-500 flex-shrink-0"></span>
                <span>解绑账号不会删除您的历史数据，可随时重新绑定</span>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* 绑定对话框 */}
      {showBindingDialog && selectedPlatform && user?.id && (
        <BindingDialog
          isOpen={showBindingDialog}
          onClose={() => {
            setShowBindingDialog(false);
            setSelectedPlatform(null);
          }}
          onSuccess={handleBindingSuccess}
          platform={selectedPlatform.key}
          platformName={selectedPlatform.name}
          userId={user.id}
        />
      )}
    </>
  );
}
