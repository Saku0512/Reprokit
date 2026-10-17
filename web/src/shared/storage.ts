// ブラウザには回答下書きとRunner発行IDを保存し、認証トークンは保存しない。
export function localRecord<T>(key: string, fallback: T): T {
  try {
    return JSON.parse(localStorage.getItem(`reprokit:${key}`) ?? "null") ?? fallback;
  } catch {
    return fallback;
  }
}

export function saveLocal(key: string, value: unknown) {
  try {
    localStorage.setItem(`reprokit:${key}`, JSON.stringify(value));
    return true;
  } catch {
    return false;
  }
}
