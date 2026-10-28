package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orenkod/backend/internal/athletes"
	"orenkod/backend/internal/auth"
	"orenkod/backend/internal/competitions"
	"orenkod/backend/internal/config"
	"orenkod/backend/internal/dashboard"
	"orenkod/backend/internal/database"
	"orenkod/backend/internal/emailcode"
	"orenkod/backend/internal/httpserver"
	"orenkod/backend/internal/integrations/extapi"
	"orenkod/backend/internal/integrations/telegram"
	"orenkod/backend/internal/notifications"
	"orenkod/backend/internal/oidc"
	"orenkod/backend/internal/platform"
	"orenkod/backend/internal/ratings"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer db.Close()

	competitionsRepo := competitions.NewRepository(db)
	competitionsService := competitions.NewService(competitionsRepo)
	athletesRepo := athletes.NewRepository(db)
	ratingsRepo := ratings.NewRepository(db)
	dashboardRepo := dashboard.NewRepository(db, competitionsRepo, ratingsRepo)
	telegramClient := telegram.NewClient(cfg)
	authService := auth.New(db)
	platformHandler := platform.New(db, authService, competitionsService, athletesRepo, telegramClient)

	competitionsHandler := competitions.NewHandler(competitionsService, telegramClient)
	athletesHandler := athletes.NewHandler(athletesRepo)
	ratingsHandler := ratings.NewHandler(ratingsRepo)
	dashboardHandler := dashboard.NewHandler(db, dashboardRepo)
	notificationsHandler := notifications.NewHandler()
	telegramHandler := telegram.NewBotHandler(telegramClient, competitionsService, ratingsRepo, db)
	go telegramHandler.Run(ctx)
	go platformHandler.RunReadinessReminders(ctx)
	platformHandler.Codes = emailcode.New(emailcode.Config{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom, BrevoKey: cfg.BrevoAPIKey})

	hooks := extapi.New(db)
	hooks.Recalc = platformHandler.RecalcPoints
	platformHandler.Hooks = hooks

	oidcProvider, err := oidc.New(ctx, db, os.Getenv("OIDC_ISSUER"))
	if err != nil {
		log.Fatalf("ФСП ID (OIDC): %v", err)
	}
	if cfg.BrevoAPIKey != "" {
		log.Printf("Почта: HTTP API Brevo настроен, отправитель %s", cfg.SMTPFrom)
	}
	if cfg.SMTPHost == "" && cfg.BrevoAPIKey == "" {
		log.Printf("Почта не настроена (нет SMTP_HOST и BREVO_API_KEY): коды входа печатаются только в лог")
	} else {
		log.Printf("SMTP настроен: %s:%s, отправитель %s", cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)
	}

	handler := httpserver.NewRouter(httpserver.Routes{
		OIDC:              oidcProvider,
		ExtAPI:            hooks,
		Extra:             platformHandler.MountExtras,
		Health:            dashboardHandler.Health,
		Dashboard:         dashboardHandler.Get,
		Competitions:      competitionsHandler.List,
		CreateCompetition: platformHandler.CreateCompetition,
		Athletes:          athletesHandler.List,
		Rating:            ratingsHandler.List,
		Team:              ratingsHandler.Team,
		Notifications:     notificationsHandler.List,
		TelegramStatus:    telegramHandler.Status,
		Auth:              authService,
		Platform: httpserver.PlatformRoutes{
			Login:              platformHandler.Login,
			Register:           platformHandler.Register,
			SendCode:           platformHandler.SendCode,
			Logout:             platformHandler.Logout,
			Me:                 platformHandler.Me,
			Switch:             platformHandler.SwitchRole,
			CreateCompetition:  platformHandler.CreateCompetition,
			ApproveAthlete:     platformHandler.ApproveAthlete,
			Catalogs:           platformHandler.Catalogs,
			PublicHome:         platformHandler.PublicHome,
			Verify:             platformHandler.VerifyCertificate,
			Publish:            platformHandler.PublishCompetition,
			Apply:              platformHandler.Apply,
			Applications:       platformHandler.Applications,
			DecideApp:          platformHandler.DecideApplication,
			RequestLink:        platformHandler.RequestLink,
			DecideLink:         platformHandler.DecideLink,
			Links:              platformHandler.Links,
			Judges:             platformHandler.Judges,
			AssignJudge:        platformHandler.AssignJudge,
			Participants:       platformHandler.Participants,
			Assignments:        platformHandler.Assignments,
			SaveResults:        platformHandler.SaveResults,
			Protocol:           platformHandler.Protocol,
			History:            platformHandler.ResultsHistory,
			RankApply:          platformHandler.RankApply,
			RankList:           platformHandler.RankList,
			RankDecide:         platformHandler.RankDecide,
			CatApply:           platformHandler.CategoryApply,
			CatList:            platformHandler.CategoryList,
			CatDecide:          platformHandler.CategoryDecide,
			TeamSave:           platformHandler.TeamSave,
			TeamOrder:          platformHandler.TeamOrder,
			Enrollment:         platformHandler.Enrollment,
			AthleteReport:      platformHandler.AthleteReport,
			RegistryXLSX:       platformHandler.RegistryXLSX,
			AddType:            platformHandler.AddType,
			Development:        platformHandler.Development,
			Audit:              platformHandler.Audit,
			Certificate:        platformHandler.IssueCertificate,
			TelegramLinkCode:   platformHandler.CreateTelegramLinkCode,
			TelegramLinkStatus: platformHandler.TelegramLinkStatus,
			TelegramUnlink:     platformHandler.UnlinkTelegram,
			Inbox:              platformHandler.Inbox,
			Rating:             platformHandler.Rating,
			TeamGet:            platformHandler.TeamGet,
		},
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      45 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("ОренКод API слушает :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
}
