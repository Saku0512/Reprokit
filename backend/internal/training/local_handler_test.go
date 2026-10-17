package training

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jphacks/sd_2601/internal/identity"
	"github.com/jphacks/sd_2601/internal/lesson"
)

func TestLocalHandlerSeparatesManagementAndLearnerAuthorization(t *testing.T) {
	service := testService(t)
	lessonID := "command-injection/filebrowser-auth-hook"
	if _, err := service.CreateCourse(CreateCourseInput{OrganizationID: "local", ActorID: "admin", ID: "secure-course", Version: "1.0.0", Title: "研修", Lessons: []LessonRef{{LessonID: lessonID, LessonVersion: "1.0.0"}}, RequiredLessons: []LessonRef{{LessonID: lessonID, LessonVersion: "1.0.0"}}}); err != nil {
		t.Fatal(err)
	}
	assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: "local", ActorID: "admin", CourseID: "secure-course", CourseVersion: "1.0.0", LearnerIDs: []string{"learner"}, DueAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), AllowRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	lessonRoot := t.TempDir()
	lessonData, err := os.ReadFile(filepath.Join("..", "..", "..", "lessons", "filebrowser-auth-hook.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lessonRoot, "lesson.json"), lessonData, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, issues, err := lesson.LoadCatalog(lessonRoot, func(string) bool { return true })
	if err != nil || len(issues) != 0 {
		t.Fatalf("load catalog: %v %v", err, issues)
	}
	study, err := lesson.NewSessionStore(filepath.Join(t.TempDir(), "study"), catalog)
	if err != nil {
		t.Fatal(err)
	}
	identityStore, err := identity.NewStore(filepath.Join(t.TempDir(), "identity"))
	if err != nil {
		t.Fatal(err)
	}
	identities, err := identity.NewService(identityStore)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = identities.Bootstrap(identity.BootstrapInput{OrganizationID: "local", OrganizationName: "Local", UserID: "admin", Subject: "local-admin", Email: "admin@local.test", DisplayName: "管理者"})
	if err != nil {
		t.Fatal(err)
	}
	admin := identity.Principal{OrganizationID: "local", UserID: "admin"}
	if _, err := identities.CreateUser(admin, identity.CreateUserInput{ID: "learner", Subject: "local-learner", Email: "learner@local.test", DisplayName: "受講者", Roles: []identity.Role{identity.RoleLearner}}); err != nil {
		t.Fatal(err)
	}
	if _, err := identities.CreateUser(admin, identity.CreateUserInput{ID: "reviewer", Subject: "local-reviewer", Email: "reviewer@local.test", DisplayName: "レビュー担当", Roles: []identity.Role{identity.RoleReviewer}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := identities.Bootstrap(identity.BootstrapInput{OrganizationID: "other", OrganizationName: "Other", UserID: "admin", Subject: "other-admin", Email: "admin@other.test", DisplayName: "別組織管理者"}); err != nil {
		t.Fatal(err)
	}
	handler, err := NewLocalHandler(service, study, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTeapot) }), LocalConfig{SessionToken: "learner-session-token", ManagementToken: "management-session-token", OrganizationID: "local", AdministratorID: "admin", LearnerID: "learner", AssignmentID: assignments[0].ID, Identity: identities, OIDC: fakeOIDCAuthenticator{}})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, header, token string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, "http://127.0.0.1"+path, bytes.NewBufferString(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		if header != "" {
			request.Header.Set(header, token)
		}
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		return writer
	}
	callAs := func(method, path, body, organizationID, userID string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(method, "http://127.0.0.1"+path, bytes.NewBufferString(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("X-Reprokit-Management-Session", "management-session-token")
		request.Header.Set(managementOrganizationHeader, organizationID)
		request.Header.Set(managementUserHeader, userID)
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, request)
		return writer
	}
	if status := call(http.MethodGet, "/api/training/courses", "", "", "").Code; status != http.StatusUnauthorized {
		t.Fatalf("unprotected management status = %d", status)
	}
	if status := call(http.MethodGet, "/api/training/courses?organizationId=other", "", "X-Reprokit-Management-Session", "management-session-token").Code; status != http.StatusForbidden {
		t.Fatalf("cross-organization status = %d", status)
	}
	if status := call(http.MethodPost, "/api/training/courses", `{"organizationId":"local","actorId":"other","id":"other","version":"1.0.0","title":"不正","lessons":[{"lessonId":"command-injection/filebrowser-auth-hook","lessonVersion":"1.0.0"}],"requiredLessons":[{"lessonId":"command-injection/filebrowser-auth-hook","lessonVersion":"1.0.0"}]}`, "X-Reprokit-Management-Session", "management-session-token").Code; status != http.StatusForbidden {
		t.Fatalf("spoofed actor status = %d", status)
	}
	if status := call(http.MethodGet, "/api/training/courses", "", "X-Reprokit-Management-Session", "management-session-token").Code; status != http.StatusOK {
		t.Fatalf("management status = %d", status)
	}
	if status := callAs(http.MethodGet, "/api/training/courses", "", "local", "reviewer").Code; status != http.StatusForbidden {
		t.Fatalf("reviewer course access status = %d", status)
	}
	if status := callAs(http.MethodGet, "/api/training/assignments/"+assignments[0].ID+"/submissions", "", "local", "reviewer").Code; status != http.StatusOK {
		t.Fatalf("reviewer submission access status = %d", status)
	}
	if status := callAs(http.MethodGet, "/api/training/courses?organizationId=local", "", "other", "admin").Code; status != http.StatusForbidden {
		t.Fatalf("cross-organization principal status = %d", status)
	}
	start := decodeResponse[struct {
		AuthorizationURL string `json:"authorizationUrl"`
	}](t, call(http.MethodPost, "/api/auth/oidc/start", `{}`, "X-Reprokit-Management-Session", "management-session-token"), http.StatusOK)
	authorizationURL, err := url.Parse(start.AuthorizationURL)
	if err != nil || authorizationURL.Query().Get("state") == "" {
		t.Fatalf("unexpected authorization URL: %#v %v", start, err)
	}
	state := authorizationURL.Query().Get("state")
	callbackBody := `{"state":"` + state + `","code":"valid-code"}`
	principal := decodeResponse[struct {
		OrganizationID string `json:"organizationId"`
		UserID         string `json:"userId"`
	}](t, call(http.MethodPost, "/api/auth/oidc/callback", callbackBody, "X-Reprokit-Management-Session", "management-session-token"), http.StatusOK)
	if principal.OrganizationID != "local" || principal.UserID != "reviewer" {
		t.Fatalf("unexpected OIDC principal: %#v", principal)
	}
	if status := call(http.MethodPost, "/api/auth/oidc/callback", callbackBody, "X-Reprokit-Management-Session", "management-session-token").Code; status != http.StatusUnauthorized {
		t.Fatalf("replayed OIDC callback status = %d", status)
	}
	context := decodeResponse[struct {
		Assignment Assignment `json:"assignment"`
	}](t, call(http.MethodGet, "/api/training/learner/assignment", "", "X-Reprokit-Session", "learner-session-token"), http.StatusOK)
	if context.Assignment.LearnerID != "learner" {
		t.Fatalf("unexpected learner assignment: %#v", context)
	}
	session, err := study.Create(lessonID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := study.SaveAnswer(session.ID, "trace-boundary", "保存済み回答"); err != nil {
		t.Fatal(err)
	}
	if status := call(http.MethodPost, "/api/training/learner/start", `{}`, "X-Reprokit-Session", "learner-session-token").Code; status != http.StatusOK {
		t.Fatalf("start status = %d", status)
	}
	response := call(http.MethodPost, "/api/training/learner/submissions", `{"studySessionId":"`+session.ID+`"}`, "X-Reprokit-Session", "learner-session-token")
	if response.Code != http.StatusCreated {
		t.Fatalf("submission status = %d: %s", response.Code, response.Body.String())
	}
	submissions, err := service.ListSubmissions("local", assignments[0].ID)
	if err != nil || len(submissions) != 1 || submissions[0].Answers["trace-boundary"] != "保存済み回答" {
		t.Fatalf("unexpected submissions: %#v %v", submissions, err)
	}
}

type fakeOIDCAuthenticator struct{}

func (fakeOIDCAuthenticator) Begin(_ context.Context, organizationID string) (string, identity.OIDCFlow, error) {
	flow := identity.OIDCFlow{OrganizationID: organizationID, State: "opaque-state", Nonce: "nonce", CodeVerifier: "verifier", ExpiresAt: time.Now().UTC().Add(time.Minute)}
	return "https://idp.example.test/authorize?state=" + flow.State, flow, nil
}

func (fakeOIDCAuthenticator) Complete(_ context.Context, flow identity.OIDCFlow, state, code string) (identity.Principal, error) {
	if flow.State != state || code != "valid-code" {
		return identity.Principal{}, identity.ErrAuthentication
	}
	return identity.Principal{OrganizationID: flow.OrganizationID, UserID: "reviewer"}, nil
}
