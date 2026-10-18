import type { MouseEvent, RefObject } from "react";
import type { LearnerState } from "./lesson-data.ts";
import { LessonList } from "./lesson-catalog.tsx";
import { ConsoleDialog, LearnerHeader, LearnerMenuDialog } from "./learner-chrome.tsx";
import { LabModal } from "./lab-modal.tsx";
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
        <LearnerHeader
          state={state}
          menuOpen={menuOpen}
          consoleOpen={consoleOpen}
          onMenuOpen={onMenuOpen}
          onDarkToggle={onDarkToggle}
        />
        <div id="view" ref={viewRef} style={{ minHeight: 0, overflow: "hidden" }}>
          {state.view === "list" ? (
            <LessonList state={state} />
          ) : (
            <LessonDetail state={state} onPhaseChange={onPhaseChange} />
          )}
        </div>
      </div>
      {state.view === "detail" && (
        <LearnerMenuDialog state={state} dialogRef={menuRef} onOpenChange={onMenuOpenChange} />
      )}
      <ConsoleDialog
        state={state}
        dialogRef={consoleDialogRef}
        onOpenChange={onConsoleOpenChange}
      />
      <div id="modal">
        <LabModal state={state} onChange={onStateChange} />
      </div>
      <div className={`toast ${message ? "on" : ""}`} id="toast" role="status">
        {message}
      </div>
    </div>
  );
}
