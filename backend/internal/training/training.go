// Package training は研修の割り当て、提出、完了判定を管理する。
// Labの実行結果や操作ログはこのパッケージの入力に含めない。
package training

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"sort"
	"time"

	"github.com/jphacks/sd_2601/internal/model"
)

const (
	maxAnswerBytes = 16 << 10
	maxStateBytes  = 2 << 20
)

var (
	idPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	lessonIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?/[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$`)
	ErrNotFound     = errors.New("training resource not found")
	ErrInvalid      = errors.New("invalid training operation")
)

// modelパッケージへ移動した値オブジェクトとエンティティを、既存利用者向けに再公開する。
type AssignmentStatus = model.AssignmentStatus
type EvaluationStatus = model.EvaluationStatus
type AuditAction = model.AuditAction
type Material = model.Material
type Cohort = model.Cohort
type Assignment = model.Assignment
type Submission = model.Submission
type Completion = model.Completion
type AuditEvent = model.AuditEvent

const (
	StatusAssigned               = model.StatusAssigned
	StatusInProgress             = model.StatusInProgress
	StatusSubmitted              = model.StatusSubmitted
	StatusCompleted              = model.StatusCompleted
	StatusNeedsRetry             = model.StatusNeedsRetry
	EvaluationPending            = model.EvaluationPending
	EvaluationAccepted           = model.EvaluationAccepted
	EvaluationNeedsRetry         = model.EvaluationNeedsRetry
	AuditAssignmentCreated       = model.AuditAssignmentCreated
	AuditAssignmentStatusChanged = model.AuditAssignmentStatusChanged
)

type CreateCohortInput struct {
	OrganizationID string
	ID             string
	Name           string
	LearnerIDs     []string
}

type CreateAssignmentsInput struct {
	OrganizationID string
	ActorID        string
	LessonID       string
	LessonVersion  string
	LearnerIDs     []string
	CohortID       string
	DueAt          time.Time
	AllowRetry     bool
}

type SubmitInput struct {
	OrganizationID string
	AssignmentID   string
	LearnerID      string
	LessonID       string
	LessonVersion  string
	Answers        map[string]string
}

type ReviewInput struct {
	OrganizationID string
	SubmissionID   string
	EvaluatorID    string
	Evaluation     EvaluationStatus
}

func validID(value string) bool { return idPattern.MatchString(value) }

func validLessonReference(id, version string) bool {
	return lessonIDPattern.MatchString(id) && version != ""
}

func scopedKey(organizationID, id string) string { return organizationID + ":" + id }
func legacyCourseKey(organizationID, id, version string) string {
	return scopedKey(organizationID, id) + "@" + version
}
func materialKey(id, version string) string { return id + "@" + version }

func uniqueIDs(values []string) bool {
	if len(values) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, value := range values {
		if !validID(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func validAssignmentStatus(status AssignmentStatus) bool {
	return status == StatusAssigned || status == StatusInProgress || status == StatusSubmitted || status == StatusCompleted || status == StatusNeedsRetry
}

func validAuditEvent(event AuditEvent) bool {
	if !validID(event.OrganizationID) || !validID(event.ID) || !validID(event.ActorID) || !validID(event.ResourceID) || event.OccurredAt.IsZero() {
		return false
	}
	switch event.Action {
	case AuditAssignmentCreated:
		return event.ResourceType == "assignment" && event.ResourceID == event.AssignmentID && validID(event.AssignmentID) && validLessonReference(event.LessonID, event.LessonVersion) && event.PreviousStatus == "" && event.Status == StatusAssigned
	case AuditAssignmentStatusChanged:
		return event.ResourceType == "assignment" && event.ResourceID == event.AssignmentID && validID(event.AssignmentID) && validLessonReference(event.LessonID, event.LessonVersion) && validAssignmentStatus(event.PreviousStatus) && validAssignmentStatus(event.Status) && event.PreviousStatus != event.Status
	default:
		return false
	}
}

func validSubmission(submission Submission) bool {
	if !validID(submission.OrganizationID) || !validID(submission.ID) || !validID(submission.AssignmentID) || !validID(submission.LearnerID) || submission.LessonID == "" || submission.LessonVersion == "" || submission.SubmittedAt.IsZero() || len(submission.Answers) == 0 || len(submission.Answers) > 64 {
		return false
	}
	for question, answer := range submission.Answers {
		if !validID(question) || len(answer) > maxAnswerBytes {
			return false
		}
	}
	if submission.Evaluation != EvaluationPending && submission.Evaluation != EvaluationAccepted && submission.Evaluation != EvaluationNeedsRetry {
		return false
	}
	return (submission.Evaluation == EvaluationPending && submission.EvaluatorID == "" && submission.EvaluatedAt == nil) || (submission.Evaluation != EvaluationPending && validID(submission.EvaluatorID) && submission.EvaluatedAt != nil)
}

func newID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Serviceは提出・評価による状態遷移を一箇所に閉じ込める。
type Service struct {
	store     *Store
	materials map[string]Material
	now       func() time.Time
}

func NewService(store *Store, materials []Material) (*Service, error) {
	if store == nil || len(materials) == 0 {
		return nil, errors.New("training store and materials are required")
	}
	indexed := make(map[string]Material, len(materials))
	for _, material := range materials {
		if !validLessonReference(material.LessonID, material.LessonVersion) || material.Title == "" || material.Product == "" || material.DurationMinutes < 1 {
			return nil, errors.New("invalid training material")
		}
		key := materialKey(material.LessonID, material.LessonVersion)
		if _, exists := indexed[key]; exists {
			return nil, errors.New("duplicate training material")
		}
		material.CVEs = append([]string{}, material.CVEs...)
		indexed[key] = material
	}
	return &Service{store: store, materials: indexed, now: time.Now}, nil
}

func (service *Service) CreateCohort(input CreateCohortInput) (Cohort, error) {
	if !validID(input.OrganizationID) || !validID(input.ID) || input.Name == "" || !uniqueIDs(input.LearnerIDs) {
		return Cohort{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	key := scopedKey(input.OrganizationID, input.ID)
	if _, exists := service.store.state.Cohorts[key]; exists {
		return Cohort{}, ErrInvalid
	}
	cohort := Cohort{ID: input.ID, OrganizationID: input.OrganizationID, Name: input.Name, LearnerIDs: append([]string{}, input.LearnerIDs...), CreatedAt: service.now().UTC()}
	service.store.state.Cohorts[key] = cohort
	if err := service.store.save(); err != nil {
		delete(service.store.state.Cohorts, key)
		return Cohort{}, err
	}
	return cohort, nil
}

func (service *Service) CreateAssignments(input CreateAssignmentsInput) ([]Assignment, error) {
	if !validID(input.OrganizationID) || !validID(input.ActorID) || !validLessonReference(input.LessonID, input.LessonVersion) || input.DueAt.IsZero() || (len(input.LearnerIDs) > 0 && input.CohortID != "") {
		return nil, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	if _, exists := service.materials[materialKey(input.LessonID, input.LessonVersion)]; !exists {
		return nil, ErrNotFound
	}
	learners := append([]string{}, input.LearnerIDs...)
	if input.CohortID != "" {
		cohort, exists := service.store.state.Cohorts[scopedKey(input.OrganizationID, input.CohortID)]
		if !exists {
			return nil, ErrNotFound
		}
		learners = append([]string{}, cohort.LearnerIDs...)
	}
	if !uniqueIDs(learners) {
		return nil, ErrInvalid
	}
	assignments := make([]Assignment, 0, len(learners))
	events := make([]AuditEvent, 0, len(learners))
	for _, learnerID := range learners {
		id, err := newID()
		if err != nil {
			return nil, err
		}
		assignment := Assignment{ID: id, OrganizationID: input.OrganizationID, LessonID: input.LessonID, LessonVersion: input.LessonVersion, LearnerID: learnerID, CohortID: input.CohortID, DueAt: input.DueAt.UTC(), AllowRetry: input.AllowRetry, Status: StatusAssigned, CreatedAt: service.now().UTC()}
		event, err := service.assignmentAudit(AuditAssignmentCreated, input.ActorID, assignment, "")
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
		events = append(events, event)
	}
	for _, assignment := range assignments {
		service.store.state.Assignments[scopedKey(input.OrganizationID, assignment.ID)] = assignment
	}
	for _, event := range events {
		service.store.state.AuditEvents[scopedKey(input.OrganizationID, event.ID)] = event
	}
	if err := service.store.save(); err != nil {
		for _, assignment := range assignments {
			delete(service.store.state.Assignments, scopedKey(input.OrganizationID, assignment.ID))
		}
		for _, event := range events {
			delete(service.store.state.AuditEvents, scopedKey(input.OrganizationID, event.ID))
		}
		return nil, err
	}
	return assignments, nil
}

func (service *Service) Assignment(organizationID, id string) (Assignment, error) {
	if !validID(organizationID) || !validID(id) {
		return Assignment{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	assignment, exists := service.store.state.Assignments[scopedKey(organizationID, id)]
	if !exists {
		return Assignment{}, ErrNotFound
	}
	return assignment, nil
}

// ListMaterialsは割り当て可能な教材版を識別子順で返す。
func (service *Service) ListMaterials(organizationID string) ([]Material, error) {
	if !validID(organizationID) {
		return nil, ErrInvalid
	}
	result := make([]Material, 0, len(service.materials))
	for _, material := range service.materials {
		material.CVEs = append([]string{}, material.CVEs...)
		result = append(result, material)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].LessonID == result[right].LessonID {
			return result[left].LessonVersion < result[right].LessonVersion
		}
		return result[left].LessonID < result[right].LessonID
	})
	return result, nil
}

// ListAssignmentsは組織内の割り当てを作成日時と識別子の順で返す。
func (service *Service) ListAssignments(organizationID string) ([]Assignment, error) {
	if !validID(organizationID) {
		return nil, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	result := []Assignment{}
	for _, assignment := range service.store.state.Assignments {
		if assignment.OrganizationID == organizationID {
			result = append(result, assignment)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].CreatedAt.Equal(result[right].CreatedAt) {
			return result[left].ID < result[right].ID
		}
		return result[left].CreatedAt.Before(result[right].CreatedAt)
	})
	return result, nil
}

func (service *Service) Start(organizationID, assignmentID, learnerID string) (Assignment, error) {
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	assignment, err := service.assignmentForLearner(organizationID, assignmentID, learnerID)
	if err != nil {
		return Assignment{}, err
	}
	if assignment.Status != StatusAssigned && !(assignment.Status == StatusNeedsRetry && assignment.AllowRetry) {
		return Assignment{}, ErrInvalid
	}
	previous := assignment
	assignment.Status = StatusInProgress
	event, err := service.assignmentAudit(AuditAssignmentStatusChanged, learnerID, assignment, previous.Status)
	if err != nil {
		return Assignment{}, err
	}
	service.store.state.Assignments[scopedKey(organizationID, assignmentID)] = assignment
	service.store.state.AuditEvents[scopedKey(organizationID, event.ID)] = event
	if err := service.store.save(); err != nil {
		service.store.state.Assignments[scopedKey(organizationID, assignmentID)] = previous
		delete(service.store.state.AuditEvents, scopedKey(organizationID, event.ID))
		return Assignment{}, err
	}
	return assignment, nil
}

func (service *Service) Submit(input SubmitInput) (Submission, error) {
	if !validID(input.OrganizationID) || !validID(input.AssignmentID) || !validID(input.LearnerID) || input.LessonID == "" || input.LessonVersion == "" || len(input.Answers) == 0 || len(input.Answers) > 64 {
		return Submission{}, ErrInvalid
	}
	for question, answer := range input.Answers {
		if !validID(question) || len(answer) > maxAnswerBytes {
			return Submission{}, ErrInvalid
		}
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	assignment, err := service.assignmentForLearner(input.OrganizationID, input.AssignmentID, input.LearnerID)
	if err != nil {
		return Submission{}, err
	}
	if assignment.Status != StatusInProgress && assignment.Status != StatusSubmitted {
		return Submission{}, ErrInvalid
	}
	if assignment.LessonID != input.LessonID || assignment.LessonVersion != input.LessonVersion {
		return Submission{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Submission{}, err
	}
	submission := Submission{ID: id, OrganizationID: input.OrganizationID, AssignmentID: input.AssignmentID, LearnerID: input.LearnerID, LessonID: input.LessonID, LessonVersion: input.LessonVersion, Answers: copyAnswers(input.Answers), SubmittedAt: service.now().UTC(), Evaluation: EvaluationPending}
	previous := assignment
	assignment.Status = StatusSubmitted
	var event AuditEvent
	if previous.Status != assignment.Status {
		event, err = service.assignmentAudit(AuditAssignmentStatusChanged, input.LearnerID, assignment, previous.Status)
		if err != nil {
			return Submission{}, err
		}
	}
	service.store.state.Submissions[scopedKey(input.OrganizationID, id)] = submission
	service.store.state.Assignments[scopedKey(input.OrganizationID, input.AssignmentID)] = assignment
	if event.ID != "" {
		service.store.state.AuditEvents[scopedKey(input.OrganizationID, event.ID)] = event
	}
	if err := service.store.save(); err != nil {
		delete(service.store.state.Submissions, scopedKey(input.OrganizationID, id))
		service.store.state.Assignments[scopedKey(input.OrganizationID, input.AssignmentID)] = previous
		if event.ID != "" {
			delete(service.store.state.AuditEvents, scopedKey(input.OrganizationID, event.ID))
		}
		return Submission{}, err
	}
	return submission, nil
}

func (service *Service) Review(input ReviewInput) (Submission, Assignment, *Completion, error) {
	if !validID(input.OrganizationID) || !validID(input.SubmissionID) || !validID(input.EvaluatorID) || (input.Evaluation != EvaluationAccepted && input.Evaluation != EvaluationNeedsRetry) {
		return Submission{}, Assignment{}, nil, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	key := scopedKey(input.OrganizationID, input.SubmissionID)
	submission, exists := service.store.state.Submissions[key]
	if !exists {
		return Submission{}, Assignment{}, nil, ErrNotFound
	}
	if submission.Evaluation != EvaluationPending {
		return Submission{}, Assignment{}, nil, ErrInvalid
	}
	assignment, exists := service.store.state.Assignments[scopedKey(input.OrganizationID, submission.AssignmentID)]
	if !exists || assignment.Status != StatusSubmitted {
		return Submission{}, Assignment{}, nil, ErrInvalid
	}
	if input.Evaluation == EvaluationNeedsRetry && !assignment.AllowRetry {
		return Submission{}, Assignment{}, nil, ErrInvalid
	}
	previousSubmission, previousAssignment := submission, assignment
	now := service.now().UTC()
	submission.Evaluation, submission.EvaluatorID, submission.EvaluatedAt = input.Evaluation, input.EvaluatorID, &now
	service.store.state.Submissions[key] = submission
	var completion *Completion
	if input.Evaluation == EvaluationNeedsRetry {
		assignment.Status = StatusNeedsRetry
	} else if service.lessonComplete(assignment) {
		assignment.Status = StatusCompleted
		id, err := newID()
		if err != nil {
			return Submission{}, Assignment{}, nil, err
		}
		value := Completion{ID: id, OrganizationID: input.OrganizationID, AssignmentID: assignment.ID, LearnerID: assignment.LearnerID, CompletedAt: now}
		completion = &value
		service.store.state.Completions[scopedKey(input.OrganizationID, id)] = value
	}
	var event AuditEvent
	if previousAssignment.Status != assignment.Status {
		var auditErr error
		event, auditErr = service.assignmentAudit(AuditAssignmentStatusChanged, input.EvaluatorID, assignment, previousAssignment.Status)
		if auditErr != nil {
			return Submission{}, Assignment{}, nil, auditErr
		}
	}
	service.store.state.Assignments[scopedKey(input.OrganizationID, assignment.ID)] = assignment
	if event.ID != "" {
		service.store.state.AuditEvents[scopedKey(input.OrganizationID, event.ID)] = event
	}
	if err := service.store.save(); err != nil {
		service.store.state.Submissions[key] = previousSubmission
		service.store.state.Assignments[scopedKey(input.OrganizationID, assignment.ID)] = previousAssignment
		if completion != nil {
			delete(service.store.state.Completions, scopedKey(input.OrganizationID, completion.ID))
		}
		if event.ID != "" {
			delete(service.store.state.AuditEvents, scopedKey(input.OrganizationID, event.ID))
		}
		return Submission{}, Assignment{}, nil, err
	}
	return submission, assignment, completion, nil
}

func (service *Service) assignmentForLearner(organizationID, assignmentID, learnerID string) (Assignment, error) {
	if !validID(organizationID) || !validID(assignmentID) || !validID(learnerID) {
		return Assignment{}, ErrInvalid
	}
	assignment, exists := service.store.state.Assignments[scopedKey(organizationID, assignmentID)]
	if !exists || assignment.LearnerID != learnerID {
		return Assignment{}, ErrNotFound
	}
	return assignment, nil
}

func (service *Service) lessonComplete(assignment Assignment) bool {
	for _, submission := range service.store.state.Submissions {
		if submission.AssignmentID == assignment.ID && submission.OrganizationID == assignment.OrganizationID && submission.LessonID == assignment.LessonID && submission.LessonVersion == assignment.LessonVersion && submission.Evaluation == EvaluationAccepted {
			return true
		}
	}
	return false
}

func copyAnswers(values map[string]string) map[string]string {
	copy := make(map[string]string, len(values))
	for key, value := range values {
		copy[key] = value
	}
	return copy
}

func (service *Service) ListSubmissions(organizationID, assignmentID string) ([]Submission, error) {
	if !validID(organizationID) || !validID(assignmentID) {
		return nil, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	if _, exists := service.store.state.Assignments[scopedKey(organizationID, assignmentID)]; !exists {
		return nil, ErrNotFound
	}
	result := []Submission{}
	for _, submission := range service.store.state.Submissions {
		if submission.OrganizationID == organizationID && submission.AssignmentID == assignmentID {
			submission.Answers = copyAnswers(submission.Answers)
			result = append(result, submission)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].SubmittedAt.Before(result[right].SubmittedAt) })
	return result, nil
}

// ListAuditEventsは組織内の監査イベントを時刻順に返す。回答本文やLab情報は返さない。
func (service *Service) ListAuditEvents(organizationID string) ([]AuditEvent, error) {
	if !validID(organizationID) {
		return nil, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	result := []AuditEvent{}
	for _, event := range service.store.state.AuditEvents {
		if event.OrganizationID == organizationID {
			result = append(result, event)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].OccurredAt.Equal(result[right].OccurredAt) {
			return result[left].ID < result[right].ID
		}
		return result[left].OccurredAt.Before(result[right].OccurredAt)
	})
	return result, nil
}

// WriteResultsCSVは受講結果を1レコードずつCSVへ書き出す。回答、Lab、操作履歴は出力しない。
func (service *Service) WriteResultsCSV(writer io.Writer, organizationID string) error {
	if writer == nil || !validID(organizationID) {
		return ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	completions := map[string]Completion{}
	for _, completion := range service.store.state.Completions {
		if completion.OrganizationID == organizationID {
			completions[completion.AssignmentID] = completion
		}
	}
	csvWriter := csv.NewWriter(writer)
	if err := writeCSVRecord(csvWriter, []string{"learner_id", "lesson_id", "lesson_version", "status", "completed_at", "evaluation_result"}); err != nil {
		return err
	}
	for _, assignment := range service.store.state.Assignments {
		if assignment.OrganizationID != organizationID {
			continue
		}
		completedAt := ""
		if completion, exists := completions[assignment.ID]; exists {
			completedAt = completion.CompletedAt.UTC().Format(time.RFC3339Nano)
		}
		if err := writeCSVRecord(csvWriter, []string{assignment.LearnerID, assignment.LessonID, assignment.LessonVersion, string(assignment.Status), completedAt, evaluationResult(assignment.Status)}); err != nil {
			return err
		}
	}
	return nil
}

func writeCSVRecord(writer *csv.Writer, record []string) error {
	if err := writer.Write(record); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func evaluationResult(status AssignmentStatus) string {
	switch status {
	case StatusSubmitted:
		return string(EvaluationPending)
	case StatusCompleted:
		return string(EvaluationAccepted)
	case StatusNeedsRetry:
		return string(EvaluationNeedsRetry)
	default:
		return ""
	}
}

func (service *Service) assignmentAudit(action AuditAction, actorID string, assignment Assignment, previousStatus AssignmentStatus) (AuditEvent, error) {
	id, err := newID()
	if err != nil {
		return AuditEvent{}, err
	}
	return AuditEvent{ID: id, OrganizationID: assignment.OrganizationID, Action: action, ActorID: actorID, ResourceType: "assignment", ResourceID: assignment.ID, LessonID: assignment.LessonID, LessonVersion: assignment.LessonVersion, AssignmentID: assignment.ID, PreviousStatus: previousStatus, Status: assignment.Status, OccurredAt: service.now().UTC()}, nil
}
