'use client';

import { ReactNode } from 'react';
import AppLayout from '@/components/layout/AppLayout';
import AdminLogin from '@/components/auth/AdminLogin';
import { useAuth } from '@/hooks/useAuth';
import { ThemeProvider } from '@/contexts/ThemeContext';

interface RootLayoutClientProps {
  children: ReactNode;
}

export default function RootLayoutClient({ children }: RootLayoutClientProps) {
  const { user, loading, handleLoginSuccess, handleLogout } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4"></div>
          <p className="text-gray-600 dark:text-gray-400">加载中...</p>
        </div>
      </div>
    );
  }

  // 如果未登录，显示登录页面
  if (!user) {
    return (
      <ThemeProvider>
        <AdminLogin onLoginSuccess={handleLoginSuccess} />
      </ThemeProvider>
    );
  }

  // 已登录，渲染布局和子组件
  return (
    <ThemeProvider>
      <AppLayout user={user} onLogout={handleLogout}>
        {children}
      </AppLayout>
    </ThemeProvider>
  );
}
