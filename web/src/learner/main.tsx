import type { LearnerState } from "./lesson-data.ts";
import { requiredElement } from "../shared/dom.ts";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { initialState } from "./lesson-data.ts";
import { LearnerShell } from "./learner-shell.tsx";
import { StudyApi } from "../shared/study-api.ts";
import { useLearnerLab } from "./use-learner-lab.ts";
import { createLearnerActionHandler } from "./use-learner-actions.ts";
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
  const action = createLearnerActionHandler({
    state,
    update,
    setState,
    setMessage,
    setMenuOpen,
    menuRef: menu,
    pendingPhaseRef: pendingPhase,
    deployLab,
    stopLab,
    destroyLab,
    showLogs,
    readMarker,
    observeService,
    compareMarker,
    setConsoleOpen,
  });

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
