package runnerapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/jphacks/sd_2601/internal/runtime"
)

type LessonReference struct {
	LessonID      string `json:"lessonId"`
	LessonVersion string `json:"lessonVersion"`
	LabRef        string `json:"labRef"`
}

// Service はLesson参照を固定済みLabへ解決するRunner境界である。
type Service interface {
	Prepare(context.Context, LessonReference) (runtime.Session, error)
	Start(context.Context, string) (runtime.Session, error)
	Stop(context.Context, string) (runtime.Session, error)
	Delete(context.Context, string, bool) (runtime.Session, error)
	Status(context.Context, string) (runtime.Session, error)
}

func NewHandler(service Service, sessionToken string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !validHost(request.Host) {
			http.Error(writer, "invalid Host", http.StatusForbidden)
			return
		}
		if request.Method != http.MethodGet && !validOrigin(request.Header.Get("Origin")) {
			http.Error(writer, "invalid Origin", http.StatusForbidden)
			return
		}
		if request.Method != http.MethodGet && subtle.ConstantTimeCompare([]byte(request.Header.Get("X-Reprokit-Session")), []byte(sessionToken)) != 1 {
			http.Error(writer, "invalid local session", http.StatusUnauthorized)
			return
		}
		path := strings.TrimPrefix(request.URL.Path, "/api/labs")
		if request.Method == http.MethodPost && path == "" {
			var reference LessonReference
			if !decode(request, &reference) || reference.LessonID == "" || reference.LessonVersion == "" || reference.LabRef == "" {
				http.Error(writer, "invalid lesson reference", http.StatusBadRequest)
				return
			}
			session, err := service.Prepare(request.Context(), reference)
			respond(writer, session, err)
			return
		}
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			http.NotFound(writer, request)
			return
		}
		id := parts[0]
		if request.Method == http.MethodGet && len(parts) == 1 {
			session, err := service.Status(request.Context(), id)
			respond(writer, session, err)
			return
		}
		if request.Method == http.MethodPost && len(parts) == 2 && parts[1] == "start" {
			var body struct {
				Confirmed bool `json:"confirmed"`
			}
			if !decode(request, &body) || !body.Confirmed {
				http.Error(writer, "explicit confirmation is required", http.StatusBadRequest)
				return
			}
			session, err := service.Start(request.Context(), id)
			respond(writer, session, err)
			return
		}
		if request.Method == http.MethodPost && len(parts) == 2 && parts[1] == "stop" {
			session, err := service.Stop(request.Context(), id)
			respond(writer, session, err)
			return
		}
		if request.Method == http.MethodDelete && len(parts) == 1 {
			policy := request.URL.Query().Get("volumePolicy")
			if policy != "retain" && policy != "delete" {
				http.Error(writer, "invalid volume policy", http.StatusBadRequest)
				return
			}
			session, err := service.Delete(request.Context(), id, policy == "delete")
			respond(writer, session, err)
			return
		}
		http.NotFound(writer, request)
	})
}

func validHost(host string) bool {
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}
	return name == "127.0.0.1"
}

func validOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && parsed.Scheme == "http" && parsed.Hostname() == "127.0.0.1" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func decode(request *http.Request, target any) bool {
	defer request.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}
func respond(writer http.ResponseWriter, session runtime.Session, err error) {
	writer.Header().Set("Content-Type", "application/json")
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, runtime.ErrNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, runtime.ErrInvalidState) {
			status = http.StatusConflict
		}
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"session": session, "failure": err.Error()})
		return
	}
	_ = json.NewEncoder(writer).Encode(session)
}
