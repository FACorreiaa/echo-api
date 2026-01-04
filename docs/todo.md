To transform **Echo** into a truly "Alive" Money OS, we need to move beyond simple data storage and focus on **pattern recognition** and **contextual normalization**. Based on your Portuguese bank samples (Caixa Geral de Depósitos), here is the technical roadmap and user story for **v0.1** in English.

### 1. User Story: v0.1 — "My First Financial Story"

**As a** new Echo user,
**I want to** create an account and upload my financial history (CSV, Excel, or PDF),
**So that** I can instantly see where I spend the most and have a "living" dashboard that reflects my real-world spending habits.

#### **Acceptance Criteria:**

* **Secure Auth:** Functional registration and login via **ConnectGo**.
* **Universal Ingestion:** Support for drag-and-drop uploads of CSV, TSV, and PDF files.
* **Insight Mapping:**
* Automatic identification of categories (e.g., mapping "COMPRAS" or "LEVANTAMENTOS" from your files).


* Cleaning of merchant names (e.g., `COMPRAS C.DEB APPLE.C`   `Apple Services`).




* **Alive Dashboard:**
* **Top Spending:** Clear visualization of the category with the highest financial volume.
* **Dynamic Balance:** A total balance calculated by summing all imported transactions across different sources.


* **Excel Logic Filter:** Echo detects if an uploaded Excel file is a data dump (Type 1) or a planning model (Type 2).

---

### 2. Technical Analysis: What is Missing for Insights?

While your current ingestion engine is fast, the "Insights" layer requires three logical components to properly map diverse data sources:

* **A. Global Deduplication Engine (The "Double-Counting" Guard):**
* 
**The Problem:** If a user uploads a CSV today and a PDF tomorrow covering the same month, their balance will be wrong.


* **The Solution:** Implement a `unique_hash` for every transaction (Date + Amount + Description). Before saving, the backend checks if this hash exists to prevent duplicate balances.


* **B. The "Sanitizer" (Merchant Normalization):**
* 
**The Problem:** Bank strings like `CAR WAL CRT DEB REVOL`  are hard for users to read in a "Wrapped" story.


* **The Solution:** A Go service using Regex or a lookup table to map these strings to clean names ("Revolut Top-up") and brand icons.


* **C. Comparative Analytics Worker:**
* **The Problem:** You can't know where a user is spending "the most" relative to their habits without a baseline.
* **The Solution:** A background worker that aggregates spending by category per month. This allows the API to return insights like: "You spent 20% more on 'Diversos'  than last month."





---

### 3. Excel Filtering Guidelines (Type 1 vs. Type 2)

Your Go backend should perform a "Personality Test" on every `.xlsx` file before processing:

1. 
**Type 1 (Data Dump):** * **Detection:** High row count, single sheet, repeating column headers (Date, Description, Value).


* **Action:** Process through the standard transaction worker pool.


2. **Type 2 (The Living Plan):** * **Detection:** Presence of formula strings (extracted via `excelize`), multiple tabs, or keywords like "Budget," "Savings %," or "Emergency Fund."
* **Action:** Map specific cell coordinates to **Tamagui widgets**. Use a JS engine like `hyperformula` to execute those formulas on the mobile device so the user can change a percentage and see the dashboard update in real-time.



---

### 4. Strategic Data Mapping (Pillars for v0.1)

| Task | Implementation Detail |
| --- | --- |
| **Categorization** | Map bank categories (e.g., "COMPRAS" ) to Echo's internal schema.

 |
| **Regional Math** | Detect European decimal commas (e.g., `20,00` ) and convert to standard integer cents.

 |
| **Balance Logic** | Sum all "Confirmed" transactions. If a bank account is linked later, the bank API becomes the "Source of Truth" balance, and CSVs provide the "Historical Context." |

**Next Step for Echo:**
Would you like the **SQL query** that calculates your "Top Spending Categories" and "Monthly Deltas" to power your first Insight cards?

If you like, I can also search for React Native chart libraries that work well with Tamagui's styling. Do you want me to do that?