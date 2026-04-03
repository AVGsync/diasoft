package apiserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/diasoft/gateway-service/internal/infrastructure/excel"
	kafkainfra "github.com/diasoft/gateway-service/internal/infrastructure/kafka"
	"github.com/diasoft/gateway-service/internal/infrastructure/qr"
	"github.com/diasoft/gateway-service/internal/infrastructure/security"
	"github.com/diasoft/gateway-service/internal/infrastructure/token"
	"github.com/diasoft/gateway-service/internal/repository/postgres"
	"github.com/diasoft/gateway-service/internal/service"
	"github.com/diasoft/gateway-service/internal/transport/http/handler"
	httpmw "github.com/diasoft/gateway-service/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

type APIServer struct {
	config      *Config
	logger      *slog.Logger
	router      *chi.Mux
	db          *postgres.DB
	httpServer  *http.Server
	kafkaWriter *kafkainfra.Producer
	kafkaReader *kafkainfra.ResultConsumer
}

func New(config *Config) *APIServer {
	return &APIServer{
		config: config,
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
		router: chi.NewRouter(),
	}
}

func (s *APIServer) Start() error {
	if err := s.configureLogger(); err != nil {
		return err
	}
	if err := s.configureDB(); err != nil {
		return err
	}
	if err := s.configureRouter(); err != nil {
		return err
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if s.kafkaReader != nil {
		go s.kafkaReader.Start(rootCtx)
	}

	s.httpServer = &http.Server{
		Addr:    s.config.BindAddr,
		Handler: s.router,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-rootCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	if s.kafkaReader != nil {
		_ = s.kafkaReader.Close()
	}
	if s.kafkaWriter != nil {
		_ = s.kafkaWriter.Close()
	}

	return s.db.Close()
}

func (s *APIServer) configureLogger() error {
	level := slog.LevelInfo
	if s.config.LogLevel == "debug" {
		level = slog.LevelDebug
	}

	s.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(s.logger)
	return nil
}

func (s *APIServer) configureDB() error {
	database := postgres.New(s.config.DB)
	if err := database.Open(); err != nil {
		return err
	}

	s.db = database
	return nil
}

func (s *APIServer) configureRouter() error {
	validator := security.NewValidator()
	hasher := security.NewBcryptHasher()
	tokenManager := token.NewManager(s.config.JWTSecret, s.config.ShareJWTSecret, s.config.AccessTokenTTL, s.config.ShareTokenTTL)
	qrGenerator := qr.NewGenerator()
	excelGenerator := excel.NewGenerator(qrGenerator, s.config.PublicBaseURL)
	kafkaWriter := kafkainfra.NewProducer(s.config.Kafka)

	authService := service.NewAuthService(s.db.University(), s.db.Admin(), hasher, tokenManager)
	adminService := service.NewAdminService(s.db.Admin(), s.db.University(), hasher)
	apiKeyService := service.NewAPIKeyService(s.db.APIKey())
	diplomaService := service.NewDiplomaService(s.db.Diploma(), kafkaWriter, excelGenerator)
	studentService := service.NewStudentService(s.db.Diploma(), tokenManager, qrGenerator, s.config.PublicBaseURL, s.config.ShareTokenTTL)
	verifyService := service.NewVerifyService(s.db.Diploma())

	if err := adminService.EnsureBootstrapAdmin(context.Background(), s.config.BootstrapAdminEmail, s.config.BootstrapAdminPassword); err != nil {
		return err
	}

	s.kafkaWriter = kafkaWriter
	s.kafkaReader = kafkainfra.NewResultConsumer(s.config.Kafka, diplomaService, s.logger)

	authHandler := handler.NewAuthHandler(authService, validator)
	adminHandler := handler.NewAdminHandler(adminService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService, validator)
	diplomaHandler := handler.NewDiplomaHandler(diplomaService, validator)
	studentHandler := handler.NewStudentHandler(studentService, validator)
	verifyHandler := handler.NewVerifyHandler(verifyService)
	authMiddleware := httpmw.New(tokenManager, apiKeyService)

	s.router.Use(chimw.RequestID)
	s.router.Use(chimw.RealIP)
	s.router.Use(chimw.Logger)
	s.router.Use(chimw.Recoverer)

	s.router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", authHandler.Register())
		r.Post("/auth/login", authHandler.Login())
		r.Get("/verify", verifyHandler.Verify())

		r.Route("/student", func(r chi.Router) {
			r.Get("/search", studentHandler.Search())
			r.Post("/share", studentHandler.Share())
			r.Get("/qr", studentHandler.QR())
			r.Get("/share/{token}", studentHandler.SharedDiploma())
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(authMiddleware.JWT)
			r.Use(authMiddleware.Admin)
			r.Post("/universities/{id}/approve", adminHandler.ApproveUniversity())
			r.Get("/stats", adminHandler.Stats())
		})

		r.Route("/vuz", func(r chi.Router) {
			r.Use(authMiddleware.JWT)
			r.Use(authMiddleware.University)
			r.Post("/api-keys", apiKeyHandler.Create())
			r.Get("/api-keys", apiKeyHandler.List())
		})

		r.Route("/diplomas", func(r chi.Router) {
			r.Use(authMiddleware.UniversityOrAPIKey)
			r.Post("/upload", diplomaHandler.Upload())
			r.Get("/batches/{batch_id}", diplomaHandler.BatchStatus())
			r.Get("/batches/{batch_id}/download", diplomaHandler.Download())
			r.Patch("/{diploma_hash}/revoke", diplomaHandler.Revoke())
		})
	})

	return nil
}
