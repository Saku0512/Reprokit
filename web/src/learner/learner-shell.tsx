import type { MouseEvent, RefObject } from "react";
import type { LearnerState } from "./lesson-data.ts";
import { LessonList } from "./lesson-catalog.tsx";
import { LabConsole } from "./lab-console.tsx";
import { LabModal } from "./lab-modal.tsx";
import { LessonOutline } from "./lesson-outline.tsx";
import { LessonDetail } from "./lesson-views.tsx";

interface LearnerShellProps {
  state: LearnerState;
  dark: boolean;
  message: string;
  menuOpen: boolean;
  consoleOpen: boolean;
  menuRef: RefObject<HTMLDialogElement | null>;
  consoleDialogRef: RefObject<HTMLDialogElement | null>;
  viewRef: RefObject<HTMLDivElement | null>;
  onAction: (event: MouseEvent<HTMLDivElement>) => void;
  onMenuOpen: () => void;
  onMenuOpenChange: (open: boolean) => void;
  onConsoleOpenChange: (open: boolean) => void;
  onDarkToggle: () => void;
  onPhaseChange: (phase: number) => void;
  onStateChange: (patch: Partial<LearnerState>) => void;
}

export function LearnerShell({
  state,
  dark,
  message,
  menuOpen,
  consoleOpen,
  menuRef,
  consoleDialogRef,
  viewRef,
  onAction,
  onMenuOpen,
  onMenuOpenChange,
  onConsoleOpenChange,
  onDarkToggle,
  onPhaseChange,
  onStateChange,
}: LearnerShellProps) {
  return (
    <div onClick={onAction}>
      <div className={`app ${state.view === "detail" ? "learning" : ""}`}>
        <header>
          {state.view === "detail" && (
            <button
              className="menu-toggle"
              aria-label="学習メニューを開く"
              aria-expanded={menuOpen}
              aria-controls="learning-menu"
              onClick={onMenuOpen}
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
            <button className="btn sm" id="theme" onClick={onDarkToggle}>
              {"表示切替"}
            </button>
          </div>
        </header>
        <div id="view" ref={viewRef} style={{ minHeight: 0, overflow: "hidden" }}>
          {state.view === "list" ? (
            <LessonList state={state} />
          ) : (
            <LessonDetail state={state} onPhaseChange={onPhaseChange} />
          )}
        </div>
      </div>
      {state.view === "detail" && (
        <dialog
          ref={menuRef}
          id="learning-menu"
          className="outline-dialog"
          aria-labelledby="learning-menu-title"
          onClose={() => onMenuOpenChange(false)}
          onClick={(event) => {
            if (event.target === event.currentTarget) menuRef.current?.close();
          }}
        >
          <div className="outline-dialog-header">
            <h2 id="learning-menu-title">{"学習メニュー"}</h2>
            <button
              className="menu-close"
              aria-label="学習メニューを閉じる"
              onClick={() => menuRef.current?.close()}
              autoFocus
            >
              {"×"}
            </button>
          </div>
          <LessonOutline state={state} id="mobile-lesson-outline" />
        </dialog>
      )}
      <dialog
        ref={consoleDialogRef}
        className="console-dialog"
        aria-labelledby="console-title"
        onClose={() => onConsoleOpenChange(false)}
        onClick={(event) => {
          if (event.target === event.currentTarget) consoleDialogRef.current?.close();
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
            {"×"}
          </button>
        </div>
        <LabConsole state={state} idPrefix="dialog-console" />
      </dialog>
      <div id="modal">
        <LabModal state={state} onChange={onStateChange} />
      </div>
      <div className={`toast ${message ? "on" : ""}`} id="toast" role="status">
        {message}
      </div>
    </div>
  );
}
