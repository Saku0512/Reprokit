package lesson

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/jphacks/sd_2601/internal/runnerapi"
	"github.com/jphacks/sd_2601/internal/runtime"
)

const maxRequestBytes = 6*maxAnswerBytes + 4096

var labSessionIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type apiHandler struct {
	catalog *Catalog
	store   *SessionStore
	runner  http.Handler
}

// NewLocalServer は#9のlistenerとトークンを共有し、学習APIを同一originへ追加する。
func NewLocalServer(catalog *Catalog, store *SessionStore, service runnerapi.Service) (*runnerapi.LocalServer, error) {
	if service == nil {
		return nil, errors.New("Runner service is required")
	}
	server, err := runnerapi.NewLocalServer(service)
	if err != nil {
		return nil, err
	}
	handler, err := NewHandler(catalog, store, server.Handler, server.SessionToken)
	if err != nil {
		server.Listener.Close()
		return nil, err
	}
	server.Handler = handler
	return server, nil
}

// NewHandler は既存Runnerハンドラーへ固定参照と定義済み操作だけを渡す。
func NewHandler(catalog *Catalog, store *SessionStore, runner http.Handler, token string) (http.Handler, error) {
	if catalog == nil || store == nil || store.catalog != catalog || runner == nil || token == "" {
		return nil, errors.New("catalog, matching session store, Runner handler and token are required")
	}
	handler := &apiHandler{catalog: catalog, store: store, runner: runner}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		if !localHost(request.Host) || !sameOrigin(request) {
			http.Error(writer, "invalid local origin", http.StatusForbidden)
			return
		}
		if subtle.ConstantTimeCompare([]byte(request.Header.Get("X-Reprokit-Session")), []byte(token)) != 1 {
			http.Error(writer, "invalid local session", http.StatusUnauthorized)
			return
		}
		if request.Method == http.MethodGet && !bodyJSON(request, &struct{}{}, true) {
			badRequest(writer)
			return
		}
		handler.serve(writer, request)
	}), nil
}

func localHost(host string) bool {
	if host == "127.0.0.1" {
		return true
	}
	name, port, err := net.SplitHostPort(host)
	return err == nil && name == "127.0.0.1" && port != ""
}

func sameOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme == "http" && parsed.Host == request.Host && parsed.Path == "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func (handler *apiHandler) serve(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	if path == "/api/labs" || strings.HasPrefix(path, "/api/labs/") {
		handler.lab(writer, request)
		return
	}
	if path == "/api/lessons" {
		if !method(writer, request, http.MethodGet) {
			return
		}
		if !query(request) {
			badRequest(writer)
			return
		}
		writeJSON(writer, http.StatusOK, handler.catalog.List())
		return
	}
	if path == "/api/lessons/detail" {
		if !method(writer, request, http.MethodGet) {
			return
		}
		if !query(request, "lessonId", "lessonVersion") {
			badRequest(writer)
			return
		}
		value, exists := handler.catalog.Get(request.URL.Query().Get("lessonId"), request.URL.Query().Get("lessonVersion"))
		if !exists {
			http.NotFound(writer, request)
			return
		}
		writeJSON(writer, http.StatusOK, learnerView(value))
		return
	}
	if !query(request) {
		badRequest(writer)
		return
	}
	if path == "/api/study-sessions" {
		if !method(writer, request, http.MethodPost) {
			return
		}
		var body struct {
			LessonID      string `json:"lessonId"`
			LessonVersion string `json:"lessonVersion"`
		}
		if !bodyJSON(request, &body, false) {
			badRequest(writer)
			return
		}
		session, err := handler.store.Create(body.LessonID, body.LessonVersion)
		handler.sessionResult(writer, session, err, http.StatusCreated)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/study-sessions/"), "/")
	if !strings.HasPrefix(path, "/api/study-sessions/") || !sessionIDPattern.MatchString(parts[0]) {
		http.NotFound(writer, request)
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if !method(writer, request, http.MethodGet) {
			return
		}
		session, err := handler.store.Get(id)
		handler.sessionResult(writer, session, err, http.StatusOK)
		return
	}
	if len(parts) == 2 && parts[1] == "advance" {
		if !method(writer, request, http.MethodPost) {
			return
		}
		if !bodyJSON(request, &struct{}{}, true) {
			badRequest(writer)
			return
		}
		session, err := handler.store.Advance(id)
		handler.sessionResult(writer, session, err, http.StatusOK)
		return
	}
	if len(parts) == 3 {
		switch parts[1] {
		case "hints":
			if !method(writer, request, http.MethodPost) {
				return
			}
			if !bodyJSON(request, &struct{}{}, true) {
				badRequest(writer)
				return
			}
			session, err := handler.store.RevealHint(id, parts[2])
			handler.sessionResult(writer, session, err, http.StatusOK)
			return
		case "answers":
			if !method(writer, request, http.MethodPut) {
				return
			}
			var body struct {
				Answer string `json:"answer"`
			}
			if !bodyJSON(request, &body, false) {
				badRequest(writer)
				return
			}
			session, err := handler.store.SaveAnswer(id, parts[2], body.Answer)
			handler.sessionResult(writer, session, err, http.StatusOK)
			return
		case "labs":
			if !method(writer, request, http.MethodGet) {
				return
			}
			reference, err := handler.store.LabBinding(id, parts[2])
			if err != nil {
				apiError(writer, err)
				return
			}
			writeJSON(writer, http.StatusOK, reference)
			return
		}
	}
	http.NotFound(writer, request)
}

func (handler *apiHandler) lab(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/api/labs" {
		if !method(writer, request, http.MethodPost) {
			return
		}
		var reference LabReference
		if !query(request) || !bodyJSON(request, &reference, false) {
			badRequest(writer)
			return
		}
		value, exists := handler.catalog.Get(reference.LessonID, reference.LessonVersion)
		if !exists {
			badRequest(writer)
			return
		}
		declared := false
		for _, lab := range value.Spec.Labs {
			if lab.Ref == reference.LabRef {
				declared = true
			}
		}
		if !declared {
			badRequest(writer)
			return
		}
		data, _ := json.Marshal(reference)
		request.Body = io.NopCloser(bytes.NewReader(data))
		request.ContentLength = int64(len(data))
	} else {
		parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/labs/"), "/")
		if !labSessionIDPattern.MatchString(parts[0]) {
			badRequest(writer)
			return
		}
		if len(parts) == 1 {
			if request.Method == http.MethodDelete {
				policy := request.URL.Query().Get("volumePolicy")
				if !query(request, "volumePolicy") || (policy != "retain" && policy != "delete") || !bodyJSON(request, &struct{}{}, true) {
					badRequest(writer)
					return
				}
			} else {
				if !method(writer, request, http.MethodGet) {
					return
				}
				if !query(request) {
					badRequest(writer)
					return
				}
			}
		} else if len(parts) == 2 && (parts[1] == "start" || parts[1] == "stop") {
			if !method(writer, request, http.MethodPost) {
				return
			}
			if !query(request) {
				badRequest(writer)
				return
			}
			if parts[1] == "start" {
				var body struct {
					Confirmed bool `json:"confirmed"`
				}
				if !bodyJSON(request, &body, false) || !body.Confirmed {
					badRequest(writer)
					return
				}
				request.Body = io.NopCloser(strings.NewReader(`{"confirmed":true}`))
				request.ContentLength = int64(len(`{"confirmed":true}`))
			} else if !bodyJSON(request, &struct{}{}, true) {
				badRequest(writer)
				return
			}
		} else if len(parts) == 3 && (parts[1] == "markers" || parts[1] == "logs") && (parts[2] == "vulnerable" || parts[2] == "fixed") {
			if !method(writer, request, http.MethodGet) || !query(request) {
				return
			}
		} else if len(parts) == 4 && parts[1] == "actions" && (parts[2] == "vulnerable" || parts[2] == "fixed") && parts[3] == string(runtime.ListTmpAction) {
			if !method(writer, request, http.MethodPost) || !query(request) || !bodyJSON(request, &struct{}{}, true) {
				badRequest(writer)
				return
			}
		} else {
			http.NotFound(writer, request)
			return
		}
	}
	handler.runner.ServeHTTP(writer, request)
}

func (handler *apiHandler) sessionResult(writer http.ResponseWriter, session StudySession, err error, status int) {
	if err != nil {
		apiError(writer, err)
		return
	}
	writeJSON(writer, status, handler.store.view(session))
}

func bodyJSON(request *http.Request, target any, emptyOK bool) bool {
	defer request.Body.Close()
	data, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	if err != nil || len(data) > maxRequestBytes {
		return false
	}
	if len(data) == 0 && emptyOK {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json" && decodeStrict(data, target) == nil
}

func query(request *http.Request, keys ...string) bool {
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil || len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if len(values[key]) != 1 || values.Get(key) == "" {
			return false
		}
	}
	return true
}

func method(writer http.ResponseWriter, request *http.Request, allowed string) bool {
	if request.Method == allowed {
		return true
	}
	writer.Header().Set("Allow", allowed)
	http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func badRequest(writer http.ResponseWriter) {
	http.Error(writer, "invalid study request", http.StatusBadRequest)
}

func apiError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(writer, "study resource not found", http.StatusNotFound)
	case errors.Is(err, ErrInvalid):
		badRequest(writer)
	default:
		http.Error(writer, "could not access local study state", http.StatusInternalServerError)
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
