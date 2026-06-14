'use client';

import { useState, useEffect, useCallback, Fragment } from 'react';
import { Dialog, Transition } from '@headlessui/react';
import { X, RefreshCw, Loader2, QrCode as QrCodeIcon, CheckCircle2, AlertCircle } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { apiFetch } from '@/lib/api';

interface BindingDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  platform: string;
  platformName: string;
  userId: string;
}

type BindingStatus = 'loading' | 'ready' | 'pending' | 'bound' | 'expired' | 'error';

interface QRCodeData {
  qr_code: string;
  qr_code_key: string;
  expires_in: number;
}

interface PollResponse {
  status: string;
  platform?: string;
  platform_uid?: string;
  username?: string;
  avatar?: string;
}

export default function BindingDialog({
  isOpen,
  onClose,
  onSuccess,
  platform,
  platformName,
  userId
}: BindingDialogProps) {
  const [status, setStatus] = useState<BindingStatus>('loading');
  const [qrCodeData, setQrCodeData] = useState<QRCodeData | null>(null);
  const [message, setMessage] = useState('');
  const [pollingInterval, setPollingInterval] = useState<NodeJS.Timeout | null>(null);
  const [countdown, setCountdown] = useState(300); // 5分钟倒计时

  // 轮询绑定状态
  const startPolling = useCallback((qrCodeKey: string) => {
    const interval = setInterval(async () => {
      try {
        const response = await apiFetch('/accounts/poll', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ qr_code_key: qrCodeKey })
        });

        const data: { code: number; data: PollResponse } = await response.json();

        if (data.code === 200 && data.data) {
          const pollData = data.data;
          
          switch (pollData.status) {
            case 'pending':
              setStatus('pending');
              setMessage('等待扫码中...');
              break;
              
            case 'bound':
              setStatus('bound');
              setMessage(`绑定成功！欢迎 ${pollData.username}`);
              clearInterval(interval);
              setPollingInterval(null);
              
              // 延迟关闭对话框
              setTimeout(() => {
                onSuccess();
                onClose();
              }, 1500);
              break;
              
            case 'expired':
              setStatus('expired');
              setMessage('二维码已过期，请刷新重试');
              clearInterval(interval);
              setPollingInterval(null);
              break;
              
            default:
              break;
          }
        }
      } catch (error) {
        console.error('Failed to poll status:', error);
      }
    }, 2000); // 每2秒轮询一次

    setPollingInterval(interval);
  }, [onSuccess, onClose]);

  // 生成二维码
  const generateQRCode = useCallback(async () => {
    setStatus('loading');
    setMessage('正在生成二维码...');
    
    try {
      const response = await apiFetch('/accounts/qrcode', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          platform,
          user_id: userId
        })
      });

      const data = await response.json();

      if (data.code === 200 && data.data) {
        console.log('QR Code generated:', data.data);
        setQrCodeData(data.data);
        setStatus('ready');
        setMessage(`请使用 ${platformName} App 扫描二维码`);
        setCountdown(data.data.expires_in || 300);
        
        // 开始轮询
        startPolling(data.data.qr_code_key);
      } else {
        setStatus('error');
        setMessage(data.message || '生成二维码失败');
      }
    } catch (error) {
      console.error('Failed to generate QR code:', error);
      setStatus('error');
      setMessage('生成二维码失败，请重试');
    }
  }, [platform, platformName, userId, startPolling]);

  // 倒计时
  useEffect(() => {
    if (status === 'ready' || status === 'pending') {
      const timer = setInterval(() => {
        setCountdown(prev => {
          if (prev <= 1) {
            setStatus('expired');
            setMessage('二维码已过期，请刷新重试');
            return 0;
          }
          return prev - 1;
        });
      }, 1000);

      return () => clearInterval(timer);
    }
  }, [status]);

  // 监听状态变化清理轮询
  useEffect(() => {
    if (status === 'expired' && pollingInterval) {
      clearInterval(pollingInterval);
      setPollingInterval(null);
    }
  }, [status, pollingInterval]);

  // 初始化
  useEffect(() => {
    if (isOpen) {
      generateQRCode();
    }
    
    return () => {
      if (pollingInterval) {
        clearInterval(pollingInterval);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  // 刷新二维码
  const handleRefresh = () => {
    if (pollingInterval) {
      clearInterval(pollingInterval);
      setPollingInterval(null);
    }
    setQrCodeData(null);
    generateQRCode();
  };

  // 关闭对话框
  const handleClose = () => {
    if (pollingInterval) {
      clearInterval(pollingInterval);
      setPollingInterval(null);
    }
    setQrCodeData(null);
    setStatus('loading');
    setCountdown(300);
    onClose();
  };

  // 格式化倒计时
  const formatCountdown = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  return (
    <Transition appear show={isOpen} as={Fragment}>
      <Dialog as="div" className="relative z-50" onClose={handleClose}>
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-300"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-200"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-black bg-opacity-25 backdrop-blur-sm" />
        </Transition.Child>

        <div className="fixed inset-0 overflow-y-auto">
          <div className="flex min-h-full items-center justify-center p-4">
            <Transition.Child
              as={Fragment}
              enter="ease-out duration-300"
              enterFrom="opacity-0 scale-95"
              enterTo="opacity-100 scale-100"
              leave="ease-in duration-200"
              leaveFrom="opacity-100 scale-100"
              leaveTo="opacity-0 scale-95"
            >
              <Dialog.Panel className="w-full max-w-md transform overflow-hidden rounded-2xl bg-white dark:bg-gray-800 p-4 sm:p-6 shadow-xl transition-all">
                {/* 头部 */}
                <div className="flex items-center justify-between mb-4 sm:mb-6">
                  <Dialog.Title className="text-base sm:text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                    <QrCodeIcon className="h-5 w-5 text-blue-600" />
                    绑定 {platformName} 账号
                  </Dialog.Title>
                  <button
                    onClick={handleClose}
                    className="p-2 -mr-2 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 transition-colors"
                  >
                    <X className="h-5 w-5 sm:h-6 sm:w-6" />
                  </button>
                </div>

                {/* 内容区 */}
                <div className="space-y-4">
                  {/* 二维码区域 */}
                  <div className="flex flex-col items-center justify-center py-6">
                    {status === 'loading' ? (
                      <div className="flex flex-col items-center gap-3">
                        <Loader2 className="h-12 w-12 text-blue-600 animate-spin" />
                        <p className="text-sm text-gray-600 dark:text-gray-400">{message}</p>
                      </div>
                    ) : status === 'error' ? (
                      <div className="flex flex-col items-center gap-3">
                        <AlertCircle className="h-12 w-12 text-red-500" />
                        <p className="text-sm text-red-600">{message}</p>
                        <button
                          onClick={handleRefresh}
                          className="mt-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium flex items-center gap-2"
                        >
                          <RefreshCw className="h-4 w-4" />
                          重新生成
                        </button>
                      </div>
                    ) : status === 'bound' ? (
                      <div className="flex flex-col items-center gap-3">
                        <CheckCircle2 className="h-12 w-12 text-green-500" />
                        <p className="text-sm text-green-600 font-medium">{message}</p>
                      </div>
                    ) : status === 'expired' ? (
                      <div className="flex flex-col items-center gap-3">
                        <AlertCircle className="h-12 w-12 text-orange-500" />
                        <p className="text-sm text-orange-600">{message}</p>
                        <button
                          onClick={handleRefresh}
                          className="mt-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium flex items-center gap-2"
                        >
                          <RefreshCw className="h-4 w-4" />
                          刷新二维码
                        </button>
                      </div>
                    ) : qrCodeData ? (
                      <>
                        <div className="bg-white p-4 rounded-xl border border-gray-200 shadow-sm relative overflow-hidden group">
                          {status === 'pending' && (
                            <div className="absolute inset-0 pointer-events-none z-10">
                              <div className="w-full h-1 bg-gradient-to-r from-transparent via-blue-500 to-transparent opacity-50 absolute top-0 animate-[scan_2.5s_ease-in-out_infinite]" />
                            </div>
                          )}
                          <QRCodeSVG
                            value={qrCodeData.qr_code}
                            size={200}
                            level="H"
                            includeMargin={true}
                            className="rounded-lg"
                          />
                        </div>
                        <p className="text-sm text-gray-600 dark:text-gray-400 mt-4 text-center">{message}</p>
                        
                        {/* 倒计时 */}
                        <div className="flex items-center gap-2 mt-3 text-xs text-gray-500 dark:text-gray-400">
                          <AlertCircle className="h-4 w-4" />
                          <span>二维码有效期: {formatCountdown(countdown)}</span>
                        </div>
                      </>
                    ) : null}
                  </div>

                  {/* 底部提示 */}
                  {(status === 'ready' || status === 'pending') && (
                    <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4 border border-blue-100 dark:border-blue-800/50">
                      <h4 className="text-sm font-medium text-blue-900 dark:text-blue-300 mb-2">绑定步骤：</h4>
                      <ol className="text-xs text-blue-800 dark:text-blue-400 space-y-1.5 pl-4 list-decimal">
                        <li>打开 {platformName} App</li>
                        <li>使用扫一扫功能扫描上方二维码</li>
                        <li>在 App 中确认登录授权</li>
                        <li>等待绑定完成</li>
                      </ol>
                    </div>
                  )}
                </div>

                {/* 底部按钮 */}
                {status !== 'loading' && status !== 'bound' && (
                  <div className="mt-6 flex gap-3">
                    <button
                      onClick={handleClose}
                      className="flex-1 px-4 py-2 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors text-sm font-medium"
                    >
                      取消
                    </button>
                    {(status === 'ready' || status === 'pending') && (
                      <button
                        onClick={handleRefresh}
                        className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium flex items-center justify-center gap-2"
                      >
                        <RefreshCw className="h-4 w-4" />
                        刷新
                      </button>
                    )}
                  </div>
                )}
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </div>
      </Dialog>
    </Transition>
  );
}
