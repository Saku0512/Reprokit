import type { MouseEvent } from "react";
import type { LearnerState } from "./lesson-data.ts";
import { requiredElement } from "../shared/dom.ts";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { initialState, PH } from "./lesson-data.ts";
import { LearnerShell } from "./learner-shell.tsx";
import { StudyApi } from "../shared/study-api.ts";
import { useLearnerLab } from "./use-learner-lab.ts";
import "@fontsource/noto-sans-jp/400.css";
import "@fontsource/noto-sans-jp/500.css";
import "@fontsource/noto-sans-jp/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./learner.css";

const api = new StudyApi(
  document.querySelector<HTMLMetaElement>('meta[name="reprokit-session"]')?.content ?? "",
);

function App() {
  const [state, setState] = useState(initialState);
  const [dark, setDark] = useState(false);
  const [message, setMessage] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const [consoleOpen, setConsoleOpen] = useState(false);
  const menu = useRef<HTMLDialogElement>(null);
  const consoleDialog = useRef<HTMLDialogElement>(null);
  const view = useRef<HTMLDivElement>(null);
  const pendingPhase = useRef<{ phase: number; previous?: number } | null>(null);
  const update = (patch: Partial<LearnerState>) =>
    setState((previous) => ({ ...previous, ...patch }));
  const trackPhase = useCallback((phase: number) => {
    setState((previous) => (previous.p === phase ? previous : { ...previous, p: phase }));
  }, []);
  const { deployLab, stopLab, destroyLab, showLogs, readMarker, observeService, compareMarker } =
    useLearnerLab({ api, state, update, setState, setMessage });

  useEffect(() => {
    if (menuOpen) menu.current?.showModal();
    else menu.current?.close();
  }, [menuOpen]);

  useEffect(() => {
    if (consoleOpen) consoleDialog.current?.showModal();
    else consoleDialog.current?.close();
  }, [consoleOpen]);

  useEffect(() => {
    const compact = window.matchMedia("(max-width: 1000px)");
    const resize = () => {
      if (!compact.matches) {
        menu.current?.close();
        setMenuOpen(false);
      }
    };
    compact.addEventListener("change", resize);
    return () => compact.removeEventListener("change", resize);
  }, []);

  // 章の追加後に移動し、通常のスクロールやLab状態更新では位置を変えない。
  useLayoutEffect(() => {
    if (pendingPhase.current === null) return;
    const { phase, previous } = pendingPhase.current;
    const heading = view.current?.querySelector<HTMLElement>(`#phase-title-${phase}`);
    pendingPhase.current = null;
    if (!heading) return;
    heading.focus({ preventScroll: true });
    const behavior = window.matchMedia("(prefers-reduced-motion: reduce)").matches
      ? "instant"
      : "smooth";
    const previousBlock =
      previous === undefined
        ? null
        : view.current?.querySelector<HTMLElement>(`#phase-${previous}`);
    if (previousBlock) {
      // 「次へ」は読み終えた章の下端を基準にし、次章の余白と導入を残す。
      const detail = previousBlock.closest<HTMLElement>(".detail")!;
      detail.scrollTo({
        top:
          detail.scrollTop +
          previousBlock.getBoundingClientRect().bottom -
          detail.getBoundingClientRect().top,
        behavior,
      });
    } else {
      heading.scrollIntoView({ behavior, block: "start" });
    }
  }, [state]);

  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
  }, [dark]);
  useEffect(() => {
    if (!message) return;
    const timer = setTimeout(() => setMessage(""), 2200);
    return () => clearTimeout(timer);
  }, [message]);
  useEffect(() => {
    if (!state.modal) return;
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") update({ modal: null });
    };
    document.addEventListener("keydown", escape);
    return () => document.removeEventListener("keydown", escape);
  }, [state.modal]);
  async function action(event: MouseEvent<HTMLDivElement>) {
    const target =
      event.target instanceof Element ? event.target.closest<HTMLElement>("[data-a]") : null;
    if (!target || (target instanceof HTMLButtonElement && target.disabled)) return;
    const { a, v, i } = target.dataset;
    switch (a) {
      case "home":
        update({ view: "list" });
        break;
      case "open":
        update({ view: "detail", p: -1 });
        break;
      case "begin":
        pendingPhase.current = { phase: 0 };
        setState((previous) => ({ ...previous, p: 0, revealed: Math.max(0, previous.revealed) }));
        break;
      case "cat":
        update({ cat: v ?? "all" });
        break;
      case "go":
        if (Number(i) > state.revealed) break;
        menu.current?.close();
        setMenuOpen(false);
        pendingPhase.current = { phase: Number(i) };
        update({ p: Number(i) });
        break;
      case "next": {
        const phase = Number(i);
        const next = Math.min(PH.length - 1, phase + 1);
        if (phase < PH.length - 1) pendingPhase.current = { phase: next, previous: phase };
        setState((previous) => ({
          ...previous,
          done: { ...previous.done, [phase]: true },
          revealed: Math.max(previous.revealed, next),
          p: next,
        }));
        if (phase === PH.length - 1) setMessage("お疲れさまでした。DESTROYでLabを片付けましょう");
        break;
      }
      case "prev":
        pendingPhase.current = { phase: Math.max(0, Number(i) - 1) };
        update({ p: Math.max(0, Number(i) - 1) });
        break;
      case "hint":
        update({ hint: Math.min(3, state.hint + 1) });
        break;
      case "repro":
        await observeService("vulnerable");
        break;
      case "rst":
        update({ repro: false, hint: 0 });
        break;
      case "pick":
        if (Number(i) <= state.pos) update({ pick: Number(i) });
        break;
      case "back":
        update({ pos: Math.max(0, state.pos - 1), pick: Math.max(0, state.pos - 1) });
        break;
      case "fwd":
        update({ pos: Math.min(5, state.pos + 1), pick: Math.min(5, state.pos + 1) });
        break;
      case "compare":
        if (v === "vulnerable" || v === "fixed") await compareMarker(v);
        break;
      case "deploy":
        update({ modal: "deploy", ok: false });
        break;
      case "mc":
        update({ modal: null });
        break;
      case "go-deploy":
        if (state.ok) await deployLab();
        break;
      case "stop":
        await stopLab();
        break;
      case "destroy":
        update({ modal: "destroy" });
        break;
      case "go-destroy":
        await destroyLab();
        break;
      case "copy":
        try {
          await navigator.clipboard.writeText(v ?? "");
          setMessage("コピーしました");
        } catch {
          setMessage("コピーできませんでした");
        }
        break;
      case "console":
        setConsoleOpen(true);
        break;
      case "close-console":
        setConsoleOpen(false);
        break;
      case "logs":
        if (v === "vulnerable" || v === "fixed") await showLogs(v);
        break;
      case "marker":
        if (v === "vulnerable" || v === "fixed") await readMarker(v);
        break;
    }
  }

  return (
    <LearnerShell
      state={state}
      dark={dark}
      message={message}
      menuOpen={menuOpen}
      consoleOpen={consoleOpen}
      menuRef={menu}
      consoleDialogRef={consoleDialog}
      viewRef={view}
      onAction={action}
      onMenuOpen={() => setMenuOpen(true)}
      onMenuOpenChange={setMenuOpen}
      onConsoleOpenChange={setConsoleOpen}
      onDarkToggle={() => setDark((previous) => !previous)}
      onPhaseChange={trackPhase}
      onStateChange={update}
    />
  );
}

createRoot(requiredElement("#root")).render(<App />);
