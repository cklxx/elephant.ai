import type { Config } from "tailwindcss";

/**
 * Tailwind Config - ALEX Visual Language
 *
 * Design Principles:
 * - Flat, line-based surfaces (borders over elevation)
 * - Clean typography with Plus Jakarta Sans
 * - Consistent radius scale across components
 * - High information density with maintained readability
 */

const config: Config = {
  darkMode: ["class"],
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx}",
    "./node_modules/streamdown/dist/index.js",
  ],
  theme: {
    extend: {
      backgroundImage: {
        "app-canvas":
          "radial-gradient(1200px circle at 12% 18%, rgba(56,189,248,0.18), transparent 52%), radial-gradient(900px circle at 88% 14%, rgba(34,211,238,0.14), transparent 48%), radial-gradient(1100px circle at 44% 92%, rgba(99,102,241,0.10), transparent 55%), linear-gradient(180deg, hsl(var(--background)) 0%, hsl(var(--muted)) 100%)",
      },
      /**
       * Console color palette
       * - Grays: Low-saturation, desaturated tones
       * - Primary: Minimal blue accent (low saturation)
       * - Destructive: Subdued red for errors
       * - All colors pass WCAG AA contrast requirements
       */
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
        // Extended grayscale palette tuned for the console UI
        gray: {
          50: "hsl(var(--gray-50))",
          100: "hsl(var(--gray-100))",
          200: "hsl(var(--gray-200))",
          300: "hsl(var(--gray-300))",
          400: "hsl(var(--gray-400))",
          500: "hsl(var(--gray-500))",
          600: "hsl(var(--gray-600))",
          700: "hsl(var(--gray-700))",
          800: "hsl(var(--gray-800))",
          900: "hsl(var(--gray-900))",
          950: "hsl(var(--gray-950))",
        },
        slate: {
          50: "hsl(var(--gray-50))",
          100: "hsl(var(--gray-100))",
          200: "hsl(var(--gray-200))",
          300: "hsl(var(--gray-300))",
          400: "hsl(var(--gray-400))",
          500: "hsl(var(--gray-500))",
          600: "hsl(var(--gray-600))",
          700: "hsl(var(--gray-700))",
          800: "hsl(var(--gray-800))",
          900: "hsl(var(--gray-900))",
          950: "hsl(var(--gray-950))",
        },
      },
      /**
       * Typography Scale
       * - Font family: Plus Jakarta Sans with system fallbacks
       * - Bold headings (600-700 weight)
       * - Regular body (400 weight)
       */
      fontFamily: {
        sans: [
          'var(--font-sans)',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'Noto Sans CJK SC',
          'sans-serif',
        ],
        mono: [
          'var(--font-mono)',
          'ui-monospace',
          'SFMono-Regular',
          'SF Mono',
          'Menlo',
          'Consolas',
          'Liberation Mono',
          'monospace',
        ],
      },
      fontSize: {
        'xs': ['0.75rem', { lineHeight: '1rem' }],
        'sm': ['0.875rem', { lineHeight: '1.25rem' }],
        'base': ['1rem', { lineHeight: '1.5rem' }],
        'lg': ['1.125rem', { lineHeight: '1.75rem' }],
        'xl': ['1.25rem', { lineHeight: '1.75rem' }],
        '2xl': ['1.5rem', { lineHeight: '2rem' }],
        '3xl': ['1.875rem', { lineHeight: '2.25rem' }],
        '4xl': ['2.25rem', { lineHeight: '2.5rem' }],
      },
      /**
       * Radius scale
       * - Driven by the global --radius token (globals.css)
       * - Keeps the app consistent even when different `rounded-*` utilities are used
       */
      borderRadius: {
        none: '0',
        sm: 'calc(var(--radius) - 4px)',
        DEFAULT: 'calc(var(--radius) - 2px)',
        md: 'calc(var(--radius) - 2px)',
        lg: 'var(--radius)',
        xl: 'var(--radius)',
        '2xl': 'var(--radius)',
        '3xl': 'var(--radius)',
        full: '9999px',
      },
      /**
       * Flat (no elevation) shadow scale
       * - Prefer borders, background tones, and spacing for hierarchy
       */
      boxShadow: {
        sm: 'none',
        DEFAULT: 'none',
        md: 'none',
        lg: 'none',
        xl: 'none',
        '2xl': 'none',
        inner: 'none',
        none: 'none',
      },
      /**
       * Spacing Scale
       * - Generous spacing for readability
       * - Consistent rhythm throughout UI
       */
      spacing: {
        '18': '4.5rem',
        '112': '28rem',
        '128': '32rem',
      },
      keyframes: {
        "accordion-down": {
          from: { height: "0" },
          to: { height: "var(--radix-accordion-content-height)" },
        },
        "accordion-up": {
          from: { height: "var(--radix-accordion-content-height)" },
          to: { height: "0" },
        },
        shimmer: {
          "0%": { "background-position": "-200% 0" },
          "100%": { "background-position": "200% 0" },
        },
        gradient: {
          "0%, 100%": {
            opacity: "1",
            transform: "scale(1) rotate(0deg)",
          },
          "50%": {
            opacity: "0.8",
            transform: "scale(1.1) rotate(5deg)",
          },
        },
        "spin-fast": {
          from: { transform: "rotate(0deg)" },
          to: { transform: "rotate(360deg)" },
        },
      },
      animation: {
        "accordion-down": "accordion-down 0.15s ease-out",
        "accordion-up": "accordion-up 0.15s ease-out",
        shimmer: "shimmer 1.2s linear infinite",
        gradient: "gradient 12s ease-in-out infinite",
        "spin-fast": "spin-fast 0.5s linear infinite",
      },
    },
  },
  plugins: [],
};

export default config;
