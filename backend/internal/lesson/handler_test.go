package lesson

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jphacks/sd_2601/internal/runnerapi"
	"github.com/jphacks/sd_2601/internal/runtime"
)

const runnerID = "12345678-1234-4234-8234-123456789abc"

type recordingRunner struct {
	calls         []string
	reference     runnerapi.LessonReference
	removeVolumes bool
}

func (runner *recordingRunner) Prepare(_ context.Context, ref runnerapi.LessonReference) (runtime.Session, error) {
	runner.calls = append(runner.calls, "prepare")
	runner.reference = ref
	return runtime.Session{ID: runnerID, State: runtime.Prepared, SafetyChecked: true}, nil
}
func (runner *recordingRunner) Start(_ context.Context, id string) (runtime.Session, error) {
	return runner.operation("start", id, runtime.Running)
}
func (runner *recordingRunner) Stop(_ context.Context, id string) (runtime.Session, error) {
	return runner.operation("stop", id, runtime.Stopped)
}
func (runner *recordingRunner) Delete(_ context.Context, id string, remove bool) (runtime.Session, error) {
	runner.removeVolumes = remove
	return runner.operation("delete", id, runtime.Deleted)
}
func (runner *recordingRunner) Status(_ context.Context, id string) (runtime.Session, error) {
	return runner.operation("status", id, runtime.Prepared)
}
func (runner *recordingRunner) ObserveMarker(_ context.Context, id, reference string) (runtime.MarkerObservation, error) {
	if id != runnerID || (reference != "vulnerable" && reference != "fixed") {
		return runtime.MarkerObservation{}, runtime.ErrNotFound
	}
	runner.calls = append(runner.calls, "marker:"+reference)
	return runtime.MarkerObservation{Service: reference}, nil
}
func (runner *recordingRunner) operation(operation, id string, state runtime.State) (runtime.Session, error) {
	if id != runnerID {
		return runtime.Session{}, runtime.ErrNotFound
	}
	runner.calls = append(runner.calls, operation)
	return runtime.Session{ID: id, State: state}, nil
}

func testHandler(t *testing.T) (http.Handler, *SessionStore, *recordingRunner) {
	t.Helper()
	catalog := testCatalog(t)
	store := testStore(t, catalog)
	runner := &recordingRunner{}
	handler, err := NewHandler(catalog, store, runnerapi.NewHandler(runner, "token"), "token")
	if err != nil {
		t.Fatal(err)
	}
	return handler, store, runner
}

func requestAPI(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	request.Header.Set("X-Reprokit-Session", "token")
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func requireStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d want=%d body=%s", response.Code, status, response.Body.String())
	}
}
func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestLearningHTTPFlow(t *testing.T) {
	handler, store, runner := testHandler(t)
	value := testLesson(t)
	listed := requestAPI(t, handler, "GET", "/api/lessons", "")
	requireStatus(t, listed, 200)
	if len(decodeResponse[[]Summary](t, listed)) != 1 {
		t.Fatal("missing lesson")
	}
	query := "?lessonId=" + url.QueryEscape(value.Metadata.ID) + "&lessonVersion=" + value.Metadata.Version
	detail := requestAPI(t, handler, "GET", "/api/lessons/detail"+query, "")
	requireStatus(t, detail, 200)
	learner := decodeResponse[LearnerLesson](t, detail)
	if len(learner.Spec.Steps) != len(value.Spec.Steps) || strings.Contains(detail.Body.String(), `"hints"`) || strings.Contains(detail.Body.String(), `"answerExample"`) || strings.Contains(detail.Body.String(), `"criteria"`) {
		t.Fatal("detail leaks restricted data or loses steps")
	}
	body := fmt.Sprintf(`{"lessonId":%q,"lessonVersion":%q}`, value.Metadata.ID, value.Metadata.Version)
	created := requestAPI(t, handler, "POST", "/api/study-sessions", body)
	requireStatus(t, created, 201)
	session := decodeResponse[SessionView](t, created)
	base := "/api/study-sessions/" + session.ID
	if len(session.Hints) != 0 {
		t.Fatal("hint leaked before reveal")
	}
	for index, step := range value.Spec.Steps {
		for count := range step.Hints {
			response := requestAPI(t, handler, "POST", base+"/hints/"+step.ID, "")
			requireStatus(t, response, 200)
			revealed := decodeResponse[SessionView](t, response)
			if len(revealed.Hints[step.ID]) != count+1 || revealed.Hints[step.ID][count] != step.Hints[count] {
				t.Fatal("hint reveal not incremental")
			}
		}
		response := requestAPI(t, handler, "POST", base+"/advance", `{}`)
		requireStatus(t, response, 200)
		if decodeResponse[SessionView](t, response).StepIndex != index+1 {
			t.Fatal("progress not advanced")
		}
	}
	answer := requestAPI(t, handler, "PUT", base+"/answers/"+value.Spec.Assessment[0].ID, `{"answer":"原因と修正の説明"}`)
	requireStatus(t, answer, 200)
	binding := requestAPI(t, handler, "GET", base+"/labs/"+value.Spec.Labs[0].Ref, "")
	requireStatus(t, binding, 200)
	fixed := decodeResponse[LabReference](t, binding)
	if fixed.LessonID != session.LessonID || fixed.LabRef != value.Spec.Labs[0].Ref {
		t.Fatal("binding lost fixed reference")
	}
	if len(runner.calls) != 0 {
		t.Fatal("learning operations invoked Runner")
	}
	reopened, err := NewSessionStore(store.root, store.catalog)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := NewHandler(store.catalog, reopened, runnerapi.NewHandler(runner, "token"), "token")
	if err != nil {
		t.Fatal(err)
	}
	response := requestAPI(t, resumed, "GET", base, "")
	requireStatus(t, response, 200)
	restored := decodeResponse[SessionView](t, response)
	if restored.Answers[value.Spec.Assessment[0].ID] != "原因と修正の説明" || restored.StepIndex != len(value.Spec.Steps) {
		t.Fatal("HTTP state not restored")
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private progress cacheable")
	}
}

func TestHTTPRunnerOperationsUseExistingHandler(t *testing.T) {
	handler, _, runner := testHandler(t)
	value := testLesson(t)
	reference := fmt.Sprintf(`{"lessonId":%q,"lessonVersion":%q,"labRef":%q}`, value.Metadata.ID, value.Metadata.Version, value.Spec.Labs[0].Ref)
	prepared := requestAPI(t, handler, "POST", "/api/labs", reference)
	requireStatus(t, prepared, 200)
	if len(runner.calls) != 1 || runner.calls[0] != "prepare" || runner.reference.LabRef != value.Spec.Labs[0].Ref {
		t.Fatal("prepare not fixed or started a Lab")
	}
	if decodeResponse[runtime.Session](t, prepared).ID != runnerID {
		t.Fatal("Runner ID changed")
	}
	for _, op := range []struct{ method, path, body string }{
		{"POST", "/api/labs/" + runnerID + "/start", `{"confirmed":true}`},
		{"GET", "/api/labs/" + runnerID, ""},
		{"GET", "/api/labs/" + runnerID + "/markers/vulnerable", ""},
		{"POST", "/api/labs/" + runnerID + "/stop", ""},
		{"DELETE", "/api/labs/" + runnerID + "?volumePolicy=retain", ""},
	} {
		requireStatus(t, requestAPI(t, handler, op.method, op.path, op.body), 200)
	}
	if strings.Join(runner.calls, ",") != "prepare,start,status,marker:vulnerable,stop,delete" || runner.removeVolumes {
		t.Fatalf("wrong lifecycle: %v", runner.calls)
	}
	requireStatus(t, requestAPI(t, handler, "DELETE", "/api/labs/"+runnerID+"?volumePolicy=delete", ""), 200)
	if !runner.removeVolumes {
		t.Fatal("explicit volume policy not passed")
	}
	requireStatus(t, requestAPI(t, handler, "GET", "/api/labs/00000000-0000-4000-8000-000000000000", ""), 404)
}

func TestHTTPRejectsExecutionInputsBeforeRunner(t *testing.T) {
	handler, _, runner := testHandler(t)
	value := testLesson(t)
	reference := fmt.Sprintf(`{"lessonId":%q,"lessonVersion":%q,"labRef":%q}`, value.Metadata.ID, value.Metadata.Version, value.Spec.Labs[0].Ref)
	for _, field := range []string{"compose", "dockerArgs", "hostPath", "image", "url", "command", "labId", "labSessionId"} {
		body := strings.TrimSuffix(reference, "}") + fmt.Sprintf(`,%q:"unsafe"}`, field)
		requireStatus(t, requestAPI(t, handler, "POST", "/api/labs", body), 400)
	}
	for _, op := range []struct{ method, path, body string }{
		{"POST", "/api/labs", strings.Replace(reference, value.Metadata.Version, "99.0.0", 1)},
		{"POST", "/api/labs", strings.Replace(reference, value.Spec.Labs[0].Ref, "unknown", 1)},
		{"POST", "/api/labs", strings.TrimSuffix(reference, "}") + `,"labRef":"other"}`},
		{"POST", "/api/labs?url=unsafe", reference},
		{"POST", "/api/labs/" + runnerID + "/start", `{"confirmed":false}`},
		{"POST", "/api/labs/" + runnerID + "/start", `{"confirmed":true,"command":"unsafe"}`},
		{"POST", "/api/labs/" + runnerID + "/stop", `{"command":"unsafe"}`},
		{"DELETE", "/api/labs/" + runnerID + "?volumePolicy=invalid", ""},
		{"DELETE", "/api/labs/" + runnerID + "?volumePolicy=delete&volumePolicy=retain", ""},
		{"GET", "/api/labs/not-issued-id", ""},
	} {
		requireStatus(t, requestAPI(t, handler, op.method, op.path, op.body), 400)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid input reached Runner: %v", runner.calls)
	}
}

func TestHTTPProtectsReadsAndWrites(t *testing.T) {
	handler, _, runner := testHandler(t)
	for _, tc := range []struct {
		host, origin, token string
		want                int
	}{
		{"evil.test", "", "token", 403},
		{"localhost:8080", "", "token", 403},
		{"127.0.0.1:8080", "http://evil.test", "token", 403},
		{"127.0.0.1:8080", "http://127.0.0.1:9999", "token", 403},
		{"127.0.0.1:8080", "null", "token", 403},
		{"127.0.0.1:8080", "", "", 401},
		{"127.0.0.1:8080", "", "wrong", 401},
		{"127.0.0.1:8080", "", "token", 200},
	} {
		request := httptest.NewRequest("GET", "http://127.0.0.1:8080/api/lessons", nil)
		request.Host = tc.host
		request.Header.Set("Origin", tc.origin)
		request.Header.Set("X-Reprokit-Session", tc.token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		requireStatus(t, response, tc.want)
	}
	if len(runner.calls) != 0 {
		t.Fatal("auth invoked Runner")
	}
}

func TestHTTPRejectsInvalidStudyRequests(t *testing.T) {
	handler, store, _ := testHandler(t)
	session := createSession(t, store)
	base := "/api/study-sessions/" + session.ID
	for _, op := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/lessons", "", 405},
		{"GET", "/api/lessons?unexpected=yes", "", 400},
		{"GET", "/api/lessons/detail?lessonId=x", "", 400},
		{"POST", "/api/study-sessions", `{"lessonId":"x"}`, 400},
		{"POST", "/api/study-sessions", `{"lessonId":"x","lessonVersion":"1.0.0","command":"unsafe"}`, 400},
		{"POST", base + "/advance", `{} {}`, 400},
		{"POST", base + "/hints/unknown", "", 400},
		{"PUT", base + "/answers/unknown", `{"answer":"x"}`, 400},
		{"PUT", base + "/answers/unknown", `{"answer":null}`, 400},
		{"PUT", base + "/answers/unknown", `{"answer":"` + strings.Repeat("x", maxRequestBytes) + `"}`, 400},
		{"GET", base + "/labs/unknown", "", 400},
		{"GET", "/api/study-sessions/" + strings.Repeat("a", 32), "", 404},
		{"GET", base + "/unknown", "", 404},
	} {
		requireStatus(t, requestAPI(t, handler, op.method, op.path, op.body), op.status)
	}
}

func TestNewHandlerRejectsIncompleteDependencies(t *testing.T) {
	catalog := testCatalog(t)
	store := testStore(t, catalog)
	if _, err := NewHandler(catalog, store, http.NotFoundHandler(), ""); err == nil {
		t.Fatal("empty token accepted")
	}
	if _, err := NewHandler(testCatalog(t), store, http.NotFoundHandler(), "token"); err == nil {
		t.Fatal("mismatching catalog accepted")
	}
}

func TestLocalServerSharesRunnerListenerAndToken(t *testing.T) {
	catalog := testCatalog(t)
	store := testStore(t, catalog)
	server, err := NewLocalServer(catalog, store, &recordingRunner{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Listener.Close()
	if !strings.HasPrefix(server.Listener.Addr().String(), "127.0.0.1:") || server.SessionToken == "" {
		t.Fatal("not local or missing token")
	}
	httpServer := &http.Server{Handler: server.Handler, ReadHeaderTimeout: time.Second}
	go func() { _ = httpServer.Serve(server.Listener) }()
	defer httpServer.Close()
	request, err := http.NewRequest(http.MethodGet, "http://"+server.Listener.Addr().String()+"/api/lessons", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Reprokit-Session", server.SessionToken)
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", response.StatusCode)
	}
	var summaries []Summary
	if err := json.NewDecoder(response.Body).Decode(&summaries); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 1 || summaries[0].ID != catalog.List()[0].ID {
		t.Fatal("local HTTP server did not return configured lesson")
	}
}
