package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPrepareDoesNotStartAndStartRechecksCompose(t *testing.T) {
	provider := &fakeProvider{}
	store := NewMemoryStore()
	manager, err := NewManager(store, provider)
	if err != nil {
		t.Fatal(err)
	}
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	if provider.starts != 0 || session.State != Prepared || !session.SafetyChecked {
		t.Fatalf("prepare = %#v, starts=%d", session, provider.starts)
	}
	session.Compose = []byte("services:\n  app:\n    image: example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n    privileged: true\n")
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), session.ID); err == nil {
		t.Fatal("unsafe stored compose started")
	}
	if provider.starts != 0 {
		t.Fatal("provider started unsafe compose")
	}
}

func TestLifecycleRollsBackAndDeleteIsIdempotent(t *testing.T) {
	provider := &fakeProvider{resources: Resources{Containers: []Container{{Name: "app", State: "running"}}}}
	manager, _ := NewManager(NewMemoryStore(), provider)
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	provider.resources = Resources{Volumes: []string{"reprokit_data"}}
	stopped, err := manager.Stop(context.Background(), session.ID)
	if err != nil || stopped.State != Stopped {
		t.Fatalf("stop = %#v, %v", stopped, err)
	}
	deleted, err := manager.Delete(context.Background(), session.ID, false)
	if err != nil || deleted.State != Deleted || len(deleted.RetainedVolumes) != 1 {
		t.Fatalf("delete = %#v, %v", deleted, err)
	}
	if _, err := manager.Delete(context.Background(), session.ID, false); err != nil {
		t.Fatal(err)
	}
	if provider.deletes != 1 {
		t.Fatalf("delete calls = %d", provider.deletes)
	}
}

func TestStartFailureCleansUp(t *testing.T) {
	provider := &fakeProvider{startErr: errors.New("daemon unavailable")}
	manager, _ := NewManager(NewMemoryStore(), provider)
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Start(context.Background(), session.ID)
	if err == nil || !strings.Contains(err.Error(), "daemon unavailable") || result.State != Failed || provider.deletes != 1 {
		t.Fatalf("start = %#v, %v, deletes=%d", result, err, provider.deletes)
	}
}

func validLab() PreparedLab {
	return PreparedLab{LabID: "vulhub/filebrowser-cve-2026-54088", SourceRevision: strings.Repeat("a", 40), Compose: []byte("services:\n  app:\n    image: example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n    ports:\n      - \"8080:80\"\n")}
}

type fakeProvider struct {
	starts, stops, deletes int
	startErr               error
	resources              Resources
}

func (provider *fakeProvider) Start(context.Context, string, []byte) error {
	provider.starts++
	return provider.startErr
}
func (provider *fakeProvider) Stop(context.Context, string, []byte) error {
	provider.stops++
	return nil
}
func (provider *fakeProvider) Delete(context.Context, string, []byte, bool) error {
	provider.deletes++
	return nil
}
func (provider *fakeProvider) Inspect(context.Context, string, []byte) (Resources, error) {
	return provider.resources, nil
}
