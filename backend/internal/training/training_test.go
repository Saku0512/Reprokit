package training

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "training"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC) }
	return service
}

func createCourse(t *testing.T, service *Service) Course {
	t.Helper()
	course, err := service.CreateCourse(CreateCourseInput{
		OrganizationID: "acme", ID: "secure-basics", Version: "1.0.0", Title: "安全な実装の基礎",
		Lessons: []LessonRef{{LessonID: "filebrowser/hook", LessonVersion: "1.0.0"}, {LessonID: "sample/fix", LessonVersion: "1.0.0"}}, RequiredLessons: []LessonRef{{LessonID: "filebrowser/hook", LessonVersion: "1.0.0"}, {LessonID: "sample/fix", LessonVersion: "1.0.0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return course
}

func TestCourseAssignmentSubmissionAndCompletionArePersisted(t *testing.T) {
	service := testService(t)
	createCourse(t, service)
	cohort, err := service.CreateCohort(CreateCohortInput{OrganizationID: "acme", ID: "new-hires", Name: "新入社員", LearnerIDs: []string{"alice", "bob"}})
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "acme", CourseID: "secure-basics", CourseVersion: "1.0.0", CohortID: cohort.ID, DueAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 2 || assignments[0].Status != StatusAssigned || assignments[0].CohortID != cohort.ID {
		t.Fatalf("unexpected assignments: %#v", assignments)
	}
	assignment := assignments[0]
	if _, err := service.Start("acme", assignment.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	first, err := service.Submit(SubmitInput{OrganizationID: "acme", AssignmentID: assignment.ID, LearnerID: "alice", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", Answers: map[string]string{"cause": "認証前にフックを実行している"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.SubmittedAt.IsZero() || first.Evaluation != EvaluationPending {
		t.Fatalf("unexpected submission: %#v", first)
	}
	_, afterFirst, completion, err := service.Review(ReviewInput{OrganizationID: "acme", SubmissionID: first.ID, EvaluatorID: "instructor", Evaluation: EvaluationAccepted})
	if err != nil {
		t.Fatal(err)
	}
	if completion != nil || afterFirst.Status != StatusSubmitted {
		t.Fatalf("first approved answer completed too early: %#v %#v", afterFirst, completion)
	}
	second, err := service.Submit(SubmitInput{OrganizationID: "acme", AssignmentID: assignment.ID, LearnerID: "alice", LessonID: "sample/fix", LessonVersion: "1.0.0", Answers: map[string]string{"prevention": "入力を検証する"}})
	if err != nil {
		t.Fatal(err)
	}
	_, completed, completion, err := service.Review(ReviewInput{OrganizationID: "acme", SubmissionID: second.ID, EvaluatorID: "instructor", Evaluation: EvaluationAccepted})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != StatusCompleted || completion == nil || completion.LearnerID != "alice" {
		t.Fatalf("completion was not created from all accepted answers: %#v %#v", completed, completion)
	}

	reopenedStore, err := NewStore(service.store.root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(reopenedStore)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.Assignment("acme", assignment.ID)
	if err != nil || persisted.Status != StatusCompleted {
		t.Fatalf("completion was not persisted: %#v %v", persisted, err)
	}
	submissions, err := reopened.ListSubmissions("acme", assignment.ID)
	hasCause := false
	for _, submission := range submissions {
		hasCause = hasCause || submission.Answers["cause"] != ""
	}
	if err != nil || len(submissions) != 2 || !hasCause {
		t.Fatalf("answers were not persisted: %#v %v", submissions, err)
	}
	if _, err := reopened.Assignment("other", assignment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("organization boundary was bypassed: %v", err)
	}
}

func TestRetryRequiresPermissionAndExplicitRestart(t *testing.T) {
	service := testService(t)
	createCourse(t, service)
	assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "acme", CourseID: "secure-basics", CourseVersion: "1.0.0", LearnerIDs: []string{"alice"}, DueAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	assignment := assignments[0]
	if _, err := service.Start("acme", assignment.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	submission, err := service.Submit(SubmitInput{OrganizationID: "acme", AssignmentID: assignment.ID, LearnerID: "alice", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", Answers: map[string]string{"cause": "回答"}})
	if err != nil {
		t.Fatal(err)
	}
	_, retried, completion, err := service.Review(ReviewInput{OrganizationID: "acme", SubmissionID: submission.ID, EvaluatorID: "instructor", Evaluation: EvaluationNeedsRetry})
	if err != nil {
		t.Fatal(err)
	}
	if retried.Status != StatusNeedsRetry || completion != nil {
		t.Fatalf("unexpected retry result: %#v %#v", retried, completion)
	}
	resumed, err := service.Start("acme", assignment.ID, "alice")
	if err != nil || resumed.Status != StatusInProgress {
		t.Fatalf("retry was not explicitly restarted: %#v %v", resumed, err)
	}
}
