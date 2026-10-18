package runtime

import (
	"context"
	"errors"
	"sync"
)

// ObserveMarker は実行中のFile Browser比較Labにある固定マーカーだけを返す。
func (manager *Manager) ObserveMarker(ctx context.Context, id, service string) (MarkerObservation, error) {
	if !idPattern.MatchString(id) {
		return MarkerObservation{}, errors.New("invalid Lab session ID")
	}
	if service != "vulnerable" && service != "fixed" {
		return MarkerObservation{}, errors.New("unknown File Browser comparison service")
	}
	lockValue, _ := manager.locks.LoadOrStore(id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	session, err := manager.store.Load(ctx, id)
	if err != nil {
		return MarkerObservation{}, err
	}
	if session.LabID != "vulhub/filebrowser-cve-2026-54088" || session.State != Running {
		return MarkerObservation{}, ErrInvalidState
	}
	reader, ok := manager.provider.(MarkerReader)
	if !ok {
		return MarkerObservation{}, errors.New("the configured Lab provider cannot observe the marker")
	}
	content, err := reader.ReadMarker(ctx, session.Project, session.Compose, service)
	if err != nil {
		return MarkerObservation{}, err
	}
	return MarkerObservation{Service: service, Exists: content != "", Content: content}, nil
}

// ReadLogs は実行中のFile Browser比較Labの固定サービスのログだけを返す。
func (manager *Manager) ReadLogs(ctx context.Context, id, service string) (LogSnapshot, error) {
	if !idPattern.MatchString(id) {
		return LogSnapshot{}, errors.New("invalid Lab session ID")
	}
	if service != "vulnerable" && service != "fixed" {
		return LogSnapshot{}, errors.New("unknown File Browser comparison service")
	}
	lockValue, _ := manager.locks.LoadOrStore(id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	session, err := manager.store.Load(ctx, id)
	if err != nil {
		return LogSnapshot{}, err
	}
	if session.LabID != "vulhub/filebrowser-cve-2026-54088" || session.State != Running {
		return LogSnapshot{}, ErrInvalidState
	}
	reader, ok := manager.provider.(LogReader)
	if !ok {
		return LogSnapshot{}, errors.New("the configured Lab provider cannot read logs")
	}
	return reader.ReadLogs(ctx, session.Project, session.Compose, service)
}

// RunAction は実行中の比較Labでレビュー済みの固定観察だけを実行する。
func (manager *Manager) RunAction(ctx context.Context, id, service string, action ObservationAction) (ActionObservation, error) {
	if !idPattern.MatchString(id) {
		return ActionObservation{}, errors.New("invalid Lab session ID")
	}
	if service != "vulnerable" && service != "fixed" {
		return ActionObservation{}, errors.New("unknown File Browser comparison service")
	}
	if action != ListTmpAction {
		return ActionObservation{}, errors.New("unknown File Browser observation action")
	}
	lockValue, _ := manager.locks.LoadOrStore(id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	session, err := manager.store.Load(ctx, id)
	if err != nil {
		return ActionObservation{}, err
	}
	if session.LabID != "vulhub/filebrowser-cve-2026-54088" || session.State != Running {
		return ActionObservation{}, ErrInvalidState
	}
	runner, ok := manager.provider.(ActionRunner)
	if !ok {
		return ActionObservation{}, errors.New("the configured Lab provider cannot run observation actions")
	}
	return runner.RunAction(ctx, session.Project, session.Compose, service, action)
}
