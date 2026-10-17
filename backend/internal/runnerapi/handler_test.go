package runnerapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jphacks/sd_2601/internal/runtime"
)

func TestHandlerRejectsUnconfirmedStart(t *testing.T) {
	service := &fakeService{}
	handler := NewHandler(service, "token")
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/labs/session/start", strings.NewReader(`{"confirmed":false}`))
	request.Host = "127.0.0.1"
	request.Header.Set("X-Reprokit-Session", "token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.starts != 0 {
		t.Fatalf("status=%d starts=%d", response.Code, service.starts)
	}
}

func TestHandlerAllowsOnlyLessonReferenceForPrepare(t *testing.T) {
	service := &fakeService{}
	handler := NewHandler(service, "token")
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/labs", strings.NewReader(`{"lessonId":"filebrowser/hook","lessonVersion":"1.0.0","labRef":"vulnerable","compose":"unsafe"}`))
	request.Host = "127.0.0.1"
	request.Header.Set("X-Reprokit-Session", "token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.reference != (LessonReference{}) {
		t.Fatalf("status=%d reference=%#v", response.Code, service.reference)
	}
}

func TestHandlerRejectsNonLoopbackOrigin(t *testing.T) {
	service := &fakeService{}
	handler := NewHandler(service, "token")
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/labs", strings.NewReader(`{"lessonId":"filebrowser/hook","lessonVersion":"1.0.0","labRef":"vulnerable"}`))
	request.Host = "127.0.0.1"
	request.Header.Set("Origin", "http://example.test")
	request.Header.Set("X-Reprokit-Session", "token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || service.reference != (LessonReference{}) {
		t.Fatalf("status=%d reference=%#v", response.Code, service.reference)
	}
}

func TestHandlerReadsOnlyDeclaredComparisonMarker(t *testing.T) {
	service := &fakeService{}
	handler := NewHandler(service, "token")
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/labs/12345678-1234-4234-8234-123456789abc/markers/vulnerable", nil)
	request.Host = "127.0.0.1"
	request.Header.Set("X-Reprokit-Session", "token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.marker != "vulnerable" {
		t.Fatalf("status=%d marker=%q", response.Code, service.marker)
	}
	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/labs/12345678-1234-4234-8234-123456789abc/markers/other", nil)
	request.Host = "127.0.0.1"
	request.Header.Set("X-Reprokit-Session", "token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || service.marker != "vulnerable" {
		t.Fatalf("status=%d marker=%q", response.Code, service.marker)
	}
}

func TestHandlerReadsOnlyDeclaredComparisonLogs(t *testing.T) {
	service := &fakeService{}
	handler := NewHandler(service, "token")
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/labs/12345678-1234-4234-8234-123456789abc/logs/fixed", nil)
	request.Host = "127.0.0.1"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || service.logs != "" {
		t.Fatalf("status=%d logs=%q", response.Code, service.logs)
	}
	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/labs/12345678-1234-4234-8234-123456789abc/logs/fixed", nil)
	request.Host = "127.0.0.1"
	request.Header.Set("X-Reprokit-Session", "token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.logs != "fixed" {
		t.Fatalf("status=%d logs=%q", response.Code, service.logs)
	}
	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/labs/12345678-1234-4234-8234-123456789abc/logs/reprokit_proxy_fixed_1", nil)
	request.Host = "127.0.0.1"
	request.Header.Set("X-Reprokit-Session", "token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || service.logs != "fixed" {
		t.Fatalf("status=%d logs=%q", response.Code, service.logs)
	}
}

type fakeService struct {
	starts    int
	reference LessonReference
	marker    string
	logs      string
}

func (service *fakeService) Prepare(_ context.Context, reference LessonReference) (runtime.Session, error) {
	service.reference = reference
	return runtime.Session{}, nil
}
func (service *fakeService) Start(context.Context, string) (runtime.Session, error) {
	service.starts++
	return runtime.Session{}, nil
}
func (service *fakeService) Stop(context.Context, string) (runtime.Session, error) {
	return runtime.Session{}, nil
}
func (service *fakeService) Delete(context.Context, string, bool) (runtime.Session, error) {
	return runtime.Session{}, nil
}
func (service *fakeService) Status(context.Context, string) (runtime.Session, error) {
	return runtime.Session{}, nil
}
func (service *fakeService) ObserveMarker(_ context.Context, _ string, reference string) (runtime.MarkerObservation, error) {
	service.marker = reference
	return runtime.MarkerObservation{Service: reference}, nil
}
func (service *fakeService) ReadLogs(_ context.Context, _ string, reference string) (runtime.LogSnapshot, error) {
	service.logs = reference
	return runtime.LogSnapshot{Service: reference}, nil
}
