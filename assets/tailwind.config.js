/** @type {import('tailwindcss').Config} */
module.exports = {
  theme: {
    extend: {
      colors: {
        uc: {
          primary:  '#7C3AED',
          accent:   '#00D4FF',
          bg:       '#0A0A0F',
          surface:  '#111118',
          border:   '#1E1E2E',
          text:     '#E4E4E7',
          muted:    '#71717A',
          faint:    '#52525B',
          success:  '#10B981',
          warning:  '#F59E0B',
          error:    '#F43F5E',
        },
      },
      fontFamily: {
        display: ['"Space Grotesk"', 'system-ui', 'sans-serif'],
        code:    ['"JetBrains Mono"', 'monospace'],
      },
    },
  },
};
