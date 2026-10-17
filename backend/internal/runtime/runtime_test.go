package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jphacks/sd_2601/internal/network"
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

func TestStartRejectsTamperedLockArtifact(t *testing.T) {
	provider := &fakeProvider{}
	store := NewMemoryStore()
	manager, _ := NewManager(store, provider)
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	session.Manifest = append(session.Manifest, []byte("# changed\n")...)
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(context.Background(), session.ID); err == nil {
		t.Fatal("tampered Manifest started")
	}
	if provider.starts != 0 {
		t.Fatal("provider started a tampered Lab artifact")
	}
}

func TestPrepareWritesCompleteLockfile(t *testing.T) {
	manager, _ := NewManager(NewMemoryStore(), &fakeProvider{})
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	lock, err := decodeLock(session.Lockfile)
	if err != nil {
		t.Fatal(err)
	}
	if lock.LockVersion != 1 || lock.Lab != session.LabID || lock.Source.URL != "https://github.com/Saku0512/vulhub.git" || lock.Source.Subdirectory != "filebrowser/CVE-2026-54088" || lock.Runtime.Platform != "linux/amd64" || lock.Images["app"].Platform != "linux/amd64" {
		t.Fatalf("lock = %#v", lock)
	}
}

func TestOfflineArtifactsListsOnlyFixedSourceAndImages(t *testing.T) {
	store := NewMemoryStore()
	manager, _ := NewManager(store, &fakeProvider{})
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := manager.OfflineArtifacts(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 || artifacts[0].Kind != "git-source" || artifacts[0].Revision != session.SourceRevision || artifacts[1].Kind != "container-image" || artifacts[1].Digest == "" {
		t.Fatalf("unexpected offline artifacts: %#v", artifacts)
	}
	for _, artifact := range artifacts {
		if strings.Contains(strings.ToLower(artifact.Reference), "password") || strings.Contains(strings.ToLower(artifact.Reference), "token") {
			t.Fatalf("credential leaked into offline artifacts: %#v", artifact)
		}
	}
}

func TestConnectionCredentialsAreNotWrittenToLockfile(t *testing.T) {
	provider, err := NewDockerComposeProviderWithNetwork(network.Config{RegistryCredentials: []network.RegistryCredential{{Registry: "registry.example", Username: "learner", Password: "secret"}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(NewMemoryStore(), provider)
	if err != nil {
		t.Fatal(err)
	}
	session, err := manager.Prepare(context.Background(), validLab())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(session.Lockfile), "secret") || strings.Contains(string(session.Lockfile), "learner") || strings.Contains(string(session.Lockfile), "registry.example") {
		t.Fatalf("connection credential leaked into lockfile: %s", session.Lockfile)
	}
}

func validLab() PreparedLab {
	return PreparedLab{
		LabID:          "vulhub/filebrowser-cve-2026-54088",
		SourceRevision: strings.Repeat("a", 40),
		Artifact: Artifact{
			RegistryRevision:   strings.Repeat("b", 40),
			ManifestPath:       "manifests/vulhub/filebrowser-cve-2026-54088.yaml",
			Manifest:           []byte("apiVersion: reprokit.dev/v1alpha1\nkind: Lab\n"),
			SourceURL:          "https://github.com/Saku0512/vulhub.git",
			SourceSubdirectory: "filebrowser/CVE-2026-54088",
			RuntimeFile:        "reprokit.compose.yml",
			Platform:           "linux/amd64",
		},
		Compose: []byte("services:\n  app:\n    image: example/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n    ports:\n      - \"8080:80\"\n"),
	}
}

func TestObserveMarkerReadsOnlyRunningComparisonService(t *testing.T) {
	provider := &fakeProvider{marker: "uid=1000(filebrowser)"}
	manager, err := NewManager(NewMemoryStore(), provider)
	if err != nil {
		t.Fatal(err)
	}
	lab := validLab()
	lab.Compose = []byte("services:\n  vulnerable:\n    image: example/vulnerable@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n    ports:\n      - \"18080:80\"\n  fixed:\n    image: example/fixed@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n    ports:\n      - \"18081:80\"\n")
	session, err := manager.Prepare(context.Background(), lab)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ObserveMarker(context.Background(), session.ID, "vulnerable"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("prepared observation error = %v", err)
	}
	if _, err := manager.Start(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}
	observation, err := manager.ObserveMarker(context.Background(), session.ID, "vulnerable")
	if err != nil {
		t.Fatal(err)
	}
	if !observation.Exists || observation.Content != provider.marker || observation.Service != "vulnerable" || provider.markerReads != 1 {
		t.Fatalf("observation = %#v, reads = %d", observation, provider.markerReads)
	}
	if _, err := manager.ObserveMarker(context.Background(), session.ID, "other"); err == nil {
		t.Fatal("unknown comparison service was accepted")
	}
}

type fakeProvider struct {
	starts, stops, deletes int
	startErr               error
	resources              Resources
	marker                 string
	markerReads            int
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
func (provider *fakeProvider) ReadMarker(context.Context, string, []byte, string) (string, error) {
	provider.markerReads++
	return provider.marker, nil
}
