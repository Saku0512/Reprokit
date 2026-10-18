package identity

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type persistedState struct {
	Organizations map[string]Organization `json:"organizations"`
	Users         map[string]User         `json:"users"`
	AuditEvents   map[string]AuditEvent   `json:"auditEvents"`
	OIDCProviders map[string]OIDCConfig   `json:"oidcProviders"`
}

type Store struct {
	root  string
	mu    sync.Mutex
	state persistedState
}

func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.New("identity directory must be private (0700)")
	}
	store := &Store{root: root, state: emptyState()}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func emptyState() persistedState {
	return persistedState{Organizations: map[string]Organization{}, Users: map[string]User{}, AuditEvents: map[string]AuditEvent{}, OIDCProviders: map[string]OIDCConfig{}}
}

func (store *Store) load() error {
	data, err := os.ReadFile(filepath.Join(store.root, "identity.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || len(data) > maxStateBytes {
		return errors.New("could not read identity state")
	}
	var state persistedState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("corrupt identity state")
	}
	if state.OIDCProviders == nil {
		state.OIDCProviders = map[string]OIDCConfig{}
	}
	if !validState(state) {
		return errors.New("corrupt identity state")
	}
	store.state = state
	return nil
}

func (store *Store) save() error {
	data, err := json.Marshal(store.state)
	if err != nil || len(data) > maxStateBytes {
		return ErrInvalid
	}
	file, err := os.CreateTemp(store.root, ".identity-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(store.root, "identity.json"))
}

func validState(state persistedState) bool {
	if state.Organizations == nil || state.Users == nil || state.AuditEvents == nil || state.OIDCProviders == nil {
		return false
	}
	for key, organization := range state.Organizations {
		if key != organization.ID || !validID(organization.ID) || organization.Name == "" || organization.CreatedAt.IsZero() {
			return false
		}
	}
	subjects := map[string]bool{}
	activeAdministrators := map[string]int{}
	for key, user := range state.Users {
		if key != userKey(user.OrganizationID, user.ID) || !validID(user.OrganizationID) || !validID(user.ID) || !validProfile(user.Subject, user.Email, user.DisplayName) || !validRoles(user.Roles) || user.CreatedAt.IsZero() || user.UpdatedAt.Before(user.CreatedAt) {
			return false
		}
		if _, exists := state.Organizations[user.OrganizationID]; !exists {
			return false
		}
		subjectKey := user.OrganizationID + "\x00" + user.Subject
		if subjects[subjectKey] {
			return false
		}
		subjects[subjectKey] = true
		if user.Active && hasRole(user.Roles, RoleOrganizationAdmin) {
			activeAdministrators[user.OrganizationID]++
		}
	}
	for organizationID := range state.Organizations {
		if activeAdministrators[organizationID] == 0 {
			return false
		}
	}
	for key, event := range state.AuditEvents {
		if key != eventKey(event.OrganizationID, event.ID) || !validAuditEvent(event) {
			return false
		}
		if _, exists := state.Organizations[event.OrganizationID]; !exists {
			return false
		}
	}
	for key, config := range state.OIDCProviders {
		if key != config.OrganizationID || !validID(config.OrganizationID) || !validOIDCIssuer(config.Issuer) || config.ClientID == "" || len(config.ClientID) > 256 || !validRedirectURL(config.RedirectURL) || config.UpdatedAt.IsZero() {
			return false
		}
		if _, exists := state.Organizations[config.OrganizationID]; !exists {
			return false
		}
	}
	return true
}
