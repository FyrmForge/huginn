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
          accent:       '#f5a623',
          'accent-soft':'#a06b10',
          'accent-glow':'#ffc55a',
          danger:       '#f24d6d',
          ok:           '#7dd181',
        },
      },
    },
  },
  plugins: [],
}
