package training

import (
	"bytes"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testMaterials = []Material{
	{LessonID: "filebrowser/hook", LessonVersion: "1.0.0", Title: "認証フック", Product: "File Browser", DurationMinutes: 60, CVEs: []string{"CVE-2026-54088"}},
	{LessonID: "sample/fix", LessonVersion: "1.0.0", Title: "修正比較", Product: "Sample", DurationMinutes: 30},
	{LessonID: "command-injection/filebrowser-auth-hook", LessonVersion: "1.1.0", Title: "認証フック", Product: "File Browser", DurationMinutes: 60, CVEs: []string{"CVE-2026-54088"}},
}

func testService(t *testing.T) *Service {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "training"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, testMaterials)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC) }
	return service
}

func TestLessonAssignmentSubmissionAndCompletionArePersisted(t *testing.T) {
	service := testService(t)
	cohort, err := service.CreateCohort(CreateCohortInput{OrganizationID: "acme", ID: "new-hires", Name: "新入社員", LearnerIDs: []string{"alice", "bob"}})
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "acme", ActorID: "trainer", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", CohortID: cohort.ID, DueAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true})
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
	_, completed, completion, err := service.Review(ReviewInput{OrganizationID: "acme", SubmissionID: first.ID, EvaluatorID: "instructor", Evaluation: EvaluationAccepted})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != StatusCompleted || completion == nil || completion.LearnerID != "alice" {
		t.Fatalf("completion was not created from the accepted answer: %#v %#v", completed, completion)
	}

	reopenedStore, err := NewStore(service.store.root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(reopenedStore, testMaterials)
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
	if err != nil || len(submissions) != 1 || !hasCause {
		t.Fatalf("answers were not persisted: %#v %v", submissions, err)
	}
	if _, err := reopened.Assignment("other", assignment.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("organization boundary was bypassed: %v", err)
	}
}

func TestRetryRequiresPermissionAndExplicitRestart(t *testing.T) {
	service := testService(t)
	assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "acme", ActorID: "trainer", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", LearnerIDs: []string{"alice"}, DueAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true})
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

func TestMaterialCatalogAndAssignmentsStayInsideOrganization(t *testing.T) {
	service := testService(t)
	if _, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "acme", ActorID: "trainer", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", LearnerIDs: []string{"alice"}, DueAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true}); err != nil {
		t.Fatal(err)
	}
	materials, err := service.ListMaterials("acme")
	if err != nil || len(materials) != 3 || materials[0].LessonID != "command-injection/filebrowser-auth-hook" {
		t.Fatalf("unexpected materials: %#v %v", materials, err)
	}
	assignments, err := service.ListAssignments("acme")
	if err != nil || len(assignments) != 1 || assignments[0].LearnerID != "alice" {
		t.Fatalf("unexpected assignments: %#v %v", assignments, err)
	}
}

func TestAuditEventsAndResultsCSVArePersistedWithoutLearnerAnswers(t *testing.T) {
	service := testService(t)
	assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "acme", ActorID: "trainer", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", LearnerIDs: []string{"alice"}, DueAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	assignment := assignments[0]
	if _, err := service.Start("acme", assignment.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	submission, err := service.Submit(SubmitInput{OrganizationID: "acme", AssignmentID: assignment.ID, LearnerID: "alice", LessonID: "filebrowser/hook", LessonVersion: "1.0.0", Answers: map[string]string{"cause": "秘密にしなければならない回答"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := service.Review(ReviewInput{OrganizationID: "acme", SubmissionID: submission.ID, EvaluatorID: "instructor", Evaluation: EvaluationAccepted}); err != nil {
		t.Fatal(err)
	}
	events, err := service.ListAuditEvents("acme")
	if err != nil {
		t.Fatal(err)
	}
	actions := map[AuditAction]bool{}
	for _, event := range events {
		actions[event.Action] = true
		if event.ActorID == "" || event.OccurredAt.IsZero() {
			t.Fatalf("incomplete audit event: %#v", event)
		}
	}
	for _, action := range []AuditAction{AuditAssignmentCreated, AuditAssignmentStatusChanged} {
		if !actions[action] {
			t.Fatalf("missing audit action %q in %#v", action, events)
		}
	}

	var output bytes.Buffer
	if err := service.WriteResultsCSV(&output, "acme"); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(&output).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][0] != "alice" || records[1][1] != "filebrowser/hook" || records[1][3] != string(StatusCompleted) || records[1][4] != "2026-10-17T09:00:00Z" || records[1][5] != string(EvaluationAccepted) {
		t.Fatalf("unexpected CSV records: %#v", records)
	}
	if bytes.Contains(output.Bytes(), []byte("秘密にしなければならない回答")) {
		t.Fatal("learner answer was exported")
	}
	stream := &failingCSVWriter{}
	if err := service.WriteResultsCSV(stream, "acme"); err == nil || stream.writes != 2 {
		t.Fatalf("CSV was not written incrementally: writes=%d err=%v", stream.writes, err)
	}

	reopenedStore, err := NewStore(service.store.root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(reopenedStore, testMaterials)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reopened.ListAuditEvents("acme")
	if err != nil || len(persisted) != len(events) {
		t.Fatalf("audit events were not persisted: %#v %v", persisted, err)
	}
}

func TestLegacySingleLessonAssignmentIsMigratedWithoutCourseData(t *testing.T) {
	root := filepath.Join(t.TempDir(), "training")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "courses":{"acme:secure-basics@1.0.0":{"id":"secure-basics","organizationId":"acme","version":"1.0.0","title":"旧研修","lessons":[{"lessonId":"filebrowser/hook","lessonVersion":"1.0.0"}],"requiredLessons":[{"lessonId":"filebrowser/hook","lessonVersion":"1.0.0"}],"createdAt":"2026-10-01T00:00:00Z"}},
  "cohorts":{},
  "assignments":{"acme:assignment":{"id":"assignment","organizationId":"acme","courseId":"secure-basics","courseVersion":"1.0.0","learnerId":"alice","dueAt":"2026-11-01T00:00:00Z","allowRetry":true,"status":"assigned","createdAt":"2026-10-01T00:00:00Z"}},
  "submissions":{},"completions":{},"auditEvents":{}
}`
	if err := os.WriteFile(filepath.Join(root, "training.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, testMaterials)
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := service.Assignment("acme", "assignment")
	if err != nil || assignment.LessonID != "filebrowser/hook" || assignment.LessonVersion != "1.0.0" {
		t.Fatalf("legacy assignment was not migrated: %#v %v", assignment, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "training.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"courses"`) || strings.Contains(string(data), `"courseId"`) {
		t.Fatalf("legacy course data remains after migration: %s", data)
	}
}

type failingCSVWriter struct{ writes int }

func (writer *failingCSVWriter) Write(value []byte) (int, error) {
	writer.writes++
	if writer.writes > 1 {
		return 0, errors.New("stop after the CSV header")
	}
	return len(value), nil
}
