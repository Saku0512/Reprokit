import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { initialState } from "./sample-data.js";
import { LessonList, LessonDetail } from "./sample-views.jsx";
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
  const view = useRef(null);
  const update = (patch) => setState((previous) => ({ ...previous, ...patch }));

  useEffect(() => {
    document.documentElement.dataset.theme = dark ? "dark" : "light";
  }, [dark]);
  useEffect(() => {
    if (!message) return;
    const timer = setTimeout(() => setMessage(""), 2200);
    return () => clearTimeout(timer);
  }, [message]);
  useEffect(() => {
    view.current?.querySelector(".detail")?.scrollTo(0, 0);
  }, [state.view, state.p]);
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
        update({ view: "detail", p: 0 });
        break;
      case "cat":
        update({ cat: v });
        break;
      case "go":
        update({ p: Number(i) });
        break;
      case "next":
        update({ done: { ...state.done, [state.p]: true }, p: Math.min(4, state.p + 1) });
        if (state.p === 4) setMessage("お疲れさまでした。DESTROYでLabを片付けましょう");
        break;
      case "prev":
        update({ p: Math.max(0, state.p - 1) });
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
      <div className="app">
        <header>
          <button className="logo" data-a="home">
            Reprokit
          </button>
          <span style={{ color: "var(--sub)", fontSize: "13.5px" }}>
            実際のCVEを、自分で試して理解する
          </span>
          <div className="hd">
            <button className="btn sm" id="theme" onClick={() => setDark(!dark)}>
              表示切替
            </button>
          </div>
        </header>
        <div id="view" ref={view} style={{ minHeight: 0, overflow: "hidden" }}>
          {state.view === "list" ? <LessonList state={state} /> : <LessonDetail state={state} />}
        </div>
      </div>
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
