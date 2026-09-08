package router

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	gcs "cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/SSpencer740/fastfso/backend/internal/actionitem"
	"github.com/SSpencer740/fastfso/backend/internal/aiusage"
	"github.com/SSpencer740/fastfso/backend/internal/audit"
	"github.com/SSpencer740/fastfso/backend/internal/auth"
	"github.com/SSpencer740/fastfso/backend/internal/chat"
	"github.com/SSpencer740/fastfso/backend/internal/database"
	"github.com/SSpencer740/fastfso/backend/internal/dd254"
	"github.com/SSpencer740/fastfso/backend/internal/digest"
	"github.com/SSpencer740/fastfso/backend/internal/email"
	"github.com/SSpencer740/fastfso/backend/internal/emailcode"
	"github.com/SSpencer740/fastfso/backend/internal/env"
	"github.com/SSpencer740/fastfso/backend/internal/errorreport"
	"github.com/SSpencer740/fastfso/backend/internal/identity"
	"github.com/SSpencer740/fastfso/backend/internal/invite"
	"github.com/SSpencer740/fastfso/backend/internal/logging"
	"github.com/SSpencer740/fastfso/backend/internal/notify"
	"github.com/SSpencer740/fastfso/backend/internal/passkey"
	"github.com/SSpencer740/fastfso/backend/internal/reminders"
	"github.com/SSpencer740/fastfso/backend/internal/report"
	"github.com/SSpencer740/fastfso/backend/internal/scan"
	"github.com/SSpencer740/fastfso/backend/internal/session"
	"github.com/SSpencer740/fastfso/backend/internal/sso"
	"github.com/SSpencer740/fastfso/backend/internal/storage"
	"github.com/SSpencer740/fastfso/backend/internal/suborg"
	"github.com/SSpencer740/fastfso/backend/internal/task"
	"github.com/SSpencer740/fastfso/backend/internal/team"
	"github.com/SSpencer740/fastfso/backend/internal/telemetry"
	"github.com/SSpencer740/fastfso/backend/internal/totp"
	"github.com/SSpencer740/fastfso/backend/internal/travel"
	"github.com/SSpencer740/fastfso/backend/internal/verification"
	"github.com/SSpencer740/fastfso/backend/internal/visitrequest"
	"github.com/SSpencer740/fastfso/backend/internal/wiki"
)

// New creates a Gin engine with all routes registered.
func New(ctx context.Context, db database.DB, logger *slog.Logger) (*gin.Engine, error) {
	r := gin.New()
	r.Use(auth.SecurityHeaders(), telemetry.Middleware(), logging.GinMiddleware(logger), gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Frontend error reporting (public, rate-limited)
	errorLogger := logging.Component(logger, "frontend_error")
	r.POST("/api/v1/errors", errorreport.RateLimitMiddleware(), errorreport.Handler(errorLogger))

	// Email service
	emailLogger := logging.Component(logger, "email")
	var emailSender email.Sender
	// Shared Cloud Tasks client; used by email send queue and verification
	// upload-verify queue. Nil outside cloud mode — both subsystems then no-op.
	var tasksClient *cloudtasks.Client
	// Cloud Tasks-authenticated route group; lazily created so non-cloud
	// environments don't expose the endpoint at all.
	var cloudTasksRoutes *gin.RouterGroup
	if env.IsCloud() {
		var err error
		tasksClient, err = cloudtasks.NewClient(ctx)
		if err != nil {
			logger.Error("failed to create cloud tasks client", "error", err)
			return nil, fmt.Errorf("cloud tasks client: %w", err)
		}
		group := r.Group("/api/tasks")
		group.Use(auth.RequireCloudTasks())
		cloudTasksRoutes = group
	}
	if env.IsCloud() && env.SendGridAPIKey() != "" {
		sendgridSender := email.NewSendGridSender(env.SendGridAPIKey(), env.EmailFrom(), emailLogger)
		emailSender = email.NewQueueSender(tasksClient, env.EmailCloudTasksQueue(), env.BackendURL(), env.BackendServiceAccount(), emailLogger)
		cloudTasksRoutes.POST("/send-email", email.HandleSendTask(sendgridSender, emailLogger))
	} else {
		emailSender = email.NewLogSender(emailLogger)
	}
	emailService := email.New(emailSender, env.FrontendURL(), emailLogger)

	// Stores
	identityStore := identity.NewStore(db)
	sessionStore := session.NewStore(db)
	auditStore := audit.NewStore(db, logging.Component(logger, "audit"))

	// Rate limiter for auth endpoints: 15 requests per 15 minutes per IP (database-backed)
	authLimiter := auth.NewRateLimiter(auditStore, 15, 15*time.Minute)
	totpStore := totp.NewStore(db)
	emailCodeStore := emailcode.NewStore(db)
	passkeyStore := passkey.NewStore(db)
	ssoStore := sso.NewStore(db)
	authStore := auth.NewStore(db)
	inviteStore := invite.NewStore(db)
	taskStore := task.NewStore(db)
	actionItemStore := actionitem.NewStore(db)
	travelStore := travel.NewStore(db)
	visitRequestStore := visitrequest.NewStore(db)
	reportStore := report.NewStore(db)

	// File storage backend
	var storageBackend storage.StorageBackend
	if env.IsCloud() {
		bucket := env.StorageBucket()
		if bucket == "" {
			return nil, fmt.Errorf("STORAGE_BUCKET is required when DEPLOY_ENV=cloud")
		}
		gcsClient, err := gcs.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("gcs client: %w", err)
		}
		storageBackend = storage.NewGCS(gcsClient, bucket)
		logger.Info("file storage configured", "backend", "gcs", "bucket", bucket)
	} else {
		storageBackend = storage.NewLocal("./uploads")
		logger.Info("file storage configured", "backend", "local", "path", "./uploads")
	}

	// Handlers
	userSettingsStore := auth.NewUserSettingsStore(db)
	// suborgStore is constructed up here because auth.Handler now depends on
	// it (for sub-org assignment during invites). The matching suborgHandler
	// stays where it was further down.
	suborgStore := suborg.NewStore(db)
	authHandler := auth.NewHandler(authStore, identityStore, sessionStore, auditStore, totpStore, passkeyStore, inviteStore, inviteStore, emailService, userSettingsStore, suborgStore, logging.Component(logger, "auth"))

	// Invite handler with tenant name lookup adapter
	tenantLookup := func(ctx context.Context, identityID uuid.UUID) string {
		memberships, err := authStore.ListTenantsByIdentity(ctx, identityID)
		if err != nil || len(memberships) == 0 {
			return ""
		}
		return memberships[0].Name
	}
	inviteHandler := invite.NewHandler(inviteStore, identityStore, sessionStore, auditStore, tenantLookup, logging.Component(logger, "invite"))
	totpHandler := totp.NewHandler(totpStore, identityStore, sessionStore, auditStore, logging.Component(logger, "totp"))
	emailCodeHandler := emailcode.NewHandler(emailCodeStore, sessionStore, auditStore, logging.Component(logger, "emailcode"))
	ssoHandler := sso.NewHandler(ssoStore, identityStore, sessionStore, auditStore, logging.Component(logger, "sso"))

	notifier := notify.New(emailService, userSettingsStore, logging.Component(logger, "notify"))
	digestService := digest.New(digest.NewStore(db), emailService, logging.Component(logger, "digest"))
	remindersService := reminders.New(reminders.NewStore(db), notifier, logging.Component(logger, "reminders"))

	var scanner scan.Scanner
	if addr := env.ClamAVAddr(); addr != "" {
		scanner = scan.NewClamAV(addr, 0)
		logger.Info("malware scanner configured", "addr", addr)
	} else {
		if env.IsCloud() {
			return nil, fmt.Errorf("CLAMAV_ADDR is required when DEPLOY_ENV=cloud")
		}
		scanner = scan.Noop{}
		logger.Warn("malware scanner not configured (CLAMAV_ADDR empty); uploads will not be scanned")
	}

	// AI upload verification: store, worker, and an enqueuer that schedules
	// verification jobs after each upload. In cloud mode the enqueuer goes
	// through Cloud Tasks; locally it runs the worker inline via a
	// goroutine (LocalEnqueuer) so developers can exercise the full flow
	// without standing up Cloud Tasks. The local path is wired further down
	// once the worker exists.
	verificationStore := verification.NewStore(db)
	type verifyEnqueuerIface interface {
		Enqueue(ctx context.Context, uploadID uuid.UUID) error
	}
	var verifyEnqueuer verifyEnqueuerIface
	if env.IsCloud() && cloudTasksRoutes != nil {
		verifyEnqueuer = verification.NewEnqueuer(tasksClient, env.VerifyCloudTasksQueue(), env.BackendURL(), env.BackendServiceAccount(), logging.Component(logger, "verification"))
	}
	taskHandler := task.NewHandler(taskStore, actionItemStore, auditStore, notifier, storageBackend, scanner, verificationStore, verifyEnqueuer, logging.Component(logger, "task"))
	actionItemHandler := actionitem.NewHandler(actionItemStore, auditStore, notifier, logging.Component(logger, "actionitem"), map[string]actionitem.SourceUpdater{
		"visit_request": func(ctx context.Context, tenantID, sourceID, reviewerID uuid.UUID, status string) error {
			return visitRequestStore.UpdateStatus(ctx, tenantID, sourceID, reviewerID, status, "")
		},
		"travel_report": func(ctx context.Context, tenantID, sourceID, reviewerID uuid.UUID, status string) error {
			return travelStore.UpdateStatus(ctx, tenantID, sourceID, status, reviewerID)
		},
		"report": func(ctx context.Context, tenantID, sourceID, reviewerID uuid.UUID, status string) error {
			mapped := status
			if status == "approved" {
				mapped = "processed"
			}
			return reportStore.UpdateStatus(ctx, tenantID, sourceID, reviewerID, mapped)
		},
		"task_submission": func(ctx context.Context, tenantID, sourceID, reviewerID uuid.UUID, status string) error {
			// sourceID is the completion ID; map action item status to task completion status
			if status == "processed" {
				return taskStore.Approve(ctx, sourceID, reviewerID)
			}
			if status == "rejected" {
				return taskStore.Reject(ctx, sourceID, reviewerID)
			}
			return nil // under_review is informational only
		},
		"travel_debrief": func(_ context.Context, _, _, _ uuid.UUID, _ string) error {
			return nil // no status update needed on the debrief itself
		},
	})
	travelHandler := travel.NewHandler(travelStore, actionItemStore, auditStore, notifier, storageBackend, scanner, logging.Component(logger, "travel"))
	dd254Store := dd254.NewStore(db)
	dd254Handler := dd254.NewHandler(dd254Store, storageBackend, auditStore, scanner, logging.Component(logger, "dd254"))
	visitRequestHandler := visitrequest.NewHandler(visitRequestStore, actionItemStore, auditStore, notifier, dd254Store, logging.Component(logger, "visitrequest"))
	reportHandler := report.NewHandler(reportStore, actionItemStore, auditStore, notifier, logging.Component(logger, "report"))
	wikiStore := wiki.NewStore(db)
	wikiHandler := wiki.NewHandler(wikiStore, storageBackend, auditStore, scanner, logging.Component(logger, "wiki"))
	suborgHandler := suborg.NewHandler(suborgStore, sessionStore, auditStore, logging.Component(logger, "suborg"))
	teamStore := team.NewStore(db)
	teamHandler := team.NewHandler(teamStore, auditStore, logging.Component(logger, "team"))

	chatStore := chat.NewStore(db)
	aiUsageStore := aiusage.NewStore(db)
	aiUsageHandler := aiusage.NewHandler(aiUsageStore, logging.Component(logger, "aiusage"))
	geminiClient, err := chat.NewClient(ctx)
	if err != nil {
		logger.Warn("gemini client unavailable — chat endpoint disabled", "error", err)
	}
	chatHandler := chat.NewHandler(actionItemStore, taskStore, chatStore, aiUsageStore, geminiClient, logging.Component(logger, "chat"))

	// Verification worker: processes upload-verification Cloud Tasks AND
	// admin feedback POSTs. Hoisted to outer scope so the taskAdminWrite
	// route group (defined later) can register HandleFeedback. Cloud Tasks
	// callback is only wired in cloud mode; in local mode we instead inject
	// a LocalEnqueuer back into the task handler so uploads still trigger
	// verification (inline goroutine). Feedback is wired whenever the
	// worker exists.
	var verifyWorker *verification.Worker
	if geminiClient != nil {
		verifier := verification.NewVerifier(geminiClient)
		verifyWorker = verification.NewWorker(db, verificationStore, verifier, storageBackend, aiUsageStore, logging.Component(logger, "verification"))
		if cloudTasksRoutes != nil {
			cloudTasksRoutes.POST("/verify-upload", verifyWorker.HandleVerifyTask)
		} else {
			localEnq, err := verification.NewLocalEnqueuer(verifyWorker, logging.Component(logger, "verification"))
			if err != nil {
				return nil, fmt.Errorf("local verification enqueuer: %w", err)
			}
			verifyEnqueuer = localEnq
			// Re-construct the task handler so it picks up the local enqueuer.
			// The earlier construction above used a nil enqueuer because the
			// worker wasn't available yet. This is the simplest path that
			// keeps the cloud branch unchanged.
			taskHandler = task.NewHandler(taskStore, actionItemStore, auditStore, notifier, storageBackend, scanner, verificationStore, verifyEnqueuer, logging.Component(logger, "task"))
		}
	}

	passkeyHandler, err := passkey.NewHandler(passkeyStore, identityStore, sessionStore, auditStore, logging.Component(logger, "passkey"))
	if err != nil {
		logger.Error("failed to initialize passkey handler", "error", err)
		passkeyHandler = nil
	}

	// Session middleware
	requireSession := auth.RequireSession(sessionStore, logging.Component(logger, "session"))

	// Public auth routes (no session required)
	authPublic := r.Group("/api/auth")
	authPublic.Use(authLimiter.Middleware())
	{
		authPublic.POST("/login", authHandler.Login)
		authPublic.POST("/login/identify", authHandler.Identify)

		// Passkey login (public)
		if passkeyHandler != nil {
			authPublic.POST("/login/passkey/begin", passkeyHandler.BeginLogin)
			authPublic.POST("/login/passkey/finish", passkeyHandler.FinishLogin)
		}

		// Invite (public)
		authPublic.POST("/invite/info", inviteHandler.Info)
		authPublic.POST("/invite/accept", inviteHandler.Accept)

		// Self-service password reset (public)
		authPublic.POST("/password-reset/request", authHandler.RequestPasswordReset)
		authPublic.POST("/password-reset/confirm", authHandler.ConfirmPasswordReset)

		// SSO (public)
		authPublic.GET("/sso/:tenant_slug", ssoHandler.InitiateSSO)
		authPublic.GET("/sso/callback/oidc", ssoHandler.OIDCCallback)
		authPublic.POST("/sso/callback/saml", ssoHandler.SAMLCallback)
	}

	// Pre-auth routes (session state=pre_auth)
	authPreAuth := r.Group("/api/auth/2fa")
	authPreAuth.Use(requireSession, auth.RequireState(session.StatePre_Auth))
	{
		authPreAuth.GET("/methods", authHandler.Get2FAMethods)
		authPreAuth.POST("/totp/verify", totpHandler.Verify)
		authPreAuth.POST("/email/send", emailCodeHandler.Send)
		authPreAuth.POST("/email/verify", emailCodeHandler.Verify)

		// Passkey as 2FA
		if passkeyHandler != nil {
			authPreAuth.POST("/passkey/begin", passkeyHandler.Begin2FA)
			authPreAuth.POST("/passkey/finish", passkeyHandler.Finish2FA)
		}
	}

	// Pre-tenant routes (session state=pre_tenant)
	authPreTenant := r.Group("/api/auth")
	authPreTenant.Use(requireSession, auth.RequireState(session.StatePreTenant))
	{
		authPreTenant.GET("/tenants", authHandler.ListTenants)
		authPreTenant.POST("/select-tenant", auth.CSRFMiddleware(), authHandler.SelectTenant)
		authPreTenant.POST("/select-admin", auth.CSRFMiddleware(), authHandler.SelectAdmin)
	}

	// TOTP enrollment — accessible from setup_2fa (mandatory first login) and authenticated (optional add)
	authTOTP := r.Group("/api/auth")
	authTOTP.Use(requireSession, auth.RequireState(session.StateSetup2FA, session.StateAuthenticated), auth.CSRFMiddleware())
	{
		authTOTP.POST("/totp/enroll", totpHandler.Enroll)
		authTOTP.POST("/totp/confirm", totpHandler.ConfirmEnroll)
	}

	// /me is intentionally available for any session state. The SPA's
	// initialize() calls it on every load to discover where in the auth
	// flow the user is (unauthenticated, pre_auth, setup_2fa, pre_tenant,
	// authenticated). The handler conditionally includes user/tenant info
	// only when those fields are populated, so it is safe for early
	// states. Without this, SSO callbacks (which land in pre_tenant) would
	// fail the state check and bounce the user back to /login.
	authMe := r.Group("/api/auth")
	authMe.Use(requireSession)
	{
		authMe.GET("/me", authHandler.Me)
	}

	// Authenticated routes (session state=authenticated)
	authAuthed := r.Group("/api/auth")
	authAuthed.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		authAuthed.POST("/logout", authHandler.Logout)
		authAuthed.POST("/switch-context", authHandler.SwitchContext)
		authAuthed.GET("/sessions", authHandler.ListSessions)
		authAuthed.DELETE("/sessions", authHandler.RevokeAllOtherSessions)
		authAuthed.DELETE("/sessions/:id", authHandler.RevokeSession)

		// TOTP management
		authAuthed.DELETE("/totp", totpHandler.Remove)

		// Passkey management
		if passkeyHandler != nil {
			authAuthed.POST("/passkeys/register/begin", passkeyHandler.BeginRegistration)
			authAuthed.POST("/passkeys/register/finish", passkeyHandler.FinishRegistration)
			authAuthed.GET("/passkeys", passkeyHandler.List)
			authAuthed.DELETE("/passkeys/:id", passkeyHandler.Remove)
		}

		// User settings
		authAuthed.POST("/change-password", authHandler.ChangePassword)
		authAuthed.GET("/security", authHandler.SecurityOverview)
		authAuthed.GET("/user-settings", authHandler.GetUserSettings)
		authAuthed.PUT("/user-settings", authHandler.UpdateUserSettings)
	}

	// Internal cron routes (called by Cloud Scheduler)
	cron := r.Group("/api/cron")
	cron.Use(auth.RequireCloudScheduler())
	{
		cron.POST("/session-cleanup", session.CleanupHandler(sessionStore, logging.Component(logger, "session_cleanup"), passkeyStore.CleanupExpiredWebAuthnSessions))
		cron.POST("/travel-debrief", travelHandler.DebriefCron)
		cron.POST("/chat-cleanup", chatHandler.CleanupCron)
		cron.POST("/daily-digest", digestService.CronHandler)
		cron.POST("/task-reminders", remindersService.CronHandler)
	}

	// Super admin routes
	adminRoutes := r.Group("/api/admin")
	adminRoutes.Use(requireSession, auth.CSRFMiddleware(), auth.RequireSuperAdmin())
	{
		adminRoutes.GET("/sessions", authHandler.AdminListSessions)
		adminRoutes.DELETE("/sessions/:id", authHandler.AdminRevokeSession)
		adminRoutes.GET("/audit", authHandler.AdminListAudit)
		adminRoutes.GET("/tenants", authHandler.AdminListTenants)
		adminRoutes.POST("/tenants", authHandler.AdminCreateTenant)
		adminRoutes.GET("/tenants/:id", authHandler.AdminGetTenant)
		adminRoutes.PUT("/tenants/:id", authHandler.AdminUpdateTenant)
		adminRoutes.DELETE("/tenants/:id", authHandler.AdminDeleteTenant)
		adminRoutes.POST("/tenants/:id/suspend", authHandler.AdminSuspendTenant)
		adminRoutes.POST("/tenants/:id/unsuspend", authHandler.AdminUnsuspendTenant)
		adminRoutes.POST("/tenants/:id/invite", authHandler.AdminInviteAdmin)
		adminRoutes.GET("/tenants/:id/ai-usage", aiUsageHandler.AdminTenantUsage)
		adminRoutes.GET("/tenants/:id/sso", ssoHandler.GetSSOConfig)
		adminRoutes.PUT("/tenants/:id/sso", ssoHandler.UpdateSSOConfig)
		adminRoutes.GET("/tenants/:id/sso/domains", ssoHandler.ListEmailDomains)
		adminRoutes.POST("/tenants/:id/sso/domains", ssoHandler.AddEmailDomain)
		adminRoutes.DELETE("/tenants/:id/sso/domains/:domain", ssoHandler.RemoveEmailDomain)
		adminRoutes.GET("/identities", authHandler.AdminListIdentities)
		adminRoutes.POST("/identities", authHandler.AdminCreateIdentity)
		adminRoutes.GET("/identities/:id", authHandler.AdminGetIdentity)
		adminRoutes.DELETE("/identities/:id", authHandler.AdminDeleteIdentity)
		adminRoutes.POST("/identities/:id/reset-password", authHandler.AdminResetPassword)
		adminRoutes.POST("/identities/:id/super-admin", authHandler.AdminSetSuperAdmin)
		adminRoutes.POST("/identities/:id/suspend", authHandler.AdminSuspendIdentity)
		adminRoutes.POST("/identities/:id/unsuspend", authHandler.AdminUnsuspendIdentity)
		adminRoutes.POST("/identities/:id/tenants", authHandler.AdminAddIdentityToTenant)
		adminRoutes.POST("/identities/:id/resend-invite", authHandler.AdminResendInvite)
		adminRoutes.PUT("/identities/:id/email", authHandler.AdminChangeEmail)
		adminRoutes.GET("/identities/:id/passkeys", passkeyHandler.AdminListPasskeys)
		adminRoutes.DELETE("/identities/:id/passkeys/:passkey_id", passkeyHandler.AdminDeletePasskey)

		// Passkey management for the super-admin's own identity. Mirrors the
		// tenant-scoped routes at /api/auth/passkeys, but reachable without a
		// tenant context so super-admin-only accounts can enroll a passkey.
		if passkeyHandler != nil {
			adminRoutes.POST("/passkeys/register/begin", passkeyHandler.BeginRegistration)
			adminRoutes.POST("/passkeys/register/finish", passkeyHandler.FinishRegistration)
			adminRoutes.GET("/passkeys", passkeyHandler.List)
			adminRoutes.DELETE("/passkeys/:id", passkeyHandler.Remove)
		}
	}

	// Tenant admin routes
	tenantAdmin := r.Group("/api/v1/admin")
	tenantAdmin.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator"))
	{
		tenantAdmin.POST("/suspend", authHandler.AdminSuspendTenant)
		tenantAdmin.POST("/unsuspend", authHandler.AdminUnsuspendTenant)
		tenantAdmin.POST("/invite", authHandler.AdminInviteAdmin)
		tenantAdmin.POST("/invite/bulk", authHandler.AdminBulkInvite)
		tenantAdmin.GET("/members", authHandler.TenantListMembers)
		tenantAdmin.PATCH("/members/:user_id/role", authHandler.TenantUpdateMemberRole)
		tenantAdmin.PUT("/members/:user_id/sub-org", suborgHandler.SetUserSubOrg)
		tenantAdmin.DELETE("/members/:user_id", authHandler.TenantRemoveMember)
		tenantAdmin.GET("/sub-orgs", suborgHandler.List)
		tenantAdmin.POST("/sub-orgs", suborgHandler.Create)
		tenantAdmin.PUT("/sub-orgs/:id", suborgHandler.Update)
		tenantAdmin.DELETE("/sub-orgs/:id", suborgHandler.Delete)
		tenantAdmin.PUT("/sub-orgs/:id/primary-fso", suborgHandler.SetPrimaryFSO)
		tenantAdmin.GET("/sso", ssoHandler.GetSSOConfig)
		tenantAdmin.PUT("/sso", ssoHandler.UpdateSSOConfig)
		tenantAdmin.GET("/sso/domains", ssoHandler.ListEmailDomains)
		tenantAdmin.POST("/sso/domains", ssoHandler.AddEmailDomain)
		tenantAdmin.DELETE("/sso/domains/:domain", ssoHandler.RemoveEmailDomain)
	}

	// Team routes — FSO+ only (no IC access per design)
	teamRead := r.Group("/api/v1/team")
	teamRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		teamRead.GET("/", teamHandler.List)
		teamRead.GET("/stats", teamHandler.Stats)
		teamRead.GET("/:user_id", teamHandler.Get)
	}
	teamWrite := r.Group("/api/v1/team")
	teamWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		teamWrite.PUT("/:user_id/clearance", teamHandler.SetClearance)
	}

	// DD254 routes — FSO+ only
	dd254Read := r.Group("/api/v1/dd254")
	dd254Read.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		dd254Read.GET("/", dd254Handler.List)
		dd254Read.GET("/stats", dd254Handler.Stats)
		dd254Read.GET("/:id", dd254Handler.Get)
		dd254Read.GET("/:id/file", dd254Handler.ViewFile)
	}
	dd254Write := r.Group("/api/v1/dd254")
	dd254Write.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		dd254Write.POST("/", dd254Handler.Upload)
		dd254Write.DELETE("/:id", dd254Handler.Delete)
		dd254Write.POST("/:id/access", dd254Handler.GrantAccess)
		dd254Write.DELETE("/:id/access/:user_id", dd254Handler.RevokeAccess)
	}

	// Tenant audit log (administrator + fso)
	tenantAudit := r.Group("/api/v1/admin/audit")
	tenantAudit.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		tenantAudit.GET("/", authHandler.TenantAuditLog)
	}

	// Task admin routes - read (administrator + fso + read_only_fso)
	taskAdminRead := r.Group("/api/v1/admin/tasks")
	taskAdminRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		taskAdminRead.GET("/", taskHandler.ListTasks)
		taskAdminRead.GET("/stats", taskHandler.GetTaskStats)
		taskAdminRead.GET("/users", taskHandler.ListUsers)
		taskAdminRead.GET("/sub-orgs", taskHandler.ListSubOrgs)
		taskAdminRead.GET("/completions/:completion_id", taskHandler.GetSubmission)
		taskAdminRead.GET("/uploads/:upload_id/download", taskHandler.DownloadUpload)
		taskAdminRead.GET("/export", taskHandler.ExportTasks)
		taskAdminRead.GET("/:id", taskHandler.GetTask)
	}
	// Task admin routes - write (administrator + fso)
	taskAdminWrite := r.Group("/api/v1/admin/tasks")
	taskAdminWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		taskAdminWrite.POST("/", taskHandler.CreateTask)
		taskAdminWrite.POST("/:id/archive", taskHandler.ArchiveTask)
		taskAdminWrite.POST("/:id/unarchive", taskHandler.UnarchiveTask)
		taskAdminWrite.POST("/:id/assignees/:user_id/approve", taskHandler.ApproveSubmission)
		taskAdminWrite.POST("/:id/assignees/:user_id/reject", taskHandler.RejectSubmission)
		if verifyWorker != nil {
			taskAdminWrite.POST("/uploads/:upload_id/verification/feedback", verifyWorker.HandleFeedback)
		}
	}

	// Me routes (any authenticated tenant user)
	meRoutes := r.Group("/api/v1/me")
	meRoutes.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		meRoutes.GET("/fso", suborgHandler.GetMyFSO)
		meRoutes.GET("/dd254-authorizations", dd254Handler.MyAuthorizations)
		meRoutes.POST("/chat", chatHandler.Chat)
		meRoutes.GET("/chat/history", chatHandler.History)
		meRoutes.GET("/chat/usage", chatHandler.Usage)
	}

	// Task IC routes (any authenticated user)
	taskIC := r.Group("/api/v1/tasks")
	taskIC.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		taskIC.GET("/", taskHandler.ListMyTasks)
		taskIC.GET("/stats", taskHandler.GetMyTaskStats)
		taskIC.GET("/:id", taskHandler.GetMyTask)
		taskIC.POST("/:id/responses", taskHandler.SaveResponses)
		taskIC.POST("/:id/submit", taskHandler.SubmitTask)
		taskIC.POST("/:id/upload/:requirement_id", taskHandler.UploadFile)
		taskIC.DELETE("/:id/upload/:upload_id", taskHandler.DeleteUpload)
		taskIC.GET("/files/*key", taskHandler.ServeLocalFile)
	}

	// Action item admin routes - read (administrator + fso + read_only_fso)
	actionItemAdminRead := r.Group("/api/v1/admin/action-items")
	actionItemAdminRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		actionItemAdminRead.GET("/", actionItemHandler.List)
		actionItemAdminRead.GET("/stats", actionItemHandler.GetStats)
		actionItemAdminRead.GET("/fso-users", actionItemHandler.ListFSOUsers)
		actionItemAdminRead.GET("/:id", actionItemHandler.Get)
	}
	// Action item admin routes - write (administrator + fso)
	actionItemAdminWrite := r.Group("/api/v1/admin/action-items")
	actionItemAdminWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		actionItemAdminWrite.POST("/:id/status", actionItemHandler.UpdateStatus)
		actionItemAdminWrite.POST("/:id/notes", actionItemHandler.UpdateNotes)
		actionItemAdminWrite.PATCH("/:id/assignee", actionItemHandler.Reassign)
	}

	// Travel IC routes (any authenticated user)
	travelIC := r.Group("/api/v1/travel")
	travelIC.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		travelIC.POST("/", travelHandler.CreateReport)
		travelIC.GET("/", travelHandler.ListMyReports)
		travelIC.GET("/stats", travelHandler.GetMyStats)
		travelIC.GET("/debriefs", travelHandler.GetMyDebriefs)
		travelIC.POST("/debriefs/:id/submit", travelHandler.SubmitDebrief)
		travelIC.GET("/:id", travelHandler.GetReport)
		travelIC.PUT("/:id", travelHandler.UpdateReport)
		travelIC.POST("/:id/submit", travelHandler.SubmitReport)
		travelIC.POST("/:id/upload", travelHandler.UploadFile)
		travelIC.DELETE("/:id/upload/:upload_id", travelHandler.DeleteUpload)
	}

	// Travel admin routes - read (administrator + fso + read_only_fso)
	travelAdminRead := r.Group("/api/v1/admin/travel")
	travelAdminRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		travelAdminRead.GET("/", travelHandler.ListReports)
		travelAdminRead.GET("/stats", travelHandler.GetStats)
		travelAdminRead.GET("/debriefs/:id", travelHandler.AdminGetDebrief)
		travelAdminRead.GET("/uploads/:upload_id/download", travelHandler.DownloadUpload)
		travelAdminRead.GET("/:id", travelHandler.AdminGetReport)
	}
	// Travel admin routes - write (administrator + fso)
	travelAdminWrite := r.Group("/api/v1/admin/travel")
	travelAdminWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		travelAdminWrite.POST("/:id/approve", travelHandler.ApproveReport)
		travelAdminWrite.POST("/:id/reject", travelHandler.RejectReport)
	}

	// Visit request IC routes (any authenticated user)
	visitIC := r.Group("/api/v1/visits")
	visitIC.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		visitIC.POST("/", visitRequestHandler.Create)
		visitIC.GET("/", visitRequestHandler.ListMy)
		visitIC.GET("/stats", visitRequestHandler.MyStats)
		visitIC.GET("/cloneable", visitRequestHandler.ListCloneable)
		visitIC.GET("/:id", visitRequestHandler.GetMy)
		visitIC.POST("/:id/cancel", visitRequestHandler.Cancel)
	}

	// Visit request admin routes - read (administrator + fso + read_only_fso)
	visitAdminRead := r.Group("/api/v1/admin/visits")
	visitAdminRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		visitAdminRead.GET("/", visitRequestHandler.AdminList)
		visitAdminRead.GET("/stats", visitRequestHandler.AdminStats)
		visitAdminRead.GET("/export", visitRequestHandler.Export)
		visitAdminRead.GET("/:id", visitRequestHandler.AdminGet)
		visitAdminRead.GET("/:id/dd254-suggestions", dd254Handler.SuggestForVisit)
	}
	// Visit request admin routes - write (administrator + fso)
	visitAdminWrite := r.Group("/api/v1/admin/visits")
	visitAdminWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		visitAdminWrite.POST("/:id/status", visitRequestHandler.AdminUpdateStatus)
		visitAdminWrite.PUT("/:id/dd254", dd254Handler.LinkVisitRequest)
	}

	// Report IC routes (any authenticated user)
	reportIC := r.Group("/api/v1/reports")
	reportIC.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		reportIC.POST("/", reportHandler.Create)
		reportIC.GET("/", reportHandler.ListMy)
		reportIC.GET("/stats", reportHandler.MyStats)
		reportIC.GET("/:id", reportHandler.GetMy)
	}

	// Report admin routes - read (administrator + fso + read_only_fso)
	reportAdminRead := r.Group("/api/v1/admin/reports")
	reportAdminRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		reportAdminRead.GET("/", reportHandler.AdminList)
		reportAdminRead.GET("/stats", reportHandler.AdminStats)
		reportAdminRead.GET("/:id", reportHandler.AdminGet)
	}
	// Report admin routes - write (administrator + fso)
	reportAdminWrite := r.Group("/api/v1/admin/reports")
	reportAdminWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		reportAdminWrite.POST("/:id/status", reportHandler.AdminUpdateStatus)
	}

	// Wiki IC routes (any authenticated user)
	wikiIC := r.Group("/api/v1/wiki")
	wikiIC.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware())
	{
		wikiIC.GET("/", wikiHandler.ListPublished)
		wikiIC.GET("/fso", wikiHandler.GetFSOInfo)
		wikiIC.GET("/files/:file_id", wikiHandler.ViewFile)
		wikiIC.GET("/local-files/*key", wikiHandler.ServeLocalFile)
	}

	// Wiki admin routes - read (administrator + fso + read_only_fso)
	wikiAdminRead := r.Group("/api/v1/admin/wiki")
	wikiAdminRead.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso", "read_only_fso"))
	{
		wikiAdminRead.GET("/", wikiHandler.AdminList)
		wikiAdminRead.GET("/:id", wikiHandler.AdminGet)
	}
	// Wiki admin routes - write (administrator + fso)
	wikiAdminWrite := r.Group("/api/v1/admin/wiki")
	wikiAdminWrite.Use(requireSession, auth.RequireTenant(), auth.CSRFMiddleware(), auth.RequireRole(authHandler, "administrator", "fso"))
	{
		wikiAdminWrite.POST("/", wikiHandler.AdminCreate)
		wikiAdminWrite.PUT("/:id", wikiHandler.AdminUpdate)
		wikiAdminWrite.DELETE("/:id", wikiHandler.AdminDelete)
		wikiAdminWrite.POST("/:id/files", wikiHandler.AdminUploadFile)
		wikiAdminWrite.DELETE("/files/:file_id", wikiHandler.AdminDeleteFile)
	}

	return r, nil
}
