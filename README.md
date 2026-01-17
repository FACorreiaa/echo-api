# Echo — Smart Finance Tracker (Alive Money OS)

Echo is a personal finance product built to feel **alive**: it doesn’t just show charts, it proactively turns your transaction data into **actions**, **monthly insights**, and a shareable **“Money Wrapped”** story.

This repo currently represents the **Echo API** (backend). Frontend is **TBA**.

## Product Vision

- **Overview that matters:** where your money goes, what’s changing, and what to do next.
- **Objective-driven insights:** progress against your goals, pacing, and gentle correction.
- **Story-driven summaries:** monthly “mini-wrapped” and a year-end “Money Wrapped” (Spotify Wrapped-style).
- **Automate good decisions:** recommend and (eventually) execute the highest-leverage actions.
- **Bring your own spreadsheet:** drop in your existing Excel/Sheets-style model and have Echo power it everywhere.

## The 6 Pillars (mapped to features)

Echo is designed to explicitly hit these 6 outcomes:

1. **Your Money Wrapped (Hook):** personalized monthly/yearly recaps with fun + useful stats.
2. **Yearly Audit (Insight):** trend + anomaly detection (“why is this 40% higher than last year?”).
3. **Financial Foundation (Logic):** net worth, runway, emergency fund, debt visibility.
4. **Free Money (Optimization):** subscription hunting, fees, interest optimization, missed benefits.
5. **Clear Goals (Target):** goal/bucket tracking with pace alerts (“ahead/behind plan”).
6. **Money Operating System (Automation):** rules + tasks now; true automation later (when banking rails allow).

## Bring Your Own Spreadsheet (BYOS)

Echo is built to work for people who already have a spreadsheet-based system.

- Upload an `.xlsx` (or select a saved template) that contains *your* categories, budgets, and formulas.
- Map Echo’s canonical fields (transactions, accounts, categories, goals) to named ranges/tables in the sheet.
- Echo keeps the sheet “fresh” across devices by re-computing outputs as new data syncs in.

Design constraints (intentional):

- No macros/VBA for safety and portability.
- Prefer deterministic, auditable calculations (Echo can show “why” a number changed).

## MVP (Prove “Alive” + Wrapped)

Focus: make something people *want to come back to* and *share*.

[x]- **Auth + sessions:** account creation, login, refresh, logout.
[x]- **Data ingestion v1:** CSV upload/import (bank exports) to avoid early aggregator complexity.
[x]- **Document import v1:** import CSV/XLSX (and later invoices) into a canonical transaction model.
- **Transaction normalization:** merchant cleanup + categorization with user overrides. User should be able to edit transactions as well as categories. 
- **Spending overview:** top categories, merchants, trends, month-to-date vs last month.
- **Foundation v1:** optional manual net worth entries + basic runway metric.
[] Backend is done - **Goals/buckets v1:** targets, timelines, progress pacing, “behind plan” nudges.
[] Backend is done - **Subscriptions v1:** detect recurring charges; one-tap “review/cancel checklist”.
[] Backend is done - **Monthly insights:** “3 things changed this month” + “1 action to take this week”.
- **Mini-wrapped:** shareable monthly summary cards (no sensitive details by default).
- **BYOS v1:** upload a spreadsheet template + field mapping; export computed views as shareable/read-only.
- **Payments (optional in MVP):** Stripe Checkout for premium plan (behind feature flags).
[x]- **XLSX Import:** Extend the import service to handle Excel files
- **Category Assignment:** Auto-categorize transactions or let users assign categories, add more metadata to transactions
- **Monthly Insights:** Aggregated spending by category/month
[x]- **Empty State Dashboard:** Prompt new users to import

- Simulate a budget for the month. Set budget as 2000. The net worth is now 2000. 
if transactions add money, that value will increase. But if the transactions are a cost, for example rent, that value will decrease. a coffee, value decreases. 
Yes, allowing users to manually set an initial net worth or "starting balance" makes perfect sense for your app. Since **Echo** does not link directly to bank accounts, a user-defined starting point is the only way to provide context for their spending and progress.

In the world of Personal Finance Management (PFM), this approach is known as a **manual or "closed" system**.

### Why This Logic Works for an "Alive OS"

Allowing the user to define their starting point serves as the foundation for all subsequent "Alive" insights:

* **Establishing a Reference Point:** Net worth is a "snapshot in time". Without a starting number, a user seeing "-€20 for coffee" has no context. With a starting net worth of €2,000, that coffee becomes a measurable 1% decrease in their total tracked wealth.
* **Zero-Risk Privacy:** Millions of users prefer manual apps specifically because they do not require bank linking, eliminating risks associated with sharing sensitive login credentials.
* **Tracking Growth vs. Spending:** Net worth is calculated as **Assets minus Liabilities**. By letting the user set this, you enable them to track their progress over months or years, showing whether their total wealth is growing or shrinking regardless of individual budget categories.

### How to Implement the "Master Balance" Flow

To keep the app's logic consistent, you should treat the user's initial input as the "Opening Balance."

1. **Initial Snapshot:** The user enters their total starting value (e.g., €2,000).
2. **Transaction Impact:** * **Debits (Costs):** Rent, coffee, or subscriptions subtract from this master value.
* **Credits (Income):** Salary or side hustles add to it.


3. **Real-Time Pacing:** You can then compare this "Actual" balance against their monthly budget to show if they are "On Track" or "Overspending".

- Onboarding
New users should be immediatly prompted to set a starting balance or plan. or 
To make the initial experience feel like a professional **"Operating System,"** you should offer a **Multi-Path Onboarding** rather than a single choice. However, the first prompt should be a **"Fast Path" to set a starting balance** because it provides the immediate foundation for your mathematical logic.

### **The Recommended Onboarding Order**

1. **Set Starting Balance (High Priority):** This is the "Heartbeat" of the OS. By setting a starting number (e.g., €2,000), every subsequent action—whether manual or imported—has context.
2. **Import CSV/Excel (The Discovery):** Once the balance is set, prompt the user to "Populate History." This is where your **Mapping Wizard** and **Aho-Corasick engine** shine by turning raw files into a "Story."
3. **Choose a Planning Template (The Goal):** Finally, ask the user if they want to apply a "Nischa-style" 50/30/20 routine or a custom manual plan.

---

### **Why the Starting Balance is the Best "Step 1"**

| Onboarding Option | Pros | Cons |
| --- | --- | --- |
| **Starting Balance** | **Instant**. Takes 5 seconds. Establishes the "Master Balance" logic immediately. | Doesn't show the "power" of the automated engine yet. |
| **CSV Import** | Highly impressive. Shows the **"Alive"** mapping and categorization engine instantly. | **High friction**. Users may not have their CSV file ready on their phone during the first open. |
| **Manual Plan** | Great for "planners." Sets expectations for Pillar 5 (Goals). | Can feel like "homework" if the user has to enter 20 categories manually. |

---

### **The "Alive" Onboarding Screen Logic**

Instead of a standard list, use your **Skia Background** and **Moti animations** to present three "Operating System" paths. This reduces cognitive load while making the app feel intelligent.

* **Option A: "The Fresh Start"** (Manual Balance) — *Best for users who want to log as they go.*
* **Option B: "The Data Ingest"** (CSV/Excel) — *Best for users who want their full history imported now.*
* **Option C: "The Blueprint"** (Nischa Template) — *Best for users who want a pre-built strategy.*

**Technical Strategy for Onboarding**

When the user sets the **Starting Balance**, your Go backend should create an "Opening Balance" entry in the ledger with a `source_type = 'system'`. This ensures that even if they import a CSV later, your **Deduplication Engine** can handle the timeline correctly without "double-counting" their money.

Design the **React Native "Onboarding Path Picker"**? I can use **Moti** to create a morphing animation where the hero text changes based on which path the user is hovering over. Would you like that?

### Refined "OS" Recommendation

Instead of just "Net Worth," you might want to allow users to set a **"Tracked Liquidity"** (cash they actually intend to spend/budget) and an **"Asset Value"** (house, car, stocks).

* **Budgeting** would only affect the **Liquidity**.
* **Net Worth** would be the sum of both.

This distinction prevents a user from feeling "poor" just because they spent money on a coffee, even if they own a €200,000 house.

Create a **"Financial Health Snapshot" widget** for your dashboard that displays this manual Net Worth alongside a "Burn Rate" (how fast their balance is dropping this month)?

## Post‑MVP (Moat: Operating System + Trust)

- **Bank connections:** Plaid / GoCardless / Teller (region dependent), incremental sync, webhooks.
- **Invoice ingestion:** parse PDFs/images into transactions (receipts/invoices), reconcile against bank data.
- **Anomaly detection:** category/merchant deviation alerts; fee discovery; duplicate charges.
- **Net worth engine:** assets/liabilities snapshots, runway, debt payoff projections.
- **Automation engine:** durable tasks + “if this then that” rules, scheduled nudges, and (where possible) transfers.
- **Money Wrapped v2:** deeper storytelling, archetypes, comparisons to your own history, goal outcomes.
- **Notifications:** push + email digests, configurable, low-noise by design.
- **Sharing & virality:** privacy-preserving “wrapped” templates, referral loop, creator-friendly exports.
- **BYOS v2:** richer spreadsheet integration (templates, versioning, collaboration), more functions coverage, offline-first caching.
- **Billing maturity:** Stripe Billing, proration, coupons, taxes, and durable entitlement logic.

---

## Integrations Roadmap

Echo's power grows as it connects to more data sources. All integrations normalize into a canonical schema — the UI doesn't care where data came from.

### Banking Data Aggregators

| Provider | Regions | What They Offer | Notes |
|----------|---------|-----------------|-------|
| **Plaid** | US, Canada, UK, EU | Accounts, transactions, balances, identity | Industry standard, best US coverage |
| **TrueLayer** | UK, EU | Open Banking API, PSD2 compliant | Strong in Europe |
| **GoCardless (Bank Account Data)** | EU, UK | Free tier available, Open Banking | Good for starting in EU |
| **Teller** | US | Direct bank connections (no screen scraping) | Developer-friendly, newer |
| **Nordigen** (now GoCardless) | EU | Free Open Banking access | Great for EU MVP |
| **Yapily** | UK, EU | Open Banking, payments | Enterprise-focused |
| **Salt Edge** | Global (5000+ banks) | Wide coverage including non-Open Banking regions | Good for emerging markets |

### Investment & Broker Data

| Provider | What They Connect | Notes |
|----------|-------------------|-------|
| **Plaid (Investments)** | US brokerages (Fidelity, Schwab, Robinhood, etc.) | Holdings, transactions, cost basis |
| **Yodlee** | Broad investment coverage | Enterprise pricing, older but comprehensive |
| **Finicity** (Mastercard) | US investments + banking | Strong investment data |
| **Snaptrade** | US/Canada brokerages | Developer-friendly, investment-focused |
| **Vezgo** | Crypto exchanges + wallets | If crypto tracking is desired |

### Neobanks (Revolut, N26, Wise, Monzo)

- **No direct APIs** for personal accounts (only business APIs available)
- **Access via Open Banking aggregators** (TrueLayer, Plaid UK/EU, GoCardless)
- **CSV export** as fallback (manual but reliable)

### Payment Platforms

| Platform | API Access | What You Can Get | Notes |
|----------|------------|------------------|-------|
| **PayPal** | ✅ Transaction Search API | Transaction history, balances, payouts | OAuth 2.0, 3-year history, requires app approval |
| **Stripe** | ✅ Full API | Charges, payouts, balances, subscriptions | Best for business/creator income tracking |
| **Venmo** | ❌ No public API | — | Access via Plaid or CSV export only |
| **Cash App** | ❌ No public API | — | Access via Plaid or CSV export only |
| **Wise (TransferWise)** | ✅ API available | Balances, transactions, multi-currency | Good for international transfers |
| **Payoneer** | ✅ API available | Balances, transactions | Freelancer/contractor focus |

**PayPal Integration Notes:**
- Use the [Transaction Search API](https://developer.paypal.com/docs/api/transaction-search/v1/) for pulling history
- Requires OAuth 2.0 authentication with user consent
- Can retrieve up to 3 years of transaction data
- Webhooks available for real-time transaction notifications

### Integration Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    EchoOS Backend                       │
├─────────────────────────────────────────────────────────┤
│                 Canonical Data Model                    │
│   (Accounts, Transactions, Holdings, Balances)          │
├──────────┬──────────┬──────────┬──────────┬────────────┤
│  CSV     │  Plaid   │ TrueLayer│ Snaptrade│  Manual    │
│  Import  │  (US)    │  (EU/UK) │ (Invest) │  Entry     │
└──────────┴──────────┴──────────┴──────────┴────────────┘
```

### Phased Rollout

| Phase | Scope | Why |
|-------|-------|-----|
| **MVP** | CSV import only | Zero dependencies, fast iteration, validates core value |
| **Post-MVP v1** | Plaid (US) or GoCardless/TrueLayer (EU) | Automated bank sync for primary market |
| **Post-MVP v2** | Investment tracking (Plaid Investments or Snaptrade) | Complete net worth picture |
| **Scale** | Multi-provider support, fallback chains, manual reconciliation | Reliability + global coverage |

---

## Pre‑MVP Enhancements (strengthen the "Alive" hook)

These add minimal complexity but amplify differentiation:

| Feature | Why It Matters |
|---------|----------------|
| **Intent Tagging** | Let users flag transactions with *intent* ("splurge", "necessary", "regret", "investment"). Enables emotional/behavioral insights beyond pure categories. |
| **Spending Pulse** | Daily/weekly lightweight digest: "Your wallet today: €47 across 3 places." Keeps Echo top-of-mind without requiring dashboard visits. |
| **Quick Capture Mode** | One-tap log for cash/offline spending. Essential in regions with mixed cash/card usage. |
| **Goal Burn Rate** | "At current pace, you'll hit Vacation in 4.2 months (3 weeks early)." Real-time pacing beats static progress bars. |
| **Merchant Emoji/Icon System** | Auto-assign or user-pick icons per merchant. Makes overviews scannable and fun. |
| **"Oops" Alert Config** | User-defined thresholds for instant alerts: "Tell me if I spend > €50 on dining in a day." |
| **Streak Mechanics** | "7 days under budget" or "30 days no impulse spending" — gamification for habit building. |
| **Tag Bundles** | Group merchants/categories into user-defined "bundles" (e.g., "Self-care" = gym + therapy + spa). |

---

## Post‑MVP Feature Deep Dive

### Automation & Orchestration

Building the "Operating System" layer that makes finance hands-off:

| Feature | Description |
|---------|-------------|
| **Money Flows Canvas** | Visual node editor showing how money moves: Income → Buckets → Bills → Goals. Drag-and-drop rule creation. |
| **Scheduled Sweeps** | Auto-move "surplus" (income − bills − buffer) into designated goal accounts monthly. |
| **If/Then Rule Engine** | "If checking drops below €1,000, pause non-essential subscriptions." User chooses: execute or notify. |
| **Deferral Queue** | "I want X but not now" wishlist. Echo reminds when the purchase won't disrupt goals. |
| **Bill Negotiation Executor** | For supported services, Echo can initiate rate reduction requests via integrated APIs. |
| **Smart Payoff Optimizer** | Given debts + interest rates, auto-suggest (or execute) optimal extra payments (avalanche vs snowball). |

### Intelligence & Coaching

Turn passive data into proactive guidance:

| Feature | Description |
|---------|-------------|
| **Financial Health Score** | Custom index combining runway, debt ratio, savings rate, and goal progress. Track trends over time. |
| **Scenario Planner** | "What if I increase rent by €200?" → instant impact on runway, goals, and timelines. |
| **Negotiation Toolkit** | Echo drafts email/chat templates for subscription cancellations, rate reductions, and fee reversals. |
| **Tax Prep Helper** | Flag potentially deductible categories; export receipts/invoices grouped by type for tax submission. |
| **Carbon/Impact Tracker** | Estimate spending-linked emissions ("Your flights this year ≈ 1.2 tons CO₂"). Opt-in only. |
| **Life Event Advisor** | Context-aware suggestions for major events: moving, having a baby, job change, retirement. |
| **Spending Forecast** | Predict next month's spending based on recurring patterns + seasonality + upcoming known expenses. |
| **Category Insights NLP** | "Why did groceries spike in March?" → "You had 4 Whole Foods trips vs 2 normally, totaling €180 extra." |

### Social & Community

Optional sharing features that build virality and accountability:

| Feature | Description |
|---------|-------------|
| **Anonymous Benchmarks** | "People your age/location spend 18% less on dining." Opt-in, differential privacy, no PII exposure. |
| **Shared Goals** | Couples/roommates contribute to joint buckets with visibility and contribution controls. |
| **Accountability Partners** | Invite a friend to see your goal progress (not amounts). Mutual encouragement. |
| **Template Marketplace** | Users share BYOS spreadsheet templates: budgets, debt payoff trackers, FIRE calculators, etc. |
| **Community Challenges** | Monthly opt-in challenges: "No-spend weekend", "Pack lunch all week", "Save €100 extra". |

### Premium & Power User

Features for advanced users and monetization:

| Feature | Description |
|---------|-------------|
| **Multi-Currency Native** | Full support for travelers and remote workers paid in multiple currencies. Auto-convert using live rates. |
| **Business Lite Mode** | Freelancers: separate personal/business views, invoice matching, quarterly VAT/tax estimates. |
| **API Access** | Let power users push Echo data to Notion, Obsidian, personal dashboards, or automations. |
| **Offline-First Sync** | Complete local storage + conflict resolution. Appeals to privacy-maximalist users. |
| **Investment Tracking Lite** | Manual or synced portfolio tracking with basic allocation views. Not a replacement for dedicated apps. |
| **Real Estate Module** | Property value estimates, rental income tracking, mortgage payoff projections. |
| **Family Dashboard** | Aggregate views for family finances with role-based access (parent/child/partner). |

---

## "Wrapped" Expansion Ideas

The Wrapped mechanic is a powerful retention + virality lever. Lean into it:

| Idea | Details |
|------|---------|
| **Quarterly Mini-Wrap** | Lighter version: top 3 changes, 1 win, 1 watch-out. Keeps engagement between annuals. |
| **Wrapped Archetypes** | Personality-style summaries: "Cautious Saver", "Spontaneous Spender", "Goal Crusher", "Optimizer". |
| **Milestone Wraps** | Celebrate goal completions: "You paid off your credit card!" as shareable cards with confetti. |
| **Comparison to Past Self** | "You spent 12% less on coffee than 2024 You." Avoid peer comparison for privacy-first positioning. |
| **Wrapped for Couples** | Opt-in joint summary for partners sharing budgets. Highlights combined wins. |
| **Category Deep Dives** | Optional per-category wraps: "Your Travel Year", "Your Food Story", "Your Subscription Stack". |
| **Streak Celebrations** | Shareable badges for streaks: "90 days under budget", "1 year no overdraft fees". |
| **Year-over-Year Trends** | Multi-year view: "Your savings rate over the last 3 years" with trajectory visualization. |

---

## Prioritization Framework

When evaluating what to build, filter through these lenses:

### Primary Filters

1. **Retention First** — Does it bring people back weekly/daily? (Pulse, streaks, insights)
2. **Action-Oriented** — Does it tell users *what to do*, not just what happened? (Nudges, checklists)
3. **Shareable** — Can users show it off without leaking sensitive data? (Wrapped, badges)
4. **Automation Potential** — Will this unlock hands-off behavior later? (Rules, sweeps)

### Effort/Impact Matrix

| | Low Effort | High Effort |
|---|---|---|
| **High Impact** | Quick wins: Intent tags, Spending Pulse, Streaks | Strategic bets: Rule Engine, Money Flows, Bank Sync |
| **Low Impact** | Nice-to-haves: Emoji icons, bonus Wrapped themes | Avoid: Complex features with niche appeal |

### Trust & Security Lens

Every feature must pass:
- Does it require new PII? If so, is it worth the compliance burden?
- Can it be implemented with minimal data exposure?
- Does it maintain user control and transparency?

---

## AI Usage (Post‑MVP, opt‑in)

AI is valuable *after* Echo has strong deterministic foundations (clean data + rules). Planned uses:

- **Merchant + category enrichment:** better normalization of messy bank strings.
- **“Explain this” insights:** natural language explanations of trends/anomalies with citations to transactions.
- **Personal finance coach:** goal strategy suggestions, tradeoff analysis, and “next best action”.
- **Natural language queries:** “how much did I spend on eating out in Lisbon?”.
- **Story generation:** personalized Wrapped narratives from precomputed stats (no raw PII in prompts).

Guardrails (non-negotiable): user consent, data minimization, no model training on user data by default, and strong redaction for shareables.

## Startup Notes (Why this could work)

Personal finance is crowded, but most products are **passive dashboards**. Echo’s differentiation is:

- **Retention hook:** frequent mini-wraps + monthly insights, not just budgeting spreadsheets.
- **Actionable automation:** turn data into a prioritized checklist, then progressively automate.
- **Viral surface area:** Wrapped-style artifacts people can share (privacy-first).

Hard parts (and the opportunity): bank data quality, trust/security, compliance, and automation rails.

## Tech Stack (planned)

- **Backend:** Go
- **API:** Connect RPC (Buf) over HTTP (type-safe contracts via Protobuf)
- **DB:** Postgres (via `pgx`)
- **Jobs:** background workers for ingestion, normalization, insights, and notifications
- **Migrations:** Goose or `migrate` (TBD)
- **Spreadsheet engine:** XLSX template parsing + formula evaluation (no macros), plus field mapping
- **Payments:** Stripe (Checkout + Billing)
- **Bank data (later):** Plaid (items, accounts, transactions) with a normalization layer into Echo’s canonical models
- **Clients (multiplatform):** Web, Android, iOS, Desktop (frontend/framework TBA)

### Why Connect RPC is a good fit

- **Type safety for money:** strict contracts reduce edge-case bugs across clients.
- **Great DX:** works cleanly over HTTP without heavy gRPC browser constraints.
- **Streaming-ready:** useful for large transaction histories and incremental sync UX.

## Suggested Backend Architecture

- **Ingestion:** raw import (CSV now, bank aggregators later) → canonical transaction model
- **Normalization:** merchant resolution + category mapping with user overrides
- **Insights pipeline:** monthly stats, anomalies, goal pacing, subscription detection
- **Delivery:** API endpoints for dashboards + Wrapped; notification scheduler

## Security & Privacy (baseline)

- Encrypt sensitive data at rest (where applicable) and always in transit (TLS).
- Minimize storage of tokens/secrets; prefer aggregator tokenization patterns.
- Default shareables to aggregate stats; never include merchant names without explicit user choice.

## Study
Echo Pillar	App to Study	Key Insight to "Steal"			
Pillar 1: Wrapped	Monzo / Spotify	Use "Archetypes" and non-sensitive stats to make it shareable.			
Pillar 3: Foundation	Monarch Money	The way they handle "Sanity Checks" on net worth data.			
Pillar 6: Automation	Sequence	The visual "Nodes and Edges" UI for money movement.			
BYOS Feature	Tiller Money	Their field-mapping logic (Canonical Data → User Sheet).			
Subscriptions	Rocket Money	The "One-tap" cancellation checklist UI.

## Status

This repository is an early-stage scaffold. The README defines the **product + technical direction** for EchoAPI.

## Local Development (TBD)

Once implementation lands, this section will include exact commands. Expected prerequisites:

- Go (toolchain version TBD)
- Postgres
- Buf (for Protobuf/Connect generation)

## TODO Excel

To transition Echo from a simple tracker to a **"Bring Your Own Spreadsheet" (BYOS)** Operating System, you need to stop treating Excel as just a data source and start treating it as a **logic template**.

Here are the guidelines to address the two distinct Excel use cases while keeping your "Alive" OS promise.

---

### **Guideline 1: The Unified Ingestion Layer**

Your Go backend shouldn't care if the file is CSV, TSV, or XLSX. Use the **Strategy Pattern**.

1. **Library:** Use `github.com/qax-os/excelize/v2` for Go. It is the gold standard for performance and formula handling.
2. **The "Probe":** When an `.xlsx` is uploaded, your "Sniffer" checks:
* Does it have a single large table?  **Type 1 (Transactions)**.
* Does it have multiple small tables, colors, and formulas (e.g., `SUM`, `VLOOKUP`)?  **Type 2 (The Plan)**.



---

### **Guideline 2: Handling Type 1 (The "Silent" Transaction Import)**

This should behave exactly like your CSV import to maintain consistency.

* **Sheet Selection:** Since Excel has tabs, the React Native UI must ask: *"Which sheet contains your transactions?"*
* **Normalization:** Reuse the **Fingerprint + Mapping** logic. Treat each row in the Excel sheet as a CSV line.
* **Performance:** Even for Excel, use your **Worker Pool** pattern. `excelize` allows you to stream rows (`GetRows`) to keep memory low on your Apple M3 Pro.

---

### **Guideline 3: Handling Type 2 (The BYOS "Living" Plan)**

This is the "moat" that makes Echo unique. Instead of rebuilding their formulas in Go, you **host their logic**.

#### **A. The "Named Range" Strategy**

Don't try to parse the whole sheet. Ask the user to define **Named Ranges** or specific cells for Echo to interact with:

* **Input Ranges:** Where Echo "injects" its canonical data (e.g., a table called `Echo_Transactions`).
* **Output Ranges:** Where the user's formulas calculate results (e.g., a cell called `Total_Budget_Remaining`).

#### **B. The "Formula Evaluator"**

To let users edit the plan *in-app* without a full Excel engine:

* **Frontend (React Native):** Use a library like **`hyperformula`** (by Handsontable). It is a headless spreadsheet cell executor that runs in JavaScript.
* **Workflow:** 1. Echo loads the user's `.xlsx` values and formulas into `hyperformula`.
2. When a user changes a number in the Tamagui UI, `hyperformula` re-calculates the rest of the "Plan" instantly.
3. The UI updates the "Alive" stats.

---

### **Guideline 4: The "Spreadsheet-to-UI" Mapping**

Users don't want to look at a grid on a phone. They want a "Dashboard" powered by their spreadsheet.

1. **Map Formulas to Widgets:** * User selects cell `B10` in their Excel (which calculates "Monthly Runway").
* Echo turns that into a **Tamagui Bento Card** on the home screen.


2. **Bi-directional Sync:**
* If the user edits their "Rent" budget in Echo, it writes the value back to the virtual spreadsheet, triggers the formulas, and updates the "Runway" card.



---

### **Guideline 5: Technical Constraints (Safety First)**

To keep the OS stable, enforce these "Design Constraints":

1. **No Macros/VBA:** Explicitly block files containing `.xlsm` macros. They are a security risk and won't run on mobile.
2. **Static Logic:** The formulas should be deterministic.
3. **Export as Source of Truth:** Every time the user makes a major change, Echo should allow them to export the "Updated" `.xlsx` so they never feel locked into your app.

---

### **The "Alive" Action Plan**

| Feature | Implementation |
| --- | --- |
| **Parsing** | Go (`excelize`) extracts values and formula strings. |
| **Logic Engine** | React Native (`hyperformula`) runs the math on-device for instant feedback. |
| **Persistence** | Store the `.xlsx` file in S3/Cloud Storage and the "Mapping Metadata" in Postgres. |
| **UI** | A "Template Gallery" where users can see how their Excel logic looks as "Echo Bento Cards." |

### **Why this works:**

You aren't replacing Excel; you are giving Excel a **Mobile Shell**. The user keeps their complex formulas (The "Brain"), and Echo provides the data and the beautiful UI (The "Body").

**Next Step:** Implement the Go service to extract both **Values** and **Raw Formula Strings** from a sample `.xlsx` using `excelize`. This will prove you can "read the mind" of the user's spreadsheet.

# Echo

Echo, a finance-focused OS built with a ConnectGo backend and a Tamagui-powered mobile frontend that prioritizes a premium "Alive" user experience. The core of the app is a high-performance categorization engine using Aho-Corasick algorithms to normalize messy bank strings in nanoseconds. My goal is to allow users to "Bring Your Own Spreadsheet" (BYOS), where they can map any custom Excel or CSV layout to a beautiful Bento-style dashboard without touching real money or dealing with rigid, hardcoded categories.

## Goals

To give your community posts more "meat," here is a **Feature Highlights** list that specifically showcases the most unique parts of **Echo**. These bullet points bridge the gap between a standard budget app and a high-performance **"Alive OS."**

---

### **🚀 Echo OS: Key Feature Highlights**

* **The Smart Mapper (BYOS Engine):** Stop fighting rigid app structures. Echo features a "Bring Your Own Spreadsheet" (BYOS) mapper that allows you to point to any column in your Excel or CSV file—regardless of language—and instantly link it to Echo's logic engine. Whether it’s cell `B31` for Rent or a column labeled "Aluguel," Echo learns your layout so you only have to map it once.
* **"Nischa-Style" Goal Tracking:** Heavily inspired by the **50/30/20** and **Accountant Payday** routines, Echo categorizes your world into *Fundamentals*, *Fun*, and *Future You*. It transforms static goal percentages into an interactive feedback loop, calculating real-time pacing like: *"At this rate, you'll hit your 'Japan Trip' goal in 4.2 months"*.
* **The "Alive" Bento Dashboard:** Your finances shouldn't be a flat list. Echo populates a dynamic Bento-style grid with intelligent widgets that pulse and change color based on "System Health". If your spending spikes, the UI morphs to prioritize that insight, acting as a proactive coach rather than a passive ledger.
* **High-Speed Categorization Engine:** Powered by a **Go-based Aho-Corasick algorithm**, Echo normalizes messy bank strings (like `CAR WAL CRT DEB REVOL`) into clean merchants in nanoseconds. It processes millions of transactions per second, ensuring your dashboard is updated the moment you hit "Upload".
* **Privacy-First "Read-Only" Philosophy:** Echo is designed for those who want total control without the risk. We never touch or move real money; we strictly handle your financial history and plans to provide an "Operating System" layer for your existing accounts.

# Community

To help you get the most out of your Discord and Reddit posts, here is a "Community Feedback" section. Including these questions at the end of your post will signal that you are building Echo **with** the community, not just **for** them, which is essential for a "v0.1" developer.

### **Community Feedback & Discussion**

I'm currently finalizing the **v0.1 MVP** and would love to get your thoughts on a few architectural decisions:

1. **Excel Mapping vs. Templates:** Do you prefer mapping your own existing "messy" spreadsheets to an app, or would you rather have a "gold standard" template (like the Nischa routine) provided as a starting point?
2. **The "Manual Gap":** Since Echo doesn't move real money, how important is a "Quick Add" manual entry feature for cash expenses to keep your monthly "Alive" dashboard accurate?
3. **Visualization Priority:** When you look at your finance "Operating System," what is the first metric you want to see? Your Net Worth "Snapshot," or your "Monthly Pacing" against your budget?

---

### **Final Polish for your Project**

Now that your vision is clear and your "Alive" engine is benchmarked, you have a solid foundation for your **App Studio**.

**Would you like me to draft a "Technical Deep Dive" comment?** This is a separate, more detailed response you can post in the comments section for the "tech-heavy" members of the community (like on r/reactnative or r/golang) to explain your use of **ConnectGo**, **Tamagui**, and the **Aho-Corasick** engine.

# Technical

This "Technical Deep Dive" is designed for more technical communities (like **r/reactnative**, **r/golang**, or developer-focused Discord channels). It explains the "why" behind your stack, positioning **Echo** as a high-fidelity engineering project.

---

### **Technical Deep Dive: Under the Hood of Echo OS**

For those interested in the stack, I’m building **Echo** with a focus on high-performance ingestion and a "Morphic" UI that feels like a native OS rather than a wrapper.

**The Stack & Architecture:**

* **Backend (The Engine):** Built with **Golang** using **ConnectGo (gRPC-Web)**. I chose this over REST for strict type safety between the server and the mobile client.
* **High-Speed Ingestion:** To handle messy bank strings, I’m using the **Aho-Corasick algorithm** (via Cloudflare's implementation) for merchant pattern matching. It allows us to scan thousands of rules in  time—basically matching all patterns in a single pass of the text.
* **Mobile Frontend:** Built with **React Native** and **Tamagui**. Tamagui allows for a shared design system that stays performant on both iOS and Android while giving me a "Bento-style" layout engine.
* **The "Alive" UI:** We’re using **React Native Skia** for organic, hardware-accelerated background animations (dynamic blurred blobs) and **Moti (Reanimated)** for layout transitions. When a user focuses on a field or triggers a "FaceID" check, the entire UI morphs and scales in sync with the haptics.
* **Data Integrity:** All financial math is handled in **integer cents** (int64) to avoid floating-point errors. For the Excel logic, we use **Excelize** in Go to evaluate formulas from user-uploaded planning sheets and map them to our canonical data models.

The goal is to move away from "Passive History" apps and create a **Financial Workbench** where the UI reacts to the data in real-time.

---

### **Final Tip for your Posts**

When you post this, try to include a **short screen recording** of your Skia "Initial Screen" or the "Mapping Wizard." Visual proof of high-fidelity animations usually gets 10x more engagement in developer communities than text alone.

**Would you like me to draft a specific "Change Log" for your v0.2 roadmap so you can show the community exactly what you're working on next?**


