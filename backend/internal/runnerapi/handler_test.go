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

type fakeService struct {
	starts    int
	reference LessonReference
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
