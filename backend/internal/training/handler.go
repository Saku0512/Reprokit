package training

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const maxRequestBytes = 256 << 10

// NewHandlerは認証済みの管理プレーンにマウントする研修管理APIを返す。
// 認可は呼び出し元で行い、すべての操作でorganizationIdを明示する。
func NewHandler(service *Service) (http.Handler, error) {
	if service == nil {
		return nil, errors.New("training service is required")
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		serve(service, writer, request)
	}), nil
}

func serve(service *Service, writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	switch path {
	case "/api/training/materials":
		if !onlyMethod(writer, request, http.MethodGet) {
			return
		}
		organizationID, ok := organizationQuery(request)
		if !ok {
			badRequest(writer)
			return
		}
		materials, err := service.ListMaterials(organizationID)
		result(writer, materials, err, http.StatusOK)
		return
	case "/api/training/cohorts":
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		var body struct {
			OrganizationID string   `json:"organizationId"`
			ID             string   `json:"id"`
			Name           string   `json:"name"`
			LearnerIDs     []string `json:"learnerIds"`
		}
		if !decodeRequest(request, &body) {
			badRequest(writer)
			return
		}
		cohort, err := service.CreateCohort(CreateCohortInput{OrganizationID: body.OrganizationID, ID: body.ID, Name: body.Name, LearnerIDs: body.LearnerIDs})
		result(writer, cohort, err, http.StatusCreated)
		return
	case "/api/training/assignments":
		if request.Method == http.MethodGet {
			organizationID, ok := organizationQuery(request)
			if !ok {
				badRequest(writer)
				return
			}
			assignments, err := service.ListAssignments(organizationID)
			result(writer, assignments, err, http.StatusOK)
			return
		}
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		var body struct {
			OrganizationID string    `json:"organizationId"`
			ActorID        string    `json:"actorId"`
			LessonID       string    `json:"lessonId"`
			LessonVersion  string    `json:"lessonVersion"`
			LearnerIDs     []string  `json:"learnerIds"`
			CohortID       string    `json:"cohortId"`
			DueAt          time.Time `json:"dueAt"`
			AllowRetry     bool      `json:"allowRetry"`
		}
		if !decodeRequest(request, &body) {
			badRequest(writer)
			return
		}
		assignments, err := service.CreateAssignments(CreateAssignmentsInput{OrganizationID: body.OrganizationID, ActorID: body.ActorID, LessonID: body.LessonID, LessonVersion: body.LessonVersion, LearnerIDs: body.LearnerIDs, CohortID: body.CohortID, DueAt: body.DueAt, AllowRetry: body.AllowRetry})
		result(writer, assignments, err, http.StatusCreated)
		return
	case "/api/training/exports/results.csv":
		if !onlyMethod(writer, request, http.MethodGet) {
			return
		}
		values := request.URL.Query()
		organizationID := values.Get("organizationId")
		if organizationID == "" || len(values) != 1 || len(values["organizationId"]) != 1 || !validID(organizationID) {
			badRequest(writer)
			return
		}
		writer.Header().Set("Content-Type", "text/csv; charset=utf-8")
		writer.Header().Set("Content-Disposition", "attachment; filename=\"reprokit-training-results.csv\"")
		if err := service.WriteResultsCSV(flushingWriter{ResponseWriter: writer}, organizationID); err != nil {
			return
		}
		return
	}

	parts := strings.Split(strings.TrimPrefix(path, "/api/training/"), "/")
	if len(parts) == 1 && parts[0] == "audit-events" {
		if !onlyMethod(writer, request, http.MethodGet) {
			return
		}
		values := request.URL.Query()
		organizationID := values.Get("organizationId")
		if organizationID == "" || len(values) != 1 || len(values["organizationId"]) != 1 {
			badRequest(writer)
			return
		}
		events, err := service.ListAuditEvents(organizationID)
		result(writer, events, err, http.StatusOK)
		return
	}
	if len(parts) == 3 && parts[0] == "assignments" && parts[2] == "start" {
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		var body struct {
			OrganizationID string `json:"organizationId"`
			LearnerID      string `json:"learnerId"`
		}
		if !decodeRequest(request, &body) {
			badRequest(writer)
			return
		}
		assignment, err := service.Start(body.OrganizationID, parts[1], body.LearnerID)
		result(writer, assignment, err, http.StatusOK)
		return
	}
	if len(parts) == 3 && parts[0] == "assignments" && parts[2] == "submissions" {
		if request.Method == http.MethodGet {
			organizationID, ok := organizationQuery(request)
			if !ok {
				badRequest(writer)
				return
			}
			submissions, err := service.ListSubmissions(organizationID, parts[1])
			result(writer, submissions, err, http.StatusOK)
			return
		}
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		var body struct {
			OrganizationID string            `json:"organizationId"`
			LearnerID      string            `json:"learnerId"`
			LessonID       string            `json:"lessonId"`
			LessonVersion  string            `json:"lessonVersion"`
			Answers        map[string]string `json:"answers"`
		}
		if !decodeRequest(request, &body) {
			badRequest(writer)
			return
		}
		submission, err := service.Submit(SubmitInput{OrganizationID: body.OrganizationID, AssignmentID: parts[1], LearnerID: body.LearnerID, LessonID: body.LessonID, LessonVersion: body.LessonVersion, Answers: body.Answers})
		result(writer, submission, err, http.StatusCreated)
		return
	}
	if len(parts) == 3 && parts[0] == "submissions" && parts[2] == "review" {
		if !onlyMethod(writer, request, http.MethodPost) {
			return
		}
		var body struct {
			OrganizationID string           `json:"organizationId"`
			EvaluatorID    string           `json:"evaluatorId"`
			Evaluation     EvaluationStatus `json:"evaluation"`
		}
		if !decodeRequest(request, &body) {
			badRequest(writer)
			return
		}
		submission, assignment, completion, err := service.Review(ReviewInput{OrganizationID: body.OrganizationID, SubmissionID: parts[1], EvaluatorID: body.EvaluatorID, Evaluation: body.Evaluation})
		if err != nil {
			result(writer, nil, err, http.StatusOK)
			return
		}
		writeJSON(writer, http.StatusOK, struct {
			Submission Submission  `json:"submission"`
			Assignment Assignment  `json:"assignment"`
			Completion *Completion `json:"completion,omitempty"`
		}{submission, assignment, completion})
		return
	}
	if len(parts) == 2 && parts[0] == "assignments" {
		if !onlyMethod(writer, request, http.MethodGet) {
			return
		}
		values := request.URL.Query()
		organizationID := values.Get("organizationId")
		if organizationID == "" || len(values) != 1 || len(values["organizationId"]) != 1 {
			badRequest(writer)
			return
		}
		assignment, err := service.Assignment(organizationID, parts[1])
		result(writer, assignment, err, http.StatusOK)
		return
	}
	http.NotFound(writer, request)
}

func organizationQuery(request *http.Request) (string, bool) {
	values := request.URL.Query()
	organizationID := values.Get("organizationId")
	return organizationID, organizationID != "" && len(values) == 1 && len(values["organizationId"]) == 1
}

func decodeRequest(request *http.Request, target any) bool {
	defer request.Body.Close()
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxRequestBytes {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func onlyMethod(writer http.ResponseWriter, request *http.Request, method string) bool {
	if request.Method == method {
		return true
	}
	writer.Header().Set("Allow", method)
	http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func result(writer http.ResponseWriter, value any, err error, status int) {
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			http.Error(writer, "training resource not found", http.StatusNotFound)
		case errors.Is(err, ErrInvalid):
			badRequest(writer)
		default:
			http.Error(writer, "could not access training state", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(writer, status, value)
}

func badRequest(writer http.ResponseWriter) {
	http.Error(writer, "invalid training request", http.StatusBadRequest)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// flushingWriterはCSVの各レコードをHTTPレスポンスへ順に流す。
type flushingWriter struct{ http.ResponseWriter }

func (writer flushingWriter) Write(value []byte) (int, error) {
	count, err := writer.ResponseWriter.Write(value)
	if flusher, ok := writer.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
	return count, err
}
