# Credit card statement analysis plan

Status: frontend preview implemented; backend implementation remains planned.

## Handoff instructions

This document is the implementation handoff and includes all statement data supplied in the planning conversation, excluding the cardholder name and CKYC identifier. A new session should read this document before designing or implementing the feature. The user subsequently authorised implementing the UI. The React Accounts page now has a credit-card Statements view with local PDF selection/preview and explicitly labelled sample analysis; it does not upload, extract, or persist statements. Backend implementation remains planned.

The user already budgets credit card spending through envelopes and wants clarity on extra charges and earned rewards, rather than overspending warnings. The immediate goal is accurate, explainable analysis of this card's PDF statements.

The supplied text is sufficient to understand requirements and reproduce the arithmetic. The actual PDF has not been supplied or inspected: obtain it only for layout validation and extraction development, not to reconstruct the planning context. Bank/card identity, debit/credit symbols, and additive reward accounting remain unconfirmed. Do not silently promote inferred links to facts.

## Goal

Explain credit card fees, taxes, interest, and rewards from uploaded PDF statements. Spending is already managed through envelope budgeting; this feature provides clarity on the cost of using a card and the rewards actually reported.

Start with one card's statement format. The supplied excerpts suggest Regalia Gold, but the bank and card variant still need confirmation.

## First-release scope

- Upload a PDF against an existing credit card account; support password-protected statements if required by the sample.
- Extract statement metadata, purchases, payments, refunds, fees, GST, signed reward points, foreign currency amounts, EMI entries, and summary sections.
- Preserve original descriptions, references, page locations, and extraction evidence.
- Review and correct extracted information before finalising analysis.
- Link fees and taxes using references; suggest relationships supported by amounts and dates.
- Reconcile extracted figures with statement summaries and flag discrepancies.
- Show charges, rewards, EMI details, and unresolved entries in an Accounts-page Statements tab.
- Suggest matches to existing Pennywise transactions without automatically creating ledger entries.

## Interpretation rules

### Charges and monetary direction

Keep purchases, payments, refunds, fees, taxes, EMI principal, and EMI interest distinct. Monetary direction and reward-point direction are separate fields. Verify currency and credit/debit markers against the PDF layout and legend rather than interpreting pasted symbols.

EMI principal is repayment of borrowing and is excluded from extra-charge totals. Card payments should match existing transfers rather than become income or expenses.

Repeated merchant names and amounts are not sufficient evidence of duplicate entries. A refund and a later charge may legitimately share both.

### Links and evidence

Record whether a relationship is confirmed by a reference, inferred from arithmetic/date evidence, or manually confirmed. Shared references establish an association but do not always establish a complete allocation of a consolidated fee. Keep unresolved amounts visible.

For the supplied sample:

- DCC fee of INR 20.94 and GST of INR 3.77 share a reference. Adobe's INR 1,196.52 purchase is a likely parent because 1.75% rounds to INR 20.94.
- DCC fee of INR 5.78 and GST of INR 1.04 share a reference. Todoist's INR 330 purchase is a likely parent because 1.75% rounds to INR 5.78.
- Redemption processing fee of INR 99 and GST of INR 17.82 share a reference and form a confirmed INR 116.82 charge group.
- Claude, Railway, and Proton total INR 4,732.96. An inferred 2% markup rounds to the consolidated INR 94.65 fee. Estimated allocations are INR 45.01, INR 11.54, and INR 38.10; the nearby GST entries of INR 8.10, INR 2.08, and INR 6.86 match 18% of those allocations after rounding. These allocations remain inferred until verified.
- The earlier GST entries of INR 6.12, INR 63.90, and INR 30.42 remain unassigned, totalling INR 100.44.

The inferred markup rate is sample evidence, not a verified card tariff or a general parsing rule.

### Rewards

Separate reported transaction points, reversals, program/bonus points, and statement balance movements. A blank rewards cell means not reported, not necessarily zero. Preserve program labels verbatim.

Do not add program points to transaction points unless the statement establishes that they are additional. The sample's conditional combined figure is 3,430 points, not a confirmed closing or available balance. Expected rewards based on card rules are outside the first release.

### EMI and international purchases

Link EMI principal and interest to loan identifiers. Store statement-specific loan snapshots, distinguishing current-cycle amounts from remaining principal, remaining interest payable, and remaining tenure.

Store original foreign currency amount alongside billed INR amount. Report explicit FX fees separately; exchange-rate premium analysis requires a chosen reference rate and belongs to a later phase.

## Proposed architecture

- Go API owns budget-scoped statements, documents, extracted records, corrections, links, and access control using the existing handler/service/repository layering.
- Reuse existing document storage infrastructure where suitable; statements need their own account-level ownership rather than being forced into transaction attachments.
- Temporal orchestrates asynchronous extraction, progress, retries, and validation.
- Cipher provides structured extraction and interpretation where AI is useful. Extract PDF text with coordinates first; use OCR for scanned pages when supported.
- Deterministic code handles arithmetic, dates, reference matching, and reconciliation. Treat document content as data and validate AI output against a schema.
- React adds a Statements tab to the credit card account view.
- Do not build the MVP on the experimental file-parser scaffold.

These are proposed responsibilities; no new routes or deployed workflows exist yet.

## Proposed data model

- Statement: budget/account ownership, document reference, card metadata, billing period, currency, reported totals, processing status, and extraction version.
- Statement entry: source section, date/time, raw description, type, monetary direction, decimal amount, original currency/amount, signed reported points, transaction reference, loan identifier, and page evidence.
- Charge relationship: parent/child entries, allocated amount where applicable, evidence, and confirmation status.
- Reward summary: original program labels and points plus opening/earned/redeemed/expired/reversed/closing fields where reported.
- Loan snapshot: loan identifier, booked amount/date, rate, tenure, outstanding principal, remaining interest, and remaining tenure.
- Ledger match: statement-entry/transaction association and review status.

Use decimal values or integer minor units for new monetary fields. Keep statement records independent of ledger transactions and retain corrections alongside original extraction evidence.

## User flow

1. Select a credit card account and upload a statement.
2. Supply an unlock password if required; use it transiently and do not persist or log it.
3. Show processing progress and extraction failures.
4. Review extracted rows, totals, uncertain fields, and charge relationships with source evidence.
5. Save corrections and confirm the analysis.
6. Review proposed matches to existing purchases and payment transfers.

The statement view contains:

- Summary: purchases, refunds, payments, principal, interest, fees, and taxes.
- Charges: breakdown by reason and purchase, confirmed/inferred links, and unassigned amounts.
- Rewards: earned points, reversals, net transaction points, program points, and balances when available.
- EMI: current-cycle principal/interest and remaining obligations by loan.
- Review: extraction errors, reconciliation discrepancies, and proposed ledger matches.

## Implementation phases

### Phase 1: Validate the statement format

Obtain one complete PDF and confirm the bank/card variant. Inspect text extraction, coordinate layout, credit/debit legend, password needs, summary fields, and rewards accounting. Use representative later statements to check layout variation. Choose PDF/OCR tooling after inspecting the actual file.

Deliverable: extraction schema and a redacted fixture with verified expected values. Do not commit personal identifiers, passwords, or unredacted statements.

### Phase 2: Extraction and persistence

Add statement storage/models/migrations, budget/account-scoped API operations, and background extraction. Preserve evidence and original results; support correction, duplicate-upload detection, and idempotent retries. Enforce upload size/type limits and prevent sensitive document contents from entering logs or AI telemetry.

### Phase 3: Analysis and reconciliation

Implement charge classification, reference-based linking, explicitly inferred allocations, signed rewards, EMI linking, and summary reconciliation. Highlight incomplete extraction instead of presenting partial totals as complete.

### Phase 4: React review and analysis

Build upload, processing, review, correction, and statement-detail screens inside Accounts. Provide source navigation and visible evidence labels for inferred relationships.

### Phase 5: Ledger matching

Suggest existing transaction matches using account, amount, direction, dates, merchant, and references. Keep ambiguous matches for review. Missing-transaction creation is a later, explicitly reviewed action.

Generic statement reconciliation now exists for any account (Settings → Reconcile, `/api/statements`; see CLAUDE.md data flow 11): PDF text extraction, LLM row extraction, deterministic one-to-one matching (`reconcileStatementRows` in `go-pennywise-api/internal/service/statementReconcile.go`) and reviewed creation of missing rows. This phase should reuse that matcher rather than build a second one; card-specific work (charge links, rewards, EMI) stays in this plan.

## Sample acceptance checks

These totals are derived from the supplied excerpts and must be verified against the original PDF before becoming authoritative fixtures.

| Check | Expected value |
|-------|----------------|
| Extracted GST equals reported GST summary | INR 140.11 |
| DCC fees | INR 26.72 |
| Redemption fee | INR 99.00 |
| Consolidated FCY markup fee | INR 94.65 |
| EMI interest | INR 396.00 |
| Fees and interest excluding GST | INR 616.37 |
| Extra charges including GST | INR 756.48 |
| EMI principal excluded from extra charges | INR 9,173.00 |
| Positive transaction reward points | 2,460 |
| Reward reversals | 20 |
| Net transaction reward points | 2,440 |
| Program points, shown separately | 990 |
| Unassigned GST after accepting inferred FX links | INR 100.44 |

Verify wrapped references, independent money/reward signs, repeated charge/refund amounts, reconciliation failures, duplicate uploads, retry idempotency, persisted corrections, and tenant isolation. Run relevant Go tests and React build/lint as implementation changes land. Update AGENTS.md and service documentation when actual responsibilities, routes, or commands change.

## Later enhancements

- Additional bank/card formats and scanned-statement OCR if not needed initially.
- Monthly charges/rewards trends and comparisons.
- Expected-versus-reported rewards using effective-dated card rules, caps, exclusions, and rounding.
- Redemption-specific monetary valuation when supported by redemption details.
- Reviewed creation of missing budget transactions.
- Automatic statement ingestion and exchange-rate premium analysis.

## Open questions

- Confirm the bank and card variant.
- Obtain the complete PDF and determine whether it is password protected or scanned.
- Confirm whether program points are additional to transaction points using the full rewards summary.
- Locate statement balance totals and evidence for the remaining INR 100.44 GST.
- Confirm document retention and whether extracted content may be sent to the configured AI provider.

## Appendix: complete supplied statement data

Dates in the source use DD/MM/YYYY; the tables below normalise them to YYYY-MM-DD. The statement says transaction times are IST. All billed amounts below are interpreted as INR, pending confirmation from the original PDF. The pasted currency glyph appeared as `C`, and an `l` appeared in the PI column; neither establishes monetary direction.

`—` in the points column means no points were shown, not verified zero points. Monetary credits are marked **apparent credit** where a separate `+` occurred before the currency/amount. They need confirmation against the PDF legend. Reward signs are preserved independently. Wrapped reference lines have been joined to their corresponding descriptions. No transaction rows were omitted from the supplied excerpts.

### Domestic transactions

| Date | Time | Original description | Signed points | Billed amount | Reference / source note |
|------|------|----------------------|---------------|---------------|-------------------------|
| 2026-08-03 | 23:49 | GOOGLE CLOUD CYBS SIMUMBAI | — | 12.14 | |
| 2026-08-03 | 15:35 | AdobeADOBE.LY/E | +25 | 1,196.52 | |
| 2026-08-03 | 00:00 | IGST-VPS2721615939508-RATE 18.0 -05 | — | 6.12 | 09999999980803000138394 |
| 2026-08-03 | 00:00 | IGST-VPS2721615939509-RATE 18.0 -05 | — | 63.90 | 09999999980803000138410 |
| 2026-08-03 | 00:00 | IGST-VPS2721615939510-RATE 18.0 -05 | — | 30.42 | 09999999980803000138436 |
| 2026-08-05 | 00:00 | 1.75% on all DCC Transaction | — | 20.94 | ST262180084000012560544 |
| 2026-08-06 | 12:40 | BPPY CC PAYMENT AS016218124020UT8dd | — | 95,180.00 | ST262190083000010066475; apparent credit |
| 2026-08-06 | 13:09 | HDFC ERGO GIC OTHERMUMBAI | — | 12,296.87 | |
| 2026-08-06 | 01:55 | ASSPLBangalore | +125 | 5,087.10 | |
| 2026-08-08 | 16:36 | TODOISTTODOIST.CO | +5 | 330.00 | |
| 2026-08-10 | 00:00 | 1.75% on all DCC Transaction | — | 5.78 | ST262230084000012103577 |
| 2026-08-12 | 13:16 | SPOTIFY SIMUMBAI | — | 149.00 | |
| 2026-08-13 | 19:39 | RAZ*Makemytrip India PrivBengaluru | +55 | 2,289.00 | |
| 2026-08-13 | 19:26 | ASSPLBangalore | +195 | 7,802.00 | |
| 2026-08-13 | 00:00 | RAZ*Makemytrip India PrivBengaluru | -5 | 343.00 | VT262330075007390000015; apparent credit |
| 2026-08-14 | 11:07 | GOOGLE PLAY CONTENT PUMUMBAI | +15 | 650.00 | |
| 2026-08-15 | 19:37 | INDIGO AINEGURGAON | +80 | 3,298.00 | |
| 2026-08-15 | 19:28 | CT FLIGHT VIA SMARTBUYBANGALORE | +150 | 6,009.00 | |
| 2026-08-16 | 00:00 | REDEMPTION PROC FEE | — | 99.00 | ST262290084000011662543 |
| 2026-08-16 | 00:00 | IGST-VPS2722951954161-RATE 18.0 -05 | — | 17.82 | ST262290084000011662543 |
| 2026-08-17 | 00:00 | ASSPLBangalore | -15 | 699.45 | Apparent credit |
| 2026-08-18 | 09:03 | BLINK COMMERCE PVT LTDBANGALORE | — | 333.00 | |
| 2026-08-21 | 18:01 | RAMLAXMI CORPORATIONNAGPUR | +600 | 24,000.00 | |
| 2026-08-21 | 19:18 | ALISHANNAGPUR | +450 | 18,073.00 | |
| 2026-08-27 | 19:22 | MAKEMYTRIP INDIA PVTNEW DELHI | +170 | 6,848.00 | |
| 2026-08-28 | 00:00 | RAZ*Makemytrip India PrivBengaluru | — | 343.00 | No credit marker shown; retain separately from August 13 |
| 2026-08-29 | 11:18 | BHARTI AIRTEL LTDGURGAON | — | 978.82 | |
| 2026-08-29 | 11:17 | BHARTI AIRTEL LTDGURGAON | — | 1,078.82 | |
| 2026-08-29 | 11:20 | BHARTI AIRTEL LIMITEDNOIDA | — | 543.98 | |
| 2026-08-30 | 13:23 | MC DONALDSPUNE | +5 | 330.76 | |
| 2026-08-30 | 13:10 | WESTSIDEPUNE | +85 | 3,407.00 | |
| 2026-08-31 | 19:58 | MAKEMYTRIP INDIA PVTNEW DELHI | +175 | 7,090.00 | |
| 2026-08-31 | 10:18 | ZEPTO MARKETPLACE PRIVATEBangalore | — | 96.00 | |
| 2026-09-01 | 17:13 | ZEPTO MARKETPLACE PRIVATEBangalore | — | 879.00 | |
| 2026-09-02 | 07:31 | ASSPLBangalore | +20 | 804.00 | |
| 2026-09-02 | 09:26 | ASSPLBangalore | +15 | 699.00 | |
| 2026-09-02 | 18:34 | MAKEMYTRIP INDIA PVTNEW DELHI | +180 | 7,383.00 | |
| 2026-09-03 | 00:00 | OFFUS EMI,PRIN NB:07,00000136506087 | — | 6,213.00 | 09999999980903000139549 |
| 2026-09-03 | 00:00 | OFFUS EMI,PRIN NB:07,00000136506088 | — | 2,960.00 | 09999999980903000139564 |
| 2026-09-03 | 00:00 | OFFUS EMI,INT NBR:07,00000136506087 | — | 268.00 | 09999999980903000139556 |
| 2026-09-03 | 00:00 | OFFUS EMI,INT NBR:07,00000136506088 | — | 128.00 | 09999999980903000139572 |

### International transactions

The source groups these GST rows under International Transactions even when they link to DCC fees in the domestic section. Preserve source section independently of classification and parent links.

| Date | Time | Original description | Original currency amount | Signed points | Billed amount | Reference |
|------|------|----------------------|--------------------------|---------------|---------------|-----------|
| 2026-08-05 | 00:00 | IGST-VPS2721817609381-RATE 18.0 -05 | — | — | 3.77 | ST262180084000012560544 |
| 2026-08-08 | 19:07 | ANTHROPIC* CLAUDE SUBANTHROPIC. | USD 23.60 | +55 | 2,250.73 | |
| 2026-08-09 | 00:00 | IGST-VPS2722227894217-RATE 18.0 -05 | — | — | 8.10 | VT262220075046980000051 |
| 2026-08-09 | 13:05 | RAILWAYRAILWAY.CO | USD 6.05 | +10 | 576.99 | |
| 2026-08-10 | 00:00 | IGST-VPS2722330299609-RATE 18.0 -05 | — | — | 1.04 | ST262230084000012103577 |
| 2026-08-10 | 00:00 | IGST-VPS2722330299610-RATE 18.0 -05 | — | — | 2.08 | VT262230075041070000051 |
| 2026-08-11 | 18:16 | PROTON* PROTON AGWWW.PROTON | USD 19.99 | +45 | 1,905.24 | |
| 2026-08-12 | 00:00 | IGST-VPS2722537805018-RATE 18.0 -05 | — | — | 6.86 | VT262250075034030000078 |
| 2026-09-03 | 00:00 | CONSOLIDATED FCY MARKUP FEE | — | — | 94.65 | VT262250075034030000078 |

### Rewards Program Points Summary

| SR NO. | Original program label | Bonus points |
|--------|------------------------|--------------|
| 1 | FCYConversion | 1 |
| 2 | Regalia Gold MilestoneUpg 750K | 1 |
| 3 | Regalia Gold Milestones Upg 100 | 1 |
| 4 | Regalia Gold Milestones Upg 75 | 1 |
| 5 | Retail Spends Greater50K | 1 |
| 6 | Reward Points_on_Grocery | 25 |
| 7 | Reward Points_on_Insurance | 305 |
| 8 | Reward Points_on_Telecom | 55 |
| 9 | SmartBuy_Bonus_5X_RP_Promo 0000 | 600 |
| | Reported total | 990 |

No opening, closing, redeemed, or expired points balance was supplied. The redemption fee establishes a charge, not the number or value of points redeemed.

### Smart EMI Loan Summary

| Loan number | Booked date | Original amount | Original tenure | Interest rate p.a. | Balance principal outstanding | Balance interest payable | Balance tenure |
|-------------|-------------|-----------------|-----------------|--------------------|-------------------------------|--------------------------|----------------|
| 136506087 | 2026-02-11 | 54,403.18 | 9 months | 17% | 12,693.18 | 271.00 | 2 months |
| 136506088 | 2026-02-11 | 25,918.67 | 9 months | 17% | 6,045.67 | 129.00 | 2 months |

Source note: pre-closure charges as applicable. Do not estimate those charges from this note. Loan identifiers in transaction descriptions have leading zeros; normalise for linking while preserving original text.

### GST Summary

| IGST | CGST | SGST | Reversal | Total GST |
|------|------|------|----------|-----------|
| 140.11 | 0 | 0 | 0 | 140.11 |

No statement date, due date, opening/closing card balance, total amount due, or minimum amount due was supplied. The transaction date range must not be substituted for the confirmed billing period.
