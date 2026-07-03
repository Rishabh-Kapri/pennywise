export const config = {
  apiBaseUrl: import.meta.env.VITE_API_URL || 'http://localhost:5151/api',
  isDevelopment: import.meta.env.DEV,
  isProduction: import.meta.env.PROD,
  mode: import.meta.env.MODE,
  demoMode: String(import.meta.env.VITE_DEMO_MODE).toLowerCase().trim().replace(/['"]/g, '') === 'true',
} as const;
