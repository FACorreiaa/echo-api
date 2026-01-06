# Echo Sprint Backlog

## Overview
Sprint tickets derived from the "Alive OS" vision for Echo - a Money Operating System that handles user finance history and helps improve it.

---

## TICKET-001: Manual Transaction Entry (Quick Capture)

**Priority:** High
**Type:** Feature
**Estimate:** Large

### Description
Implement a "Quick Capture" workflow that allows users to manually enter transactions using natural language input, bridging the "Cash Gap" for expenses not captured by bank syncs or file uploads.

### Acceptance Criteria
- [ ] User can type natural language entries like "Coffee 1$" or "Dinner with friends 20$"
- [ ] Auto-categorization engine parses the input and maps to appropriate category
- [ ] Amount is converted to integer cents (e.g., "1$" → 100 cents)
- [ ] Transactions are saved with `source_type = 'manual'` in the database
- [ ] Impact feedback is shown immediately (e.g., "This puts you at 95% of your Fun budget")
- [ ] Manual entries flow through the same Canonical Data Model as CSV/Bank data

### Technical Notes
```proto
message CreateManualTransactionRequest {
  string raw_text = 1; // "Coffee 1$"
  int64 amount_cents = 2;
  string category_id = 3;
  string description = 4;
  google.protobuf.Timestamp date = 5;
}
```

### Sub-tasks
- [ ] Create gRPC endpoint for manual transaction creation
- [ ] Implement NLP parser for natural language input
- [ ] Integrate with Categorization Engine
- [ ] Add real-time budget impact calculation
- [ ] Design React Native "Quick Add" Bottom Sheet with Tamagui

---

## TICKET-002: Component vs Widget Architecture Refactor

**Priority:** High
**Type:** Technical Debt / Architecture
**Estimate:** Medium

### Description
Reorganize the frontend codebase to distinguish between atomic **Components** (stateless, generic UI) and intelligent **Widgets** (data-aware, Echo-specific modules).

### Folder Structure
```
src/
├── components/        # Atomic UI (Buttons, Inputs, Skia Blobs)
│   ├── ui/
│   └── animations/
├── widgets/           # Intelligent Financial Modules
│   ├── bento/         # Pacing, Top Spend, Net Worth Cards
│   ├── ingestion/     # Pulse, Success Summary, Mapping Wizard
│   └── planning/      # Goals, Actuals, What-If Sliders
```

### Acceptance Criteria
- [ ] Create `/widgets` folder structure
- [ ] Move data-aware components to widgets (BalancePulseWidget, PacingProgressBar, etc.)
- [ ] Keep atomic components in `/components` (EchoButton, SkiaBackground, StatusBadge, BentoShell)
- [ ] Implement `BaseWidgetProps` interface for consistent widget structure
- [ ] All widgets handle Loading, Success, Error, and Empty states

### BaseWidget Interface
```typescript
export interface BaseWidgetProps {
  title: string;
  isLoading: boolean;
  error?: string;
  lastUpdated?: Date;
  onAction?: () => void;
}
```

### Components to Keep
- `EchoButton` - Theme colors and haptics
- `SkiaBackground` - Pulsing lava-lamp blobs
- `StatusBadge` - Colored pill for status
- `BentoShell` - Empty elevated container

### Components to Move to Widgets
- `BalancePulseWidget` - 24h change + Safe to Spend logic
- `PacingProgressBar` - Budget pacing calculation
- `CategoryInsightCard` - Mini-Wrapped card
- `QuickAddSheet` - Manual transaction entry

---

## TICKET-003: Financial Core - Net Worth & Spend Tracking

**Priority:** High
**Type:** Feature
**Estimate:** Large

### Description
Implement the core financial metrics system that serves as the "System Health" indicators for the Echo OS.

### Concepts
- **Net Worth (The State):** Total value of assets minus liabilities - a calculated snapshot
- **Total Spend (The Flow):** Real-time counter of outflows within a calendar month

### Acceptance Criteria
- [ ] Calculate Net Worth from all user assets and liabilities
- [ ] Track Total Spend per month with real-time updates
- [ ] Ingestion identifies "Debit" (Spend) vs "Credit" (Income) transactions
- [ ] Net Worth updates automatically: `previous_total + (Income - Spend)`
- [ ] Support for balance data integration

### Workflow
1. **Ingestion:** User uploads CSV
2. **Processing:** Echo identifies Debit/Credit
3. **Output:** Total Spend increases, Net Worth recalculated

---

## TICKET-004: Dynamic Schema / Mapping Wizard

**Priority:** High
**Type:** Feature
**Estimate:** Large

### Description
Implement a "Smart Mapper" UI that allows users to map columns from their Excel/CSV files to Echo's canonical fields, supporting any language and column structure.

### Acceptance Criteria
- [ ] Display horizontal preview of first 3-5 rows from uploaded file
- [ ] Allow users to map each column to Echo canonical fields
- [ ] Support mapping by cell coordinates (e.g., B31) for Planning Sheets
- [ ] Language-agnostic mapping (works with any column labels)
- [ ] Save mappings to `user_plans` JSONB column (user only maps once)
- [ ] Unmapped columns are ignored (Clean Room principle)

### Canonical Fields
```typescript
const ECHO_FIELDS = [
  { id: 'date', label: 'Transaction Date' },
  { id: 'description', label: 'Description / Merchant' },
  { id: 'amount', label: 'Amount' },
  { id: 'category', label: 'Category (Optional)' },
];
```

### UI Components
- `MappingWizard` - Multi-step modal
- `ColumnMapperCard` - Per-column mapping selector
- Validation pulse for suggested mappings
- "Finalize Mapping" button (requires Date & Amount minimum)

### Sub-tasks
- [ ] Create MappingWizard React Native component
- [ ] Implement ColumnMapperCard with preview rows
- [ ] Add smart suggestions (pulse effect for likely matches)
- [ ] Create Go backend to save mappings in JSONB
- [ ] Implement coordinate-based mapping for Planning Sheets (Type 2)

---

## TICKET-005: Ingestion Engine - Multi-Format Support

**Priority:** Medium
**Type:** Feature
**Estimate:** Large

### Description
Build a robust ingestion pipeline that supports multiple file formats with deduplication and merchant sanitization.

### Acceptance Criteria
- [ ] Support CSV file import
- [ ] Support PDF file import (future)
- [ ] Support XLSX file import with transactions
- [ ] Implement Deduplication Engine (same date/amount detection)
- [ ] Implement Merchant Sanitizer (e.g., "COMPRA PGO DOCE" → "Pingo Doce (Groceries)")
- [ ] Manual entries merge with bank records when duplicates detected

### Reconciliation Loop
When a user uploads a CSV containing a transaction that matches a manual entry:
1. Detect duplicate (same date + amount)
2. Merge manual entry with bank record
3. Preserve categorization from manual entry if present

---

## TICKET-006: Bento Dashboard / Workbench

**Priority:** Medium
**Type:** Feature
**Estimate:** Large

### Description
Create a dynamic Bento Grid Dashboard where manual entries and imported plans coexist as interactive "levers" and "pacing meters."

### Acceptance Criteria
- [ ] Dynamic dashboard that renders widgets based on data
- [ ] Each Excel row becomes a Widget Card
- [ ] Widgets react to data changes (optimistic updates)
- [ ] Link between Excel budgets and actual spending (e.g., "€320 Spent / €500 Budgeted")
- [ ] Support for "What-If" sliders in planning widgets

### Key Widgets
- Balance overview with 24h change
- Budget pacing per category
- Goal progress (e.g., "Trip to Japan")
- Top merchants/categories

---

## TICKET-007: Monthly Mini-Wrapped

**Priority:** Medium
**Type:** Feature
**Estimate:** Medium

### Description
Generate monthly "Mini-Wrapped" stories that turn boring transactions into shareable, behavioral archetypes.

### Acceptance Criteria
- [ ] Generate monthly spending insights automatically
- [ ] Create behavioral archetypes (e.g., "Coffee Enthusiast", "Subscription King")
- [ ] Show top merchant/category breakdowns
- [ ] Display spending trends and comparisons
- [ ] Make insights shareable

---

## TICKET-008: Goal Pacing & Projections

**Priority:** Low
**Type:** Feature
**Estimate:** Medium

### Description
Link savings goals to Net Worth calculations and provide pacing projections.

### Acceptance Criteria
- [ ] Connect goals (e.g., "Trip to Japan") to Net Worth tracking
- [ ] Calculate time-to-goal based on current savings pace
- [ ] Show projections: "At this pace, you'll reach your goal in X months"
- [ ] Update projections in real-time as balance changes

---

## TICKET-009: Transaction Enrichment

**Priority:** Low
**Type:** Feature
**Estimate:** Medium

### Description
Implement a "Sanitizer" worker to clean and enrich raw transaction descriptions.

### Acceptance Criteria
- [ ] Clean raw merchant strings (e.g., "COMPRA PGO DOCE" → "Pingo Doce")
- [ ] Auto-categorize based on merchant patterns
- [ ] Learn from user corrections over time
- [ ] Support multiple languages

---

## Future Roadmap (Out of Sprint)

| Feature | Phase | Description |
|---------|-------|-------------|
| Bank Account Linking | Future | Automates transaction import |
| Crypto/Stocks Integration | Post-Future | Updates Net Worth with market data |
| Drag-and-Drop Dashboard | Post-Future | User customizable widget layout |

---

## Dependencies

```
TICKET-001 (Manual Entry) ← TICKET-002 (Widget Architecture)
TICKET-004 (Mapping Wizard) ← TICKET-005 (Ingestion Engine)
TICKET-003 (Financial Core) ← TICKET-006 (Bento Dashboard)
TICKET-007 (Wrapped) ← TICKET-003 (Financial Core)
TICKET-008 (Goal Pacing) ← TICKET-003 (Financial Core)
```

---

## Sprint Priority Order

1. **TICKET-002** - Component/Widget Architecture (enables all other UI work)
2. **TICKET-003** - Financial Core (foundational for all metrics)
3. **TICKET-001** - Manual Transaction Entry (key user feature)
4. **TICKET-004** - Mapping Wizard (unlocks Excel import power)
5. **TICKET-005** - Ingestion Engine (multi-format support)
6. **TICKET-006** - Bento Dashboard (the "Workbench")
7. **TICKET-007** - Monthly Wrapped (engagement feature)
8. **TICKET-008** - Goal Pacing
9. **TICKET-009** - Transaction Enrichment
