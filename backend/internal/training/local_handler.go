package training

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jphacks/sd_2601/internal/identity"
	"github.com/jphacks/sd_2601/internal/lesson"
)

const (
	managementOrganizationHeader = "X-Reprokit-Organization-ID"
	managementUserHeader         = "X-Reprokit-User-ID"
)

// OIDCAuthenticatorはWebシェルが中継するOIDCフローの境界である。
type OIDCAuthenticator interface {
	Begin(context.Context, string) (string, identity.OIDCFlow, error)
	Complete(context.Context, identity.OIDCFlow, string, string) (identity.Principal, error)
}

// LocalConfigはWebシェルから渡す管理主体と、この端末の固定受講者を定義する。
type LocalConfig struct {
	SessionToken    string
	ManagementToken string
	OrganizationID  string
	AdministratorID string
	LearnerID       string
	AssignmentID    string
	Identity        *identity.Service
	OIDC            OIDCAuthenticator
}

type localHandler struct {
	service    *Service
	study      *lesson.SessionStore
	management http.Handler
	next       http.Handler
	config     LocalConfig
	flowsMu    sync.Mutex
	flows      map[string]identity.OIDCFlow
}

// NewLocalHandlerは管理APIと受講APIを、用途別の認証情報でローカルRunnerへ接続する。
func NewLocalHandler(service *Service, study *lesson.SessionStore, next http.Handler, config LocalConfig) (http.Handler, error) {
	if service == nil || study == nil || next == nil || config.Identity == nil || config.SessionToken == "" || config.ManagementToken == "" || !validID(config.OrganizationID) || !validID(config.AdministratorID) || !validID(config.LearnerID) || !validID(config.AssignmentID) {
		return nil, errors.New("training service, study store, local handlers and identities are required")
	}
	management, err := NewHandler(service)
	if err != nil {
		return nil, err
	}
	handler := &localHandler{service: service, study: study, management: management, next: next, config: config, flows: map[string]identity.OIDCFlow{}}
	return http.HandlerFunc(handler.serveHTTP), nil
}

func (handler *localHandler) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	isTraining := strings.HasPrefix(request.URL.Path, "/api/training/")
	isOIDC := request.URL.Path == "/api/auth/oidc/start" || request.URL.Path == "/api/auth/oidc/callback"
	if !isTraining && !isOIDC {
		handler.next.ServeHTTP(writer, request)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if !trainingLocalHost(request.Host) || !trainingSameOrigin(request) {
		http.Error(writer, "invalid local origin", http.StatusForbidden)
		return
	}
	if isOIDC {
		if !constantToken(request.Header.Get("X-Reprokit-Management-Session"), handler.config.ManagementToken) {
			http.Error(writer, "invalid management session", http.StatusUnauthorized)
			return
		}
		handler.serveOIDC(writer, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/training/learner/") {
		if !constantToken(request.Header.Get("X-Reprokit-Session"), handler.config.SessionToken) {
			http.Error(writer, "invalid local session", http.StatusUnauthorized)
			return
		}
		if err := handler.config.Identity.Authorize(identity.Principal{OrganizationID: handler.config.OrganizationID, UserID: handler.config.LearnerID}, identity.PermissionAccessLearning); err != nil {
			http.Error(writer, "learner access forbidden", http.StatusForbidden)
			return
		}
		handler.serveLearner(writer, request)
		return
	}
	if !constantToken(request.Header.Get("X-Reprokit-Management-Session"), handler.config.ManagementToken) {
		http.Error(writer, "invalid management session", http.StatusUnauthorized)
		return
	}
	principal, ok := handler.managementPrincipal(request)
	if !ok {
		http.Error(writer, "invalid management principal", http.StatusForbidden)
		return
	}
	permission, ok := managementPermission(request)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	if err := handler.config.Identity.Authorize(principal, permission); err != nil {
		http.Error(writer, "management permission denied", http.StatusForbidden)
		return
	}
	if !handler.applyPrincipal(request, principal) {
		http.Error(writer, "management scope does not match the authenticated principal", http.StatusForbidden)
		return
	}
	handler.management.ServeHTTP(writer, request)
}

func (handler *localHandler) serveOIDC(writer http.ResponseWriter, request *http.Request) {
	if handler.config.OIDC == nil {
		http.Error(writer, "OIDC login is unavailable", http.StatusServiceUnavailable)
		return
	}
	switch request.URL.Path {
	case "/api/auth/oidc/start":
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		if request.URL.RawQuery != "" || !decodeRequest(request, &struct{}{}) {
			badRequest(writer)
			return
		}
		authorizationURL, flow, err := handler.config.OIDC.Begin(request.Context(), handler.config.OrganizationID)
		if err != nil || !handler.storeOIDCFlow(flow) {
			http.Error(writer, "could not start OIDC login", http.StatusServiceUnavailable)
			return
		}
		writeJSON(writer, http.StatusOK, struct {
			AuthorizationURL string `json:"authorizationUrl"`
		}{AuthorizationURL: authorizationURL})
	case "/api/auth/oidc/callback":
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		if request.URL.RawQuery != "" {
			badRequest(writer)
			return
		}
		var body struct {
			State string `json:"state"`
			Code  string `json:"code"`
		}
		if !decodeRequest(request, &body) || body.State == "" || body.Code == "" {
			badRequest(writer)
			return
		}
		flow, exists := handler.takeOIDCFlow(body.State)
		if !exists {
			_ = handler.config.Identity.RecordAuthentication(handler.config.OrganizationID, "", false)
			http.Error(writer, "OIDC authentication failed", http.StatusUnauthorized)
			return
		}
		principal, err := handler.config.OIDC.Complete(request.Context(), flow, body.State, body.Code)
		if err != nil {
			http.Error(writer, "OIDC authentication failed", http.StatusUnauthorized)
			return
		}
		writeJSON(writer, http.StatusOK, struct {
			OrganizationID string `json:"organizationId"`
			UserID         string `json:"userId"`
		}{OrganizationID: principal.OrganizationID, UserID: principal.UserID})
	default:
		http.NotFound(writer, request)
	}
}

func (handler *localHandler) storeOIDCFlow(flow identity.OIDCFlow) bool {
	if flow.OrganizationID != handler.config.OrganizationID || flow.State == "" || flow.ExpiresAt.IsZero() {
		return false
	}
	handler.flowsMu.Lock()
	defer handler.flowsMu.Unlock()
	now := time.Now().UTC()
	for state, existing := range handler.flows {
		if !existing.ExpiresAt.After(now) {
			delete(handler.flows, state)
		}
	}
	if len(handler.flows) >= 64 {
		return false
	}
	handler.flows[flow.State] = flow
	return true
}

func (handler *localHandler) takeOIDCFlow(state string) (identity.OIDCFlow, bool) {
	handler.flowsMu.Lock()
	defer handler.flowsMu.Unlock()
	flow, exists := handler.flows[state]
	delete(handler.flows, state)
	return flow, exists
}

func (handler *localHandler) managementPrincipal(request *http.Request) (identity.Principal, bool) {
	organizationID := request.Header.Get(managementOrganizationHeader)
	userID := request.Header.Get(managementUserHeader)
	if organizationID == "" && userID == "" {
		organizationID = handler.config.OrganizationID
		userID = handler.config.AdministratorID
	}
	if !validID(organizationID) || !validID(userID) {
		return identity.Principal{}, false
	}
	return identity.Principal{OrganizationID: organizationID, UserID: userID}, true
}

func managementPermission(request *http.Request) (identity.Permission, bool) {
	path := request.URL.Path
	switch {
	case path == "/api/training/courses" && (request.Method == http.MethodGet || request.Method == http.MethodPost):
		return identity.PermissionManageCourses, true
	case path == "/api/training/cohorts" && request.Method == http.MethodPost:
		return identity.PermissionManageAssignments, true
	case path == "/api/training/assignments" && (request.Method == http.MethodGet || request.Method == http.MethodPost):
		return identity.PermissionManageAssignments, true
	case path == "/api/training/audit-events" && request.Method == http.MethodGet:
		return identity.PermissionReadAudit, true
	case path == "/api/training/exports/results.csv" && request.Method == http.MethodGet:
		return identity.PermissionReadAudit, true
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/training/"), "/")
	if len(parts) == 2 && parts[0] == "assignments" && request.Method == http.MethodGet {
		return identity.PermissionManageAssignments, true
	}
	if len(parts) == 3 && parts[0] == "assignments" && parts[2] == "submissions" && request.Method == http.MethodGet {
		return identity.PermissionReviewSubmissions, true
	}
	if len(parts) == 3 && parts[0] == "submissions" && parts[2] == "review" && request.Method == http.MethodPost {
		return identity.PermissionReviewSubmissions, true
	}
	return "", false
}

func (handler *localHandler) applyPrincipal(request *http.Request, principal identity.Principal) bool {
	if request.Method == http.MethodGet {
		values := request.URL.Query()
		if organization := values.Get("organizationId"); organization != "" && organization != principal.OrganizationID {
			return false
		}
		values.Set("organizationId", principal.OrganizationID)
		request.URL.RawQuery = values.Encode()
		return true
	}
	if request.Header.Get("Content-Type") != "application/json" {
		return true
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxRequestBytes {
		return false
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil || body == nil {
		return false
	}
	if organization, exists := body["organizationId"]; exists && organization != principal.OrganizationID {
		return false
	}
	body["organizationId"] = principal.OrganizationID
	if request.URL.Path == "/api/training/courses" || request.URL.Path == "/api/training/assignments" {
		if actor, exists := body["actorId"]; exists && actor != principal.UserID {
			return false
		}
		body["actorId"] = principal.UserID
	}
	if strings.HasSuffix(request.URL.Path, "/review") {
		if evaluator, exists := body["evaluatorId"]; exists && evaluator != principal.UserID {
			return false
		}
		body["evaluatorId"] = principal.UserID
	}
	data, err = json.Marshal(body)
	if err != nil {
		return false
	}
	request.Body = io.NopCloser(bytes.NewReader(data))
	request.ContentLength = int64(len(data))
	return true
}

func (handler *localHandler) serveLearner(writer http.ResponseWriter, request *http.Request) {
	assignment, err := handler.service.Assignment(handler.config.OrganizationID, handler.config.AssignmentID)
	if err != nil {
		result(writer, nil, err, http.StatusOK)
		return
	}
	if assignment.LearnerID != handler.config.LearnerID {
		http.Error(writer, "training resource not found", http.StatusNotFound)
		return
	}
	switch request.URL.Path {
	case "/api/training/learner/assignment":
		if !onlyMethod(writer, request, http.MethodGet) {
			return
		}
		if request.URL.RawQuery != "" {
			badRequest(writer)
			return
		}
		courses, courseErr := handler.service.ListCourses(handler.config.OrganizationID)
		if courseErr != nil {
			result(writer, nil, courseErr, http.StatusOK)
			return
		}
		for _, course := range courses {
			if course.ID == assignment.CourseID && course.Version == assignment.CourseVersion {
				writeJSON(writer, http.StatusOK, struct {
					Assignment Assignment `json:"assignment"`
					Course     Course     `json:"course"`
				}{assignment, course})
				return
			}
		}
		http.Error(writer, "training resource not found", http.StatusNotFound)
	case "/api/training/learner/start":
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		if request.URL.RawQuery != "" || !decodeRequest(request, &struct{}{}) {
			badRequest(writer)
			return
		}
		if assignment.Status != StatusAssigned && assignment.Status != StatusNeedsRetry {
			writeJSON(writer, http.StatusOK, assignment)
			return
		}
		started, startErr := handler.service.Start(handler.config.OrganizationID, assignment.ID, handler.config.LearnerID)
		result(writer, started, startErr, http.StatusOK)
	case "/api/training/learner/submissions":
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		if request.URL.RawQuery != "" {
			badRequest(writer)
			return
		}
		var body struct {
			StudySessionID string `json:"studySessionId"`
		}
		if !decodeRequest(request, &body) {
			badRequest(writer)
			return
		}
		session, sessionErr := handler.study.Get(body.StudySessionID)
		if sessionErr != nil || len(session.Answers) == 0 {
			badRequest(writer)
			return
		}
		submission, submitErr := handler.service.Submit(SubmitInput{OrganizationID: handler.config.OrganizationID, AssignmentID: assignment.ID, LearnerID: handler.config.LearnerID, LessonID: session.LessonID, LessonVersion: session.LessonVersion, Answers: session.Answers})
		result(writer, submission, submitErr, http.StatusCreated)
	default:
		http.NotFound(writer, request)
	}
}

func constantToken(actual, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func trainingLocalHost(host string) bool {
	if host == "127.0.0.1" {
		return true
	}
	name, port, err := net.SplitHostPort(host)
	return err == nil && name == "127.0.0.1" && port != ""
}

func trainingSameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme == "http" && parsed.Host == request.Host && parsed.Path == "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
