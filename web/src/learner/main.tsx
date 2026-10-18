import type { MouseEvent } from "react";
import type { LearnerState } from "./lesson-data.ts";
import type { LabSession, LabRef } from "../shared/types.ts";
import { requiredElement } from "../shared/dom.ts";
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { initialState, PH } from "./lesson-data.ts";
import { LabConsole, LessonList, LessonDetail, LessonOutline } from "./lesson-views.tsx";
import { LabModal } from "./lab-modal.tsx";
import { ApiError, StudyApi, canStart } from "../shared/study-api.ts";
import "@fontsource/noto-sans-jp/400.css";
import "@fontsource/noto-sans-jp/500.css";
import "@fontsource/noto-sans-jp/700.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./learner.css";

const api = new StudyApi(
  document.querySelector<HTMLMetaElement>('meta[name="reprokit-session"]')?.content ?? "",
);
const labSessionStorageKey = "reprokit:sample:filebrowser:lab-session";

function labState(session: LabSession | null): LearnerState["lab"] {
  if (session?.state === "running") return "running";
  if (session?.state === "stopped") return "stopped";
  if (session?.state === "error") return "failed";
  return "ready";
}

function rememberLabSession(session: LabSession) {
  if (!session?.labSessionId) return;
  try {
    window.localStorage.setItem(labSessionStorageKey, session.labSessionId);
  } catch {
    // Lab Sessionは画面内の状態だけで継続する。
  }
}

function forgetLabSession() {
  try {
    window.localStorage.removeItem(labSessionStorageKey);
  } catch {
    // 保存先が使えない場合も、削除操作の結果は維持する。
  }
}

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
  const deployProgressTimer = useRef<number | null>(null);
  const deployProgressValue = useRef(0);
  const comparisonRequests = useRef({ vulnerable: 0, fixed: 0 });
  const update = (patch: Partial<LearnerState>) =>
    setState((previous) => ({ ...previous, ...patch }));
  const trackPhase = useCallback((phase: number) => {
    setState((previous) => (previous.p === phase ? previous : { ...previous, p: phase }));
  }, []);

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
  useEffect(
    () => () => {
      if (deployProgressTimer.current) window.clearInterval(deployProgressTimer.current);
    },
    [],
  );
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
  // 再読み込み後もRunnerが所有するSessionだけを状態取得する。
  useEffect(() => {
    let cancelled = false;
    let id;
    try {
      id = window.localStorage.getItem(labSessionStorageKey);
    } catch {
      return undefined;
    }
    if (!id) return undefined;
    api
      .status(id)
      .then((runtime) => {
        if (cancelled) return;
        setState((previous) => ({
          ...previous,
          lab: labState(runtime),
          runtime,
          labFailure: null,
        }));
      })
      .catch(() => {
        if (!cancelled) forgetLabSession();
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function advanceDeployProgress(target: number): Promise<void> {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      deployProgressValue.current = target;
      update({ seg: target });
      return Promise.resolve();
    }
    return new Promise<void>((resolve) => {
      if (deployProgressTimer.current) window.clearInterval(deployProgressTimer.current);
      const timer = window.setInterval(() => {
        const seg = Math.min(target, deployProgressValue.current + 1);
        deployProgressValue.current = seg;
        update({ seg });
        if (seg === target) {
          window.clearInterval(timer);
          if (deployProgressTimer.current === timer) deployProgressTimer.current = null;
          resolve();
        }
      }, 120);
      deployProgressTimer.current = timer;
    });
  }

  async function withDeployProgress<T>(operation: Promise<T>, target: number): Promise<T> {
    // 通信失敗も直ちに捕捉し、進捗表示の完了後に同じエラーとして扱う。
    const result = operation.then(
      (value) => ({ value, error: undefined }),
      (error: unknown) => ({ value: undefined, error }),
    );
    await advanceDeployProgress(target);
    const { value, error } = await result;
    if (error) throw error;
    return value as T;
  }

  async function deployLab() {
    if (state.lab === "deploying") return;
    resetComparison();
    deployProgressValue.current = 0;
    update({
      lab: "deploying",
      seg: 0,
      modal: null,
      labFailure: null,
      logs: {},
      logFailure: null,
      markers: {},
      markerFailure: null,
    });
    try {
      let runtime = state.runtime;
      if (!canStart(runtime))
        runtime = await withDeployProgress(api.prepare("1.0.0", "vulnerable"), 10);
      if (!canStart(runtime))
        throw new Error("安全性とloopback公開を確認できないため、Labを起動できません。");
      rememberLabSession(runtime);
      update({ runtime });
      runtime = await withDeployProgress(api.start(runtime.labSessionId), 19);
      if (runtime.state !== "running") throw new Error("Labの起動状態を確認できませんでした。");
      rememberLabSession(runtime);
      deployProgressValue.current = 20;
      update({ lab: "running", runtime, seg: 20 });
      setMessage("Labが起動しました。「別タブで開く」から画面を開けます");
    } catch (error) {
      const runtime = error instanceof ApiError ? error.session : undefined;
      if (runtime?.labSessionId) rememberLabSession(runtime);
      update({
        lab: runtime ? labState(runtime) : "ready",
        runtime: runtime ?? state.runtime,
        labFailure: error instanceof Error ? error.message : "Labの起動に失敗しました。",
      });
      setMessage("Labを起動できませんでした。状態を確認してください。");
    }
  }

  async function stopLab() {
    if (!state.runtime?.labSessionId || state.lab === "deploying") return;
    resetComparison();
    update({ lab: "deploying", seg: 16, labFailure: null });
    try {
      const runtime = await api.stop(state.runtime.labSessionId);
      rememberLabSession(runtime);
      update({
        lab: labState(runtime),
        runtime,
        logs: {},
        logFailure: null,
        markers: {},
        markerFailure: null,
      });
      setMessage("Labを停止しました");
    } catch (error) {
      const runtime = error instanceof ApiError ? error.session : undefined;
      if (runtime?.labSessionId) rememberLabSession(runtime);
      update({
        lab: runtime ? labState(runtime) : "running",
        runtime: runtime ?? state.runtime,
        labFailure: error instanceof Error ? error.message : "Labの停止に失敗しました。",
      });
      setMessage("Labを停止できませんでした。状態を確認してください。");
    }
  }

  async function destroyLab() {
    if (!state.runtime?.labSessionId || state.lab === "deploying") return;
    resetComparison();
    update({ lab: "deploying", seg: 18, modal: null, labFailure: null });
    try {
      const runtime = await api.delete(
        state.runtime.labSessionId,
        state.vol === "keep" ? "retain" : "delete",
      );
      if (runtime.state !== "deleted") throw new Error("Labの削除状態を確認できませんでした。");
      forgetLabSession();
      update({ ...resetLabState(), lab: "ready", runtime: null });
      setMessage(`削除しました（Volume：${state.vol === "keep" ? "保持" : "削除"}）`);
    } catch (error) {
      const runtime = error instanceof ApiError ? error.session : undefined;
      if (runtime?.labSessionId) rememberLabSession(runtime);
      update({
        lab: runtime ? labState(runtime) : "failed",
        runtime: runtime ?? state.runtime,
        labFailure: error instanceof Error ? error.message : "Labの削除に失敗しました。",
      });
      setMessage("Labを削除できませんでした。状態を確認してください。");
    }
  }

  async function showLogs(service: LabRef) {
    if (!state.runtime?.labSessionId || state.lab !== "running") return;
    update({ logLoading: service, logFailure: null });
    try {
      const logs = await api.logs(state.runtime.labSessionId, service);
      setState((previous) => ({
        ...previous,
        logs: { ...previous.logs, [service]: logs },
        logLoading: null,
      }));
    } catch (error) {
      update({
        logLoading: null,
        logFailure: error instanceof Error ? error.message : "ログを取得できませんでした。",
      });
    }
  }

  async function readMarker(service: LabRef) {
    if (!state.runtime?.labSessionId || state.lab !== "running") return;
    update({ markerLoading: service, markerFailure: null });
    try {
      const marker = await api.marker(state.runtime.labSessionId, service);
      setState((previous) => ({
        ...previous,
        markers: { ...previous.markers, [service]: marker },
        markerLoading: null,
      }));
    } catch (error) {
      update({
        markerLoading: null,
        markerFailure: error instanceof Error ? error.message : "マーカーを確認できませんでした。",
      });
    }
  }

  async function observeService(service: LabRef) {
    if (!state.runtime?.labSessionId || state.lab !== "running") return;
    update({
      logLoading: service,
      markerLoading: service,
      logFailure: null,
      markerFailure: null,
    });
    const [logsResult, markerResult] = await Promise.allSettled([
      api.logs(state.runtime.labSessionId, service),
      api.marker(state.runtime.labSessionId, service),
    ]);
    setState((previous) => ({
      ...previous,
      repro: previous.repro || (service === "vulnerable" && markerResult.status === "fulfilled"),
      logs:
        logsResult.status === "fulfilled"
          ? { ...previous.logs, [service]: logsResult.value }
          : previous.logs,
      markers:
        markerResult.status === "fulfilled"
          ? { ...previous.markers, [service]: markerResult.value }
          : previous.markers,
      logLoading: null,
      markerLoading: null,
      logFailure:
        logsResult.status === "rejected"
          ? logsResult.reason instanceof Error
            ? logsResult.reason.message
            : "ログを取得できませんでした。"
          : null,
      markerFailure:
        markerResult.status === "rejected"
          ? markerResult.reason instanceof Error
            ? markerResult.reason.message
            : "マーカーを確認できませんでした。"
          : null,
    }));
  }

  function resetComparison() {
    // 停止・削除・再起動前の遅延レスポンスを比較結果へ戻さない。
    comparisonRequests.current.vulnerable++;
    comparisonRequests.current.fixed++;
    update({ comparison: {} });
  }

  async function compareMarker(service: LabRef) {
    if (
      !state.runtime?.labSessionId ||
      state.lab !== "running" ||
      state.comparison[service]?.loading
    )
      return;
    const sessionID = state.runtime.labSessionId;
    const requestID = ++comparisonRequests.current[service];
    setState((previous) => ({
      ...previous,
      comparison: { ...previous.comparison, [service]: { loading: true } },
    }));
    try {
      const marker = await api.marker(sessionID, service);
      if (marker.service !== service || typeof marker.exists !== "boolean")
        throw new Error("対象のマーカーを確認できませんでした。再取得してください。");
      if (requestID !== comparisonRequests.current[service]) return;
      setState((previous) => ({
        ...previous,
        comparison: { ...previous.comparison, [service]: { loading: false, marker } },
      }));
    } catch (error) {
      if (requestID !== comparisonRequests.current[service]) return;
      setState((previous) => ({
        ...previous,
        comparison: {
          ...previous.comparison,
          [service]: {
            loading: false,
            failure: error instanceof Error ? error.message : "マーカーを取得できませんでした。",
          },
        },
      }));
    }
  }

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
          <div className="hd">
            {state.lab === "running" && (
              <button
                className="btn sm"
                data-a="console"
                aria-haspopup="dialog"
                aria-expanded={consoleOpen}
              >
                {"検証コンソール"}
              </button>
            )}
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
      <dialog
        ref={consoleDialog}
        className="console-dialog"
        aria-labelledby="console-title"
        onClose={() => setConsoleOpen(false)}
        onClick={(event) => {
          if (event.target === event.currentTarget) consoleDialog.current?.close();
        }}
      >
        <div className="console-dialog-header">
          <h2 id="console-title">{"検証コンソール"}</h2>
          <button
            className="menu-close"
            data-a="close-console"
            aria-label="検証コンソールを閉じる"
            autoFocus
          >
            ×
          </button>
        </div>
        <LabConsole state={state} idPrefix="dialog-console" />
      </dialog>
      <div id="modal">
        <LabModal state={state} onChange={update} />
      </div>
      <div className={`toast ${message ? "on" : ""}`} id="toast" role="status">
        {message}
      </div>
    </div>
  );
}

function resetLabState(): Partial<LearnerState> {
  return {
    repro: false,
    comparison: {},
    logs: {},
    logLoading: null,
    logFailure: null,
    markers: {},
    markerLoading: null,
    markerFailure: null,
  };
}

createRoot(requiredElement("#root")).render(<App />);
