// Package excel provides ML-based prediction for Excel imports.
package excel

import (
	"strings"
	"sync"
)

// ============================================================================
// ML Predictor (Singleton)
// Uses keyword-based scoring with weighted matches for tag prediction
// ============================================================================

var (
	mlPredictor     *MLPredictor
	mlPredictorOnce sync.Once
)

// MLPredictor provides text classification for budget items
type MLPredictor struct {
	tagPatterns map[ItemTag][]string
	mu          sync.RWMutex
}

// GetMLPredictor returns the singleton ML predictor
func GetMLPredictor() *MLPredictor {
	mlPredictorOnce.Do(func() {
		mlPredictor = newMLPredictor()
	})
	return mlPredictor
}

// newMLPredictor creates and seeds a new predictor
func newMLPredictor() *MLPredictor {
	p := &MLPredictor{
		tagPatterns: make(map[ItemTag][]string),
	}

	// Seed with multi-language budget terms
	p.seedTextModel()

	return p
}

// PredictTag predicts the semantic tag for a category name
func (p *MLPredictor) PredictTag(categoryName string) ItemTag {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Normalize input
	categoryName = strings.ToLower(strings.TrimSpace(categoryName))
	if categoryName == "" {
		return TagBudget
	}

	// Score each tag
	scores := make(map[ItemTag]int)

	for tag, patterns := range p.tagPatterns {
		for _, pattern := range patterns {
			if strings.Contains(categoryName, pattern) {
				scores[tag]++
				// Exact match bonus
				if categoryName == pattern {
					scores[tag] += 2
				}
			}
		}
	}

	// Find highest scoring tag
	bestTag := TagBudget
	bestScore := 0
	for tag, score := range scores {
		if score > bestScore {
			bestScore = score
			bestTag = tag
		}
	}

	return bestTag
}

// Learn teaches the model a new association
func (p *MLPredictor) Learn(text string, tag ItemTag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return
	}

	// Add to patterns if not already present
	for _, existing := range p.tagPatterns[tag] {
		if existing == text {
			return // Already exists
		}
	}
	p.tagPatterns[tag] = append(p.tagPatterns[tag], text)
}

// seedTextModel initializes the model with multi-language budget terms
func (p *MLPredictor) seedTextModel() {
	// =========================================================================
	// RECURRING (R) - Fixed/Regular expenses
	// =========================================================================
	p.tagPatterns[TagRecurring] = []string{
		// Portuguese
		"aluguel", "renda", "aluguer", "hipoteca", "prestação",
		"água", "luz", "gás", "eletricidade", "internet", "telefone",
		"seguro", "seguros", "assinatura", "assinaturas", "mensalidade",
		"netflix", "spotify", "hbo", "disney", "amazon prime",
		"condomínio", "iptu", "ipva", "fixos", "fixas",

		// English
		"rent", "mortgage", "insurance", "subscription", "subscriptions",
		"utilities", "electricity", "water", "gas", "phone",
		"netflix", "spotify", "hulu", "gym", "membership", "recurring",

		// German
		"miete", "versicherung", "strom", "wasser", "heizung",
		"abonnement", "mitgliedschaft",

		// French
		"loyer", "assurance", "électricité", "eau", "chauffage",

		// Spanish
		"alquiler", "seguro", "suscripción", "electricidad",
	}

	// =========================================================================
	// BUDGET (B) - Variable/discretionary spending
	// =========================================================================
	p.tagPatterns[TagBudget] = []string{
		// Portuguese
		"alimentação", "supermercado", "mercado", "compras", "comida",
		"transporte", "combustível", "gasolina", "uber", "táxi",
		"restaurante", "café", "lazer", "entretenimento", "diversão",
		"roupas", "vestuário", "beleza", "saúde", "farmácia",
		"educação", "livros", "cursos", "presentes", "viagem",
		"manutenção", "casa", "carro", "habitação", "despesas",
		"variáveis", "variavel",

		// English
		"groceries", "food", "dining", "restaurant", "coffee",
		"transportation", "fuel", "uber", "taxi", "parking",
		"entertainment", "movies", "shopping", "clothes", "clothing",
		"health", "pharmacy", "beauty", "haircut", "education",
		"books", "gifts", "travel", "vacation", "maintenance",
		"home", "car", "pet", "pets", "budget", "expenses",

		// German
		"lebensmittel", "essen", "restaurant", "transport", "benzin",
		"kleidung", "gesundheit", "apotheke", "bildung", "reise",

		// French
		"alimentation", "nourriture", "restaurant", "transport", "essence",
		"vêtements", "santé", "pharmacie", "éducation", "voyage",

		// Spanish
		"alimentación", "comida", "restaurante", "transporte", "gasolina",
		"ropa", "salud", "farmacia", "educación", "viaje",
	}

	// =========================================================================
	// SAVINGS (S) - Goals and investments
	// =========================================================================
	p.tagPatterns[TagSavings] = []string{
		// Portuguese
		"poupança", "investimento", "investimentos", "reserva",
		"emergência", "fundo de emergência", "meta", "metas",
		"aposentadoria", "previdência", "ações", "fundos",
		"cripto", "bitcoin", "tesouro", "cdb", "lci", "lca",
		"boleto pessoal",

		// English
		"savings", "investment", "investments", "emergency fund",
		"retirement", "401k", "ira", "stocks", "bonds", "etf",
		"crypto", "bitcoin", "goal", "goals", "fund",

		// German
		"sparen", "investition", "notfall", "rente", "aktien",

		// French
		"épargne", "investissement", "retraite", "actions",

		// Spanish
		"ahorro", "inversión", "jubilación", "acciones",
	}

	// =========================================================================
	// INCOME (IN) - Sources of income
	// =========================================================================
	p.tagPatterns[TagIncome] = []string{
		// Portuguese
		"salário", "renda", "receita", "rendimento", "rendimentos",
		"freelance", "bônus", "décimo terceiro", "férias",
		"dividendos", "aluguel recebido", "extra", "receitas",
		"renda total", "total receitas",

		// English
		"salary", "income", "wages", "paycheck", "revenue",
		"freelance", "bonus", "dividends", "rental income", "side hustle",

		// German
		"gehalt", "einkommen", "lohn",

		// French
		"salaire", "revenu", "revenus",

		// Spanish
		"salario", "ingreso", "ingresos", "sueldo",
	}

	// =========================================================================
	// DEBT (D) - Debt payments
	// =========================================================================
	p.tagPatterns[TagDebt] = []string{
		// Portuguese
		"dívida", "dívidas", "empréstimo", "financiamento",
		"cartão de crédito", "parcelamento", "juros",

		// English
		"debt", "loan", "credit card", "mortgage payment",
		"student loan", "car payment", "interest",

		// German
		"schulden", "kredit", "darlehen",

		// French
		"dette", "prêt", "crédit",

		// Spanish
		"deuda", "préstamo", "crédito",
	}
}
