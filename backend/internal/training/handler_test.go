package training

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://training.example"+path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, req)
	return writer
}

func decodeResponse[T any](t *testing.T, writer *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if writer.Code != status {
		t.Fatalf("status = %d, want %d: %s", writer.Code, status, writer.Body.String())
	}
	var result T
	if err := json.NewDecoder(writer.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHandlerCreatesAndCompletesTrainingAssignment(t *testing.T) {
	service := testService(t)
	handler, err := NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	materials := decodeResponse[[]Material](t, request(t, handler, http.MethodGet, "/api/training/materials?organizationId=acme", ""), http.StatusOK)
	if len(materials) != 3 || materials[0].LessonID != "command-injection/filebrowser-auth-hook" {
		t.Fatalf("unexpected material list: %#v", materials)
	}
	assignments := decodeResponse[[]Assignment](t, request(t, handler, http.MethodPost, "/api/training/assignments", `{"organizationId":"acme","actorId":"trainer","lessonId":"filebrowser/hook","lessonVersion":"1.0.0","learnerIds":["alice"],"cohortId":"","dueAt":"2026-11-01T00:00:00Z","allowRetry":true}`), http.StatusCreated)
	if len(assignments) != 1 {
		t.Fatalf("unexpected assignments: %#v", assignments)
	}
	assignment := assignments[0]
	listed := decodeResponse[[]Assignment](t, request(t, handler, http.MethodGet, "/api/training/assignments?organizationId=acme", ""), http.StatusOK)
	if len(listed) != 1 || listed[0].ID != assignment.ID {
		t.Fatalf("unexpected assignment list: %#v", listed)
	}
	started := decodeResponse[Assignment](t, request(t, handler, http.MethodPost, "/api/training/assignments/"+assignment.ID+"/start", `{"organizationId":"acme","learnerId":"alice"}`), http.StatusOK)
	if started.Status != StatusInProgress {
		t.Fatalf("unexpected status: %#v", started)
	}
	submission := decodeResponse[Submission](t, request(t, handler, http.MethodPost, "/api/training/assignments/"+assignment.ID+"/submissions", `{"organizationId":"acme","learnerId":"alice","lessonId":"filebrowser/hook","lessonVersion":"1.0.0","answers":{"cause":"認証処理の前に実行される"}}`), http.StatusCreated)
	submissions := decodeResponse[[]Submission](t, request(t, handler, http.MethodGet, "/api/training/assignments/"+assignment.ID+"/submissions?organizationId=acme", ""), http.StatusOK)
	if len(submissions) != 1 || submissions[0].ID != submission.ID {
		t.Fatalf("unexpected submission list: %#v", submissions)
	}
	response := decodeResponse[struct {
		Submission Submission  `json:"submission"`
		Assignment Assignment  `json:"assignment"`
		Completion *Completion `json:"completion"`
	}](t, request(t, handler, http.MethodPost, "/api/training/submissions/"+submission.ID+"/review", `{"organizationId":"acme","evaluatorId":"instructor","evaluation":"accepted"}`), http.StatusOK)
	if response.Submission.Evaluation != EvaluationAccepted || response.Assignment.Status != StatusCompleted || response.Completion == nil {
		t.Fatalf("unexpected review response: %#v", response)
	}
	loaded := decodeResponse[Assignment](t, request(t, handler, http.MethodGet, "/api/training/assignments/"+assignment.ID+"?organizationId=acme", ""), http.StatusOK)
	if loaded.Status != StatusCompleted {
		t.Fatalf("unexpected persisted assignment: %#v", loaded)
	}
	csv := request(t, handler, http.MethodGet, "/api/training/exports/results.csv?organizationId=acme", "")
	if csv.Code != http.StatusOK || csv.Header().Get("Content-Type") != "text/csv; charset=utf-8" || !strings.Contains(csv.Body.String(), "learner_id,lesson_id") || strings.Contains(csv.Body.String(), "認証処理の前に実行される") {
		t.Fatalf("unexpected CSV response: status=%d header=%q body=%q", csv.Code, csv.Header().Get("Content-Type"), csv.Body.String())
	}
	events := decodeResponse[[]AuditEvent](t, request(t, handler, http.MethodGet, "/api/training/audit-events?organizationId=acme", ""), http.StatusOK)
	if len(events) == 0 || events[0].OrganizationID != "acme" {
		t.Fatalf("unexpected audit events: %#v", events)
	}
}

func TestHandlerRejectsUnknownFieldsAndOrganizationBypass(t *testing.T) {
	service := testService(t)
	handler, err := NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	invalid := request(t, handler, http.MethodPost, "/api/training/assignments", `{"organizationId":"acme","actorId":"trainer","lessonId":"filebrowser/hook","lessonVersion":"1.0.0","learnerIds":["alice"],"dueAt":"2026-11-01T00:00:00Z","allowRetry":true,"command":"unsafe"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", invalid.Code)
	}
	notFound := request(t, handler, http.MethodGet, "/api/training/assignments/not-real?organizationId=other", "")
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("organization bypass status = %d", notFound.Code)
	}
}
