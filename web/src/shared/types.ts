// Goの公開レスポンスのうち、ブラウザが利用する項目を表す。
export type LabRef = "vulnerable" | "fixed";
export type VolumePolicy = "retain" | "delete";
export type LabState =
  | "absent"
  | "preparing"
  | "prepared"
  | "starting"
  | "running"
  | "stopping"
  | "stopped"
  | "deleting"
  | "deleted"
  | "error";

export interface Endpoint {
  service: string;
  address: string;
  port: number;
}

export interface Container {
  name: string;
  state: string;
}

export interface LabSession {
  labSessionId: string;
  labId: string;
  sourceRevision: string;
  registryRevision: string;
  state: LabState;
  safetyChecked: boolean;
  externalExposure: boolean;
  guardrails: {
    maxRuntimeSeconds: number;
    cpuCores: number;
    memoryMiB: number;
    pids: number;
    diskMiB: number;
    egress: string;
  };
  expiresAt?: string;
  stopReason?: string;
  endpoints: Endpoint[];
  resources: { containers: Container[]; networks: string[]; volumes: string[] };
  retainedVolumes: string[];
  failure: { operation: string; message: string } | null;
  updatedAt: string;
}

// 復元時の通信失敗では、IDと未確認状態だけを保持する。
export type LabSnapshot = Pick<LabSession, "labSessionId" | "state"> & Partial<LabSession>;

export interface MarkerObservation {
  service: string;
  exists: boolean;
  content?: string;
}

export interface LogSnapshot {
  service: string;
  content: string;
  truncated: boolean;
}

export interface ActionObservation {
  service: string;
  action: "list_tmp";
  output: string;
  truncated: boolean;
}

export interface LessonSummary {
  id: string;
  version: string;
  title: string;
  audience: string;
  durationMinutes: number;
  product: string;
  cves?: string[];
}

export interface LearnerLesson {
  apiVersion: string;
  kind: string;
  metadata: { id: string; version: string; title: string };
  spec: {
    audience: string;
    prerequisites: string[];
    durationMinutes: number;
    objectives: string[];
    case: { product: string; affectedVersions: string; background: string };
    evidence: { cves: string[]; references: { type: string; title: string; url: string }[] };
    license: { name: string; url: string; attribution: string };
    labs: { ref: LabRef; labId: string; purpose: string }[];
    steps: {
      id: string;
      title: string;
      labRef?: LabRef;
      instruction: string;
      expectedResult: string;
      observation: string;
      hintCount: number;
    }[];
    analysis: { cause: string; impact: string; remediation: string };
    comparison: { vulnerable: string; fixed: string; difference: string; fixedLabRef?: string };
    prevention: string[];
    assessment: { id: string; question: string }[];
  };
}

export interface StudySession {
  id: string;
  lessonId: string;
  lessonVersion: string;
  stepIndex: number;
  revealedHints: Record<string, number>;
  answers: Record<string, string>;
  hints: Record<string, string[]>;
}

export interface LabBinding {
  lessonId: string;
  lessonVersion: string;
  labRef: LabRef;
}

export type AssignmentStatus =
  "assigned" | "in_progress" | "submitted" | "completed" | "needs_retry";

export interface Material {
  lessonId: string;
  lessonVersion: string;
  title: string;
  product: string;
  durationMinutes: number;
  cves?: string[];
}

export interface Assignment {
  id: string;
  organizationId: string;
  lessonId: string;
  lessonVersion: string;
  learnerId: string;
  cohortId?: string;
  dueAt: string;
  allowRetry: boolean;
  status: AssignmentStatus;
  createdAt: string;
}

export interface Submission {
  id: string;
  organizationId: string;
  assignmentId: string;
  learnerId: string;
  lessonId: string;
  lessonVersion: string;
  answers: Record<string, string>;
  submittedAt: string;
  evaluation: "pending" | "accepted" | "needs_retry";
  evaluatorId?: string;
  evaluatedAt?: string;
}
