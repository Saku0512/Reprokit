// ブラウザには回答下書きとRunner発行IDを保存し、認証トークンは保存しない。
export function localRecord(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(`reprokit:${key}`)) ?? fallback;
  } catch {
    return fallback;
  }
}

export function saveLocal(key, value) {
  try {
    localStorage.setItem(`reprokit:${key}`, JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}
