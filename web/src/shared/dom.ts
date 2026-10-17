// 固定HTMLに必須の要素がない場合は、初期化時に検出する。
export function requiredElement<T extends HTMLElement = HTMLElement>(
  selector: string,
  root: ParentNode = document,
): T {
  const element = root.querySelector<T>(selector);
  if (!element) throw new Error(`画面に必要な要素がありません: ${selector}`);
  return element;
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
