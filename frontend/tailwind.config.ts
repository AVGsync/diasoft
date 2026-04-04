import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        sand: "#f8e9a1",
        ink: "#24305e",
        sage: "#a8d0e6",
        moss: "#374785",
        clay: "#f76c6c"
      },
      boxShadow: {
        quiet: "0 24px 70px rgba(36, 48, 94, 0.12)"
      }
    }
  },
  plugins: []
} satisfies Config;
