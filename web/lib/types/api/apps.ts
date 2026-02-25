export interface AppPluginConfig {
  id: string;
  name?: string;
  description?: string;
  capabilities?: string[];
  integration_note?: string;
  sources?: string[];
}

export interface AppsConfig {
  plugins: AppPluginConfig[];
}

export interface AppsConfigSnapshot {
  apps: AppsConfig;
  path?: string;
}

export interface AppsConfigUpdatePayload {
  apps: AppsConfig;
}
