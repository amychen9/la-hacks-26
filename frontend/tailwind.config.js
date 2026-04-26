const colors = require("tailwindcss/colors")

module.exports = {
  content: [
    "./index.html",
    "./public/**/*.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  important: true,
  theme: {
    extend: {
      fontSize: {
        xs: ["0.813rem", "1rem"],
      },
    },
    colors: {
      transparent: "transparent",
      current: "currentColor",
      "pale-green": "#E7E6DA",
      "light-green": "#8FA47C",
      "ligher-green": "#F5F3EB",
      green: "#6F7F5F",
      "dark-green": "#56674A",
      "darkest-green": "#425138",
      "light-blue": "#AAB8A0",
      blue: "#748A64",
      orange: "#B7905C",
      yellow: "#E9DCC2",
      "dark-yellow": "#7B6747",
      white: "#FFFEFB",
      "off-white": "#F6F2E8",
      black: "#1F231C",
      gray: "#C9C3B7",
      "dark-gray": "#7B7469",
      "very-dark-gray": "#59544C",
      "light-gray": "#F0ECE2",
      "light-gray-stroke": "#D9D2C5",
      "avail-green": colors.emerald, // The green used for marking availability
      red: "#DB1616",
    },
    screens: {
      sm: "640px",
      md: "768px",
      mdlg: "896px",
      lg: "1024px",
      xl: "1280px",
      "2xl": "1536px",
      "publift-s": "755px",
      "publift-m": "995px",
      "publift-l": "1225px",
      "publift-xl": "1475px",
    },
  },
  plugins: [],
  prefix: "tw-",
  safelist: [],
}
