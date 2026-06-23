import axios from 'axios';
import { shouldAttachAuthHeader } from './requestAuth.mjs';
import type { 
  ApiResponse, 
  Video, 
  VideoDetail,
  VideoFilesResponse,
  QRCodeResponse, 
  LoginStatus, 
  VideoSubmissionRequest,
  UploadValidation 
} from '@/types';

/**
 * 获取API基础URL的统一方法
 * 开发环境使用代理路径，生产环境使用环境变量或默认值
 */
export const getApiBaseUrl = (): string => {
  if (process.env.NODE_ENV === 'development') {
    return '/api/v1';
  }
  return process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8096/api/v1';
};

/**
 * 获取完整的API基础URL（包含协议和域名）
 * 用于需要完整URL的场景（如OAuth回调）
 */
export const getFullApiBaseUrl = (): string => {
  if (typeof window !== 'undefined') {
    const { protocol, hostname, port } = window.location;
    return `${protocol}//${hostname}${port ? ':' + port : ''}`;
  }
  return 'http://localhost:8096';
};

// 前后端分离配置：直接调用后端API
const API_BASE_URL = getApiBaseUrl();

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
  withCredentials: true, // 支持跨域Cookie
});

// 请求拦截器
api.interceptors.request.use(
  async (config) => {
    // 添加管理员 token 到请求头（如果已登录）
    try {
      const token = localStorage.getItem('admin_token');
      if (token) {
        config.headers['Authorization'] = `Bearer ${token}`;
      }
    } catch (error) {
      console.error('Failed to get admin token:', error);
    }
    
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器
api.interceptors.response.use(
  (response) => {
    return response.data;
  },
  (error) => {
    console.error('API Error:', error);
    return Promise.reject(error);
  }
);

// 认证相关 API
export const authApi = {
  // 获取登录二维码
  getQRCode: (): Promise<ApiResponse<QRCodeResponse>> => {
    return api.get('/auth/qrcode');
  },

  // 检查登录状态
  checkLoginStatus: (authCode: string): Promise<ApiResponse<LoginStatus>> => {
    return api.post('/auth/check', { auth_code: authCode });
  },

  // 获取当前用户状态
  getUserStatus: (): Promise<ApiResponse<LoginStatus>> => {
    return api.get('/auth/status');
  },

  // 退出登录
  logout: (): Promise<ApiResponse> => {
    return api.post('/auth/logout');
  },
};

// 视频相关 API
export const videoApi = {
  // 获取视频列表
  getVideos: (page = 1, limit = 10): Promise<ApiResponse<{ videos: Video[], total: number }>> => {
    return api.get(`/videos?page=${page}&limit=${limit}`);
  },

  // 获取单个视频详情
  getVideo: (id: number): Promise<ApiResponse<Video>> => {
    return api.get(`/videos/${id}`);
  },

  // 获取视频详细信息（包含任务步骤）
  getVideoDetail: (id: string): Promise<ApiResponse<VideoDetail>> => {
    return api.get(`/videos/${id}`);
  },

  // 获取视频文件列表
  getVideoFiles: (id: string): Promise<ApiResponse<VideoFilesResponse>> => {
    return api.get(`/videos/${id}/files`);
  },

  // 重试任务步骤
  retryTaskStep: (videoId: string, stepName: string): Promise<ApiResponse> => {
    return api.post(`/videos/${videoId}/steps/${stepName}/retry`);
  },

  // 提交新视频
  submitVideo: (data: VideoSubmissionRequest): Promise<ApiResponse<Video>> => {
    return api.post('/submit', data);
  },

  // 验证视频上传
  validateUpload: (videoId: string): Promise<ApiResponse<UploadValidation>> => {
    return api.post('/upload/validate', { video_id: videoId });
  },

  // 删除视频
  deleteVideo: (id: number): Promise<ApiResponse> => {
    return api.delete(`/videos/${id}`);
  },
};

// 字幕相关 API
export const subtitleApi = {
  // 获取视频字幕
  getSubtitles: (videoId: string): Promise<ApiResponse<any>> => {
    return api.get(`/subtitles/${videoId}`);
  },

  // 更新字幕
  updateSubtitles: (videoId: string, subtitles: any): Promise<ApiResponse> => {
    return api.put(`/subtitles/${videoId}`, { subtitles });
  },
};

/**
 * 通用的fetch封装，使用统一的BASE_URL配置
 * 适用于不使用axios的场景
 */
export const apiFetch = async (endpoint: string, options?: RequestInit): Promise<Response> => {
  const baseUrl = getApiBaseUrl();
  const url = endpoint.startsWith('http') ? endpoint : `${baseUrl}${endpoint}`;
  const headers = new Headers(options?.headers);

  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  if (typeof window !== 'undefined' && shouldAttachAuthHeader(url, baseUrl, window.location.origin)) {
    const token = localStorage.getItem('admin_token');
    if (token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${token}`);
    }
  }

  return fetch(url, {
    ...options,
    credentials: 'include',
    headers,
  });
};

export default api;
