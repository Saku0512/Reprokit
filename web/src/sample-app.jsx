import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { initialState } from "./sample-data.js";
import { LessonList, LessonDetail, LessonOutline } from "./sample-views.jsx";
import { LabModal } from "./sample-modal.jsx";
import "@fontsource/noto-sans-jp/400.css";
import "@fontsource/noto-sans-jp/500.css";
import "@fontsource/noto-sans-jp/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./sample.css";

function App() {
  const [state, setState] = useState(initialState);
  const [dark, setDark] = useState(false);
  const [message, setMessage] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const menu = useRef(null);
  const view = useRef(null);
  const pendingPhase = useRef(null);
  const update = (patch) => setState((previous) => ({ ...previous, ...patch }));
  const trackPhase = useCallback((phase) => {
    setState((previous) => (previous.p === phase ? previous : { ...previous, p: phase }));
  }, []);

  useEffect(() => {
    if (menuOpen) menu.current?.showModal();
    else menu.current?.close();
  }, [menuOpen]);

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
    const heading = view.current?.querySelector(`#phase-title-${phase}`);
    pendingPhase.current = null;
    if (!heading) return;
    heading.focus({ preventScroll: true });
    const behavior = window.matchMedia("(prefers-reduced-motion: reduce)").matches
      ? "instant"
      : "smooth";
    const previousBlock =
      previous === undefined ? null : view.current.querySelector(`#phase-${previous}`);
    if (previousBlock) {
      // 「次へ」は読み終えた章の下端を基準にし、次章の余白と導入を残す。
      const detail = previousBlock.closest(".detail");
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
    const escape = (event) => {
      if (event.key === "Escape") update({ modal: null });
    };
    document.addEventListener("keydown", escape);
    return () => document.removeEventListener("keydown", escape);
  }, [state.modal]);
  // サンプルと同じ表示用の進捗。RunnerやDockerへのリクエストは発行しない。
  useEffect(() => {
    if (state.lab !== "deploying") return;
    const timer = setInterval(() => {
      setState((previous) => {
        if (previous.lab !== "deploying") return previous;
        const seg = previous.seg + 1;
        return { ...previous, seg, lab: seg >= 20 ? "running" : "deploying" };
      });
    }, 350);
    return () => clearInterval(timer);
  }, [state.lab]);
  useEffect(() => {
    if (state.lab === "running")
      setMessage("Labが起動しました。「別タブで開く」から画面を開けます");
  }, [state.lab]);

  async function action(event) {
    const target = event.target.closest("[data-a]");
    if (!target || target.disabled) return;
    const { a, v, i } = target.dataset;
    const reset = { repro: false, runV: false, runF: false, normF: false };
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
        update({ cat: v });
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
        const next = Math.min(4, phase + 1);
        if (phase < 4) pendingPhase.current = { phase: next, previous: phase };
        setState((previous) => ({
          ...previous,
          done: { ...previous.done, [phase]: true },
          revealed: Math.max(previous.revealed, next),
          p: next,
        }));
        if (phase === 4) setMessage("お疲れさまでした。DESTROYでLabを片付けましょう");
        break;
      }
      case "prev":
        pendingPhase.current = { phase: Math.max(0, Number(i) - 1) };
        update({ p: Math.max(0, Number(i) - 1) });
        break;
      case "lg":
        update({ login: v });
        break;
      case "hint":
        update({ hint: Math.min(3, state.hint + 1) });
        break;
      case "repro":
        update({ repro: true });
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
      case "hl":
        update({ diffHl: v });
        break;
      case "runV":
      case "runF":
      case "normF":
        update({ [a]: true });
        break;
      case "deploy":
        update({ modal: "deploy", ok: false });
        break;
      case "mc":
        update({ modal: null });
        break;
      case "go-deploy":
        if (state.ok) update({ lab: "deploying", seg: 0, modal: null });
        break;
      case "cancel":
        update({ lab: "ready" });
        setMessage("起動を中止しました");
        break;
      case "reset":
        update(reset);
        setMessage("Labを初期状態に戻しました");
        break;
      case "stop":
        update({ lab: "stopped" });
        setMessage("Labを停止しました");
        break;
      case "destroy":
        update({ modal: "destroy" });
        break;
      case "go-destroy":
        update({ ...reset, lab: "ready", modal: null });
        setMessage(`削除しました（Volume：${state.vol === "keep" ? "保持" : "削除"}）`);
        break;
      case "copy":
        try {
          await navigator.clipboard.writeText(v);
          setMessage("コピーしました");
        } catch {
          setMessage("コピーできませんでした");
        }
        break;
    }
  }

  return (
    <div onClick={action}>
      <div className={`app ${state.view === "detail" ? "learning" : ""}`}>
        <header>
          {state.view === "detail" && (
            <button
              className="menu-toggle"
              aria-label="学習メニューを開く"
              aria-expanded={menuOpen}
              aria-controls="learning-menu"
              onClick={() => setMenuOpen(true)}
            >
              <span aria-hidden="true">
                <i />
                <i />
                <i />
              </span>
            </button>
          )}
          <button className="logo" data-a="home" aria-label="Reprokit ホーム">
            <img src="/assets/reprokit-logo.png" alt="Reprokit" />
          </button>
          <span className="header-tagline" style={{ color: "var(--sub)", fontSize: "13.5px" }}>
            実際のCVEを、自分で試して理解する
          </span>
          <div className="hd">
            <button className="btn sm" id="theme" onClick={() => setDark(!dark)}>
              表示切替
            </button>
          </div>
        </header>
        <div id="view" ref={view} style={{ minHeight: 0, overflow: "hidden" }}>
          {state.view === "list" ? (
            <LessonList state={state} />
          ) : (
            <LessonDetail state={state} onPhaseChange={trackPhase} />
          )}
        </div>
      </div>
      {state.view === "detail" && (
        <dialog
          ref={menu}
          id="learning-menu"
          className="outline-dialog"
          aria-labelledby="learning-menu-title"
          onClose={() => setMenuOpen(false)}
          onClick={(event) => {
            if (event.target === event.currentTarget) menu.current?.close();
          }}
        >
          <div className="outline-dialog-header">
            <h2 id="learning-menu-title">学習メニュー</h2>
            <button
              className="menu-close"
              aria-label="学習メニューを閉じる"
              onClick={() => menu.current?.close()}
              autoFocus
            >
              ×
            </button>
          </div>
          <LessonOutline state={state} id="mobile-lesson-outline" />
        </dialog>
      )}
      <div id="modal">
        <LabModal state={state} onChange={update} />
      </div>
      <div className={`toast ${message ? "on" : ""}`} id="toast" role="status">
        {message}
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")).render(<App />);
