package training

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/jphacks/sd_2601/internal/lesson"
)

// LocalConfigはWebシェルから渡す管理主体と、この端末の固定受講者を定義する。
type LocalConfig struct {
	SessionToken    string
	ManagementToken string
	OrganizationID  string
	AdministratorID string
	LearnerID       string
	AssignmentID    string
}

type localHandler struct {
	service    *Service
	study      *lesson.SessionStore
	management http.Handler
	next       http.Handler
	config     LocalConfig
}

// NewLocalHandlerは管理APIと受講APIを、用途別の認証情報でローカルRunnerへ接続する。
func NewLocalHandler(service *Service, study *lesson.SessionStore, next http.Handler, config LocalConfig) (http.Handler, error) {
	if service == nil || study == nil || next == nil || config.SessionToken == "" || config.ManagementToken == "" || !validID(config.OrganizationID) || !validID(config.AdministratorID) || !validID(config.LearnerID) || !validID(config.AssignmentID) {
		return nil, errors.New("training service, study store, local handlers and identities are required")
	}
	management, err := NewHandler(service)
	if err != nil {
		return nil, err
	}
	handler := &localHandler{service: service, study: study, management: management, next: next, config: config}
	return http.HandlerFunc(handler.serveHTTP), nil
}

func (handler *localHandler) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	if !strings.HasPrefix(request.URL.Path, "/api/training/") {
		handler.next.ServeHTTP(writer, request)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if !trainingLocalHost(request.Host) || !trainingSameOrigin(request) {
		http.Error(writer, "invalid local origin", http.StatusForbidden)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/training/learner/") {
		if !constantToken(request.Header.Get("X-Reprokit-Session"), handler.config.SessionToken) {
			http.Error(writer, "invalid local session", http.StatusUnauthorized)
			return
		}
		handler.serveLearner(writer, request)
		return
	}
	if !constantToken(request.Header.Get("X-Reprokit-Management-Session"), handler.config.ManagementToken) {
		http.Error(writer, "invalid management session", http.StatusUnauthorized)
		return
	}
	if !handler.applyPrincipal(request) {
		http.Error(writer, "management scope does not match the authenticated principal", http.StatusForbidden)
		return
	}
	handler.management.ServeHTTP(writer, request)
}

func (handler *localHandler) applyPrincipal(request *http.Request) bool {
	if request.Method == http.MethodGet {
		values := request.URL.Query()
		if organization := values.Get("organizationId"); organization != "" && organization != handler.config.OrganizationID {
			return false
		}
		values.Set("organizationId", handler.config.OrganizationID)
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
	if organization, exists := body["organizationId"]; exists && organization != handler.config.OrganizationID {
		return false
	}
	body["organizationId"] = handler.config.OrganizationID
	if request.URL.Path == "/api/training/courses" || request.URL.Path == "/api/training/assignments" {
		if actor, exists := body["actorId"]; exists && actor != handler.config.AdministratorID {
			return false
		}
		body["actorId"] = handler.config.AdministratorID
	}
	if strings.HasSuffix(request.URL.Path, "/review") {
		if evaluator, exists := body["evaluatorId"]; exists && evaluator != handler.config.AdministratorID {
			return false
		}
		body["evaluatorId"] = handler.config.AdministratorID
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
