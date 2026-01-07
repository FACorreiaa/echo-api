// Package insights contains wrapped summary generation and archetype detection.
package insights

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// ArchetypeID constants
const (
	ArchetypeCoffeeEnthusiast = "coffee_enthusiast"
	ArchetypeSubscriptionKing = "subscription_king"
	ArchetypeGroceryGuru      = "grocery_guru"
	ArchetypeNightOwl         = "night_owl"
	ArchetypeWeekendWarrior   = "weekend_warrior"
	ArchetypeFoodieExplorer   = "foodie_explorer"
	ArchetypeTransportTitan   = "transport_titan"
	ArchetypeShoppingSpree    = "shopping_spree"
	ArchetypeHomebody         = "homebody"
	ArchetypeTechEnthusiast   = "tech_enthusiast"
)

// Archetype represents a behavioral archetype
type Archetype struct {
	ID           string
	Title        string
	Emoji        string
	Description  string
	Rank         int
	CategoryID   *uuid.UUID
	MerchantName *string
	AmountMinor  int64
}

// ArchetypeRule defines a rule for detecting an archetype
type ArchetypeRule struct {
	ID          string
	Title       string
	Emoji       string
	Description func(amountMinor int64) string
	Matcher     func(stats *SpendingStats) (bool, int64) // Returns match and amount
}

// SpendingStats contains aggregated spending data for archetype detection
type SpendingStats struct {
	TotalSpendMinor     int64
	CategorySpend       map[string]int64 // category name -> amount
	MerchantSpend       map[string]int64 // merchant name -> amount
	SubscriptionCount   int
	WeekendSpendPercent float64
	NightSpendPercent   float64 // Spend after 10pm
	TransactionCount    int
	DaysActive          int
}

// archetypeRules defines all archetype detection rules
var archetypeRules = []ArchetypeRule{
	{
		ID:    ArchetypeCoffeeEnthusiast,
		Title: "Coffee Enthusiast",
		Emoji: "☕",
		Description: func(amount int64) string {
			return fmt.Sprintf("You spent €%.0f on coffee this month", float64(amount)/100)
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			coffee := stats.CategorySpend["Coffee"] + stats.MerchantSpend["Starbucks"] +
				stats.MerchantSpend["Costa"] + stats.CategorySpend["Coffee Shop"]
			return coffee >= 5000, coffee // €50+
		},
	},
	{
		ID:    ArchetypeSubscriptionKing,
		Title: "Subscription King",
		Emoji: "👑",
		Description: func(amount int64) string {
			return fmt.Sprintf("You have %d active subscriptions totaling €%.0f", int(amount/1000), float64(amount)/100)
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			// Check subscription category or common subscription merchants
			subs := stats.CategorySpend["Streaming"] + stats.CategorySpend["Subscriptions"] +
				stats.MerchantSpend["Netflix"] + stats.MerchantSpend["Spotify"] +
				stats.MerchantSpend["Disney+"] + stats.MerchantSpend["Apple"]
			return stats.SubscriptionCount >= 5 || subs >= 5000, subs
		},
	},
	{
		ID:    ArchetypeNightOwl,
		Title: "Night Owl",
		Emoji: "🦉",
		Description: func(amount int64) string {
			return fmt.Sprintf("%.0f%% of your spending happens after 10pm", float64(amount))
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			return stats.NightSpendPercent >= 30, int64(stats.NightSpendPercent)
		},
	},
	{
		ID:    ArchetypeWeekendWarrior,
		Title: "Weekend Warrior",
		Emoji: "🎉",
		Description: func(amount int64) string {
			return fmt.Sprintf("%.0f%% of your spending is on weekends", float64(amount))
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			return stats.WeekendSpendPercent >= 60, int64(stats.WeekendSpendPercent)
		},
	},
	{
		ID:    ArchetypeFoodieExplorer,
		Title: "Foodie Explorer",
		Emoji: "🍽️",
		Description: func(amount int64) string {
			return fmt.Sprintf("You spent €%.0f on dining out", float64(amount)/100)
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			food := stats.CategorySpend["Food & Drink"] + stats.CategorySpend["Restaurant"] +
				stats.CategorySpend["Dining"] + stats.CategorySpend["Fast Food"]
			totalPercent := float64(food) / float64(stats.TotalSpendMinor) * 100
			return totalPercent >= 20 && food >= 10000, food // 20%+ and €100+
		},
	},
	{
		ID:    ArchetypeTransportTitan,
		Title: "Transport Titan",
		Emoji: "🚗",
		Description: func(amount int64) string {
			return fmt.Sprintf("You spent €%.0f on transport", float64(amount)/100)
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			transport := stats.CategorySpend["Transport"] + stats.CategorySpend["Rideshare"] +
				stats.MerchantSpend["Uber"] + stats.MerchantSpend["Bolt"] + stats.MerchantSpend["Taxi"]
			return transport >= 15000, transport // €150+
		},
	},
	{
		ID:    ArchetypeGroceryGuru,
		Title: "Grocery Guru",
		Emoji: "🛒",
		Description: func(amount int64) string {
			return fmt.Sprintf("Smart shopping - groceries are only %.0f%% of your spend", float64(amount))
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			groceries := stats.CategorySpend["Groceries"] + stats.CategorySpend["Supermarket"]
			if stats.TotalSpendMinor == 0 {
				return false, 0
			}
			percent := float64(groceries) / float64(stats.TotalSpendMinor) * 100
			return percent <= 15 && percent > 0, int64(percent)
		},
	},
	{
		ID:    ArchetypeShoppingSpree,
		Title: "Shopping Spree",
		Emoji: "🛍️",
		Description: func(amount int64) string {
			return fmt.Sprintf("You spent €%.0f on shopping", float64(amount)/100)
		},
		Matcher: func(stats *SpendingStats) (bool, int64) {
			shopping := stats.CategorySpend["Shopping"] + stats.CategorySpend["Clothing"] +
				stats.MerchantSpend["Amazon"] + stats.MerchantSpend["Zara"] + stats.MerchantSpend["H&M"]
			return shopping >= 20000, shopping // €200+
		},
	},
}

// DetectArchetypes detects behavioral archetypes from spending stats
func DetectArchetypes(_ context.Context, stats *SpendingStats) []Archetype {
	var matched []Archetype

	for _, rule := range archetypeRules {
		if ok, amount := rule.Matcher(stats); ok {
			matched = append(matched, Archetype{
				ID:          rule.ID,
				Title:       rule.Title,
				Emoji:       rule.Emoji,
				Description: rule.Description(amount),
				AmountMinor: amount,
			})
		}
	}

	// Sort by amount (highest first) and assign ranks
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].AmountMinor > matched[j].AmountMinor
	})

	// Keep top 3 and assign ranks
	if len(matched) > 3 {
		matched = matched[:3]
	}
	for i := range matched {
		matched[i].Rank = i + 1
	}

	return matched
}
