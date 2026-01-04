// Package insights contains the insights service adapter for use by other services.
package insights

import (
	"context"

	importservice "github.com/FACorreiaa/smart-finance-tracker/internal/domain/import/service"
)

// ServiceAdapter wraps the insights Service to implement the import service's InsightsService interface.
type ServiceAdapter struct {
	svc *Service
}

// NewServiceAdapter creates a new adapter for the insights service.
func NewServiceAdapter(svc *Service) *ServiceAdapter {
	return &ServiceAdapter{svc: svc}
}

// UpsertImportInsights adapts the import service's ImportInsights to the insights domain's ImportJobInsights.
func (a *ServiceAdapter) UpsertImportInsights(ctx context.Context, importInsights *importservice.ImportInsights) error {
	// Convert import service types to insight domain types
	var issues []ImportIssue
	for _, issue := range importInsights.Issues {
		issues = append(issues, ImportIssue{
			Type:         issue.Type,
			AffectedRows: issue.AffectedRows,
			SampleValue:  issue.SampleValue,
			Suggestion:   issue.Suggestion,
		})
	}

	insights := &ImportJobInsights{
		ImportJobID:        importInsights.ImportJobID,
		InstitutionName:    importInsights.InstitutionName,
		CategorizationRate: importInsights.CategorizationRate,
		DateQualityScore:   importInsights.DateQualityScore,
		AmountQualityScore: importInsights.AmountQualityScore,
		EarliestDate:       importInsights.EarliestDate,
		LatestDate:         importInsights.LatestDate,
		TotalIncome:        importInsights.TotalIncome,
		TotalExpenses:      importInsights.TotalExpenses,
		CurrencyCode:       importInsights.CurrencyCode,
		DuplicatesSkipped:  importInsights.DuplicatesSkipped,
		Issues:             issues,
	}

	return a.svc.UpsertImportInsights(ctx, insights)
}

// RefreshDataSourceHealth delegates to the underlying service.
func (a *ServiceAdapter) RefreshDataSourceHealth(ctx context.Context) error {
	return a.svc.RefreshDataSourceHealth(ctx)
}
