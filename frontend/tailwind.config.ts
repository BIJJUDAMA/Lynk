import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        canvas: "var(--canvas)",
        surface: "var(--surface)",
        "surface-elevated": "var(--surface-elevated)",
        ink: "var(--ink)",
        "ink-secondary": "var(--ink-secondary)",
        line: "var(--line)",
        "accent-muted": "var(--accent-muted)",
        "accent-deep": "var(--accent-deep)",
        "accent-soft": "var(--accent-soft)",
        "accent-warm": "var(--accent-warm)",
        border: "var(--line)",
        input: "var(--line)",
        ring: "var(--accent-muted)",
        background: "var(--canvas)",
        foreground: "var(--ink)",
        primary: {
          DEFAULT: "var(--ink)",
          foreground: "var(--canvas)",
        },
        secondary: {
          DEFAULT: "var(--surface)",
          foreground: "var(--ink)",
        },
        muted: {
          DEFAULT: "var(--surface)",
          foreground: "var(--ink-secondary)",
        },
        accent: {
          DEFAULT: "var(--surface)",
          foreground: "var(--ink)",
        },
        card: {
          DEFAULT: "var(--surface-elevated)",
          foreground: "var(--ink)",
        },
        popover: {
          DEFAULT: "var(--surface-elevated)",
          foreground: "var(--ink)",
        },
        destructive: {
          DEFAULT: "#C0392B",
          foreground: "#FAF9F5",
        },
        info: {
          DEFAULT: "var(--info)",
          foreground: "var(--info-foreground)",
        },
        success: {
          DEFAULT: "var(--success)",
          foreground: "var(--success-foreground)",
        },
        warning: {
          DEFAULT: "var(--warning)",
          foreground: "var(--warning-foreground)",
        },
        pastel: {
          red: "var(--pastel-red)",
          redText: "var(--pastel-red-text)",
          blue: "var(--pastel-blue)",
          blueText: "var(--pastel-blue-text)",
          green: "var(--pastel-green)",
          greenText: "var(--pastel-green-text)",
          yellow: "var(--pastel-yellow)",
          yellowText: "var(--pastel-yellow-text)",
        },
      },
      fontFamily: {
        display: ["var(--font-display)", "Georgia", "serif"],
        sans: ["var(--font-sans)", "-apple-system", "BlinkMacSystemFont", "sans-serif"],
        mono: ["var(--font-mono)", "\"SF Mono\"", "monospace"],
        serif: ["var(--font-display)", "Georgia", "serif"],
      },
      borderRadius: {
        none: "0px",
        sm: "2px",
        DEFAULT: "4px",
        md: "4px",
        lg: "6px",
        xl: "8px",
        "2xl": "12px",
        full: "9999px",
      },
    },
  },
  plugins: [],
};

export default config;
