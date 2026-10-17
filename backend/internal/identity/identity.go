// Package identity は組織、ユーザー、役割と認証監査を管理する。
package identity

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

const maxStateBytes = 2 << 20

var (
	idPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
	ErrInvalid   = errors.New("invalid identity operation")
	ErrNotFound  = errors.New("identity resource not found")
	ErrForbidden = errors.New("identity operation forbidden")
)

type Role string

const (
	RoleOrganizationAdmin Role = "organization_admin"
	RoleTrainingManager   Role = "training_manager"
	RoleContentAuthor     Role = "content_author"
	RoleReviewer          Role = "reviewer"
	RoleLearner           Role = "learner"
)

type Permission string

const (
	PermissionManageOrganization Permission = "organization.manage"
	PermissionManageUsers        Permission = "users.manage"
	PermissionManageIdentity     Permission = "identity.manage"
	PermissionManageCourses      Permission = "courses.manage"
	PermissionManageAssignments  Permission = "assignments.manage"
	PermissionReviewSubmissions  Permission = "submissions.review"
	PermissionManageContent      Permission = "content.manage"
	PermissionAccessLearning     Permission = "learning.access"
	PermissionReadAudit          Permission = "audit.read"
)

type AuditAction string

const (
	AuditUserCreated          AuditAction = "user_created"
	AuditUserRolesChanged     AuditAction = "user_roles_changed"
	AuditUserDisabled         AuditAction = "user_disabled"
	AuditAuthenticationOK     AuditAction = "authentication_succeeded"
	AuditAuthenticationFailed AuditAction = "authentication_failed"
	AuditIdentityConfigured   AuditAction = "identity_provider_configured"
)

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Subject        string    `json:"subject"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"displayName"`
	Roles          []Role    `json:"roles"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Principalは認証済みユーザーへの不変参照である。権限は常にStoreから再取得する。
type Principal struct {
	OrganizationID string
	UserID         string
}

// AuditEventは認証と権限変更だけを記録し、トークンやメールアドレスを保持しない。
type AuditEvent struct {
	ID             string      `json:"id"`
	OrganizationID string      `json:"organizationId"`
	Action         AuditAction `json:"action"`
	ActorID        string      `json:"actorId,omitempty"`
	TargetUserID   string      `json:"targetUserId,omitempty"`
	BeforeRoles    []Role      `json:"beforeRoles,omitempty"`
	AfterRoles     []Role      `json:"afterRoles,omitempty"`
	OccurredAt     time.Time   `json:"occurredAt"`
}

type persistedState struct {
	Organizations map[string]Organization `json:"organizations"`
	Users         map[string]User         `json:"users"`
	AuditEvents   map[string]AuditEvent   `json:"auditEvents"`
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
	return persistedState{Organizations: map[string]Organization{}, Users: map[string]User{}, AuditEvents: map[string]AuditEvent{}}
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
	if err := decoder.Decode(&state); err != nil || decoder.Decode(&struct{}{}) != io.EOF || !validState(state) {
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

type Service struct {
	store *Store
	now   func() time.Time
}

func NewService(store *Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("identity store is required")
	}
	return &Service{store: store, now: time.Now}, nil
}

type BootstrapInput struct {
	OrganizationID   string
	OrganizationName string
	UserID           string
	Subject          string
	Email            string
	DisplayName      string
}

func (service *Service) Bootstrap(input BootstrapInput) (Organization, User, error) {
	if !validID(input.OrganizationID) || input.OrganizationName == "" || !validID(input.UserID) || !validProfile(input.Subject, input.Email, input.DisplayName) {
		return Organization{}, User{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	if _, exists := service.store.state.Organizations[input.OrganizationID]; exists {
		return Organization{}, User{}, ErrInvalid
	}
	now := service.now().UTC()
	organization := Organization{ID: input.OrganizationID, Name: input.OrganizationName, CreatedAt: now}
	user := User{ID: input.UserID, OrganizationID: input.OrganizationID, Subject: input.Subject, Email: input.Email, DisplayName: input.DisplayName, Roles: []Role{RoleOrganizationAdmin}, Active: true, CreatedAt: now, UpdatedAt: now}
	event, err := newAudit(input.OrganizationID, AuditUserCreated, input.UserID, input.UserID, nil, user.Roles, now)
	if err != nil {
		return Organization{}, User{}, err
	}
	service.store.state.Organizations[organization.ID] = organization
	service.store.state.Users[userKey(user.OrganizationID, user.ID)] = user
	service.store.state.AuditEvents[eventKey(event.OrganizationID, event.ID)] = event
	if err := service.store.save(); err != nil {
		delete(service.store.state.Organizations, organization.ID)
		delete(service.store.state.Users, userKey(user.OrganizationID, user.ID))
		delete(service.store.state.AuditEvents, eventKey(event.OrganizationID, event.ID))
		return Organization{}, User{}, err
	}
	return organization, user, nil
}

type CreateUserInput struct {
	ID          string
	Subject     string
	Email       string
	DisplayName string
	Roles       []Role
}

func (service *Service) CreateUser(principal Principal, input CreateUserInput) (User, error) {
	if !validID(input.ID) || !validProfile(input.Subject, input.Email, input.DisplayName) || !validRoles(input.Roles) {
		return User{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	actor, err := service.authorizedUser(principal, PermissionManageUsers)
	if err != nil {
		return User{}, err
	}
	key := userKey(actor.OrganizationID, input.ID)
	if _, exists := service.store.state.Users[key]; exists || service.subjectExists(actor.OrganizationID, input.Subject) {
		return User{}, ErrInvalid
	}
	now := service.now().UTC()
	user := User{ID: input.ID, OrganizationID: actor.OrganizationID, Subject: input.Subject, Email: input.Email, DisplayName: input.DisplayName, Roles: normalizedRoles(input.Roles), Active: true, CreatedAt: now, UpdatedAt: now}
	event, err := newAudit(actor.OrganizationID, AuditUserCreated, actor.ID, user.ID, nil, user.Roles, now)
	if err != nil {
		return User{}, err
	}
	service.store.state.Users[key] = user
	service.store.state.AuditEvents[eventKey(event.OrganizationID, event.ID)] = event
	if err := service.store.save(); err != nil {
		delete(service.store.state.Users, key)
		delete(service.store.state.AuditEvents, eventKey(event.OrganizationID, event.ID))
		return User{}, err
	}
	return copyUser(user), nil
}

func (service *Service) SetRoles(principal Principal, userID string, roles []Role) (User, error) {
	if !validID(userID) || !validRoles(roles) {
		return User{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	actor, err := service.authorizedUser(principal, PermissionManageUsers)
	if err != nil {
		return User{}, err
	}
	key := userKey(actor.OrganizationID, userID)
	user, exists := service.store.state.Users[key]
	if !exists {
		return User{}, ErrNotFound
	}
	next := normalizedRoles(roles)
	if hasRole(user.Roles, RoleOrganizationAdmin) && !hasRole(next, RoleOrganizationAdmin) && service.activeAdminCount(actor.OrganizationID) == 1 {
		return User{}, ErrInvalid
	}
	previous := copyUser(user)
	user.Roles = next
	user.UpdatedAt = service.now().UTC()
	event, err := newAudit(actor.OrganizationID, AuditUserRolesChanged, actor.ID, user.ID, previous.Roles, user.Roles, user.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	service.store.state.Users[key] = user
	service.store.state.AuditEvents[eventKey(event.OrganizationID, event.ID)] = event
	if err := service.store.save(); err != nil {
		service.store.state.Users[key] = previous
		delete(service.store.state.AuditEvents, eventKey(event.OrganizationID, event.ID))
		return User{}, err
	}
	return copyUser(user), nil
}

func (service *Service) DisableUser(principal Principal, userID string) (User, error) {
	if !validID(userID) {
		return User{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	actor, err := service.authorizedUser(principal, PermissionManageUsers)
	if err != nil {
		return User{}, err
	}
	key := userKey(actor.OrganizationID, userID)
	user, exists := service.store.state.Users[key]
	if !exists {
		return User{}, ErrNotFound
	}
	if !user.Active || actor.ID == user.ID || (hasRole(user.Roles, RoleOrganizationAdmin) && service.activeAdminCount(actor.OrganizationID) == 1) {
		return User{}, ErrInvalid
	}
	previous := user
	user.Active = false
	user.UpdatedAt = service.now().UTC()
	event, err := newAudit(actor.OrganizationID, AuditUserDisabled, actor.ID, user.ID, user.Roles, user.Roles, user.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	service.store.state.Users[key] = user
	service.store.state.AuditEvents[eventKey(event.OrganizationID, event.ID)] = event
	if err := service.store.save(); err != nil {
		service.store.state.Users[key] = previous
		delete(service.store.state.AuditEvents, eventKey(event.OrganizationID, event.ID))
		return User{}, err
	}
	return copyUser(user), nil
}

func (service *Service) PrincipalForSubject(organizationID, subject string) (Principal, error) {
	if !validID(organizationID) || subject == "" {
		return Principal{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	for _, user := range service.store.state.Users {
		if user.OrganizationID == organizationID && user.Subject == subject && user.Active {
			return Principal{OrganizationID: organizationID, UserID: user.ID}, nil
		}
	}
	return Principal{}, ErrNotFound
}

func (service *Service) Authorize(principal Principal, permission Permission) error {
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	_, err := service.authorizedUser(principal, permission)
	return err
}

func (service *Service) User(principal Principal, userID string) (User, error) {
	if !validID(userID) {
		return User{}, ErrInvalid
	}
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	actor, err := service.authorizedUser(principal, PermissionManageUsers)
	if err != nil {
		return User{}, err
	}
	user, exists := service.store.state.Users[userKey(actor.OrganizationID, userID)]
	if !exists {
		return User{}, ErrNotFound
	}
	return copyUser(user), nil
}

func (service *Service) ListUsers(principal Principal) ([]User, error) {
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	actor, err := service.authorizedUser(principal, PermissionManageUsers)
	if err != nil {
		return nil, err
	}
	users := []User{}
	for _, user := range service.store.state.Users {
		if user.OrganizationID == actor.OrganizationID {
			users = append(users, copyUser(user))
		}
	}
	sort.Slice(users, func(left, right int) bool { return users[left].ID < users[right].ID })
	return users, nil
}

func (service *Service) ListAuditEvents(principal Principal) ([]AuditEvent, error) {
	service.store.mu.Lock()
	defer service.store.mu.Unlock()
	actor, err := service.authorizedUser(principal, PermissionReadAudit)
	if err != nil {
		return nil, err
	}
	events := []AuditEvent{}
	for _, event := range service.store.state.AuditEvents {
		if event.OrganizationID == actor.OrganizationID {
			event.BeforeRoles = append([]Role{}, event.BeforeRoles...)
			event.AfterRoles = append([]Role{}, event.AfterRoles...)
			events = append(events, event)
		}
	}
	sort.Slice(events, func(left, right int) bool {
		if events[left].OccurredAt.Equal(events[right].OccurredAt) {
			return events[left].ID < events[right].ID
		}
		return events[left].OccurredAt.Before(events[right].OccurredAt)
	})
	return events, nil
}

func (service *Service) authorizedUser(principal Principal, permission Permission) (User, error) {
	if !validID(principal.OrganizationID) || !validID(principal.UserID) {
		return User{}, ErrForbidden
	}
	user, exists := service.store.state.Users[userKey(principal.OrganizationID, principal.UserID)]
	if !exists || !user.Active || !allowed(user.Roles, permission) {
		return User{}, ErrForbidden
	}
	return user, nil
}

func (service *Service) subjectExists(organizationID, subject string) bool {
	for _, user := range service.store.state.Users {
		if user.OrganizationID == organizationID && user.Subject == subject {
			return true
		}
	}
	return false
}

func (service *Service) activeAdminCount(organizationID string) int {
	count := 0
	for _, user := range service.store.state.Users {
		if user.OrganizationID == organizationID && user.Active && hasRole(user.Roles, RoleOrganizationAdmin) {
			count++
		}
	}
	return count
}

func allowed(roles []Role, permission Permission) bool {
	for _, role := range roles {
		if role == RoleOrganizationAdmin {
			return true
		}
		switch role {
		case RoleTrainingManager:
			if permission == PermissionManageCourses || permission == PermissionManageAssignments || permission == PermissionReadAudit {
				return true
			}
		case RoleContentAuthor:
			if permission == PermissionManageContent {
				return true
			}
		case RoleReviewer:
			if permission == PermissionReviewSubmissions {
				return true
			}
		case RoleLearner:
			if permission == PermissionAccessLearning {
				return true
			}
		}
	}
	return false
}

func validState(state persistedState) bool {
	if state.Organizations == nil || state.Users == nil || state.AuditEvents == nil {
		return false
	}
	for key, organization := range state.Organizations {
		if key != organization.ID || !validID(organization.ID) || organization.Name == "" || organization.CreatedAt.IsZero() {
			return false
		}
	}
	for key, user := range state.Users {
		if key != userKey(user.OrganizationID, user.ID) || !validID(user.OrganizationID) || !validID(user.ID) || !validProfile(user.Subject, user.Email, user.DisplayName) || !validRoles(user.Roles) || user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
			return false
		}
		if _, exists := state.Organizations[user.OrganizationID]; !exists {
			return false
		}
	}
	for key, event := range state.AuditEvents {
		if key != eventKey(event.OrganizationID, event.ID) || !validID(event.OrganizationID) || !validID(event.ID) || event.OccurredAt.IsZero() {
			return false
		}
	}
	return true
}

func validProfile(subject, email, displayName string) bool {
	return subject != "" && len(subject) <= 512 && email != "" && len(email) <= 254 && displayName != "" && len(displayName) <= 256
}

func validID(value string) bool { return idPattern.MatchString(value) }

func validRoles(roles []Role) bool {
	if len(roles) == 0 || len(roles) > 5 {
		return false
	}
	seen := map[Role]bool{}
	for _, role := range roles {
		if !validRole(role) || seen[role] {
			return false
		}
		seen[role] = true
	}
	return true
}

func validRole(role Role) bool {
	return role == RoleOrganizationAdmin || role == RoleTrainingManager || role == RoleContentAuthor || role == RoleReviewer || role == RoleLearner
}

func normalizedRoles(roles []Role) []Role {
	result := append([]Role{}, roles...)
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

func hasRole(roles []Role, expected Role) bool {
	for _, role := range roles {
		if role == expected {
			return true
		}
	}
	return false
}

func copyUser(user User) User {
	user.Roles = append([]Role{}, user.Roles...)
	return user
}

func userKey(organizationID, userID string) string   { return organizationID + ":" + userID }
func eventKey(organizationID, eventID string) string { return organizationID + ":" + eventID }

func newAudit(organizationID string, action AuditAction, actorID, targetID string, before, after []Role, now time.Time) (AuditEvent, error) {
	id, err := newID()
	if err != nil {
		return AuditEvent{}, err
	}
	return AuditEvent{ID: id, OrganizationID: organizationID, Action: action, ActorID: actorID, TargetUserID: targetID, BeforeRoles: append([]Role{}, before...), AfterRoles: append([]Role{}, after...), OccurredAt: now}, nil
}

func newID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
