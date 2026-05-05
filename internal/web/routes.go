package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"kong-cert-lite/internal/usecase"
)

//go:embed static/css/*
//go:embed static/icons/*
//go:embed static/js/*
//go:embed static/tabler/*
var staticFiles embed.FS

func NewApp(logger *slog.Logger, certificates *usecase.CertificateUseCase, acme *usecase.ACMEUseCase, kongSync *usecase.KongSyncUseCase, kongTargets *usecase.KongTargetUseCase, jobs *usecase.JobUseCase, authConfig ...BasicAuthConfig) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	handler := NewHandler(logger, certificates, acme, kongSync, kongTargets, jobs)
	auth := BasicAuthConfig{}
	if len(authConfig) > 0 {
		auth = authConfig[0]
	}

	app.Get("/healthz", handler.Healthz)
	app.Get("/login", LoginPageHandler(auth))
	app.Post("/login", LoginPostHandler(auth))
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(staticFS()),
	}))
	app.Use(NewSessionAuthMiddleware(auth))
	app.Get("/", handler.Home)
	app.Post("/logout", LogoutHandler())
	app.Get("/certificates", handler.Certificates)
	app.Get("/certificates/new", handler.NewCertificate)
	app.Post("/certificates", handler.CreateCertificate)
	app.Get("/certificates/:id/edit", handler.EditCertificate)
	app.Get("/certificates/:id", handler.CertificateDetail)
	app.Post("/certificates/:id", handler.UpdateCertificate)
	app.Post("/certificates/:id/issue", handler.IssueCertificate)
	app.Post("/certificates/:id/renew", handler.RenewCertificate)
	app.Post("/certificates/:id/delete", handler.DeleteCertificate)
	app.Post("/certificates/:id/targets", handler.UpdateCertificateTargets)
	app.Post("/certificates/:id/sync", handler.SyncCertificate)
	app.Post("/certificates/:id/targets/:target_id/sync", handler.SyncCertificateTarget)
	app.Get("/kong-targets", handler.KongTargets)
	app.Get("/kong-targets/new", handler.NewKongTarget)
	app.Post("/kong-targets", handler.CreateKongTarget)
	app.Get("/kong-targets/:id/edit", handler.EditKongTarget)
	app.Post("/kong-targets/:id", handler.UpdateKongTarget)
	app.Post("/kong-targets/:id/delete", handler.DeleteKongTarget)
	app.Post("/kong-targets/:id/test", handler.TestKongTarget)
	app.Get("/jobs", handler.Jobs)
	app.Post("/jobs/clear", handler.ClearJobs)
	app.Get("/jobs/:id", handler.JobDetail)

	return app
}

func staticFS() fs.FS {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	return staticFS
}
