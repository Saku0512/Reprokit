package identity

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func testService(t *testing.T) *Service {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "identity"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC) }
	return service
}

func bootstrap(t *testing.T, service *Service) Principal {
	t.Helper()
	_, user, err := service.Bootstrap(BootstrapInput{OrganizationID: "acme", OrganizationName: "Acme", UserID: "admin", Subject: "idp-admin", Email: "admin@acme.test", DisplayName: "管理者"})
	if err != nil {
		t.Fatal(err)
	}
	return Principal{OrganizationID: user.OrganizationID, UserID: user.ID}
}

func TestOrganizationUsersRolesAndAuditArePersisted(t *testing.T) {
	service := testService(t)
	admin := bootstrap(t, service)
	reviewer, err := service.CreateUser(admin, CreateUserInput{ID: "reviewer", Subject: "idp-reviewer", Email: "reviewer@acme.test", DisplayName: "レビュー担当", Roles: []Role{RoleReviewer}})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Authorize(Principal{OrganizationID: "acme", UserID: reviewer.ID}, PermissionReviewSubmissions); err != nil {
		t.Fatal(err)
	}
	if err := service.Authorize(Principal{OrganizationID: "acme", UserID: reviewer.ID}, PermissionManageAssignments); !errors.Is(err, ErrForbidden) {
		t.Fatalf("reviewer gained training manager permission: %v", err)
	}
	updated, err := service.SetRoles(admin, reviewer.ID, []Role{RoleLearner, RoleTrainingManager})
	if err != nil || len(updated.Roles) != 2 || updated.Roles[0] != RoleLearner {
		t.Fatalf("unexpected role update: %#v %v", updated, err)
	}
	events, err := service.ListAuditEvents(admin)
	hasRoleChange := false
	for _, event := range events {
		hasRoleChange = hasRoleChange || event.Action == AuditUserRolesChanged
	}
	if err != nil || len(events) != 3 || !hasRoleChange {
		t.Fatalf("unexpected audit events: %#v %v", events, err)
	}
	reopenedStore, err := NewStore(service.store.root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(reopenedStore)
	if err != nil {
		t.Fatal(err)
	}
	users, err := reopened.ListUsers(admin)
	if err != nil || len(users) != 2 {
		t.Fatalf("identity state was not persisted: %#v %v", users, err)
	}
}

func TestOrganizationBoundaryAndLastAdminAreEnforced(t *testing.T) {
	service := testService(t)
	admin := bootstrap(t, service)
	learner, err := service.CreateUser(admin, CreateUserInput{ID: "learner", Subject: "idp-learner", Email: "learner@acme.test", DisplayName: "受講者", Roles: []Role{RoleLearner}})
	if err != nil {
		t.Fatal(err)
	}
	_, otherAdmin, err := service.Bootstrap(BootstrapInput{OrganizationID: "other", OrganizationName: "Other", UserID: "admin", Subject: "other-admin", Email: "admin@other.test", DisplayName: "別組織管理者"})
	if err != nil {
		t.Fatal(err)
	}
	other := Principal{OrganizationID: "other", UserID: otherAdmin.ID}
	if _, err := service.User(other, learner.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-organization user access was allowed: %v", err)
	}
	if _, err := service.SetRoles(admin, admin.UserID, []Role{RoleTrainingManager}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("last administrator was removed: %v", err)
	}
	if _, err := service.DisableUser(admin, admin.UserID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("administrator disabled itself: %v", err)
	}
	if _, err := service.PrincipalForSubject("other", "idp-learner"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("subject crossed organization boundary: %v", err)
	}
}

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		role    Role
		allowed Permission
		denied  Permission
	}{
		{RoleOrganizationAdmin, PermissionManageIdentity, Permission("")},
		{RoleTrainingManager, PermissionManageAssignments, PermissionManageUsers},
		{RoleContentAuthor, PermissionManageContent, PermissionReviewSubmissions},
		{RoleReviewer, PermissionReviewSubmissions, PermissionManageContent},
		{RoleLearner, PermissionAccessLearning, PermissionReadAudit},
	}
	for _, item := range cases {
		if !allowed([]Role{item.role}, item.allowed) {
			t.Errorf("%s should allow %s", item.role, item.allowed)
		}
		if item.denied != "" && allowed([]Role{item.role}, item.denied) {
			t.Errorf("%s should deny %s", item.role, item.denied)
		}
	}
	if allowed([]Role{RoleOrganizationAdmin}, Permission("unknown.permission")) {
		t.Fatal("organization administrator was allowed an unknown permission")
	}
}

func TestPersistedStateRejectsDuplicateSubjectsAndMissingAdministrator(t *testing.T) {
	service := testService(t)
	admin := bootstrap(t, service)
	user, err := service.CreateUser(admin, CreateUserInput{ID: "reviewer", Subject: "idp-reviewer", Email: "reviewer@acme.test", DisplayName: "レビュー担当", Roles: []Role{RoleReviewer}})
	if err != nil {
		t.Fatal(err)
	}
	state := copyPersistedStateForTest(service.store.state)
	user.Subject = "idp-admin"
	state.Users[userKey(user.OrganizationID, user.ID)] = user
	if validState(state) {
		t.Fatal("duplicate OIDC subjects were accepted")
	}
	state = copyPersistedStateForTest(service.store.state)
	administrator := state.Users[userKey("acme", "admin")]
	administrator.Active = false
	state.Users[userKey("acme", "admin")] = administrator
	if validState(state) {
		t.Fatal("organization without an active administrator was accepted")
	}
}

func copyPersistedStateForTest(source persistedState) persistedState {
	result := emptyState()
	for key, value := range source.Organizations {
		result.Organizations[key] = value
	}
	for key, value := range source.Users {
		result.Users[key] = value
	}
	for key, value := range source.AuditEvents {
		result.AuditEvents[key] = value
	}
	for key, value := range source.OIDCProviders {
		result.OIDCProviders[key] = value
	}
	return result
}

func TestOIDCConfigurationAndAuthenticationAudit(t *testing.T) {
	service := testService(t)
	admin := bootstrap(t, service)
	config, err := service.ConfigureOIDC(admin, "https://idp.acme.test", "reprokit", "https://training.acme.test/api/auth/oidc/callback")
	if err != nil || config.OrganizationID != "acme" {
		t.Fatalf("unexpected OIDC config: %#v %v", config, err)
	}
	loaded, err := service.OIDCConfiguration("acme")
	if err != nil || loaded.ClientID != "reprokit" {
		t.Fatalf("OIDC config was not persisted: %#v %v", loaded, err)
	}
	if err := service.RecordAuthentication("acme", "admin", true); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordAuthentication("acme", "", false); err != nil {
		t.Fatal(err)
	}
	events, err := service.ListAuditEvents(admin)
	if err != nil {
		t.Fatal(err)
	}
	actions := map[AuditAction]bool{}
	for _, event := range events {
		actions[event.Action] = true
	}
	for _, action := range []AuditAction{AuditIdentityConfigured, AuditAuthenticationOK, AuditAuthenticationFailed} {
		if !actions[action] {
			t.Fatalf("missing audit action %q: %#v", action, events)
		}
	}
	if _, err := service.ConfigureOIDC(admin, "http://idp.example.test", "reprokit", "https://training.acme.test/callback"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("insecure remote issuer was accepted: %v", err)
	}
}
