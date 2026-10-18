import type { AssignmentStatus, Material } from "../shared/types.ts";
import { STATUS_META } from "./assignment-model.ts";

export function textElement<K extends keyof HTMLElementTagNameMap>(
  name: K,
  className: string,
  value: string,
) {
  const element = document.createElement(name);
  if (className) element.className = className;
  element.textContent = String(value);
  return element;
}

export function tableCell(row: HTMLTableRowElement, value: string, className = "", label = "") {
  const item = textElement("td", className, value);
  if (label) item.dataset.label = label;
  row.append(item);
  return item;
}

export function materialName(material?: Material) {
  if (!material) return "教材情報なし";
  const evidence = material.cves?.join("、") || material.lessonId.split("/").at(-1);
  return `${material.product} / ${evidence}`;
}

export function formatDate(value: string) {
  const date = new Date(value);
  return Number.isFinite(date.getTime())
    ? date.toLocaleString("ja-JP", { dateStyle: "medium", timeStyle: "short" })
    : "日時不明";
}

export function statusBadge(status: AssignmentStatus) {
  const meta = STATUS_META[status] ?? { label: status, tone: "neutral" };
  return textElement("span", `status-badge status-${meta.tone}`, meta.label);
}
