// Package cron provides scheduled background jobs using robfig/cron.
package cron

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	planrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	planservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/service"
)

// Scheduler manages background scheduled jobs using robfig/cron.
type Scheduler struct {
	cron        *cron.Cron
	planRepo    planrepo.PlanRepository
	planService *planservice.PlanService
	logger      *slog.Logger
}

// NewScheduler creates a new job scheduler.
func NewScheduler(planRepo planrepo.PlanRepository, planService *planservice.PlanService, logger *slog.Logger) *Scheduler {
	// Create cron with seconds disabled (standard 5-field format)
	c := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(slog.NewLogLogger(logger.Handler(), slog.LevelDebug))))

	return &Scheduler{
		cron:        c,
		planRepo:    planRepo,
		planService: planService,
		logger:      logger,
	}
}

// Start begins scheduled jobs.
func (s *Scheduler) Start() error {
	// Plan actuals sync: runs daily at 2:00 AM
	_, err := s.cron.AddFunc("0 2 * * *", s.syncAllActivePlans)
	if err != nil {
		return err
	}

	s.cron.Start()
	s.logger.Info("cron scheduler started",
		slog.Int("jobs", len(s.cron.Entries())),
	)
	return nil
}

// Stop gracefully stops all scheduled jobs.
func (s *Scheduler) Stop() context.Context {
	s.logger.Info("cron scheduler stopping")
	return s.cron.Stop()
}

// RunNow manually triggers the plan actuals sync (for testing/admin).
func (s *Scheduler) RunNow() {
	go s.syncAllActivePlans()
}

// syncAllActivePlans syncs actuals for all active plans.
func (s *Scheduler) syncAllActivePlans() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s.logger.Info("starting daily plan actuals sync")

	// Get all active plans across all users
	plans, err := s.planRepo.ListAllActivePlans(ctx, 1000, 0)
	if err != nil {
		s.logger.Error("failed to list active plans", slog.Any("error", err))
		return
	}

	// Calculate period: current month
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	synced := 0
	failed := 0

	for _, plan := range plans {
		input := &planservice.ComputePlanActualsInput{
			StartDate: startOfMonth,
			EndDate:   endOfMonth,
			Persist:   true,
		}

		result, err := s.planService.ComputePlanActuals(ctx, plan.UserID, plan.ID, input)
		if err != nil {
			s.logger.Warn("failed to sync plan actuals",
				slog.String("plan_id", plan.ID.String()),
				slog.String("user_id", plan.UserID.String()),
				slog.Any("error", err),
			)
			failed++
			continue
		}

		s.logger.Debug("synced plan actuals",
			slog.String("plan_id", plan.ID.String()),
			slog.Int("items_updated", result.ItemsUpdated),
			slog.Int("transactions_matched", result.TransactionsMatched),
		)
		synced++
	}

	s.logger.Info("daily plan actuals sync completed",
		slog.Int("plans_synced", synced),
		slog.Int("plans_failed", failed),
	)
}
