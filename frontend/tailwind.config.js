/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色调 - Teal/Cyan 青色系
        primary: {
          50: '#fbf6eb',
          100: '#f5e8ca',
          200: '#ead3a5',
          300: '#dec087',
          400: '#d3b574',
          500: '#c9aa71',
          600: '#987137',
          700: '#795726',
          800: '#614623',
          900: '#44321d',
          950: '#271e13'
        },
        // 辅助色 - 深蓝灰
        accent: {
          50: '#faf8f3',
          100: '#f2eee6',
          200: '#e2ddd2',
          300: '#c9c1b3',
          400: '#aaa196',
          500: '#81796f',
          600: '#665f57',
          700: '#4a453f',
          800: '#302d29',
          900: '#201e1b',
          950: '#12110f'
        },
        // 深色模式背景
        dark: {
          50: '#f5f2ea',
          100: '#e6e1d7',
          200: '#cbc4b7',
          300: '#b9b3a8',
          400: '#a29c91',
          500: '#878178',
          600: '#4b4b48',
          700: '#30343a',
          800: '#181b20',
          900: '#111418',
          950: '#0b0d10'
        }
      },
      fontFamily: {
        display: ['Georgia', 'Times New Roman', 'serif'],
        sans: [
          'PingFang SC',
          'Hiragino Sans GB',
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
        glow: '0 0 20px rgba(201, 170, 113, 0.25)',
        'glow-lg': '0 0 40px rgba(201, 170, 113, 0.35)',
        card: '0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06)',
        'card-hover': '0 10px 40px rgba(0, 0, 0, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #c9aa71 0%, #987137 100%)',
        'gradient-dark': 'linear-gradient(135deg, #181b20 0%, #111418 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 40% 20%, rgba(201, 170, 113, 0.12) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(177, 167, 147, 0.08) 0px, transparent 50%), radial-gradient(at 0% 50%, rgba(201, 170, 113, 0.08) 0px, transparent 50%)'
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
          '0%': { boxShadow: '0 0 20px rgba(201, 170, 113, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(201, 170, 113, 0.4)' }
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
