"use client";

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { User, LogOut, Settings, ListChecks, Clock, Puzzle, Link2, Moon, Sun, Menu, X } from 'lucide-react';
import { useTheme } from '@/contexts/ThemeContext';

interface UserInfo {
  id: string;
  name: string;
  mid: string;
  avatar?: string;
}

interface AppLayoutProps {
  children: React.ReactNode;
}

interface AppLayoutWithAuthProps extends AppLayoutProps {
  user: UserInfo;
  onLogout: () => void;
}

export default function AppLayout({ children, user, onLogout }: AppLayoutWithAuthProps) {
  const pathname = usePathname();
  const { theme, toggleTheme } = useTheme();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  // 已登录状态 - 显示完整的应用布局
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* 顶部导航 */}
      <header className="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700">
        <div className="container mx-auto px-4">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center space-x-4">
              {/* 移动端汉堡菜单按钮 */}
              <button
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                className="lg:hidden p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white"
              >
                {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
              </button>

              <Link href="/" className="text-xl font-semibold text-gray-900 dark:text-white">
                YTB2BILI Web
              </Link>
            </div>

            <div className="flex items-center space-x-2 md:space-x-4">
              <div className="hidden md:flex items-center space-x-2 text-sm text-gray-600 dark:text-gray-400">
                <User className="w-4 h-4" />
                <span>{user.name}</span>
              </div>

              <button
                onClick={toggleTheme}
                className="flex items-center space-x-2 px-2 md:px-3 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
                title={theme === 'light' ? '切换到深色模式' : '切换到浅色模式'}
              >
                {theme === 'light' ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
              </button>

              <button
                onClick={onLogout}
                className="flex items-center space-x-2 px-2 md:px-3 py-2 text-sm text-gray-600 dark:text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
              >
                <LogOut className="w-4 h-4" />
                <span className="hidden md:inline">退出登录</span>
              </button>
            </div>
          </div>
        </div>
      </header>

      <div className="container mx-auto px-4 py-4 md:py-8">
        <div className="flex gap-4 md:gap-8">
          {/* 侧边栏 - 桌面端固定显示，移动端折叠 */}
          <div className={`
            fixed lg:static inset-0 z-40 lg:z-auto
            ${mobileMenuOpen ? 'block' : 'hidden lg:block'}
            lg:w-64 lg:flex-shrink-0
          `}>
            {/* 移动端遮罩层 */}
            {mobileMenuOpen && (
              <div
                className="lg:hidden fixed inset-0 bg-black/50"
                onClick={() => setMobileMenuOpen(false)}
              />
            )}

            {/* 导航菜单 */}
            <nav className={`
              lg:relative fixed left-0 top-0 bottom-0
              w-64 lg:w-full
              bg-white dark:bg-gray-800 lg:rounded-lg shadow-sm p-4
              overflow-y-auto
              transform transition-transform lg:transform-none
              ${mobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
            `}>
              {/* 移动端关闭按钮 */}
              <div className="lg:hidden flex justify-between items-center mb-4 pb-4 border-b border-gray-200 dark:border-gray-700">
                <span className="text-lg font-semibold text-gray-900 dark:text-white">菜单</span>
                <button
                  onClick={() => setMobileMenuOpen(false)}
                  className="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <ul className="space-y-2"
                onClick={() => setMobileMenuOpen(false)} /* 点击导航项后关闭菜单 */
              >
                <li>
                  <Link
                    href="/"
                    className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                      pathname === '/'
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                    }`}
                  >
                    <User className="w-5 h-5" />
                    <span>主页</span>
                  </Link>
                </li>

                <li>
                  <Link
                    href="/dashboard"
                    className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                      pathname === '/dashboard'
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                    }`}
                  >
                    <ListChecks className="w-5 h-5" />
                    <span>任务队列</span>
                  </Link>
                </li>

                <li>
                  <Link
                    href="/schedule"
                    className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                      pathname === '/schedule'
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                    }`}
                  >
                    <Clock className="w-5 h-5" />
                    <span>定时上传</span>
                  </Link>
                </li>

                <li>
                  <Link
                    href="/extension"
                    className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                      pathname === '/extension'
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                    }`}
                  >
                    <Puzzle className="w-5 h-5" />
                    <span>浏览器插件</span>
                  </Link>
                </li>

                <li>
                  <Link
                    href="/accounts"
                    className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                      pathname === '/accounts'
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                    }`}
                  >
                    <Link2 className="w-5 h-5" />
                    <span>账号绑定</span>
                  </Link>
                </li>

                <li>
                  <Link
                    href="/settings"
                    className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${
                      pathname === '/settings'
                        ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
                        : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                    }`}
                  >
                    <Settings className="w-5 h-5" />
                    <span>设置</span>
                  </Link>
                </li>
              </ul>
            </nav>
          </div>

          {/* 主内容区 */}
          <div className="flex-1">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
