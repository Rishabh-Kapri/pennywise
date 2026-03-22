import { heroui } from "@heroui/react";

export default heroui({
  themes: {
    dark: {
      colors: {
        // Primary color from index.css
        primary: {
          50: "#edf4f8",
          100: "#d4e5ef",
          200: "#a9cbe0",
          300: "#7eb1d0",
          400: "#5d9cbb",
          500: "#4483a2",
          600: "#376a84",
          700: "#2a5166",
          800: "#1c3848",
          900: "#0f1f2a",
          DEFAULT: "#4483a2",
          foreground: "#ffffff",
        },
        // Background colors matching index.css
        background: "#0d0d0d",
        foreground: "#f6f6f6",
        // Content surfaces matching --color-surface variables
        content1: "#1f1f1f",
        content2: "#323232",
        content3: "#424242",
        content4: "#525252",
        // Divider color
        divider: "#626262",
        // Focus ring
        focus: "#4483a2",
        // Status colors from index.css
        success: {
          DEFAULT: "#10b981",
          foreground: "#ffffff",
        },
        danger: {
          DEFAULT: "#ef4444",
          foreground: "#ffffff",
        },
        warning: {
          DEFAULT: "#f59e0b",
          foreground: "#000000",
        },
        secondary: {
          DEFAULT: "#f6f6f6",
          foreground: "#0d0d0d",
        },
        default: {
          50: "#1f1f1f",
          100: "#2a2a2a",
          200: "#3a3a3a",
          300: "#4a4a4a",
          400: "#5a5a5a",
          500: "#6a6a6a",
          600: "#7a7a7a",
          700: "#8a8a8a",
          800: "#9a9a9a",
          900: "#aaaaaa",
          DEFAULT: "#3a3a3a",
          foreground: "#f6f6f6",
        },
      },
    },
    light: {
      colors: {
        primary: {
          50: "#edf4f8",
          100: "#d4e5ef",
          200: "#a9cbe0",
          300: "#7eb1d0",
          400: "#5d9cbb",
          500: "#4483a2",
          600: "#376a84",
          700: "#2a5166",
          800: "#1c3848",
          900: "#0f1f2a",
          DEFAULT: "#4483a2",
          foreground: "#ffffff",
        },
        background: "#ffffff",
        foreground: "#213547",
        content1: "#f9f9f9",
        content2: "#f0f0f0",
        content3: "#e5e5e5",
        content4: "#d4d4d4",
        divider: "#d4d4d4",
        focus: "#4483a2",
        success: {
          DEFAULT: "#10b981",
          foreground: "#ffffff",
        },
        danger: {
          DEFAULT: "#ef4444",
          foreground: "#ffffff",
        },
        warning: {
          DEFAULT: "#f59e0b",
          foreground: "#000000",
        },
        secondary: {
          DEFAULT: "#f6f6f6",
          foreground: "#213547",
        },
        default: {
          50: "#fafafa",
          100: "#f4f4f5",
          200: "#e4e4e7",
          300: "#d4d4d8",
          400: "#a1a1aa",
          500: "#71717a",
          600: "#52525b",
          700: "#3f3f46",
          800: "#27272a",
          900: "#18181b",
          DEFAULT: "#e4e4e7",
          foreground: "#213547",
        },
      },
    },
  },
  defaultTheme: "dark",
  layout: {
    radius: {
      small: "0.25rem",
      medium: "0.5rem",
      large: "0.75rem",
    },
    borderWidth: {
      small: "1px",
      medium: "1px",
      large: "2px",
    },
  },
});
