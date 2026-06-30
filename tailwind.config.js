/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./internal/web/**/*.templ",
    "./internal/web/**/*.go",
    "./static/js/**/*.js",
  ],
  theme: {
    extend: {
      fontFamily: {
        mono: [
          '"JetBrains Mono"',
          '"Fira Code"',
          'ui-monospace',
          'SFMono-Regular',
          'Menlo',
          'Monaco',
          'Consolas',
          'monospace',
        ],
      },
      colors: {
        huginn: {
          bg:           '#0a0a0a',
          surface:      '#111114',
          panel:        '#16161b',
          line:         '#1f1f26',
          mute:         '#5a5a66',
          dim:          '#7c7c87',
          fg:           '#d6d6d9',
          hi:           '#f4f4f5',
          accent:       '#bf5af2',
          'accent-soft':'#7b3fa0',
          'accent-glow':'#d49bff',
          danger:       '#f24d6d',
          ok:           '#7dd181',
        },
      },
    },
  },
  plugins: [],
}
