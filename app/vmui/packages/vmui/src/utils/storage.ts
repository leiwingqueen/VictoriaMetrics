/**
 * Do not use this type in local storage type
 * @deprecated
 * */
type DeprecatedStorageKeys = "QUERY_HISTORY" | "QUERY_FAVORITES";

export type StorageKeys = "AUTOCOMPLETE"
  | "NO_CACHE"
  | "QUERY_TRACING"
  | "SERIES_LIMITS"
  | "TABLE_COMPACT"
  | "TIMEZONE"
  | "DISABLED_DEFAULT_TIMEZONE"
  | "THEME"
  | "EXPLORE_METRICS_TIPS"
  | "METRICS_QUERY_HISTORY"
  | "SERVER_URL"
  | "RAW_JSON_LIVE_VIEW"
  | DeprecatedStorageKeys;


export const saveToStorage = (key: StorageKeys, value: string | boolean | Record<string, unknown>): void => {
  if (value) {
    // keeping object in storage so that keeping the string is not different from keeping
    window.localStorage.setItem(key, JSON.stringify({ value }));
  } else {
    removeFromStorage([key]);
  }
  window.dispatchEvent(new Event("storage"));
};

// TODO: make this aware of data type that is stored
export const getFromStorage = (key: StorageKeys): undefined | boolean | string | Record<string, unknown> => {
  const valueObj = window.localStorage.getItem(key);
  if (valueObj === null) {
    return undefined;
  } else {
    try {
      return JSON.parse(valueObj)?.value; // see comment in "saveToStorage"
    } catch (e) {
      return valueObj; // fallback for corrupted json
    }
  }
};

export const removeFromStorage = (keys: StorageKeys[]): void => keys.forEach(k => window.localStorage.removeItem(k));
