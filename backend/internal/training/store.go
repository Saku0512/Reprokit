package training

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// persistedStateは研修管理のローカル永続化形式である。
type persistedState struct {
	Cohorts     map[string]Cohort     `json:"cohorts"`
	Assignments map[string]Assignment `json:"assignments"`
	Submissions map[string]Submission `json:"submissions"`
	Completions map[string]Completion `json:"completions"`
	AuditEvents map[string]AuditEvent `json:"auditEvents"`
}

// legacyStateはCourseを保存していた旧形式を教材単位へ一度だけ移行するための形式である。
type legacyLessonRef struct {
	LessonID      string `json:"lessonId"`
	LessonVersion string `json:"lessonVersion"`
}

type legacyCourse struct {
	ID              string            `json:"id"`
	OrganizationID  string            `json:"organizationId"`
	Version         string            `json:"version"`
	Title           string            `json:"title"`
	Lessons         []legacyLessonRef `json:"lessons"`
	RequiredLessons []legacyLessonRef `json:"requiredLessons"`
	CreatedAt       time.Time         `json:"createdAt"`
}

type legacyAssignment struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	CourseID       string           `json:"courseId"`
	CourseVersion  string           `json:"courseVersion"`
	LearnerID      string           `json:"learnerId"`
	CohortID       string           `json:"cohortId,omitempty"`
	DueAt          time.Time        `json:"dueAt"`
	AllowRetry     bool             `json:"allowRetry"`
	Status         AssignmentStatus `json:"status"`
	CreatedAt      time.Time        `json:"createdAt"`
}

type legacyAuditEvent struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organizationId"`
	Action         AuditAction      `json:"action"`
	ActorID        string           `json:"actorId"`
	ResourceType   string           `json:"resourceType"`
	ResourceID     string           `json:"resourceId"`
	CourseID       string           `json:"courseId,omitempty"`
	CourseVersion  string           `json:"courseVersion,omitempty"`
	AssignmentID   string           `json:"assignmentId,omitempty"`
	PreviousStatus AssignmentStatus `json:"previousStatus,omitempty"`
	Status         AssignmentStatus `json:"status,omitempty"`
	OccurredAt     time.Time        `json:"occurredAt"`
}

type legacyState struct {
	Courses     map[string]legacyCourse     `json:"courses"`
	Cohorts     map[string]Cohort           `json:"cohorts"`
	Assignments map[string]legacyAssignment `json:"assignments"`
	Submissions map[string]Submission       `json:"submissions"`
	Completions map[string]Completion       `json:"completions"`
	AuditEvents map[string]legacyAuditEvent `json:"auditEvents"`
}

// Storeは組織をまたがない研修管理データを、privateなローカルJSONへ保存する。
type Store struct {
	root  string
	mu    sync.Mutex
	state persistedState
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("training directory must be private (0700)")
	}
	store := &Store{root: root, state: emptyState()}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func emptyState() persistedState {
	return persistedState{Cohorts: map[string]Cohort{}, Assignments: map[string]Assignment{}, Submissions: map[string]Submission{}, Completions: map[string]Completion{}, AuditEvents: map[string]AuditEvent{}}
}

func (store *Store) load() error {
	data, err := os.ReadFile(filepath.Join(store.root, "training.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || len(data) > maxStateBytes {
		return errors.New("could not read training state")
	}
	var state persistedState
	if !decodeState(data, &state) {
		var legacy legacyState
		if !decodeState(data, &legacy) {
			return errors.New("corrupt training state")
		}
		migrated, err := migrateLegacyState(legacy)
		if err != nil || !validState(migrated) {
			return errors.New("legacy training state cannot be migrated")
		}
		store.state = migrated
		return store.save()
	}
	// 監査イベント導入前の状態ファイルは、空の監査イベント集合として移行する。
	if state.AuditEvents == nil {
		state.AuditEvents = map[string]AuditEvent{}
	}
	if !validState(state) {
		return errors.New("corrupt training state")
	}
	store.state = state
	return nil
}

func decodeState(data []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil && decoder.Decode(&struct{}{}) == io.EOF
}

func migrateLegacyState(legacy legacyState) (persistedState, error) {
	state := emptyState()
	if legacy.Courses == nil || legacy.Cohorts == nil || legacy.Assignments == nil || legacy.Submissions == nil || legacy.Completions == nil {
		return persistedState{}, ErrInvalid
	}
	state.Cohorts = legacy.Cohorts
	state.Submissions = legacy.Submissions
	state.Completions = legacy.Completions
	for key, value := range legacy.Assignments {
		course, exists := legacy.Courses[legacyCourseKey(value.OrganizationID, value.CourseID, value.CourseVersion)]
		if !exists || len(course.Lessons) != 1 || len(course.RequiredLessons) != 1 || course.Lessons[0] != course.RequiredLessons[0] {
			return persistedState{}, ErrInvalid
		}
		lesson := course.Lessons[0]
		state.Assignments[key] = Assignment{
			ID: value.ID, OrganizationID: value.OrganizationID,
			LessonID: lesson.LessonID, LessonVersion: lesson.LessonVersion,
			LearnerID: value.LearnerID, CohortID: value.CohortID, DueAt: value.DueAt,
			AllowRetry: value.AllowRetry, Status: value.Status, CreatedAt: value.CreatedAt,
		}
	}
	for key, value := range legacy.AuditEvents {
		if value.Action != AuditAssignmentCreated && value.Action != AuditAssignmentStatusChanged {
			continue
		}
		assignment, exists := state.Assignments[scopedKey(value.OrganizationID, value.AssignmentID)]
		if !exists {
			return persistedState{}, ErrInvalid
		}
		state.AuditEvents[key] = AuditEvent{
			ID: value.ID, OrganizationID: value.OrganizationID, Action: value.Action,
			ActorID: value.ActorID, ResourceType: value.ResourceType, ResourceID: value.ResourceID,
			LessonID: assignment.LessonID, LessonVersion: assignment.LessonVersion,
			AssignmentID: value.AssignmentID, PreviousStatus: value.PreviousStatus,
			Status: value.Status, OccurredAt: value.OccurredAt,
		}
	}
	return state, nil
}

func (store *Store) save() error {
	data, err := json.Marshal(store.state)
	if err != nil || len(data) > maxStateBytes {
		return ErrInvalid
	}
	file, err := os.CreateTemp(store.root, ".training-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(store.root, "training.json"))
}

func validState(state persistedState) bool {
	if state.Cohorts == nil || state.Assignments == nil || state.Submissions == nil || state.Completions == nil || state.AuditEvents == nil {
		return false
	}
	for key, cohort := range state.Cohorts {
		if key != scopedKey(cohort.OrganizationID, cohort.ID) || !validID(cohort.OrganizationID) || !validID(cohort.ID) || cohort.Name == "" || cohort.CreatedAt.IsZero() || !uniqueIDs(cohort.LearnerIDs) {
			return false
		}
	}
	for key, assignment := range state.Assignments {
		if key != scopedKey(assignment.OrganizationID, assignment.ID) || !validID(assignment.OrganizationID) || !validID(assignment.ID) || !validLessonReference(assignment.LessonID, assignment.LessonVersion) || !validID(assignment.LearnerID) || assignment.DueAt.IsZero() || assignment.CreatedAt.IsZero() || !validAssignmentStatus(assignment.Status) {
			return false
		}
		if assignment.CohortID != "" {
			if cohort, exists := state.Cohorts[scopedKey(assignment.OrganizationID, assignment.CohortID)]; !exists || !contains(cohort.LearnerIDs, assignment.LearnerID) {
				return false
			}
		}
	}
	for key, submission := range state.Submissions {
		if key != scopedKey(submission.OrganizationID, submission.ID) || !validSubmission(submission) {
			return false
		}
		assignment, exists := state.Assignments[scopedKey(submission.OrganizationID, submission.AssignmentID)]
		if !exists || assignment.LearnerID != submission.LearnerID || assignment.LessonID != submission.LessonID || assignment.LessonVersion != submission.LessonVersion {
			return false
		}
	}
	for key, completion := range state.Completions {
		if key != scopedKey(completion.OrganizationID, completion.ID) || !validID(completion.OrganizationID) || !validID(completion.ID) || !validID(completion.AssignmentID) || !validID(completion.LearnerID) || completion.CompletedAt.IsZero() {
			return false
		}
		assignment, exists := state.Assignments[scopedKey(completion.OrganizationID, completion.AssignmentID)]
		if !exists || assignment.LearnerID != completion.LearnerID || assignment.Status != StatusCompleted {
			return false
		}
	}
	for key, event := range state.AuditEvents {
		if key != scopedKey(event.OrganizationID, event.ID) || !validAuditEvent(event) {
			return false
		}
	}
	return true
}
