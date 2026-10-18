// Package model は研修管理で永続化・公開するドメインエンティティを定義する。
// Labの実行結果、コンテナログ、操作履歴はこのモデルに含めない。
package model

import "time"

type AssignmentStatus string

const (
	StatusAssigned   AssignmentStatus = "assigned"
	StatusInProgress AssignmentStatus = "in_progress"
	StatusSubmitted  AssignmentStatus = "submitted"
	StatusCompleted  AssignmentStatus = "completed"
	StatusNeedsRetry AssignmentStatus = "needs_retry"
)

type EvaluationStatus string

const (
	EvaluationPending    EvaluationStatus = "pending"
	EvaluationAccepted   EvaluationStatus = "accepted"
	EvaluationNeedsRetry EvaluationStatus = "needs_retry"
)

// AuditActionは研修管理で追跡する、固定された操作種別である。
type AuditAction string

const (
	AuditAssignmentCreated       AuditAction = "assignment_created"
	AuditAssignmentStatusChanged AuditAction = "assignment_status_changed"
)

// Materialは管理画面へ公開する、版まで固定された教材である。
type Material struct {
	LessonID        string   `json:"lessonId"`
	LessonVersion   string   `json:"lessonVersion"`
	Title           string   `json:"title"`
	Product         string   `json:"product"`
	DurationMinutes int      `json:"durationMinutes"`
	CVEs            []string `json:"cves,omitempty"`
}

// Cohortは同じ教材を割り当てる受講者グループである。
type Cohort struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	LearnerIDs     []string  `json:"learnerIds"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Assignmentは一人の受講者へ固定教材版を割り当てた記録である。
type Assignment struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	LessonID       string           `json:"lessonId"`
	LessonVersion  string           `json:"lessonVersion"`
	LearnerID      string           `json:"learnerId"`
	CohortID       string           `json:"cohortId,omitempty"`
	DueAt          time.Time        `json:"dueAt"`
	AllowRetry     bool             `json:"allowRetry"`
	Status         AssignmentStatus `json:"status"`
	CreatedAt      time.Time        `json:"createdAt"`
}

// Submissionは確認課題の回答と評価だけを保持する。実行ログは保持しない。
type Submission struct {
	ID             string            `json:"id"`
	OrganizationID string            `json:"organizationId"`
	AssignmentID   string            `json:"assignmentId"`
	LearnerID      string            `json:"learnerId"`
	LessonID       string            `json:"lessonId"`
	LessonVersion  string            `json:"lessonVersion"`
	Answers        map[string]string `json:"answers"`
	SubmittedAt    time.Time         `json:"submittedAt"`
	Evaluation     EvaluationStatus  `json:"evaluation"`
	EvaluatorID    string            `json:"evaluatorId,omitempty"`
	EvaluatedAt    *time.Time        `json:"evaluatedAt,omitempty"`
}

// Completionは提出の評価結果から生成される。Labの起動結果から生成してはならない。
type Completion struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	AssignmentID   string    `json:"assignmentId"`
	LearnerID      string    `json:"learnerId"`
	CompletedAt    time.Time `json:"completedAt"`
}

// AuditEventは研修運用上の状態変化だけを記録する。回答、Lab、操作履歴は含めない。
type AuditEvent struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	Action         AuditAction      `json:"action"`
	ActorID        string           `json:"actorId"`
	ResourceType   string           `json:"resourceType"`
	ResourceID     string           `json:"resourceId"`
	LessonID       string           `json:"lessonId,omitempty"`
	LessonVersion  string           `json:"lessonVersion,omitempty"`
	AssignmentID   string           `json:"assignmentId,omitempty"`
	PreviousStatus AssignmentStatus `json:"previousStatus,omitempty"`
	Status         AssignmentStatus `json:"status,omitempty"`
	OccurredAt     time.Time        `json:"occurredAt"`
}
