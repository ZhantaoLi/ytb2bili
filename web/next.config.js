/** @type {import('next').NextConfig} */
const nextConfig = {
  outputFileTracingRoot: __dirname,

  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8096/api/v1',
  },

  // API 代理配置（开发环境）
  async rewrites() {
    return [
      {
        source: '/api/v1/:path*',
        destination: 'http://localhost:8096/api/v1/:path*',
      },
    ];
  },

  // 图片域名白名单配置
  images: {
    remotePatterns: [
      {
        protocol: 'https',
        hostname: 'passport.bilibili.com',
        pathname: '/x/passport-tv-login/h5/qrcode/auth/**',
      },
    ],
  },
}

module.exports = nextConfig
