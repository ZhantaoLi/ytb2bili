'use client';

import { FormEvent, ReactNode, useEffect, useState } from 'react';
import {
  AlertCircle,
  ArrowRight,
  CheckCircle2,
  Cookie,
  KeyRound,
  LogIn,
  RefreshCw,
  ShieldCheck,
  Terminal,
  UserPlus,
} from 'lucide-react';

interface AdminLoginProps {
  onLoginSuccess?: (user: any) => void;
}

interface ApiEnvelope<T> {
  code: number;
  message?: string;
  data?: T;
}

interface AdminAuthData {
  token: string;
  user: any;
}

interface SetupStatusData {
  setup_required: boolean;
  env_admin_configured: boolean;
  tool_dependencies?: {
    yt_dlp_required: boolean;
    ffmpeg_required: boolean;
  };
}

type AuthMode = 'checking' | 'setup' | 'login';

const foundations = [
  {
    icon: ShieldCheck,
    title: '本地管理员认证',
    description: '首次启动需创建本机控制台管理员，管理下载、字幕、元数据和上传任务',
  },
  {
    icon: KeyRound,
    title: '能力增强配置项',
    description: 'Proxy代理，DeepLX翻译，DeepSeek翻译，OpenAI接口，Gemini模型等',
  },
  {
    icon: Terminal,
    title: '工具自行接入',
    description: 'yt-dlp(下载 YouTube 视频) 与 ffmpeg(用于合并、音频处理、封装和后续媒体链路)',
  },
  {
    icon: Cookie,
    title: 'Cookies 自行配置',
    description: 'YouTube cookies 用于下载验证，Bilibili cookies 用于账号登录与上传授权',
  },
];

async function readJson<T>(response: Response): Promise<ApiEnvelope<T>> {
  try {
    return await response.json();
  } catch {
    return {
      code: response.status,
      message: response.statusText || '请求失败',
    };
  }
}

export default function AdminLogin({ onLoginSuccess }: AdminLoginProps) {
  const [mode, setMode] = useState<AuthMode>('checking');
  const [envAdminConfigured, setEnvAdminConfigured] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [setupUsername, setSetupUsername] = useState('admin');
  const [setupEmail, setSetupEmail] = useState('');
  const [setupPassword, setSetupPassword] = useState('');
  const [setupPasswordConfirm, setSetupPasswordConfirm] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const loadSetupStatus = async () => {
      setError('');
      setMode('checking');

      try {
        const response = await fetch('/api/v1/auth/admin/setup-status', {
          cache: 'no-store',
        });
        const data = await readJson<SetupStatusData>(response);

        if (!response.ok || data.code !== 200 || !data.data) {
          throw new Error(data.message || '无法读取首次启动状态');
        }

        setEnvAdminConfigured(Boolean(data.data.env_admin_configured));
        setMode(data.data.setup_required ? 'setup' : 'login');
      } catch (err) {
        console.error('Setup status error:', err);
        setError(err instanceof Error ? err.message : '无法读取首次启动状态');
        setMode('login');
      }
    };

    loadSetupStatus();
  }, []);

  const completeLogin = (authData?: AdminAuthData) => {
    if (!authData?.token || !authData?.user) {
      throw new Error('登录响应缺少 token 或用户信息');
    }

    localStorage.setItem('admin_token', authData.token);
    localStorage.setItem('admin_user', JSON.stringify(authData.user));
    onLoginSuccess?.(authData.user);
  };

  const handleLoginSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const response = await fetch('/api/v1/auth/admin/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });
      const data = await readJson<AdminAuthData>(response);

      if (!response.ok || data.code !== 200) {
        throw new Error(data.message || '登录失败，请检查用户名和密码');
      }

      completeLogin(data.data);
    } catch (err) {
      console.error('Login error:', err);
      setError(err instanceof Error ? err.message : '登录失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  const handleSetupSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      if (setupPassword !== setupPasswordConfirm) {
        throw new Error('两次输入的密码不一致');
      }
      if (setupPassword.length < 12) {
        throw new Error('密码至少需要 12 个字符');
      }

      const response = await fetch('/api/v1/auth/admin/setup', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          username: setupUsername,
          password: setupPassword,
          email: setupEmail,
        }),
      });
      const data = await readJson<AdminAuthData>(response);

      if (!response.ok || data.code !== 200) {
        throw new Error(data.message || '创建管理员失败');
      }

      completeLogin(data.data);
    } catch (err) {
      console.error('Setup error:', err);
      setError(err instanceof Error ? err.message : '创建管理员失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  const renderError = () => {
    if (!error) {
      return null;
    }

    return (
      <div className="rounded-2xl border border-red-200 bg-red-50/90 p-4 text-sm text-red-800 shadow-sm dark:border-red-500/25 dark:bg-red-500/10 dark:text-red-300">
        <div className="flex items-start gap-3">
          <AlertCircle className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-500" />
          <p>{error}</p>
        </div>
      </div>
    );
  };

  const renderShell = (children: ReactNode) => (
    <div className="relative min-h-screen overflow-hidden bg-gray-50 text-gray-900 dark:bg-gray-900 dark:text-gray-100">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute left-[-12rem] top-[-10rem] h-[28rem] w-[28rem] rounded-full bg-blue-200/50 blur-3xl dark:bg-blue-500/10" />
        <div className="absolute bottom-[-14rem] right-[-10rem] h-[30rem] w-[30rem] rounded-full bg-cyan-100/70 blur-3xl dark:bg-cyan-500/10" />
        <div className="absolute inset-0 bg-[linear-gradient(rgba(15,23,42,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(15,23,42,0.035)_1px,transparent_1px)] bg-[size:42px_42px] dark:bg-[linear-gradient(rgba(255,255,255,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.035)_1px,transparent_1px)]" />
      </div>

      <main className="relative mx-auto flex min-h-screen w-full max-w-6xl flex-col justify-center px-4 py-8 sm:px-6 lg:px-8">
        <div className="mb-6 flex items-center justify-between">
          <div>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-2xl border border-blue-200 bg-white shadow-sm dark:border-white/[0.08] dark:bg-[#131722]">
                <ShieldCheck className="h-5 w-5 text-blue-600 dark:text-blue-400" />
              </div>
              <div>
                <div className="text-sm font-semibold tracking-[0.28em] text-slate-500 dark:text-slate-400">YTB2BILI</div>
                <div className="text-xs text-slate-500 dark:text-slate-500">Local workflow console</div>
              </div>
            </div>
          </div>
          <div className="hidden rounded-full border border-slate-200 bg-white/70 px-4 py-2 text-xs font-medium text-slate-600 shadow-sm backdrop-blur-xl dark:border-white/[0.08] dark:bg-[#131722]/70 dark:text-slate-400 sm:block">
            http://localhost:8096
          </div>
        </div>

        <div className="grid overflow-hidden rounded-[2rem] border border-white/80 bg-white/75 shadow-2xl shadow-slate-900/10 backdrop-blur-2xl dark:border-white/[0.08] dark:bg-[#131722]/80 dark:shadow-black/35 lg:grid-cols-[0.9fr_1.1fr]">
          <aside className="relative border-b border-slate-200/70 p-6 dark:border-white/[0.06] lg:border-b-0 lg:border-r lg:p-8">
            <div className="relative">
              <div className="space-y-3">
                {foundations.map((item) => {
                  const Icon = item.icon;
                  return (
                    <div
                      key={item.title}
                      className="group rounded-2xl border border-slate-200/80 bg-white/70 p-4 shadow-sm transition duration-300 hover:-translate-y-0.5 hover:border-blue-200 hover:shadow-lg hover:shadow-blue-950/5 dark:border-white/[0.06] dark:bg-white/[0.03] dark:hover:border-blue-500/30 dark:hover:shadow-blue-500/10"
                    >
                      <div className="flex gap-3">
                        <div className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-slate-100 text-blue-600 transition group-hover:bg-blue-50 dark:bg-white/[0.05] dark:text-blue-400 dark:group-hover:bg-blue-500/10">
                          <Icon className="h-5 w-5" />
                        </div>
                        <div>
                          <div className="text-sm font-semibold text-slate-900 dark:text-slate-100">{item.title}</div>
                          <p className="mt-1 text-xs leading-5 text-slate-600 dark:text-slate-400">{item.description}</p>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </aside>

          <section className="p-6 sm:p-8 lg:p-10">{children}</section>
        </div>
      </main>
    </div>
  );

  if (mode === 'checking') {
    return renderShell(
      <div className="flex min-h-[520px] flex-col items-center justify-center text-center">
        <div className="mb-6 flex h-14 w-14 items-center justify-center rounded-2xl border border-blue-100 bg-blue-50 dark:border-blue-500/20 dark:bg-blue-500/10">
          <RefreshCw className="h-6 w-6 animate-spin text-blue-600 dark:text-blue-400" />
        </div>
        <h2 className="text-2xl font-semibold tracking-tight text-slate-950 dark:text-white">正在检查首次启动状态</h2>
        <p className="mt-3 max-w-sm text-sm leading-6 text-slate-600 dark:text-slate-400">
          正在确认数据库中是否已有管理员，以及是否启用了环境变量引导
        </p>
      </div>,
    );
  }

  if (mode === 'setup') {
    return renderShell(
      <div className="mx-auto max-w-xl">
        <div className="mb-8">
          <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-blue-100 bg-blue-50 px-3 py-1.5 text-xs font-medium text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-300">
            <UserPlus className="h-4 w-4" />
            First start
          </div>
          <h2 className="text-3xl font-semibold tracking-tight text-slate-950 dark:text-white">创建第一个管理员</h2>
          <p className="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">
            设置本地控制台账号后，会直接进入后台继续配置 B 站账号、下载工具和任务链
          </p>
        </div>

        <div className="space-y-5">
          {renderError()}

          <form onSubmit={handleSetupSubmit} className="space-y-5">
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <label htmlFor="setup-username" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-300">
                  管理员用户名
                </label>
                <input
                  id="setup-username"
                  type="text"
                  value={setupUsername}
                  onChange={(e) => setSetupUsername(e.target.value)}
                  className="w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-slate-950 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-500/10 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-white"
                  required
                  autoComplete="username"
                />
              </div>

              <div>
                <label htmlFor="setup-email" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-300">
                  邮箱，可选
                </label>
                <input
                  id="setup-email"
                  type="email"
                  value={setupEmail}
                  onChange={(e) => setSetupEmail(e.target.value)}
                  className="w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-slate-950 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-500/10 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-white"
                  placeholder="owner@example.com"
                  autoComplete="email"
                />
              </div>
            </div>

            <div>
              <label htmlFor="setup-password" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-300">
                管理员密码
              </label>
              <input
                id="setup-password"
                type="password"
                value={setupPassword}
                onChange={(e) => setSetupPassword(e.target.value)}
                className="w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-slate-950 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-500/10 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-white"
                placeholder="至少 12 个字符"
                required
                minLength={12}
                autoComplete="new-password"
              />
            </div>

            <div>
              <label htmlFor="setup-password-confirm" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-300">
                确认密码
              </label>
              <input
                id="setup-password-confirm"
                type="password"
                value={setupPasswordConfirm}
                onChange={(e) => setSetupPasswordConfirm(e.target.value)}
                className="w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-slate-950 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-500/10 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-white"
                placeholder="再次输入密码"
                required
                minLength={12}
                autoComplete="new-password"
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              className="group flex w-full items-center justify-center gap-2 rounded-2xl bg-blue-600 px-4 py-3.5 font-medium text-white shadow-lg shadow-blue-500/20 transition hover:-translate-y-0.5 hover:bg-blue-700 hover:shadow-xl hover:shadow-blue-500/25 focus:outline-none focus:ring-4 focus:ring-blue-500/20 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:translate-y-0"
            >
              {loading ? <RefreshCw className="h-4 w-4 animate-spin" /> : <UserPlus className="h-4 w-4" />}
              {loading ? '正在创建...' : '创建管理员并进入控制台'}
              {!loading && <ArrowRight className="h-4 w-4 transition group-hover:translate-x-0.5" />}
            </button>
          </form>
        </div>
      </div>,
    );
  }

  return renderShell(
    <div className="mx-auto max-w-xl">
      <div className="mb-8">
        <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-blue-100 bg-blue-50 px-3 py-1.5 text-xs font-medium text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-300">
          <LogIn className="h-4 w-4" />
          Admin access
        </div>
        <h2 className="text-3xl font-semibold tracking-tight text-slate-950 dark:text-white">管理员登录</h2>
        <p className="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-400">
          登录后管理下载、字幕、元数据和上传任务
        </p>
        {envAdminConfigured && (
          <div className="mt-5 flex gap-3 rounded-2xl border border-blue-100 bg-blue-50/80 p-4 text-sm text-blue-800 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-200">
            <CheckCircle2 className="mt-0.5 h-5 w-5 flex-shrink-0" />
            <p>检测到环境变量管理员配置，请使用对应用户名和密码登录</p>
          </div>
        )}
      </div>

      <div className="space-y-5">
        {renderError()}

        <form onSubmit={handleLoginSubmit} className="space-y-5">
          <div>
            <label htmlFor="username" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-300">
              用户名
            </label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-slate-950 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-500/10 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-white"
              placeholder="admin"
              required
              autoComplete="username"
            />
          </div>

          <div>
            <label htmlFor="password" className="mb-2 block text-sm font-medium text-slate-700 dark:text-slate-300">
              密码
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full rounded-2xl border border-slate-200 bg-white px-4 py-3 text-slate-950 shadow-sm outline-none transition focus:border-blue-400 focus:ring-4 focus:ring-blue-500/10 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-white"
              placeholder="请输入管理员密码"
              required
              autoComplete="current-password"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="group flex w-full items-center justify-center gap-2 rounded-2xl bg-blue-600 px-4 py-3.5 font-medium text-white shadow-lg shadow-blue-500/20 transition hover:-translate-y-0.5 hover:bg-blue-700 hover:shadow-xl hover:shadow-blue-500/25 focus:outline-none focus:ring-4 focus:ring-blue-500/20 disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:translate-y-0"
          >
            {loading ? <RefreshCw className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
            {loading ? '登录中...' : '登录'}
            {!loading && <ArrowRight className="h-4 w-4 transition group-hover:translate-x-0.5" />}
          </button>
        </form>
      </div>
    </div>,
  );
}
