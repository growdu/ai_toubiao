/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        brand: {
          50:  '#eef4ff',
          100: '#d9e6ff',
          200: '#bcd2ff',
          300: '#8eb4ff',
          400: '#5b8bff',
          500: '#3567f6',
          600: '#224be0',
          700: '#1c3bb4',
          800: '#1b3490',
          900: '#1c3073',
          950: '#131e47',
        },
        ink: {
          50:  '#f7f8fb',
          100: '#eef0f5',
          200: '#dde2ec',
          300: '#bcc4d4',
          400: '#8a95ab',
          500: '#5e6a82',
          600: '#404a61',
          700: '#2c3447',
          800: '#1c2233',
          900: '#10142a',
        },
        // Semantic dark-mode tokens
        surface: {
          DEFAULT: '#ffffff',
          subtle:  '#f7f8fb',
          muted:   '#eef0f5',
          inverse: '#10142a',
        },
      },
      fontFamily: {
        sans: ['Inter', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        'soft': '0 1px 2px rgba(16, 20, 42, 0.04), 0 4px 16px rgba(16, 20, 42, 0.06)',
        'pop':  '0 6px 24px rgba(34, 75, 224, 0.15)',
        'inset-soft': 'inset 0 1px 2px rgba(16, 20, 42, 0.06)',
      },
      backgroundImage: {
        'brand-gradient': 'linear-gradient(135deg, #3567f6 0%, #5b8bff 50%, #8eb4ff 100%)',
        'brand-gradient-soft': 'linear-gradient(135deg, #eef4ff 0%, #ffffff 100%)',
        'mesh-1': 'radial-gradient(at 20% 10%, rgba(139,180,255,0.25) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(53,103,246,0.18) 0px, transparent 50%), radial-gradient(at 80% 100%, rgba(139,180,255,0.12) 0px, transparent 50%)',
      },
      animation: {
        'fade-in':    'fadeIn 200ms ease-out',
        'slide-up':   'slideUp 240ms cubic-bezier(0.16, 1, 0.3, 1)',
        'slide-down': 'slideDown 240ms cubic-bezier(0.16, 1, 0.3, 1)',
        'scale-in':   'scaleIn 180ms cubic-bezier(0.16, 1, 0.3, 1)',
        'shimmer':    'shimmer 2s linear infinite',
        'pulse-soft': 'pulseSoft 2.4s ease-in-out infinite',
      },
      keyframes: {
        fadeIn:   { '0%': { opacity: '0' }, '100%': { opacity: '1' } },
        slideUp:  { '0%': { opacity: '0', transform: 'translateY(8px)' }, '100%': { opacity: '1', transform: 'translateY(0)' } },
        slideDown:{ '0%': { opacity: '0', transform: 'translateY(-8px)' }, '100%': { opacity: '1', transform: 'translateY(0)' } },
        scaleIn:  { '0%': { opacity: '0', transform: 'scale(0.96)' }, '100%': { opacity: '1', transform: 'scale(1)' } },
        shimmer:  { '0%': { backgroundPosition: '-200% 0' }, '100%': { backgroundPosition: '200% 0' } },
        pulseSoft:{ '0%,100%': { opacity: '1' }, '50%': { opacity: '0.55' } },
      },
    },
  },
  plugins: [],
}