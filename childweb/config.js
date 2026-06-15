export const DEFAULT_CONFIG = Object.freeze({
  baseURL: '',
  accessCode: '',
  deviceId: '9c:13:9e:8b:af:28',
});

const STORAGE_KEYS = Object.freeze({
  baseURL: 'anban_childweb_base_url',
  accessCode: 'anban_childweb_access_code',
  deviceId: 'anban_childweb_device_id',
});

function normalizeBaseURL(value) {
  return String(value || '').trim().replace(/\/+$/, '');
}

export function normalizeConfig(config = {}) {
  return {
    baseURL: normalizeBaseURL(config.baseURL),
    accessCode: String(config.accessCode || '').trim(),
    deviceId: String(config.deviceId || DEFAULT_CONFIG.deviceId).trim() || DEFAULT_CONFIG.deviceId,
  };
}

export function loadConfig(storage = globalThis.localStorage) {
  if (!storage) return { ...DEFAULT_CONFIG };

  return normalizeConfig({
    baseURL: storage.getItem(STORAGE_KEYS.baseURL) ?? DEFAULT_CONFIG.baseURL,
    accessCode: storage.getItem(STORAGE_KEYS.accessCode) ?? DEFAULT_CONFIG.accessCode,
    deviceId: storage.getItem(STORAGE_KEYS.deviceId) ?? DEFAULT_CONFIG.deviceId,
  });
}

export function saveConfig(config, storage = globalThis.localStorage) {
  const normalized = normalizeConfig(config);
  if (!storage) return normalized;

  for (const [name, key] of Object.entries(STORAGE_KEYS)) {
    storage.setItem(key, normalized[name]);
  }
  return normalized;
}
