// Package training は研修の割り当て、提出、完了判定を管理する。
// Labの実行結果や操作ログはこのパッケージの入力に含めない。
package training

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

const (
	maxAnswerBytes = 16 << 10
	maxStateBytes  = 2 << 20
)

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	ErrNotFound = errors.New("training resource not found")
	ErrInvalid  = errors.New("invalid training operation")
)

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

// LessonRefはCourseへ取り込んだ、版まで固定された教材参照である。
type LessonRef struct {
	LessonID      string `json:"lessonId"`
	LessonVersion string `json:"lessonVersion"`
}

// Courseは組織内で公開される不変の研修コース版である。
type Course struct {
	ID              string      `json:"id"`
	OrganizationID  string      `json:"organizationId"`
	Version         string      `json:"version"`
	Title           string      `json:"title"`
	Lessons         []LessonRef `json:"lessons"`
	RequiredLessons []LessonRef `json:"requiredLessons"`
	CreatedAt       time.Time   `json:"createdAt"`
}

// Cohortは同じCourseを割り当てる受講者グループである。
type Cohort struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	LearnerIDs     []string  `json:"learnerIds"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Assignmentは一人の受講者へ固定Course版を割り当てた記録である。
type Assignment struct {
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

type CreateCourseInput struct {
	OrganizationID  string
	ID              string
	Version         string
	Title           string
	Lessons         []LessonRef
	RequiredLessons []LessonRef
}

type CreateCohortInput struct {
	OrganizationID string
	ID             string
	Name           string
	LearnerIDs     []string
}

type CreateAssignmentsInput struct {
	OrganizationID string
	CourseID       string
	CourseVersion  string
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

type persistedState struct {
	Courses     map[string]Course     `json:"courses"`
	Cohorts     map[string]Cohort     `json:"cohorts"`
	Assignments map[string]Assignment `json:"assignments"`
	Submissions map[string]Submission `json:"submissions"`
	Completions map[string]Completion `json:"completions"`
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
	return persistedState{Courses: map[string]Course{}, Cohorts: map[string]Cohort{}, Assignments: map[string]Assignment{}, Submissions: map[string]Submission{}, Completions: map[string]Completion{}}
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
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil || decoder.Decode(&struct{}{}) != io.EOF || !validState(state) {
		return errors.New("corrupt training state")
	}
	store.state = state
	return nil
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

func validID(value string) bool { return idPattern.MatchString(value) }

func validLessons(lessons []LessonRef) bool {
	if len(lessons) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, lesson := range lessons {
		if lesson.LessonID == "" || lesson.LessonVersion == "" || seen[lesson.LessonID+"@"+lesson.LessonVersion] {
			return false
		}
		seen[lesson.LessonID+"@"+lesson.LessonVersion] = true
	}
	return true
}

func validRequiredLessons(lessons, required []LessonRef) bool {
	if !validLessons(required) {
		return false
	}
	for _, lesson := range required {
		if !courseContains(Course{Lessons: lessons}, lesson.LessonID, lesson.LessonVersion) {
			return false
		}
	}
	return true
}

func validState(state persistedState) bool {
	if state.Courses == nil || state.Cohorts == nil || state.Assignments == nil || state.Submissions == nil || state.Completions == nil {
		return false
	}
	for key, course := range state.Courses {
		if key != courseKey(course.OrganizationID, course.ID, course.Version) || !validID(course.OrganizationID) || !validID(course.ID) || course.Version == "" || course.Title == "" || course.CreatedAt.IsZero() || !validLessons(course.Lessons) || !validRequiredLessons(course.Lessons, course.RequiredLessons) {
			return false
		}
	}
	for key, cohort := range state.Cohorts {
		if key != scopedKey(cohort.OrganizationID, cohort.ID) || !validID(cohort.OrganizationID) || !validID(cohort.ID) || cohort.Name == "" || cohort.CreatedAt.IsZero() || !uniqueIDs(cohort.LearnerIDs) {
			return false
		}
	}
	for key, assignment := range state.Assignments {
		if key != scopedKey(assignment.OrganizationID, assignment.ID) || !validID(assignment.OrganizationID) || !validID(assignment.ID) || !validID(assignment.CourseID) || assignment.CourseVersion == "" || !validID(assignment.LearnerID) || assignment.DueAt.IsZero() || assignment.CreatedAt.IsZero() || !validAssignmentStatus(assignment.Status) {
			return false
		}
		if _, exists := state.Courses[courseKey(assignment.OrganizationID, assignment.CourseID, assignment.CourseVersion)]; !exists {
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
		if !exists || assignment.LearnerID != submission.LearnerID || !courseContains(state.Courses[courseKey(assignment.OrganizationID, assignment.CourseID, assignment.CourseVersion)], submission.LessonID, submission.LessonVersion) {
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
	return true
}

func scopedKey(organizationID, id string) string { return organizationID + ":" + id }
func courseKey(organizationID, id, version string) string {
	return scopedKey(organizationID, id) + "@" + version
}

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

func courseContains(course Course, id, version string) bool {
	for _, lesson := range course.Lessons {
		if lesson.LessonID == id && lesson.LessonVersion == version {
			return true
		}
	}
	return false
}

func validAssignmentStatus(status AssignmentStatus) bool {
	return status == StatusAssigned || status == StatusInProgress || status == StatusSubmitted || status == StatusCompleted || status == StatusNeedsRetry
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
	store *Store
	now   func() time.Time
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("training store is required")
	}
	return &Service{store: store, now: time.Now}, nil
}

func (service *Service) CreateCourse(input CreateCourseInput) (Course, error) {
	if !validID(input.OrganizationID) || !validID(input.ID) || input.Version == "" || input.Title == "" || !validLessons(input.Lessons) || !validRequiredLessons(input.Lessons, input.RequiredLessons) {
		return Course{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	key := courseKey(input.OrganizationID, input.ID, input.Version)
	if _, exists := service.store.state.Courses[key]; exists {
		return Course{}, ErrInvalid
	}
	course := Course{ID: input.ID, OrganizationID: input.OrganizationID, Version: input.Version, Title: input.Title, Lessons: append([]LessonRef{}, input.Lessons...), RequiredLessons: append([]LessonRef{}, input.RequiredLessons...), CreatedAt: service.now().UTC()}
	service.store.state.Courses[key] = course
	if err := service.store.save(); err != nil {
		delete(service.store.state.Courses, key)
		return Course{}, err
	}
	return course, nil
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
	if !validID(input.OrganizationID) || !validID(input.CourseID) || input.CourseVersion == "" || input.DueAt.IsZero() || (len(input.LearnerIDs) > 0 && input.CohortID != "") {
		return nil, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	if _, exists := service.store.state.Courses[courseKey(input.OrganizationID, input.CourseID, input.CourseVersion)]; !exists {
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
	for _, learnerID := range learners {
		id, err := newID()
		if err != nil {
			return nil, err
		}
		assignment := Assignment{ID: id, OrganizationID: input.OrganizationID, CourseID: input.CourseID, CourseVersion: input.CourseVersion, LearnerID: learnerID, CohortID: input.CohortID, DueAt: input.DueAt.UTC(), AllowRetry: input.AllowRetry, Status: StatusAssigned, CreatedAt: service.now().UTC()}
		service.store.state.Assignments[scopedKey(input.OrganizationID, id)] = assignment
		assignments = append(assignments, assignment)
	}
	if err := service.store.save(); err != nil {
		for _, assignment := range assignments {
			delete(service.store.state.Assignments, scopedKey(input.OrganizationID, assignment.ID))
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
	service.store.state.Assignments[scopedKey(organizationID, assignmentID)] = assignment
	if err := service.store.save(); err != nil {
		service.store.state.Assignments[scopedKey(organizationID, assignmentID)] = previous
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
	course := service.store.state.Courses[courseKey(input.OrganizationID, assignment.CourseID, assignment.CourseVersion)]
	if !courseContains(course, input.LessonID, input.LessonVersion) {
		return Submission{}, ErrInvalid
	}
	id, err := newID()
	if err != nil {
		return Submission{}, err
	}
	submission := Submission{ID: id, OrganizationID: input.OrganizationID, AssignmentID: input.AssignmentID, LearnerID: input.LearnerID, LessonID: input.LessonID, LessonVersion: input.LessonVersion, Answers: copyAnswers(input.Answers), SubmittedAt: service.now().UTC(), Evaluation: EvaluationPending}
	previous := assignment
	assignment.Status = StatusSubmitted
	service.store.state.Submissions[scopedKey(input.OrganizationID, id)] = submission
	service.store.state.Assignments[scopedKey(input.OrganizationID, input.AssignmentID)] = assignment
	if err := service.store.save(); err != nil {
		delete(service.store.state.Submissions, scopedKey(input.OrganizationID, id))
		service.store.state.Assignments[scopedKey(input.OrganizationID, input.AssignmentID)] = previous
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
	} else if service.courseComplete(assignment) {
		assignment.Status = StatusCompleted
		id, err := newID()
		if err != nil {
			return Submission{}, Assignment{}, nil, err
		}
		value := Completion{ID: id, OrganizationID: input.OrganizationID, AssignmentID: assignment.ID, LearnerID: assignment.LearnerID, CompletedAt: now}
		completion = &value
		service.store.state.Completions[scopedKey(input.OrganizationID, id)] = value
	}
	service.store.state.Assignments[scopedKey(input.OrganizationID, assignment.ID)] = assignment
	if err := service.store.save(); err != nil {
		service.store.state.Submissions[key] = previousSubmission
		service.store.state.Assignments[scopedKey(input.OrganizationID, assignment.ID)] = previousAssignment
		if completion != nil {
			delete(service.store.state.Completions, scopedKey(input.OrganizationID, completion.ID))
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

func (service *Service) courseComplete(assignment Assignment) bool {
	course := service.store.state.Courses[courseKey(assignment.OrganizationID, assignment.CourseID, assignment.CourseVersion)]
	accepted := make(map[string]bool)
	for _, submission := range service.store.state.Submissions {
		if submission.AssignmentID == assignment.ID && submission.OrganizationID == assignment.OrganizationID && submission.Evaluation == EvaluationAccepted {
			accepted[submission.LessonID+"@"+submission.LessonVersion] = true
		}
	}
	for _, lesson := range course.RequiredLessons {
		if !accepted[lesson.LessonID+"@"+lesson.LessonVersion] {
			return false
		}
	}
	return true
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
