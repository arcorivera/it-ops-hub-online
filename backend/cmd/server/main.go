package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"itopshub/backend/internal/audit"
	authpkg "itopshub/backend/internal/auth"
	"itopshub/backend/internal/backup"
	"itopshub/backend/internal/categories"
	"itopshub/backend/internal/config"
	"itopshub/backend/internal/dashboard"
	"itopshub/backend/internal/database"
	"itopshub/backend/internal/escalation"
	"itopshub/backend/internal/followup"
	"itopshub/backend/internal/incidents"
	appLogger "itopshub/backend/internal/logger"
	appMiddleware "itopshub/backend/internal/middleware"
	"itopshub/backend/internal/notifications"
	"itopshub/backend/internal/projects"
	"itopshub/backend/internal/rca"
	"itopshub/backend/internal/reports"
	"itopshub/backend/internal/savedviews"
	"itopshub/backend/internal/scheduler"
	"itopshub/backend/internal/search"
	"itopshub/backend/internal/settings"
	"itopshub/backend/internal/sla"
	"itopshub/backend/internal/teams"
	testingpkg "itopshub/backend/internal/testing"
	"itopshub/backend/internal/tickets"
	"itopshub/backend/internal/users"
)

const version = "0.8.0-phase8"

func main() {
	cfg := config.Load()

	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("failed to create data directories: %v", err)
	}

	sl, err := appLogger.New(cfg.LogsDir)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	sl.Info("starting IT Operations Hub", "version", version, "port", cfg.Port)

	db, err := database.Open(cfg.DBPath, sl)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	sl.Info("database ready", "path", cfg.DBPath)

	// --- repositories & services ---
	userRepo := users.NewRepository(db)
	sessionStore := authpkg.NewStore(db, cfg.SessionTTLHrs)
	auditLogger := audit.NewLogger(db)
	teamRepo := teams.NewRepository(db)
	categoryRepo := categories.NewRepository(db)
	ticketRepo := tickets.NewRepository(db)
	notifRepo := notifications.NewRepository(db)

	// --- handlers ---
	authHandler := authpkg.NewHandler(userRepo, sessionStore, auditLogger, sl, cfg.Environment == "production")
	usersHandler := users.NewHandler(userRepo, auditLogger, sl, authpkg.HashPassword)
	teamsHandler := teams.NewHandler(teamRepo, auditLogger, sl)
	categoriesHandler := categories.NewHandler(categoryRepo, auditLogger, sl)
	ticketsHandler := tickets.NewHandler(ticketRepo, auditLogger, sl, cfg.UploadsDir, userRepo, notifRepo)
	slaHandler := sla.NewHandler(db, sl)
	notifHandler := notifications.NewHandler(notifRepo, sl)
	followupHandler := followup.NewHandler(db, auditLogger, notifRepo, sl)
	escalationHandler := escalation.NewHandler(db, auditLogger, notifRepo, sl)
	dashboardHandler := dashboard.NewHandler(db, sl)

	projectRepo := projects.NewRepository(db)
	projectsHandler := projects.NewHandler(projectRepo, auditLogger, sl)

	testingRepo := testingpkg.NewRepository(db)
	testingHandler := testingpkg.NewHandler(testingRepo, auditLogger, notifRepo, sl)
	ticketsHandler.SetUATChecker(testingRepo)

	incidentRepo := incidents.NewRepository(db)
	incidentsHandler := incidents.NewHandler(incidentRepo, auditLogger, sl, userRepo)

	rcaRepo := rca.NewRepository(db)
	rcaHandler := rca.NewHandler(rcaRepo, auditLogger, sl)
	incidentsHandler.SetRCAChecker(rcaRepo)

	searchHandler := search.NewHandler(db, sl)

	reportsRepo := reports.NewRepository(db)
	reportsHandler := reports.NewHandler(reportsRepo, sl)

	savedViewsRepo := savedviews.NewRepository(db)
	savedViewsHandler := savedviews.NewHandler(savedViewsRepo, sl)

	backupRepo := backup.NewRepository(db, cfg.BackupsDir, cfg.DBPath)
	backupHandler := backup.NewHandler(backupRepo, db, auditLogger, sl, cfg.DBPath)

	settingsRepo := settings.NewRepository(db)
	settingsHandler := settings.NewHandler(settingsRepo, auditLogger, sl)

	auditHandler := audit.NewHandler(auditLogger)

	// --- background scheduler: follow-up + escalation engines ---
	followupEngine := followup.NewEngine(db, notifRepo, sl)
	escalationEngine := escalation.NewEngine(db, notifRepo, sl)
	sched := scheduler.New(time.Duration(cfg.SchedulerEvery)*time.Second, followupEngine, escalationEngine, sl)
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	go sched.Start(schedCtx)

	// --- router ---
	r := chi.NewRouter()
	r.Use(appMiddleware.SecurityHeaders)
	r.Use(appMiddleware.RequestLogger(sl))

	rl := appMiddleware.NewRateLimiter(300, time.Minute)
	r.Use(rl.Middleware)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		dbStatus := "ok"
		if err := db.Ping(); err != nil {
			dbStatus = "error"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"database": dbStatus,
			"version":  version,
			"time":     time.Now().UTC(),
		})
	})

	r.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(a chi.Router) {
			a.Get("/status", authHandler.Status)
			a.Post("/setup", authHandler.Setup)
			a.Post("/login", authHandler.Login)
			a.Post("/logout", authHandler.Logout)
			a.Get("/me", authHandler.Me)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(appMiddleware.RequireAuth(sessionStore, userRepo, sl))

			protected.Route("/users", func(u chi.Router) {
				u.Get("/", usersHandler.List)
				u.With(appMiddleware.RequireRole(users.RoleAdmin)).Post("/", usersHandler.Create)
				u.Get("/{id}", usersHandler.Get)
				u.With(appMiddleware.RequireRole(users.RoleAdmin)).Put("/{id}", usersHandler.Update)
			})

			protected.Route("/teams", func(t chi.Router) {
				t.Get("/", teamsHandler.List)
				t.Get("/{id}", teamsHandler.Get)
				t.With(appMiddleware.RequireRole(users.RoleAdmin, users.RoleITManager)).Post("/", teamsHandler.Create)
				t.With(appMiddleware.RequireRole(users.RoleAdmin, users.RoleITManager)).Put("/{id}", teamsHandler.Update)
			})

			protected.Route("/categories", func(c chi.Router) {
				c.Get("/", categoriesHandler.List)
				c.With(appMiddleware.RequireRole(users.RoleAdmin, users.RoleITManager)).Post("/", categoriesHandler.Create)
			})

			protected.Route("/tickets", func(t chi.Router) {
				t.Get("/", ticketsHandler.List)
				t.Post("/", ticketsHandler.Create)
				t.Get("/{id}", ticketsHandler.Get)
				t.Post("/{id}/assign", ticketsHandler.Assign)
				t.Post("/{id}/status", ticketsHandler.ChangeStatus)
				t.Post("/{id}/severity", ticketsHandler.ChangeSeverity)
				t.Post("/{id}/priority", ticketsHandler.ChangePriority)

				t.Get("/{id}/comments", ticketsHandler.ListComments)
				t.Post("/{id}/comments", ticketsHandler.AddComment)
				t.Delete("/{id}/comments/{commentId}", ticketsHandler.DeleteComment)

				t.Get("/{id}/history", ticketsHandler.ListHistory)

				t.Get("/{id}/attachments", ticketsHandler.ListAttachments)
				t.Post("/{id}/attachments", ticketsHandler.UploadAttachment)
				t.Get("/{id}/attachments/{attachmentId}/download", ticketsHandler.DownloadAttachment)

				t.Get("/{id}/sla", slaHandler.GetTicketSLA)

				t.Get("/{id}/followups", followupHandler.List)
				t.Post("/{id}/follow-up", followupHandler.Trigger)

				t.Get("/{id}/escalations", escalationHandler.List)
				t.Post("/{id}/escalate", escalationHandler.Trigger)

				t.Get("/{id}/test-cases", testingHandler.ListTestCases)
				t.Post("/{id}/test-cases", testingHandler.CreateTestCase)
				t.Post("/{id}/test-cases/{caseId}/execute", testingHandler.Execute)

				t.Get("/{id}/deployments", testingHandler.ListDeployments)
				t.Post("/{id}/deployments", testingHandler.CreateDeployment)
				t.Post("/{id}/deployments/{deploymentId}/validation", testingHandler.UpdateDeploymentValidation)
			})

			protected.Route("/projects", func(p chi.Router) {
				p.Get("/", projectsHandler.List)
				p.Post("/", projectsHandler.Create)
				p.Get("/{id}", projectsHandler.Get)
				p.Put("/{id}", projectsHandler.Update)
				p.Get("/{id}/stats", projectsHandler.Stats)
				p.Get("/{id}/milestones", projectsHandler.ListMilestones)
				p.Post("/{id}/milestones", projectsHandler.CreateMilestone)
				p.Put("/{id}/milestones/{milestoneId}", projectsHandler.UpdateMilestone)
			})

			protected.Route("/incidents", func(i chi.Router) {
				i.Get("/", incidentsHandler.List)
				i.Post("/", incidentsHandler.Create)
				i.Get("/{id}", incidentsHandler.Get)
				i.Post("/{id}/status", incidentsHandler.ChangeStatus)

				i.Get("/{id}/rca", rcaHandler.Get)
				i.Put("/{id}/rca", rcaHandler.Update)
				i.Post("/{id}/rca/status", rcaHandler.ChangeStatus)
			})

			protected.Get("/search", searchHandler.Search)

			protected.Route("/reports", func(rp chi.Router) {
				rp.Get("/summary", reportsHandler.Summary)
				rp.Get("/management", reportsHandler.Management)
				rp.Get("/export.csv", reportsHandler.ExportCSV)
			})

			protected.Route("/saved-views", func(sv chi.Router) {
				sv.Get("/", savedViewsHandler.List)
				sv.Post("/", savedViewsHandler.Create)
				sv.Delete("/{id}", savedViewsHandler.Delete)
			})

			protected.Post("/tickets/import", ticketsHandler.ImportCSV)

			protected.Route("/backups", func(b chi.Router) {
				b.With(appMiddleware.RequireRole(users.RoleAdmin)).Get("/", backupHandler.List)
				b.With(appMiddleware.RequireRole(users.RoleAdmin)).Post("/", backupHandler.Create)
				b.With(appMiddleware.RequireRole(users.RoleAdmin)).Post("/{id}/restore", backupHandler.Restore)
			})

			protected.Route("/settings", func(st chi.Router) {
				st.Get("/", settingsHandler.List)
				st.With(appMiddleware.RequireRole(users.RoleAdmin)).Put("/{key}", settingsHandler.Set)
				st.With(appMiddleware.RequireRole(users.RoleAdmin)).Delete("/{key}", settingsHandler.Delete)
			})

			protected.With(appMiddleware.RequireRole(users.RoleAdmin)).Get("/audit-logs", auditHandler.List)

			protected.Get("/sla-policies", slaHandler.ListPolicies)

			protected.Route("/dashboard", func(d chi.Router) {
				d.Get("/attention", dashboardHandler.Attention)
				d.Get("/my-work", dashboardHandler.MyWork)
				d.Get("/priority-queue", dashboardHandler.PriorityQueue)
			})

			protected.Route("/notifications", func(n chi.Router) {
				n.Get("/", notifHandler.List)
				n.Get("/unread-count", notifHandler.UnreadCount)
				n.Post("/{id}/read", notifHandler.MarkRead)
				n.Post("/read-all", notifHandler.MarkAllRead)
			})
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	sl.Info("server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		sl.Error("server stopped", "error", err)
		_ = slog.Default()
	}
}
