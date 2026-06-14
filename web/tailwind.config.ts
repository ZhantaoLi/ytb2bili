import type { Config } from "tailwindcss";

export default {
  darkMode: 'class', // 通过 class="dark" 切换深色模式
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        background: "var(--background)",
        foreground: "var(--foreground)",
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
        },
        bilibili: {
          pink: '#fb7299',
          blue: '#00a1d6',
        }
      },
      keyframes: {
        scan: {
          '0%': { top: '0%' },
          '100%': { top: '100%' },
        }
      },
      animation: {
        scan: 'scan 2.5s linear infinite',
      },
    },
  },
  plugins: [],
} satisfies Config;