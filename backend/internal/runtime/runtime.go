package runtime

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jphacks/sd_2601/internal/security"
	"gopkg.in/yaml.v3"
)

type State string

const (
	Prepared State = "prepared"
	Running  State = "running"
	Stopped  State = "stopped"
	Deleted  State = "deleted"
	Failed   State = "error"
)

var (
	ErrNotFound     = errors.New("Lab session was not found")
	ErrInvalidState = errors.New("Lab session is not in a state that permits this operation")
	idPattern       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

type Endpoint struct {
	Service string `json:"service"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type Container struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type Resources struct {
	Containers []Container `json:"containers"`
	Networks   []string    `json:"networks"`
	Volumes    []string    `json:"volumes"`
}

type Failure struct {
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

type Session struct {
	ID                 string     `json:"labSessionId"`
	LabID              string     `json:"labId"`
	SourceRevision     string     `json:"sourceRevision"`
	RegistryRevision   string     `json:"registryRevision"`
	Project            string     `json:"-"`
	State              State      `json:"state"`
	SafetyChecked      bool       `json:"safetyChecked"`
	ExternalExpose     bool       `json:"externalExposure"`
	ManifestPath       string     `json:"-"`
	Manifest           []byte     `json:"-"`
	SourceURL          string     `json:"-"`
	SourceSubdirectory string     `json:"-"`
	RuntimeFile        string     `json:"-"`
	Platform           string     `json:"-"`
	SourceCompose      []byte     `json:"-"`
	Compose            []byte     `json:"-"`
	Lockfile           []byte     `json:"-"`
	Endpoints          []Endpoint `json:"endpoints"`
	Resources          Resources  `json:"resources"`
	RetainedVolumes    []string   `json:"retainedVolumes"`
	Failure            *Failure   `json:"failure"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type PreparedLab struct {
	LabID          string
	SourceRevision string
	Artifact       Artifact
	Compose        []byte
}

// Provider はRunnerが所有するCompose Projectだけを操作する境界である。
type Provider interface {
	Start(context.Context, string, []byte) error
	Stop(context.Context, string, []byte) error
	Delete(context.Context, string, []byte, bool) error
	Inspect(context.Context, string, []byte) (Resources, error)
}

type Store interface {
	Load(context.Context, string) (Session, error)
	Save(context.Context, Session) error
}

type MemoryStore struct {
	mu sync.Mutex
	m  map[string]Session
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]Session{}} }

func (store *MemoryStore) Load(_ context.Context, id string) (Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.m[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return copySession(session), nil
}

func (store *MemoryStore) Save(_ context.Context, session Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.m[session.ID] = copySession(session)
	return nil
}

type Manager struct {
	store    Store
	provider Provider
	locks    sync.Map
}

func NewManager(store Store, provider Provider) (*Manager, error) {
	if store == nil || provider == nil {
		return nil, errors.New("runtime store and provider are required")
	}
	return &Manager{store: store, provider: provider}, nil
}

// Prepare は安全性検査済みの固定Labを登録するだけで、コンテナを起動しない。
func (manager *Manager) Prepare(ctx context.Context, lab PreparedLab) (Session, error) {
	if lab.LabID != "vulhub/filebrowser-cve-2026-54088" || !shaPattern.MatchString(lab.SourceRevision) {
		return Session{}, errors.New("only the fixed File Browser Lab revision may be prepared")
	}
	inspection, err := security.InspectCompose(lab.Compose)
	if err != nil {
		return Session{}, fmt.Errorf("inspect Compose: %w", err)
	}
	lockfile, err := createLock(lab, inspection.Normalized)
	if err != nil {
		return Session{}, err
	}
	endpoints, err := endpoints(inspection.Normalized)
	if err != nil {
		return Session{}, err
	}
	id, err := newID()
	if err != nil {
		return Session{}, err
	}
	session := Session{ID: id, LabID: lab.LabID, SourceRevision: lab.SourceRevision, RegistryRevision: lab.Artifact.RegistryRevision, Project: "reprokit_" + strings.ReplaceAll(id[:13], "-", ""), State: Prepared, SafetyChecked: true, ManifestPath: lab.Artifact.ManifestPath, Manifest: append([]byte(nil), lab.Artifact.Manifest...), SourceURL: lab.Artifact.SourceURL, SourceSubdirectory: lab.Artifact.SourceSubdirectory, RuntimeFile: lab.Artifact.RuntimeFile, Platform: lab.Artifact.Platform, SourceCompose: append([]byte(nil), lab.Compose...), Compose: inspection.Normalized, Lockfile: lockfile, Endpoints: endpoints, UpdatedAt: time.Now().UTC()}
	if err := manager.store.Save(ctx, session); err != nil {
		return Session{}, err
	}
	return copySession(session), nil
}

func (manager *Manager) Start(ctx context.Context, id string) (Session, error) {
	return manager.change(ctx, id, func(session *Session) error {
		if session.State != Prepared && session.State != Stopped {
			return ErrInvalidState
		}
		if !session.SafetyChecked {
			return errors.New("prepared Compose safety check no longer matches")
		}
		compose, err := verifyArtifact(*session)
		if err != nil {
			return err
		}
		if err := manager.provider.Start(ctx, session.Project, compose); err != nil {
			return manager.startFailure(ctx, session, err)
		}
		resources, err := manager.provider.Inspect(ctx, session.Project, session.Compose)
		if err != nil {
			return manager.startFailure(ctx, session, err)
		}
		session.State, session.Resources, session.Failure = Running, resources, nil
		return nil
	})
}

func (manager *Manager) Stop(ctx context.Context, id string) (Session, error) {
	return manager.change(ctx, id, func(session *Session) error {
		if session.State == Deleted || session.State == Prepared || session.State == Stopped {
			return nil
		}
		if err := manager.provider.Stop(ctx, session.Project, session.Compose); err != nil {
			session.State, session.Failure = Failed, &Failure{Operation: "stop", Message: err.Error()}
			return err
		}
		resources, err := manager.provider.Inspect(ctx, session.Project, session.Compose)
		if err != nil {
			return err
		}
		session.State, session.Resources, session.Failure = Stopped, resources, nil
		return nil
	})
}

func (manager *Manager) Delete(ctx context.Context, id string, removeVolumes bool) (Session, error) {
	return manager.change(ctx, id, func(session *Session) error {
		if session.State == Deleted {
			return nil
		}
		if err := manager.provider.Delete(ctx, session.Project, session.Compose, removeVolumes); err != nil {
			session.State, session.Failure = Failed, &Failure{Operation: "delete", Message: err.Error()}
			return err
		}
		resources, err := manager.provider.Inspect(ctx, session.Project, session.Compose)
		if err != nil {
			return err
		}
		if !removeVolumes {
			session.RetainedVolumes, resources.Volumes = append([]string(nil), resources.Volumes...), nil
		}
		if len(resources.Containers) != 0 || len(resources.Networks) != 0 || len(resources.Volumes) != 0 {
			session.State, session.Resources, session.Failure = Failed, resources, &Failure{Operation: "delete", Message: "resources remain after delete"}
			return errors.New("resources remain after delete")
		}
		session.State, session.Resources, session.Failure = Deleted, Resources{}, nil
		return nil
	})
}

func (manager *Manager) Status(ctx context.Context, id string) (Session, error) {
	return manager.change(ctx, id, func(session *Session) error {
		resources, err := manager.provider.Inspect(ctx, session.Project, session.Compose)
		if err == nil {
			session.Resources = resources
		}
		return err
	})
}

func (manager *Manager) startFailure(ctx context.Context, session *Session, startErr error) error {
	cleanupErr := manager.provider.Delete(ctx, session.Project, session.Compose, false)
	resources, _ := manager.provider.Inspect(ctx, session.Project, session.Compose)
	message := startErr.Error()
	if cleanupErr != nil {
		message += "; cleanup: " + cleanupErr.Error()
	}
	session.State, session.Resources, session.Failure = Failed, resources, &Failure{Operation: "start", Message: message}
	return errors.New(message)
}

func (manager *Manager) change(ctx context.Context, id string, operation func(*Session) error) (Session, error) {
	if !idPattern.MatchString(id) {
		return Session{}, errors.New("invalid Lab session ID")
	}
	lockValue, _ := manager.locks.LoadOrStore(id, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	session, err := manager.store.Load(ctx, id)
	if err != nil {
		return Session{}, err
	}
	err = operation(&session)
	session.UpdatedAt = time.Now().UTC()
	if saveErr := manager.store.Save(ctx, session); saveErr != nil {
		return Session{}, saveErr
	}
	return copySession(session), err
}

func endpoints(compose []byte) ([]Endpoint, error) {
	var document struct {
		Services map[string]struct {
			Ports []string `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(compose, &document); err != nil {
		return nil, err
	}
	var result []Endpoint
	for service, definition := range document.Services {
		for _, value := range definition.Ports {
			parts := strings.Split(value, ":")
			if len(parts) != 3 || parts[0] != "127.0.0.1" {
				return nil, errors.New("non-loopback port in prepared Compose")
			}
			var port int
			if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
				return nil, err
			}
			result = append(result, Endpoint{Service: service, Address: parts[0], Port: port})
		}
	}
	return result, nil
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[:4], value[4:6], value[6:8], value[8:10], value[10:]), nil
}

func copySession(session Session) Session {
	session.Compose = append([]byte(nil), session.Compose...)
	session.Manifest = append([]byte(nil), session.Manifest...)
	session.SourceCompose = append([]byte(nil), session.SourceCompose...)
	session.Lockfile = append([]byte(nil), session.Lockfile...)
	session.Endpoints = append([]Endpoint(nil), session.Endpoints...)
	session.Resources.Containers = append([]Container(nil), session.Resources.Containers...)
	session.Resources.Networks = append([]string(nil), session.Resources.Networks...)
	session.Resources.Volumes = append([]string(nil), session.Resources.Volumes...)
	session.RetainedVolumes = append([]string(nil), session.RetainedVolumes...)
	if session.Failure != nil {
		failure := *session.Failure
		session.Failure = &failure
	}
	return session
}
