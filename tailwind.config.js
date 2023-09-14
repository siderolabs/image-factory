/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./internal/frontend/http/templates/**/*.{html,js}"],
  theme: {
    extend: {},
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
  ],
}
