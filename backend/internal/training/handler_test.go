package training

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	course := decodeResponse[Course](t, request(t, handler, http.MethodPost, "/api/training/courses", `{"organizationId":"acme","id":"secure-basics","version":"1.0.0","title":"安全な実装の基礎","lessons":[{"lessonId":"filebrowser/hook","lessonVersion":"1.0.0"}],"requiredLessons":[{"lessonId":"filebrowser/hook","lessonVersion":"1.0.0"}]}`), http.StatusCreated)
	if course.OrganizationID != "acme" || len(course.Lessons) != 1 {
		t.Fatalf("unexpected course: %#v", course)
	}
	assignments := decodeResponse[[]Assignment](t, request(t, handler, http.MethodPost, "/api/training/assignments", `{"organizationId":"acme","courseId":"secure-basics","courseVersion":"1.0.0","learnerIds":["alice"],"cohortId":"","dueAt":"2026-11-01T00:00:00Z","allowRetry":true}`), http.StatusCreated)
	if len(assignments) != 1 {
		t.Fatalf("unexpected assignments: %#v", assignments)
	}
	assignment := assignments[0]
	started := decodeResponse[Assignment](t, request(t, handler, http.MethodPost, "/api/training/assignments/"+assignment.ID+"/start", `{"organizationId":"acme","learnerId":"alice"}`), http.StatusOK)
	if started.Status != StatusInProgress {
		t.Fatalf("unexpected status: %#v", started)
	}
	submission := decodeResponse[Submission](t, request(t, handler, http.MethodPost, "/api/training/assignments/"+assignment.ID+"/submissions", `{"organizationId":"acme","learnerId":"alice","lessonId":"filebrowser/hook","lessonVersion":"1.0.0","answers":{"cause":"認証処理の前に実行される"}}`), http.StatusCreated)
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
}

func TestHandlerRejectsUnknownFieldsAndOrganizationBypass(t *testing.T) {
	service := testService(t)
	handler, err := NewHandler(service)
	if err != nil {
		t.Fatal(err)
	}
	invalid := request(t, handler, http.MethodPost, "/api/training/courses", `{"organizationId":"acme","id":"secure-basics","version":"1.0.0","title":"研修","lessons":[],"requiredLessons":[],"command":"unsafe"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", invalid.Code)
	}
	notFound := request(t, handler, http.MethodGet, "/api/training/assignments/not-real?organizationId=other", "")
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("organization bypass status = %d", notFound.Code)
	}
}
