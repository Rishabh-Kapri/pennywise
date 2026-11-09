# Transaction sources and reconciliation

Design notes for replacing `payee_rules` + `transaction_embeddings`, and for checking transactions against bank statements. Discussed 2026-11-07.

## The flow

1. An email arrives, a transaction is created, and learning records who it was.
2. A statement arrives. Each line is parsed and matched to an existing transaction.
3. On a match: verify, and add what was missing (narration, time, reward points).
4. No match, or a mismatch: the line goes to the review pile.
5. Learning updates at the end.

## Decisions

**Lookup and reconciliation are different questions.** "Who is this?" is fuzzy, so aliases and embeddings fit. "Is this the same payment?" is about identity: two ₹750 payments to one person a week apart look identical as vectors. Reconciliation matches on structured fields. Embeddings can only break ties between candidates.

**One transaction, many sources.** The `transactions` row is what the user sees and edits. Each email or statement line is its own row in `transaction_sources`. A transaction is verified when its sources agree; a mismatch is two sources that disagree, shown side by side.

**The lookup key is `CHANNEL | MERCHANT`**, e.g. `UPI | SAMPLE PERSON`. That is the only thing normalised. We don't build a canonical narration or split out aggregator, city, country or IFSC; they cost upkeep and don't help lookup.

**No cloud LLM.** Bank text is private. `cipher/internal/narration` is rule-based and runs in-process. Embeddings, when added, use local Ollama.

**`raw_text` is stored exactly as the bank gave it:** the email body, the narration, or the card statement's description cell.

**Matching, by channel** (not built yet):

| Tier | Match on | Result |
|---|---|---|
| 1 | UPI/IMPS/NEFT: account + `ref`. Card: card + amount + time within minutes | Match |
| 2 | Account + amount + date within a few days, counterparty as tie-break | Match if exactly one candidate |
| 3 | Several candidates, or none | Review pile |

Date drift (value date vs transaction date) is normal. An amount difference is a real mismatch. Reconciliation should run both ways: transactions in the period that aren't on the statement also go to review.

## Tables

`transaction_sources` and `counterparty_aliases` are in migration `00019_transaction_sources.sql`.

- **`transaction_sources`**: `kind`, `raw_text`, `channel`, `merchant`, `lookup_key` (generated), `vpa`, `ref`, `account_hint`, `direction`, `amount`, `currency`, `occurred_on`, `occurred_at`, `rewards_points`, `statement_import_id`. Unique on `(transaction_id, kind)`. An empty `transaction_id` is an unmatched statement line.
- **`counterparty_aliases`**: `channel`, `merchant`, `lookup_key` (generated), `payee_id`, `category_id`, `embedding` (empty for now), `source`, `hit_count`, `last_seen_at`. Unique on `(budget_id, channel, merchant)`.
- **`transactions`** is unchanged for now. Planned: add `occurred_at` and `reconciled_at`; drop `raw_bank_text` and `summary` once the pipeline writes sources.

Deliberately left out:

- **Card authorization codes.** Short, reused, never on the statement. They are not a `ref`, and they aren't stored.
- **Parser version column.** The backfill re-parses every row and updates only what changed.

## Backfill

`make run-backfill-sources` in `backend/cipher` (`BACKFILL_ARGS="-dry-run -budget <id>"`).

- Runs in one database transaction: any error writes nothing.
- Safe to re-run, and re-running is how a parser fix reaches existing rows. It keeps `raw_text` and `rewards_points`.
- Statement lines take amount, direction, date, currency and account from their `statement_imports` row, because a narration doesn't state them.
- For each key, the payee most of its transactions were filed under wins. Keys with several payees are listed in the report.

## Findings from the data

- HDFC emails mask UPI IDs from 2026-10-15 (`*************25-1@oksbi`). The name isn't masked, so the name stays the key. Unmasked IDs still appear in statement narrations.
- Card payments have no reference number on either side, so they match on card + amount + time.
- `UPI | CRED CLUB` and `UPI | AXIS BANK BBPS` go to different credit card transfer payees. The key alone can't say which card was paid.
- Some keys conflict because of duplicate payees ("Swiggy" vs "Zomato/Swiggy", "Anthropic" vs "Anthropic (Claude)"). `RAZ*MAKEMYTRIP INDIA P` was learned as Redbus, which looks like mis-filed transactions.
- OCR puts spaces inside words in statement imports (`UTIB000011 4`, `PAYMEN T`, `@OK SBI`). These lines will fail the UPI rule once sources are written straight from imports.

## Open

- A `BANK` channel for the bank's own entries (interest, charges, fees), with dates stripped: `BANK | INTEREST PAID`. Without it those lines have no key.
- Whether aliases also carry an unmasked `vpa` for exact matching.
- Alias embeddings, and trigram vs embedding as the fuzzy matcher.
- Rewards ledger for credit cards (needs its own design).

## Built so far

The jobs pipeline's lookup step (`cipher/internal/adapters/alias`) matches the exact `CHANNEL | MERCHANT` key against `counterparty_aliases`. A miss, or an alias with no category, goes on to Predict. There is no fuzzy matching.

`transaction.create` writes the transaction. After the report, a `transaction.learn` job writes its `transaction_sources` row (`origin = 'PIPELINE'`) and the alias together. Learning keeps an alias's payee, and only fills in a category it didn't have. The backfill skips transactions with a pipeline source.

When the user corrects or approves a pipeline transaction, a `transaction.learn` job runs and its alias takes that payee and category and becomes `MANUAL`; later automatic learning can't change either.

Not built: the matching tiers above. The older Temporal flow still reads `payee_rules` and `transaction_embeddings`.
