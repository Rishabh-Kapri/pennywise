interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly VITE_GOOGLE_CLIENT_ID?: string;
  readonly VITE_DEMO_MODE?: string;
  readonly VITE_MAP_TILE_URL?: string;
  readonly VITE_MAP_TILE_ATTRIBUTION?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
