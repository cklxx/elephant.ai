import type { Config } from "tailwindcss";

/**
 * Tailwind Config - ALEX Visual Language
 *
 * Design Principles:
 * - Low-saturation grayscale palette
 * - Minimal accent colors (used sparingly)
 * - Clean typography with Inter font family
 * - Subtle borders and minimal shadows
 * - High information density with maintained readability
 */

const config: Config = {
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
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
      },
      /**
       * Typography Scale
       * - Font family: Inter with system fallbacks
       * - Bold headings (600-700 weight)
       * - Regular body (400 weight)
       */
      fontFamily: {
        sans: [
          'Inter',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'sans-serif',
        ],
        mono: [
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
       * Minimal Border Radius
       * - Sharp corners with subtle rounding (2-4px)
       * - Avoids overly rounded modern UI trends
       */
      borderRadius: {
        none: '0',
        sm: '2px',
        DEFAULT: '3px',
        md: '4px',
        lg: '4px',
        full: '9999px',
      },
      /**
       * Subtle Shadow System
       * - Minimal elevation changes
       * - Low opacity, small blur radius
       * - Used sparingly for depth hierarchy
       */
      boxShadow: {
        sm: '0 1px 2px 0 rgba(0, 0, 0, 0.03)',
        DEFAULT: '0 1px 3px 0 rgba(0, 0, 0, 0.05)',
        md: '0 2px 4px 0 rgba(0, 0, 0, 0.06)',
        lg: '0 4px 6px 0 rgba(0, 0, 0, 0.07)',
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
    },
  },
  plugins: [],
};

export default config;
