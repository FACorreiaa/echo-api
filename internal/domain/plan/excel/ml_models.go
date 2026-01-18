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
	tagPatterns  map[ItemTag][]string
	userPatterns map[ItemTag][]string // User-learned patterns (higher priority)
	mu           sync.RWMutex
}

// TagPrediction contains the predicted tag and confidence score
type TagPrediction struct {
	Tag        ItemTag
	Confidence float64
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
		tagPatterns:  make(map[ItemTag][]string),
		userPatterns: make(map[ItemTag][]string),
	}

	// Seed with multi-language budget terms
	p.seedTextModel()

	return p
}

// PredictTag predicts the semantic tag for a category name
func (p *MLPredictor) PredictTag(categoryName string) ItemTag {
	prediction := p.PredictTagWithConfidence(categoryName)
	return prediction.Tag
}

// PredictTagWithConfidence predicts the tag with a confidence score
func (p *MLPredictor) PredictTagWithConfidence(categoryName string) TagPrediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Normalize input
	categoryName = strings.ToLower(strings.TrimSpace(categoryName))
	if categoryName == "" {
		return TagPrediction{Tag: TagBudget, Confidence: 0.5}
	}

	// Score each tag (user patterns have higher weight)
	scores := make(map[ItemTag]float64)
	maxPossibleScore := 0.0

	// Check user-learned patterns first (3x weight)
	for tag, patterns := range p.userPatterns {
		for _, pattern := range patterns {
			maxPossibleScore += 3.0
			if strings.Contains(categoryName, pattern) {
				scores[tag] += 3.0
				// Exact match bonus
				if categoryName == pattern {
					scores[tag] += 6.0
					maxPossibleScore += 6.0
				}
			}
		}
	}

	// Check baseline patterns (1x weight)
	for tag, patterns := range p.tagPatterns {
		for _, pattern := range patterns {
			maxPossibleScore += 1.0
			if strings.Contains(categoryName, pattern) {
				scores[tag] += 1.0
				// Exact match bonus
				if categoryName == pattern {
					scores[tag] += 2.0
					maxPossibleScore += 2.0
				}
			}
		}
	}

	// Find highest scoring tag
	bestTag := TagBudget
	bestScore := 0.0
	totalScore := 0.0
	for tag, score := range scores {
		totalScore += score
		if score > bestScore {
			bestScore = score
			bestTag = tag
		}
	}

	// Calculate confidence
	// If no matches, return Budget with low confidence
	if bestScore == 0 {
		return TagPrediction{Tag: TagBudget, Confidence: 0.3}
	}

	// Confidence = proportion of total score that goes to the winning tag
	// Also factor in absolute match strength
	confidence := 0.5
	if totalScore > 0 {
		// Relative confidence (how much better than alternatives)
		relativeConfidence := bestScore / totalScore
		// Absolute confidence (did we match well?)
		absoluteConfidence := 0.50
		switch {
		case bestScore >= 3.0:
			absoluteConfidence = 0.95 // Strong match
		case bestScore >= 2.0:
			absoluteConfidence = 0.85
		case bestScore >= 1.0:
			absoluteConfidence = 0.70
		}
		confidence = (relativeConfidence + absoluteConfidence) / 2.0
	}

	return TagPrediction{Tag: bestTag, Confidence: confidence}
}

// Learn teaches the model a new association (user-specific learning)
func (p *MLPredictor) Learn(text string, tag ItemTag) {
	p.mu.Lock()
	defer p.mu.Unlock()

	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return
	}

	// Add to user patterns (higher priority than baseline)
	for _, existing := range p.userPatterns[tag] {
		if existing == text {
			return // Already exists
		}
	}
	p.userPatterns[tag] = append(p.userPatterns[tag], text)
}

// LearnBatch teaches multiple associations at once (for DB hydration)
func (p *MLPredictor) LearnBatch(corrections []UserMLCorrection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, c := range corrections {
		text := strings.ToLower(strings.TrimSpace(c.Term))
		if text == "" {
			continue
		}

		tag := c.CorrectedTag
		// Check if already exists
		found := false
		for _, existing := range p.userPatterns[tag] {
			if existing == text {
				found = true
				break
			}
		}
		if !found {
			p.userPatterns[tag] = append(p.userPatterns[tag], text)
		}
	}
}

// UserMLCorrection represents a user's correction stored in DB
type UserMLCorrection struct {
	Term         string
	PredictedTag ItemTag
	CorrectedTag ItemTag
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

		// Russian
		"аренда", "ипотека", "страховка", "подписка", "коммунальные",
		"электричество", "вода", "газ", "интернет", "телефон",
		"netflix", "spotify", "членство",
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
		"dinner", "lunch", "fun",

		// German
		"lebensmittel", "essen", "restaurant", "transport", "benzin",
		"kleidung", "gesundheit", "apotheke", "bildung", "reise",
		"spaß", "unterhaltung",

		// French
		"alimentation", "nourriture", "restaurant", "transport", "essence",
		"vêtements", "santé", "pharmacie", "éducation", "voyage",
		"courses", "loisirs",

		// Spanish
		"alimentación", "comida", "restaurante", "transporte", "gasolina",
		"ropa", "salud", "farmacia", "educación", "viaje",
		"compras", "ocio",

		// Russian
		"продукты", "еда", "ресторан", "транспорт", "бензин",
		"одежда", "здоровье", "аптека", "образование", "путешествие",
		"развлечения", "покупки",
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
		"crypto", "bitcoin", "goal", "goals", "fund", "emergency",

		// German
		"sparen", "investition", "notfall", "rente", "aktien",

		// French
		"épargne", "investissement", "retraite", "actions",

		// Spanish
		"ahorro", "inversión", "jubilación", "acciones",

		// Russian
		"сбережения", "инвестиции", "накопления", "акции",
		"криптовалюта", "биткоин", "пенсия", "резерв",
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

		// Russian
		"зарплата", "доход", "заработок", "оклад",
		"дивиденды", "премия", "фриланс",
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

		// Russian
		"долг", "кредит", "займ", "ипотечный платёж",
	}
}
