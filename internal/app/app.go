package app

import (
	"database/sql"
	"log/slog"

	acmeadapter "kong-cert-lite/internal/adapter/acme"
	kongadapter "kong-cert-lite/internal/adapter/kong"
	sqliteadapter "kong-cert-lite/internal/adapter/sqlite"
	"kong-cert-lite/internal/db"
	"kong-cert-lite/internal/usecase"
	"kong-cert-lite/internal/web"

	"github.com/gofiber/fiber/v2"
)

type App struct {
	cfg          Config
	db           *sql.DB
	certificates *usecase.CertificateUseCase
	acme         *usecase.ACMEUseCase
	kongSync     *usecase.KongSyncUseCase
	kongTargets  *usecase.KongTargetUseCase
	jobs         *usecase.JobUseCase
	logger       *slog.Logger
}

func New(cfg Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	certificateRepository := sqliteadapter.NewCertificateRepository(database)
	kongTargetRepository := sqliteadapter.NewKongTargetRepository(database)
	jobRepository := sqliteadapter.NewJobRepository(database)
	jobUseCase := usecase.NewJobUseCase(jobRepository)
	acmeClient := acmeadapter.NewLegoClient(cfg.AccountDir, cfg.LetsEncryptEnv, cfg.CloudflareToken)
	acmeUseCase := usecase.NewACMEUseCase(certificateRepository, jobUseCase, acmeClient, cfg.CertDir)
	kongAdminClient := kongadapter.NewAdminClient(nil)
	kongSyncUseCase := usecase.NewKongSyncUseCase(certificateRepository, kongTargetRepository, jobUseCase, kongAdminClient)

	return &App{
		cfg:          cfg,
		db:           database,
		certificates: usecase.NewCertificateUseCase(certificateRepository),
		acme:         acmeUseCase,
		kongSync:     kongSyncUseCase,
		kongTargets:  usecase.NewKongTargetUseCase(kongTargetRepository, kongAdminClient, jobUseCase),
		jobs:         jobUseCase,
		logger:       logger,
	}, nil
}

func (a *App) HTTPApp() *fiber.App {
	return web.NewApp(a.logger, a.certificates, a.acme, a.kongSync, a.kongTargets, a.jobs)
}

func (a *App) Close() error {
	if a.db == nil {
		return nil
	}

	return a.db.Close()
}
