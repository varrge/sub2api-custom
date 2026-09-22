/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - 黑金（铜金），与营销首页 --copper #e5b99a 同族
        primary: {
          50: '#faf4ed',
          100: '#f7e6d8',
          200: '#f2d7c0',
          300: '#eec9a8',
          400: '#e9c1a1',
          500: '#e5b99a',
          600: '#c9976f',
          700: '#a97b53',
          800: '#8a5c24',
          900: '#6b4520',
          950: '#3d2812'
        },
        // 辅助色 - 中性灰（日间文本与浅底层级）
        accent: {
          50: '#fafafa',
          100: '#f5f5f7',
          200: '#e5e5e9',
          300: '#d1d1d7',
          400: '#a1a1a9',
          500: '#6e6e76',
          600: '#515158',
          700: '#3a3a41',
          800: '#2b2b31',
          900: '#1d1d1f',
          950: '#0b0b0c'
        },
        // 深色模式背景 - 黑灰层级
        dark: {
          50: '#f5f5f7',
          100: '#e8e8ed',
          200: '#d6d6de',
          300: '#b7b7c0',
          400: '#9999a3',
          500: '#6f6f78',
          600: '#4a4a51',
          700: '#37373e',
          800: '#27272d',
          900: '#1e1e23',
          950: '#17171b'
        }
      },
      // Text uses deeper gold on light surfaces; surfaces retain the copper palette.
      textColor: {
        'on-primary': '#0b0a09',
        primary: {
          400: 'rgb(var(--primary-text-400) / <alpha-value>)',
          500: 'rgb(var(--primary-text-500) / <alpha-value>)',
          600: 'rgb(var(--primary-text-600) / <alpha-value>)',
          700: 'rgb(var(--primary-text-700) / <alpha-value>)'
        },
        dark: {
          400: '#b0b0bb',
          500: '#9e9ea9'
        }
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(0, 0, 0, 0.08)',
        'glass-sm': '0 4px 16px rgba(0, 0, 0, 0.06)',
        glow: '0 0 20px rgba(229, 185, 154, 0.25)',
        'glow-lg': '0 0 40px rgba(229, 185, 154, 0.35)',
        card: '0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06)',
        'card-hover': '0 10px 40px rgba(0, 0, 0, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #eec9a8 0%, #e5b99a 100%)',
        'gradient-dark': 'linear-gradient(135deg, #303036 0%, #1e1e23 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 40% 20%, rgba(229, 185, 154, 0.12) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(201, 151, 111, 0.08) 0px, transparent 50%), radial-gradient(at 0% 50%, rgba(229, 185, 154, 0.08) 0px, transparent 50%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(229, 185, 154, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(229, 185, 154, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
