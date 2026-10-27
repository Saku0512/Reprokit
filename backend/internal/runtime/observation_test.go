package runtime

import (
	"context"
	"testing"
)

func TestObservationRequiresProviderCapability(t *testing.T) {
	provider := &lifecycleOnlyProvider{}
	manager, err := NewManager(NewMemoryStore(), provider)
	if err != nil {
		t.Fatal(err)
	}
	lab := validLab()
	lab.Compose = comparisonComposeForTest()
	session, err := manager.Prepare(context.Background(), lab)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.ObserveMarker(context.Background(), session.ID, "vulnerable"); err == nil {
		t.Fatal("marker observation was allowed without MarkerReader")
	}
	if _, err := manager.ReadLogs(context.Background(), session.ID, "vulnerable"); err == nil {
		t.Fatal("log reading was allowed without LogReader")
	}
	if _, err := manager.RunAction(context.Background(), session.ID, "vulnerable", ListTmpAction); err == nil {
		t.Fatal("action execution was allowed without ActionRunner")
	}
}

func TestActionsAreBoundToTheirLabAndRunningState(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	provider := &fakeProvider{}
	manager, err := NewManager(store, provider)
	if err != nil {
		t.Fatal(err)
	}
	session := Session{ID: "01234567-89ab-4cde-8f01-23456789abcd", LabID: "reprokit/go-git-cve-2026-71557", State: Running}
	if err := store.Save(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RunAction(ctx, session.ID, "fixed", ReferenceCheckAction); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RunAction(ctx, session.ID, "fixed", ListTmpAction); err == nil {
		t.Fatal("File Browser action accepted for go-git")
	}
	session.LabID = "reprokit/filebrowser-cve-2026-54088"
	if err := store.Save(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RunAction(ctx, session.ID, "fixed", ReferenceCheckAction); err == nil {
		t.Fatal("go-git action accepted for File Browser")
	}
	session.LabID = "reprokit/go-git-cve-2026-71557"
	session.State = Prepared
	if err := store.Save(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.RunAction(ctx, session.ID, "fixed", ReferenceCheckAction); err == nil {
		t.Fatal("action accepted before the Lab was started")
	}
	if provider.actionRuns != 1 {
		t.Fatalf("action runs = %d, want 1", provider.actionRuns)
	}
}

func comparisonComposeForTest() []byte {
	return []byte(`services:
  vulnerable:
    image: example/vulnerable@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    ports:
      - "18080:80"
  fixed:
    image: example/fixed@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
    ports:
      - "18081:80"
`)
}

type lifecycleOnlyProvider struct{}

func (*lifecycleOnlyProvider) Start(context.Context, string, []byte) error { return nil }
func (*lifecycleOnlyProvider) Stop(context.Context, string, []byte) error  { return nil }
func (*lifecycleOnlyProvider) Delete(context.Context, string, []byte, bool) error {
	return nil
}
func (*lifecycleOnlyProvider) Inspect(context.Context, string, []byte) (Resources, error) {
	return Resources{}, nil
}
func (*lifecycleOnlyProvider) DiskUsage(context.Context, string) (int64, error) { return 0, nil }
