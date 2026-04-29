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
//go:embed static/js/*
//go:embed static/tabler/*
var staticFiles embed.FS

func NewApp(logger *slog.Logger, certificates *usecase.CertificateUseCase, acme *usecase.ACMEUseCase, kongSync *usecase.KongSyncUseCase, kongTargets *usecase.KongTargetUseCase, jobs *usecase.JobUseCase) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	handler := NewHandler(logger, certificates, acme, kongSync, kongTargets, jobs)

	app.Get("/healthz", handler.Healthz)
	app.Get("/", handler.Home)
	app.Get("/certificates", handler.Certificates)
	app.Get("/certificates/new", handler.NewCertificate)
	app.Post("/certificates", handler.CreateCertificate)
	app.Get("/certificates/:id/edit", handler.EditCertificate)
	app.Get("/certificates/:id", handler.CertificateDetail)
	app.Post("/certificates/:id", handler.UpdateCertificate)
	app.Post("/certificates/:id/issue", handler.IssueCertificate)
	app.Post("/certificates/:id/delete", handler.DeleteCertificate)
	app.Post("/certificates/:id/targets", handler.UpdateCertificateTargets)
	app.Post("/certificates/:id/sync", handler.SyncCertificate)
	app.Get("/kong-targets", handler.KongTargets)
	app.Get("/kong-targets/new", handler.NewKongTarget)
	app.Post("/kong-targets", handler.CreateKongTarget)
	app.Get("/kong-targets/:id/edit", handler.EditKongTarget)
	app.Post("/kong-targets/:id", handler.UpdateKongTarget)
	app.Post("/kong-targets/:id/delete", handler.DeleteKongTarget)
	app.Post("/kong-targets/:id/test", handler.TestKongTarget)
	app.Get("/jobs", handler.Jobs)
	app.Get("/jobs/:id", handler.JobDetail)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(staticFS),
	}))

	return app
}
