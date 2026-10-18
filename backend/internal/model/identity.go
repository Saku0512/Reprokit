// Package model はアプリケーション横断で利用するドメインモデルを定義する。
package model

import "time"

type IdentityRole string

const (
	IdentityRoleOrganizationAdmin IdentityRole = "organization_admin"
	IdentityRoleTrainingManager   IdentityRole = "training_manager"
	IdentityRoleContentAuthor     IdentityRole = "content_author"
	IdentityRoleReviewer          IdentityRole = "reviewer"
	IdentityRoleLearner           IdentityRole = "learner"
)

type IdentityPermission string

const (
	IdentityPermissionManageOrganization IdentityPermission = "organization.manage"
	IdentityPermissionManageUsers        IdentityPermission = "users.manage"
	IdentityPermissionManageIdentity     IdentityPermission = "identity.manage"
	IdentityPermissionManageAssignments  IdentityPermission = "assignments.manage"
	IdentityPermissionReviewSubmissions  IdentityPermission = "submissions.review"
	IdentityPermissionManageContent      IdentityPermission = "content.manage"
	IdentityPermissionAccessLearning     IdentityPermission = "learning.access"
	IdentityPermissionReadAudit          IdentityPermission = "audit.read"
)

type IdentityAuditAction string

const (
	IdentityAuditUserCreated          IdentityAuditAction = "user_created"
	IdentityAuditUserRolesChanged     IdentityAuditAction = "user_roles_changed"
	IdentityAuditUserDisabled         IdentityAuditAction = "user_disabled"
	IdentityAuditAuthenticationOK     IdentityAuditAction = "authentication_succeeded"
	IdentityAuditAuthenticationFailed IdentityAuditAction = "authentication_failed"
	IdentityAuditIdentityConfigured   IdentityAuditAction = "identity_provider_configured"
)

type IdentityOrganization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type IdentityUser struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Subject        string         `json:"subject"`
	Email          string         `json:"email"`
	DisplayName    string         `json:"displayName"`
	Roles          []IdentityRole `json:"roles"`
	Active         bool           `json:"active"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// IdentityPrincipalは認証済みユーザーへの不変参照である。権限は常にStoreから再取得する。
type IdentityPrincipal struct {
	OrganizationID string
	UserID         string
}

// IdentityAuditEventは認証と権限変更だけを記録し、トークンやメールアドレスを保持しない。
type IdentityAuditEvent struct {
	ID             string              `json:"id"`
	OrganizationID string              `json:"organizationId"`
	Action         IdentityAuditAction `json:"action"`
	ActorID        string              `json:"actorId,omitempty"`
	TargetUserID   string              `json:"targetUserId,omitempty"`
	BeforeRoles    []IdentityRole      `json:"beforeRoles,omitempty"`
	AfterRoles     []IdentityRole      `json:"afterRoles,omitempty"`
	OccurredAt     time.Time           `json:"occurredAt"`
}

// IdentityOIDCConfigは公開可能なIdP設定だけを保持する。Client Secretは実行時に注入する。
type IdentityOIDCConfig struct {
	OrganizationID string    `json:"organizationId"`
	Issuer         string    `json:"issuer"`
	ClientID       string    `json:"clientId"`
	RedirectURL    string    `json:"redirectUrl"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
