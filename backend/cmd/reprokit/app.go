package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jphacks/sd_2601/internal/lab"
	"github.com/jphacks/sd_2601/internal/lesson"
	"github.com/jphacks/sd_2601/internal/runnerapi"
	"github.com/jphacks/sd_2601/internal/runtime"
	"github.com/jphacks/sd_2601/internal/training"
)

const (
	lessonID      = "command-injection/filebrowser-auth-hook"
	lessonVersion = "1.0.0"
	labID         = "vulhub/filebrowser-cve-2026-54088"
	localCourseID = "secure-development-foundations"
)

type configuration struct {
	repository, state                          string
	organizationID, administratorID, learnerID string
}
type materializeSource func(context.Context, lab.ResolvedLab, string) (lab.MaterializedSource, error)
type application struct {
	server          *runnerapi.LocalServer
	snapshot        string
	managementToken string
	organizationID  string
	administratorID string
}

func newApplication(ctx context.Context, config configuration, provider runtime.Provider, materialize materializeSource) (*application, error) {
	if !filepath.IsAbs(config.repository) || !filepath.IsAbs(config.state) || provider == nil || materialize == nil {
		return nil, errors.New("absolute application paths and Runner dependencies are required")
	}
	if config.organizationID == "" {
		config.organizationID = "local"
	}
	if config.administratorID == "" {
		config.administratorID = "admin"
	}
	if config.learnerID == "" {
		config.learnerID = "local-learner"
	}
	for _, directory := range []string{config.state, filepath.Join(config.state, "labs"), filepath.Join(config.state, "study"), filepath.Join(config.state, "training")} {
		if err := privateDirectory(directory); err != nil {
			return nil, err
		}
	}
	root, err := os.MkdirTemp(config.state, ".catalog-*")
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(root)
		}
	}()
	readContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	revision, err := snapshot(readContext, config.repository, root)
	if err != nil {
		return nil, err
	}
	resolved, err := (lab.Resolver{}).Resolve(filepath.Join(root, "registry"), revision, labID)
	if err != nil {
		return nil, err
	}
	catalog, issues, err := lesson.LoadCatalog(filepath.Join(root, "lessons"), func(id string) bool { return id == resolved.ID })
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		return nil, fmt.Errorf("committed Lesson was rejected: %v", issues[0])
	}
	value, exists := catalog.Get(lessonID, lessonVersion)
	if !exists {
		return nil, errors.New("the reviewed File Browser Lesson Version is missing")
	}
	for _, ref := range []string{"vulnerable", "fixed"} {
		found := false
		for _, reference := range value.Spec.Labs {
			if reference.Ref == ref && reference.LabID == labID {
				found = true
			}
		}
		if !found {
			return nil, errors.New("File Browser comparison references do not match the Registry")
		}
	}
	study, err := lesson.NewSessionStore(filepath.Join(config.state, "study"), catalog)
	if err != nil {
		return nil, err
	}
	labs, err := runtime.NewFileStore(filepath.Join(config.state, "labs"))
	if err != nil {
		return nil, err
	}
	manager, err := runtime.NewManager(labs, provider)
	if err != nil {
		return nil, err
	}
	if err := manager.Recover(ctx); err != nil {
		return nil, fmt.Errorf("recover Lab resources from a previous Runner: %w", err)
	}
	service := &lazyService{manager: manager, resolved: resolved, materialize: materialize, state: config.state}
	server, err := lesson.NewLocalServer(catalog, study, service)
	if err != nil {
		return nil, err
	}
	trainingStore, err := training.NewStore(filepath.Join(config.state, "training"))
	if err != nil {
		server.Listener.Close()
		return nil, err
	}
	trainingService, err := training.NewService(trainingStore)
	if err != nil {
		server.Listener.Close()
		return nil, err
	}
	assignment, err := ensureLocalAssignment(trainingService, config)
	if err != nil {
		server.Listener.Close()
		return nil, err
	}
	managementToken, err := localSecret()
	if err != nil {
		server.Listener.Close()
		return nil, err
	}
	server.Handler, err = training.NewLocalHandler(trainingService, study, server.Handler, training.LocalConfig{SessionToken: server.SessionToken, ManagementToken: managementToken, OrganizationID: config.organizationID, AdministratorID: config.administratorID, LearnerID: config.learnerID, AssignmentID: assignment.ID})
	if err != nil {
		server.Listener.Close()
		return nil, err
	}
	cleanup = false
	return &application{server: server, snapshot: root, managementToken: managementToken, organizationID: config.organizationID, administratorID: config.administratorID}, nil
}

func ensureLocalAssignment(service *training.Service, config configuration) (training.Assignment, error) {
	courses, err := service.ListCourses(config.organizationID)
	if err != nil {
		return training.Assignment{}, err
	}
	foundCourse := false
	for _, course := range courses {
		foundCourse = foundCourse || (course.ID == localCourseID && course.Version == lessonVersion)
	}
	if !foundCourse {
		_, err = service.CreateCourse(training.CreateCourseInput{OrganizationID: config.organizationID, ActorID: config.administratorID, ID: localCourseID, Version: lessonVersion, Title: "実在事例から学ぶセキュア開発", Lessons: []training.LessonRef{{LessonID: lessonID, LessonVersion: lessonVersion}}, RequiredLessons: []training.LessonRef{{LessonID: lessonID, LessonVersion: lessonVersion}}})
		if err != nil {
			return training.Assignment{}, err
		}
	}
	assignments, err := service.ListAssignments(config.organizationID)
	if err != nil {
		return training.Assignment{}, err
	}
	for _, assignment := range assignments {
		if assignment.CourseID == localCourseID && assignment.CourseVersion == lessonVersion && assignment.LearnerID == config.learnerID {
			return assignment, nil
		}
	}
	created, err := service.CreateAssignments(training.CreateAssignmentsInput{OrganizationID: config.organizationID, ActorID: config.administratorID, CourseID: localCourseID, CourseVersion: lessonVersion, LearnerIDs: []string{config.learnerID}, DueAt: time.Now().UTC().AddDate(1, 0, 0), AllowRetry: true})
	if err != nil {
		return training.Assignment{}, err
	}
	return created[0], nil
}

func localSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("Runner state directories must be private real directories (0700)")
	}
	return nil
}

func (app *application) Close() { _ = app.server.Listener.Close(); _ = os.RemoveAll(app.snapshot) }

// 起動と教材閲覧では取得もDocker操作もしない。最初のPrepareだけが固定ソースを取得する。
type lazyService struct {
	manager     *runtime.Manager
	resolved    lab.ResolvedLab
	materialize materializeSource
	state       string
	mu          sync.Mutex
	service     *runnerapi.LifecycleService
}

func (service *lazyService) Prepare(ctx context.Context, reference runnerapi.LessonReference) (runtime.Session, error) {
	expected := runnerapi.LessonReference{LessonID: lessonID, LessonVersion: lessonVersion, LabRef: "vulnerable"}
	if reference != expected {
		return runtime.Session{}, errors.New("only the reviewed File Browser comparison reference can be prepared")
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.service == nil {
		root, err := os.MkdirTemp(service.state, ".source-*")
		if err != nil {
			return runtime.Session{}, err
		}
		defer os.RemoveAll(root)
		fetchContext, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		materialized, err := service.materialize(fetchContext, service.resolved, filepath.Join(root, "lab"))
		if err != nil {
			return runtime.Session{}, err
		}
		prepared, err := lab.ToPreparedLab(service.resolved, materialized)
		if err != nil {
			return runtime.Session{}, err
		}
		fixed, err := runnerapi.NewLifecycleService(service.manager, runnerapi.FixedLab{Lesson: expected, Prepared: prepared})
		if err != nil {
			return runtime.Session{}, err
		}
		service.service = fixed
	}
	return service.service.Prepare(ctx, reference)
}
func (service *lazyService) Start(ctx context.Context, id string) (runtime.Session, error) {
	return service.manager.Start(ctx, id)
}
func (service *lazyService) Stop(ctx context.Context, id string) (runtime.Session, error) {
	return service.manager.Stop(ctx, id)
}
func (service *lazyService) Delete(ctx context.Context, id string, remove bool) (runtime.Session, error) {
	return service.manager.Delete(ctx, id, remove)
}
func (service *lazyService) Status(ctx context.Context, id string) (runtime.Session, error) {
	return service.manager.Status(ctx, id)
}
func (service *lazyService) ObserveMarker(ctx context.Context, id, reference string) (runtime.MarkerObservation, error) {
	return service.manager.ObserveMarker(ctx, id, reference)
}
