package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/diasoft/gateway-service/internal/infrastructure/excel"
	kafkainfra "github.com/diasoft/gateway-service/internal/infrastructure/kafka"
	"github.com/diasoft/gateway-service/internal/infrastructure/qr"
	"github.com/diasoft/gateway-service/internal/infrastructure/security"
	"github.com/diasoft/gateway-service/internal/infrastructure/token"
	"github.com/diasoft/gateway-service/internal/model"
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
	keyEncryptor, err := security.NewKeyEncryptor(s.config.SigningKeysMasterKey)
	if err != nil {
		return err
	}
	qrPayloadCodec, err := security.NewQRPayloadCodec(s.config.QRPayloadEncryptionSecret)
	if err != nil {
		return err
	}
	s.db.SetQRPayloadDecoder(qrPayloadCodec)
	recordPayloadCipher, err := security.NewKeyEncryptor(s.config.QRPayloadEncryptionSecret)
	if err != nil {
		return err
	}
	s.db.SetRecordPayloadCipher(recordPayloadCipher)
	tokenManager := token.NewManager(s.config.JWTSecret, s.config.ShareJWTSecret, s.config.AccessTokenTTL, s.config.ShareTokenTTL)
	qrGenerator := qr.NewGenerator()
	excelGenerator := excel.NewGenerator(qrGenerator, s.config.PublicBaseURL)
	kafkaWriter := kafkainfra.NewProducer(s.config.Kafka)

	authService := service.NewAuthService(s.db.University(), s.db.Admin(), hasher, tokenManager)
	adminService := service.NewAdminService(s.db.Admin(), s.db.University(), hasher)
	apiKeyService := service.NewAPIKeyService(s.db.APIKey())
	signingKeyService := service.NewSigningKeyService(s.db.University(), s.db.SigningKey(), keyEncryptor)
	diplomaService := service.NewDiplomaService(s.db.Diploma(), kafkaWriter, excelGenerator)
	studentService := service.NewStudentService(s.db.Diploma(), tokenManager, qrGenerator, s.config.PublicBaseURL, s.config.ShareTokenTTL)
	universityService := service.NewUniversityCabinetService(s.db.University(), s.db.Diploma())

	if err := adminService.EnsureBootstrapAdmin(context.Background(), s.config.BootstrapAdminEmail, s.config.BootstrapAdminPassword); err != nil {
		return err
	}
	s.logger.Info("bootstrap admin ensured", "email", s.config.BootstrapAdminEmail)
	if err := s.ensureDemoUniversity(context.Background(), hasher, signingKeyService); err != nil {
		return err
	}

	s.kafkaWriter = kafkaWriter
	s.kafkaReader = kafkainfra.NewResultConsumer(s.config.Kafka, diplomaService, s.logger)

	authHandler := handler.NewAuthHandler(authService, validator)
	adminHandler := handler.NewAdminHandler(adminService)
	apiKeyHandler := handler.NewAPIKeyHandler(apiKeyService, validator)
	signingKeyHandler := handler.NewSigningKeyHandler(signingKeyService, validator)
	diplomaHandler := handler.NewDiplomaHandler(diplomaService, validator)
	studentHandler := handler.NewStudentHandler(studentService, validator)
	universityHandler := handler.NewUniversityHandler(universityService)
	authMiddleware := httpmw.New(tokenManager, apiKeyService, s.db.University())

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

		r.Route("/student", func(r chi.Router) {
			r.Get("/search", studentHandler.Search())
			r.Post("/share", studentHandler.Share())
			r.Get("/qr", studentHandler.QR())
			r.Get("/share/{token}", studentHandler.SharedDiploma())
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(authMiddleware.JWT)
			r.Use(authMiddleware.Admin)
			r.Get("/universities", adminHandler.ListUniversities())
			r.Get("/universities/{id}", adminHandler.GetUniversity())
			r.Post("/universities/{id}/approve", adminHandler.ApproveUniversity())
			r.Patch("/universities/{id}", adminHandler.UpdateUniversityStatus())
			r.Delete("/universities/{id}", adminHandler.DeleteUniversity())
			r.Get("/stats", adminHandler.Stats())
		})

		r.Route("/vuz", func(r chi.Router) {
			r.Use(authMiddleware.JWT)
			r.Use(authMiddleware.University)
			r.Get("/profile", universityHandler.Profile())
			r.Get("/batches", universityHandler.ListBatches())
			r.Post("/api-keys", apiKeyHandler.Create())
			r.Get("/api-keys", apiKeyHandler.List())
			r.Put("/signing-key", signingKeyHandler.Upsert())
			r.Get("/signing-key", signingKeyHandler.Status())
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

func (s *APIServer) ensureDemoUniversity(ctx context.Context, hasher security.Hasher, signingKeyService *service.SigningKeyService) error {
	values := []string{
		s.config.DemoUniversityName,
		s.config.DemoUniversityVUZCode,
		s.config.DemoUniversityINN,
		s.config.DemoUniversityOGRN,
		s.config.DemoUniversityEmail,
		s.config.DemoUniversityPassword,
	}

	filled := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filled++
		}
	}

	if filled == 0 {
		return nil
	}
	if filled != len(values) {
		return errors.New("demo university configuration is incomplete")
	}

	passwordHash, err := hasher.Hash(s.config.DemoUniversityPassword)
	if err != nil {
		return err
	}

	university, err := s.db.University().UpsertDemo(ctx, &model.RegisterUniversityRequest{
		Name:     s.config.DemoUniversityName,
		VuzCode:  s.config.DemoUniversityVUZCode,
		INN:      s.config.DemoUniversityINN,
		OGRN:     s.config.DemoUniversityOGRN,
		Email:    s.config.DemoUniversityEmail,
		Password: s.config.DemoUniversityPassword,
	}, passwordHash, model.UniversityStatusActive)
	if err != nil {
		return fmt.Errorf("ensure demo university: %w", err)
	}

	privateKeyPath := strings.TrimSpace(s.config.DemoUniversityPrivateKey)
	if privateKeyPath == "" {
		s.logger.Info("demo university ensured", "email", university.Email, "vuz_code", university.VuzCode, "signing_key_configured", false)
		return nil
	}

	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("read demo university private key: %w", err)
	}

	if _, err := signingKeyService.Upsert(ctx, university.ID, &model.UpsertSigningKeyRequest{
		PrivateKeyPEM: string(privateKeyPEM),
	}); err != nil {
		return fmt.Errorf("configure demo university signing key: %w", err)
	}

	s.logger.Info("demo university ensured", "email", university.Email, "vuz_code", university.VuzCode, "signing_key_configured", true)
	return nil
}
