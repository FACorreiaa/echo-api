package api

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/user"
	userhandler "github.com/FACorreiaa/smart-finance-tracker/internal/domain/user/handler"

	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/auth/handler"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/auth/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/auth/service"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/balance"
	balancehandler "github.com/FACorreiaa/smart-finance-tracker/internal/domain/balance/handler"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/categorization"
	financehandler "github.com/FACorreiaa/smart-finance-tracker/internal/domain/finance/handler"
	importhandler "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/handler"
	importrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/repository"
	importservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/insights"
	insightshandler "github.com/FACorreiaa/smart-finance-tracker/internal/domain/insights/handler"

	"github.com/FACorreiaa/smart-finance-tracker/pkg/config"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/db"
	"github.com/FACorreiaa/smart-finance-tracker/pkg/push"
)

// Dependencies holds all application dependencies
type Dependencies struct {
	Config *config.Config
	DB     *db.DB
	Logger *slog.Logger

	// Repositories
	AuthRepo           repository.AuthRepository
	UserRepo           user.UserRepo
	ImportRepo         importrepo.ImportRepository
	CategorizationRepo *categorization.Repository
	InsightsRepo       *insights.Repository
	BalanceRepo        *balance.Repository

	// Services
	TokenManager          service.TokenManager
	AuthService           *service.AuthService
	UserSvc               user.UserService
	ImportService         *importservice.ImportService
	CategorizationService *categorization.Service
	InsightsService       *insights.Service
	PushService           *push.Service
	BalanceService        *balance.Service

	// Handlers
	AuthHandler     *handler.AuthHandler
	UserHandler     *userhandler.UserHandler
	FinanceHandler  *financehandler.FinanceHandler
	ImportHandler   *importhandler.ImportHandler
	InsightsHandler *insightshandler.InsightsHandler
	BalanceHandler  *balancehandler.BalanceHandler
}

// InitDependencies initializes all application dependencies
func InitDependencies(cfg *config.Config, logger *slog.Logger) (*Dependencies, error) {
	deps := &Dependencies{
		Config: cfg,
		Logger: logger,
	}

	// Initialize database
	if err := deps.initDatabase(); err != nil {
		return nil, fmt.Errorf("failed to init database: %w", err)
	}

	// Initialize repositories
	if err := deps.initRepositories(); err != nil {
		return nil, fmt.Errorf("failed to init repositories: %w", err)
	}

	// Initialize handler
	if err := deps.initServices(); err != nil {
		return nil, fmt.Errorf("failed to init services: %w", err)
	}

	// Initialize service
	if err := deps.initHandlers(); err != nil {
		return nil, fmt.Errorf("failed to init handlers: %w", err)
	}

	logger.Info("all dependencies initialized successfully")

	return deps, nil
}

// initDatabase initializes the database connection and runs migrations
func (d *Dependencies) initDatabase() error {
	database, err := db.New(db.Config{
		DSN:             d.Config.Database.DSN(),
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: 5 * time.Minute,
		MaxConnIdleTime: 10 * time.Minute,
	}, d.Logger)
	if err != nil {
		return err
	}

	d.DB = database

	// Run migrations
	if err := d.DB.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	d.Logger.Info("database connected and migrations completed successfully")
	return nil
}

// initRepositories initializes all repository layer dependencies
func (d *Dependencies) initRepositories() error {
	d.AuthRepo = repository.NewPostgresAuthRepository(d.DB.Pool)
	d.ImportRepo = importrepo.NewPostgresImportRepository(d.DB.Pool)
	d.CategorizationRepo = categorization.NewRepository(d.DB.Pool)
	d.InsightsRepo = insights.NewRepository(d.DB.Pool)
	d.BalanceRepo = balance.NewRepository(d.DB.Pool)

	d.Logger.Info("repositories initialized")
	return nil
}

// initServices initializes all service layer dependencies
func (d *Dependencies) initServices() error {
	jwtSecret := []byte(d.Config.Auth.JWTSecret)
	if len(jwtSecret) == 0 {
		return fmt.Errorf("jwt secret is required")
	}

	accessTokenTTL := 1 * time.Hour // Increased from 15 minutes for better UX
	refreshTokenTTL := 30 * 24 * time.Hour

	d.TokenManager = service.NewTokenManager(jwtSecret, jwtSecret, accessTokenTTL, refreshTokenTTL)
	emailService := service.NewEmailService()
	d.AuthService = service.NewAuthService(
		d.AuthRepo,
		d.TokenManager,
		emailService,
		d.Logger,
		refreshTokenTTL,
	)

	d.UserSvc = user.NewUserService(d.UserRepo, d.Logger)

	// Categorization service for transaction enrichment
	d.CategorizationService = categorization.NewService(d.CategorizationRepo)

	// Import service with categorization wired in
	d.ImportService = importservice.NewImportService(d.ImportRepo, d.Logger)
	d.ImportService.WithCategorizationService(newCategorizationAdapter(d.CategorizationService))

	// Push notification service
	d.PushService = push.NewService(d.Logger)

	// Insights service for spending pulse and dashboard (with push notifications)
	d.InsightsService = insights.NewService(d.InsightsRepo, d.PushService, d.AuthRepo, d.Logger)

	// Balance service for computing user balances
	d.BalanceService = balance.NewService(d.BalanceRepo)

	d.Logger.Info("services initialized")
	return nil
}

// initHandlers initializes all handler dependencies
func (d *Dependencies) initHandlers() error {
	d.AuthHandler = handler.NewAuthHandler(d.AuthService)
	d.FinanceHandler = financehandler.NewFinanceHandler(d.ImportService, d.ImportRepo, d.CategorizationService)
	d.ImportHandler = importhandler.NewImportHandler(d.ImportService, d.Logger)
	d.InsightsHandler = insightshandler.NewInsightsHandler(d.InsightsService)
	d.BalanceHandler = balancehandler.NewBalanceHandler(d.BalanceService)

	d.Logger.Info("handlers initialized")
	return nil
}

// Cleanup closes all resources
func (d *Dependencies) Cleanup() {
	if d.DB != nil {
		d.DB.Close()
	}
	d.Logger.Info("cleanup completed")
}
