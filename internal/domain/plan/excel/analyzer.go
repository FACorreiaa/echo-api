// Package excel provides structural analysis for Excel imports.
package excel

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ============================================================================
// Column Profile System
// ============================================================================

// ColumnProfile captures statistical features of a column for ML classification
type ColumnProfile struct {
	Index          int     `json:"index"`
	Letter         string  `json:"letter"`
	NumericDensity float64 `json:"numeric_density"` // numericCount / totalRows
	FormulaDensity float64 `json:"formula_density"` // formulaCount / totalRows
	EmptyDensity   float64 `json:"empty_density"`   // emptyCount / totalRows
	TextDensity    float64 `json:"text_density"`    // textCount / totalRows
	UniqueRatio    float64 `json:"unique_ratio"`    // uniqueValues / totalRows
	AvgTextLength  float64 `json:"avg_text_length"`
}

// RowFeatures for structural ML classification
type RowFeatures struct {
	HasValue       bool    `json:"has_value"`       // Value column has data
	IsBold         bool    `json:"is_bold"`         // Category cell has bold styling
	IsUpperCase    bool    `json:"is_uppercase"`    // Category text is ALL CAPS
	Indentation    int     `json:"indentation"`     // Leading spaces
	EmptyToRight   int     `json:"empty_to_right"`  // Empty cells after value column
	RowPosition    float64 `json:"row_position"`    // Normalized position (0-1)
	HasFormula     bool    `json:"has_formula"`     // Value cell contains formula
	ValueMagnitude float64 `json:"value_magnitude"` // Log scale of value for comparison
}

// ============================================================================
// Analysis Tree (ML Response)
// ============================================================================

// NodeType represents the structural role of a row
type NodeType string

const (
	NodeTypeGroup  NodeType = "GROUP"  // Category header
	NodeTypeItem   NodeType = "ITEM"   // Budget line item
	NodeTypeIgnore NodeType = "IGNORE" // Skip this row
)

// ItemTag represents the semantic classification of an item
type ItemTag string

const (
	TagBudget    ItemTag = "B"  // Budget expense
	TagRecurring ItemTag = "R"  // Recurring expense
	TagSavings   ItemTag = "S"  // Savings goal
	TagIncome    ItemTag = "IN" // Income source
	TagDebt      ItemTag = "D"  // Debt payment
	TagUnknown   ItemTag = ""   // Unknown/unclassified
)

// ConfidenceThreshold is the "Sovereign Certainty" bar.
// Items with confidence >= this threshold are auto-approved.
// Items below this threshold require user review.
const ConfidenceThreshold = 0.80

// AnalysisNode represents a node in the hierarchical analysis tree
type AnalysisNode struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Value          float64        `json:"value"`
	Type           NodeType       `json:"type"`           // GROUP, ITEM, IGNORE
	Tag            ItemTag        `json:"tag"`            // B, R, S, IN
	Confidence     float64        `json:"confidence"`     // 0.0 - 1.0
	NeedsReview    bool           `json:"needsReview"`    // confidence < 0.80
	IsAutoApproved bool           `json:"isAutoApproved"` // confidence >= 0.80
	ExcelCell      string         `json:"excelCell"`
	ExcelRow       int            `json:"excelRow"`
	Formula        string         `json:"formula,omitempty"`
	Children       []AnalysisNode `json:"children,omitempty"`
}

// AnalysisTreeResponse is the API response for tree analysis
type AnalysisTreeResponse struct {
	SheetName          string          `json:"sheetName"`
	Nodes              []AnalysisNode  `json:"nodes"`
	ColumnProfiles     []ColumnProfile `json:"columnProfiles,omitempty"`
	DetectedMapping    *ColumnMapping  `json:"detectedMapping,omitempty"`
	TotalGroups        int             `json:"totalGroups"`
	TotalItems         int             `json:"totalItems"`
	OverallConfidence  float64         `json:"overallConfidence"`
	ItemsNeedingReview int             `json:"itemsNeedingReview"` // Count of items with confidence < 0.80
	AutoApprovedItems  int             `json:"autoApprovedItems"`  // Count of items with confidence >= 0.80
}

// ============================================================================
// Structural Analyzer
// ============================================================================

// StructuralAnalyzer uses heuristics and ML for Excel analysis
type StructuralAnalyzer struct {
	file      *excelize.File
	predictor *MLPredictor
}

// NewStructuralAnalyzer creates an analyzer with ML support
func NewStructuralAnalyzer(f *excelize.File) *StructuralAnalyzer {
	return &StructuralAnalyzer{
		file:      f,
		predictor: GetMLPredictor(), // Singleton ML predictor
	}
}

// BuildColumnProfiles analyzes column content and builds statistical profiles
func (a *StructuralAnalyzer) BuildColumnProfiles(sheetName string, maxRows int) ([]ColumnProfile, error) {
	rows, err := a.file.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	if maxRows <= 0 || maxRows > len(rows) {
		maxRows = len(rows)
	}

	// Find max column count
	maxCols := 0
	for _, row := range rows[:maxRows] {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	profiles := make([]ColumnProfile, maxCols)

	// Initialize profiles
	for i := 0; i < maxCols; i++ {
		profiles[i] = ColumnProfile{
			Index:  i + 1,
			Letter: idxToColLetter(i + 1),
		}
	}

	// Collect column statistics
	colUniques := make([]map[string]struct{}, maxCols)
	colTextLengths := make([][]int, maxCols)
	for i := range colUniques {
		colUniques[i] = make(map[string]struct{})
		colTextLengths[i] = make([]int, 0)
	}

	totalRows := float64(maxRows)
	startRow := 5 // Skip potential headers

	for rowIdx := startRow; rowIdx <= maxRows && rowIdx <= len(rows); rowIdx++ {
		row := rows[rowIdx-1]
		for colIdx := 0; colIdx < maxCols; colIdx++ {
			var cellValue string
			if colIdx < len(row) {
				cellValue = strings.TrimSpace(row[colIdx])
			}

			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
			formula, _ := a.file.GetCellFormula(sheetName, cell)

			if cellValue == "" {
				profiles[colIdx].EmptyDensity++
			} else {
				colUniques[colIdx][cellValue] = struct{}{}
				colTextLengths[colIdx] = append(colTextLengths[colIdx], len(cellValue))

				if _, err := parseNumericValue(cellValue); err == nil {
					profiles[colIdx].NumericDensity++
				} else {
					profiles[colIdx].TextDensity++
				}
			}

			if formula != "" {
				profiles[colIdx].FormulaDensity++
			}
		}
	}

	// Normalize densities
	rowsAnalyzed := float64(maxRows - startRow + 1)
	if rowsAnalyzed == 0 {
		rowsAnalyzed = 1
	}

	for i := range profiles {
		profiles[i].NumericDensity /= rowsAnalyzed
		profiles[i].FormulaDensity /= rowsAnalyzed
		profiles[i].EmptyDensity /= rowsAnalyzed
		profiles[i].TextDensity /= rowsAnalyzed
		profiles[i].UniqueRatio = float64(len(colUniques[i])) / totalRows

		// Average text length
		if len(colTextLengths[i]) > 0 {
			sum := 0
			for _, l := range colTextLengths[i] {
				sum += l
			}
			profiles[i].AvgTextLength = float64(sum) / float64(len(colTextLengths[i]))
		}
	}

	return profiles, nil
}

// AnalyzeSheetTree builds a hierarchical analysis tree using structural heuristics
func (a *StructuralAnalyzer) AnalyzeSheetTree(sheetName string, catCol, valCol string, startRow int) (*AnalysisTreeResponse, error) {
	rows, err := a.file.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	catColIdx := colLetterToIdx(catCol)
	valColIdx := colLetterToIdx(valCol)

	nodes := make([]AnalysisNode, 0)
	var currentGroup *AnalysisNode
	totalRows := len(rows)

	for rowIdx := startRow; rowIdx <= totalRows; rowIdx++ {
		row := rows[rowIdx-1]

		// Get cell values
		var catValue, valValue string
		if catColIdx <= len(row) {
			catValue = strings.TrimSpace(row[catColIdx-1])
		}
		if valColIdx <= len(row) {
			valValue = strings.TrimSpace(row[valColIdx-1])
		}

		if catValue == "" {
			continue
		}

		// Get cell references
		catCell, _ := excelize.CoordinatesToCellName(catColIdx, rowIdx)
		valCell, _ := excelize.CoordinatesToCellName(valColIdx, rowIdx)

		// Build row features for classification
		features := a.buildRowFeatures(sheetName, catCell, valCell, catValue, valValue, rowIdx, totalRows)

		// Classify the row
		nodeType, structuralConfidence := a.classifyRow(features, catValue, valValue)

		// Parse value
		var value float64
		formula, _ := a.file.GetCellFormula(sheetName, valCell)
		if v, err := parseNumericValue(valValue); err == nil {
			value = v
		}

		// Predict tag using ML with confidence
		tagPrediction := a.predictor.PredictTagWithConfidence(catValue)

		// Combined confidence: average of structural and tag confidence
		combinedConfidence := (structuralConfidence + tagPrediction.Confidence) / 2.0

		node := AnalysisNode{
			ID:             generateNodeID(),
			Name:           catValue,
			Value:          value,
			Type:           nodeType,
			Tag:            tagPrediction.Tag,
			Confidence:     combinedConfidence,
			NeedsReview:    combinedConfidence < ConfidenceThreshold,
			IsAutoApproved: combinedConfidence >= ConfidenceThreshold,
			ExcelCell:      catCell,
			ExcelRow:       rowIdx,
			Formula:        formula,
		}

		switch nodeType {
		case NodeTypeGroup:
			// Save previous group
			if currentGroup != nil {
				nodes = append(nodes, *currentGroup)
			}
			currentGroup = &node
			currentGroup.Children = make([]AnalysisNode, 0)
		case NodeTypeItem:
			if currentGroup != nil {
				currentGroup.Children = append(currentGroup.Children, node)
			} else {
				// No group yet - create default group
				currentGroup = &AnalysisNode{
					ID:             generateNodeID(),
					Name:           "Imported Items",
					Type:           NodeTypeGroup,
					Confidence:     0.5,
					NeedsReview:    true, // Default groups need review
					IsAutoApproved: false,
					Children:       []AnalysisNode{node},
				}
			}
		}
	}

	// Don't forget the last group
	if currentGroup != nil {
		nodes = append(nodes, *currentGroup)
	}

	// Calculate totals and confidence
	totalGroups := len(nodes)
	totalItems := 0
	confidenceSum := 0.0
	itemsNeedingReview := 0
	autoApprovedItems := 0

	for _, n := range nodes {
		totalItems += len(n.Children)
		confidenceSum += n.Confidence
		for _, c := range n.Children {
			confidenceSum += c.Confidence
			if c.NeedsReview {
				itemsNeedingReview++
			} else {
				autoApprovedItems++
			}
		}
	}

	overallConfidence := 0.0
	totalNodes := float64(totalGroups + totalItems)
	if totalNodes > 0 {
		overallConfidence = confidenceSum / totalNodes
	}

	return &AnalysisTreeResponse{
		SheetName:          sheetName,
		Nodes:              nodes,
		TotalGroups:        totalGroups,
		TotalItems:         totalItems,
		OverallConfidence:  overallConfidence,
		ItemsNeedingReview: itemsNeedingReview,
		AutoApprovedItems:  autoApprovedItems,
	}, nil
}

// buildRowFeatures extracts structural features from a row
func (a *StructuralAnalyzer) buildRowFeatures(sheetName, catCell, valCell, catValue, valValue string, rowIdx, totalRows int) RowFeatures {
	// Check for bold styling
	styleID, _ := a.file.GetCellStyle(sheetName, catCell)
	isBold := styleID > 0 // Simplified: styled cells often bold

	// Check for formula
	formula, _ := a.file.GetCellFormula(sheetName, valCell)

	// Calculate indentation (leading spaces)
	indentation := len(catValue) - len(strings.TrimLeft(catValue, " "))

	// Check if uppercase
	isUpperCase := catValue == strings.ToUpper(catValue) && len(catValue) > 2

	// Parse value magnitude
	var valueMag float64
	if v, err := parseNumericValue(valValue); err == nil && v > 0 {
		valueMag = v
	}

	return RowFeatures{
		HasValue:       valValue != "" && valValue != "0",
		IsBold:         isBold,
		IsUpperCase:    isUpperCase,
		Indentation:    indentation,
		RowPosition:    float64(rowIdx) / float64(totalRows),
		HasFormula:     formula != "",
		ValueMagnitude: valueMag,
	}
}

// classifyRow determines if a row is a GROUP, ITEM, or IGNORE
func (a *StructuralAnalyzer) classifyRow(features RowFeatures, catValue, _ string) (NodeType, float64) {
	// =========================================================================
	// RULE 1: The Empty-Value Rule (Highest Confidence)
	// If category has text but value is empty → GROUP
	// =========================================================================
	if catValue != "" && !features.HasValue {
		return NodeTypeGroup, 0.95
	}

	// =========================================================================
	// RULE 2: The UPPERCASE Rule
	// All-caps text → likely a section header
	// =========================================================================
	if features.IsUpperCase && len(catValue) > 3 {
		return NodeTypeGroup, 0.85
	}

	// =========================================================================
	// RULE 3: Style-based detection
	// Bold or styled cells → likely headers
	// =========================================================================
	if features.IsBold && !features.HasValue {
		return NodeTypeGroup, 0.80
	}

	// =========================================================================
	// RULE 4: Indentation indicates sub-item
	// Indented rows are always items, not groups
	// =========================================================================
	if features.Indentation > 0 {
		return NodeTypeItem, 0.90
	}

	// =========================================================================
	// Default: If it has a value, it's an item
	// =========================================================================
	if features.HasValue {
		confidence := 0.70
		if features.HasFormula {
			confidence = 0.85 // Formulas = budget items
		}
		return NodeTypeItem, confidence
	}

	// Empty row or unclear
	return NodeTypeIgnore, 0.50
}

// generateNodeID creates a short unique ID for nodes
func generateNodeID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
