const env = import.meta.env;

export const config = {
  apiBaseUrl: env.VITE_API_URL || 'http://localhost:5151/api',
  /** Empty when Google sign-in isn't set up (the Docker demo). */
  googleClientId: env.VITE_GOOGLE_CLIENT_ID || '',
  demoMode: String(env.VITE_DEMO_MODE).toLowerCase().trim().replace(/['"]/g, '') === 'true',
  // Overridable so a blocked or poor basemap is an env change rather than a
  // release. OSM's volunteer tile servers require a Referer and are explicitly
  // not intended for application traffic; MapTiler and Mapbox free tiers take a
  // key in the URL and are far more dependable.
  mapTileUrl: env.VITE_MAP_TILE_URL || 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
  mapTileAttribution:
    env.VITE_MAP_TILE_ATTRIBUTION ||
    '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
} as const;
