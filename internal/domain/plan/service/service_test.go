package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	importrepo "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/repository"
	"github.com/FACorreiaa/smart-finance-tracker/internal/domain/plan/repository"
	"github.com/google/uuid"
)

// Helpers for pointers
func ptrStr(s string) *string { return &s }
func ptrInt64(i int64) *int64 { return &i }

// fakePlanRepository implements repository.PlanRepository for testing
type fakePlanRepository struct{}

func (f *fakePlanRepository) UpdatePlanStructure(ctx context.Context, planID uuid.UUID, groups []*repository.PlanCategoryGroup, categories []*repository.PlanCategory, items []*repository.PlanItem) error {
	return nil
}

func (f *fakePlanRepository) CreatePlan(ctx context.Context, plan *repository.UserPlan) error {
	return nil
}
func (f *fakePlanRepository) CreatePlanWithStructure(ctx context.Context, plan *repository.UserPlan, groups []*repository.PlanCategoryGroup, categories []*repository.PlanCategory, items []*repository.PlanItem) error {
	return nil
}
func (f *fakePlanRepository) GetPlans(ctx context.Context, userID uuid.UUID) ([]*repository.UserPlan, error) {
	return nil, nil
}
func (f *fakePlanRepository) ListPlansByUser(ctx context.Context, userID uuid.UUID, status *repository.PlanStatus, limit, offset int) ([]*repository.UserPlan, int, error) {
	return nil, 0, nil
}
func (f *fakePlanRepository) ListAllActivePlans(ctx context.Context, limit, offset int) ([]*repository.UserPlan, error) {
	return nil, nil
}
func (f *fakePlanRepository) GetPlanByID(ctx context.Context, planID uuid.UUID) (*repository.UserPlan, error) {
	return &repository.UserPlan{ID: planID, UserID: uuid.MustParse("92131338-3069-42b7-84bc-8c3866be237a")}, nil
}
func (f *fakePlanRepository) UpdatePlan(ctx context.Context, plan *repository.UserPlan) error {
	return nil
}
func (f *fakePlanRepository) DeletePlan(ctx context.Context, planID uuid.UUID) error {
	return nil
}
func (f *fakePlanRepository) SetActivePlan(ctx context.Context, userID, planID uuid.UUID) error {
	return nil
}
func (f *fakePlanRepository) GetActivePlan(ctx context.Context, userID uuid.UUID) (*repository.UserPlan, error) {
	return &repository.UserPlan{
		ID:     uuid.New(),
		UserID: userID,
		Status: repository.PlanStatusActive,
		Name:   "Mock Active Plan",
	}, nil
}
func (f *fakePlanRepository) IncrementPlanItemActual(ctx context.Context, itemID uuid.UUID, amountMinor int64) error {
	return nil
}

// Category Groups
func (f *fakePlanRepository) CreateCategoryGroup(ctx context.Context, group *repository.PlanCategoryGroup) error {
	return nil
}
func (f *fakePlanRepository) GetCategoryGroupsByPlan(ctx context.Context, planID uuid.UUID) ([]*repository.PlanCategoryGroup, error) {
	return nil, nil
}

// Categories
func (f *fakePlanRepository) CreateCategory(ctx context.Context, category *repository.PlanCategory) error {
	return nil
}
func (f *fakePlanRepository) GetCategoriesByPlan(ctx context.Context, planID uuid.UUID) ([]*repository.PlanCategory, error) {
	return nil, nil
}
func (f *fakePlanRepository) GetCategoriesByGroup(ctx context.Context, groupID uuid.UUID) ([]*repository.PlanCategory, error) {
	return nil, nil
}

// Items
func (f *fakePlanRepository) CreateItem(ctx context.Context, item *repository.PlanItem) error {
	return nil
}
func (f *fakePlanRepository) GetItemsByPlan(ctx context.Context, planID uuid.UUID) ([]*repository.PlanItem, error) {
	return nil, nil
}
func (f *fakePlanRepository) GetItemsByCategory(ctx context.Context, categoryID uuid.UUID) ([]*repository.PlanItem, error) {
	return nil, nil
}
func (f *fakePlanRepository) UpdateItem(ctx context.Context, item *repository.PlanItem) error {
	return nil
}
func (f *fakePlanRepository) UpdateItemBudget(ctx context.Context, itemID uuid.UUID, budgetedMinor int64) error {
	return nil
}
func (f *fakePlanRepository) UpdatePlanItemActual(ctx context.Context, itemID uuid.UUID, actualMinor int64) error {
	return nil
}

// Bulk
func (f *fakePlanRepository) DuplicatePlan(ctx context.Context, sourcePlanID uuid.UUID, newName string, userID uuid.UUID) (*repository.UserPlan, error) {
	return nil, nil
}

// Item Configs
func (f *fakePlanRepository) ListItemConfigs(ctx context.Context, userID uuid.UUID) ([]*repository.ItemConfig, error) {
	return nil, nil
}
func (f *fakePlanRepository) GetItemConfigByID(ctx context.Context, configID uuid.UUID) (*repository.ItemConfig, error) {
	return nil, nil
}
func (f *fakePlanRepository) CreateItemConfig(ctx context.Context, config *repository.ItemConfig) error {
	return nil
}
func (f *fakePlanRepository) UpdateItemConfig(ctx context.Context, config *repository.ItemConfig) error {
	return nil
}
func (f *fakePlanRepository) DeleteItemConfig(ctx context.Context, configID uuid.UUID) error {
	return nil
}

// Filtered
func (f *fakePlanRepository) GetItemsByTabWithTotals(ctx context.Context, planID uuid.UUID, tab repository.TargetTab) ([]repository.PlanItemWithConfig, int64, int64, error) {
	return nil, 0, 0, nil
}

// fakeImportRepository
type fakeImportRepository struct{}

func (f *fakeImportRepository) GetMappingByFingerprint(ctx context.Context, fingerprint string, userID *uuid.UUID) (*importrepo.BankMapping, error) {
	return nil, nil
}
func (f *fakeImportRepository) CreateMapping(ctx context.Context, mapping *importrepo.BankMapping) error {
	return nil
}
func (f *fakeImportRepository) UpdateMapping(ctx context.Context, mapping *importrepo.BankMapping) error {
	return nil
}
func (f *fakeImportRepository) ListUserMappings(ctx context.Context, userID uuid.UUID) ([]*importrepo.BankMapping, error) {
	return nil, nil
}
func (f *fakeImportRepository) GetAccountCurrency(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) (string, error) {
	return "EUR", nil
}
func (f *fakeImportRepository) CreateUserFile(ctx context.Context, file *importrepo.UserFile) error {
	return nil
}
func (f *fakeImportRepository) GetUserFileByID(ctx context.Context, id uuid.UUID) (*importrepo.UserFile, error) {
	return nil, nil
}
func (f *fakeImportRepository) CreateImportJob(ctx context.Context, job *importrepo.ImportJob) error {
	return nil
}
func (f *fakeImportRepository) GetImportJobByID(ctx context.Context, id uuid.UUID) (*importrepo.ImportJob, error) {
	return nil, nil
}
func (f *fakeImportRepository) GetImportJobStats(ctx context.Context, importJobID uuid.UUID) (*importrepo.ImportJobStats, error) {
	return nil, nil
}
func (f *fakeImportRepository) UpdateImportJobProgress(ctx context.Context, id uuid.UUID, rowsImported, rowsFailed int) error {
	return nil
}
func (f *fakeImportRepository) UpdateImportJobStatus(ctx context.Context, id uuid.UUID, status string, errorMessage *string) error {
	return nil
}
func (f *fakeImportRepository) FinishImportJob(ctx context.Context, id uuid.UUID, status string, rowsImported, rowsFailed int, errorMessage *string) error {
	return nil
}
func (f *fakeImportRepository) BulkInsertTransactions(ctx context.Context, userID uuid.UUID, accountID *uuid.UUID, currencyCode string, importJobID uuid.UUID, institutionName string, txs []*importrepo.ParsedTransaction) (int, error) {
	return 0, nil
}
func (f *fakeImportRepository) InsertTransaction(ctx context.Context, tx *importrepo.Transaction) error {
	return nil
}
func (f *fakeImportRepository) ListTransactions(ctx context.Context, userID uuid.UUID, filter importrepo.ListTransactionsFilter) ([]*importrepo.Transaction, int64, error) {
	return nil, 0, nil
}
func (f *fakeImportRepository) DeleteByImportJobID(ctx context.Context, userID uuid.UUID, importJobID uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeImportRepository) GetCategoryTotals(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) ([]importrepo.CategoryTotal, error) {
	return nil, nil
}

func TestUpdatePlanStructure_Service(t *testing.T) {
	repo := &fakePlanRepository{}
	importRepo := &fakeImportRepository{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := NewPlanService(repo, importRepo, logger)

	ctx := context.Background()
	userID := uuid.MustParse("92131338-3069-42b7-84bc-8c3866be237a")
	planID := uuid.New()

	allowedGroups := []CreateCategoryGroupInput{
		{
			Name:          "New Group",
			TargetPercent: 50,
			Categories: []CreateCategoryInput{
				{
					Name: "New Category",
					Items: []CreateItemInput{
						{
							Name:               "New Item",
							BudgetedMinor:      1000,
							ItemType:           repository.ItemTypeBudget,
							ConfigID:           ptrStr("1"),
							InitialActualMinor: ptrInt64(0),
						},
						{
							Name:               "Goal Item",
							BudgetedMinor:      5000,
							ItemType:           repository.ItemTypeGoal,
							ConfigID:           ptrStr("3"),
							InitialActualMinor: ptrInt64(1000), // Pre-saved amount
						},
					},
				},
			},
		},
	}

	_, err := svc.UpdatePlanStructure(ctx, userID, planID, allowedGroups)
	if err != nil {
		t.Fatalf("UpdatePlanStructure failed: %v", err)
	}
}

func TestCreatePlan_Service_WithInitialActual(t *testing.T) {
	repo := &fakePlanRepository{}
	importRepo := &fakeImportRepository{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := NewPlanService(repo, importRepo, logger)

	ctx := context.Background()
	userID := uuid.New()

	input := CreatePlanInput{
		Name:         "Test Plan",
		Description:  ptrStr("Test Description"),
		CurrencyCode: "EUR",
		CategoryGroups: []CreateCategoryGroupInput{
			{
				Name:          "Savings",
				TargetPercent: 10,
				Categories: []CreateCategoryInput{
					{
						Name: "Savings",
						Items: []CreateItemInput{
							{
								Name:               "Goal",
								BudgetedMinor:      1000,
								ItemType:           repository.ItemTypeGoal,
								ConfigID:           ptrStr("3"),
								InitialActualMinor: ptrInt64(200),
							},
						},
					},
				},
			},
		},
	}

	_, err := svc.CreatePlan(ctx, userID, &input)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}
}
