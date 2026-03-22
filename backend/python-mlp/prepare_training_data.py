"""
Prepare training data for the Pennywise MLP models.

Two modes:
  fetch    — Pull real prediction+correction data from the API and convert to training format
  generate — Create synthetic bank emails for bootstrapping new payees/categories/accounts
  merge    — Combine fetched predictions with synthetic data into a single training file

Usage:
  python prepare_training_data.py fetch --api-url http://localhost:5151 --budget-id <uuid>
  python prepare_training_data.py generate --input new_labels.json --output data/synthetic.json
  python prepare_training_data.py merge --predictions data/predictions.json --synthetic data/synthetic.json
  python prepare_training_data.py stats --input data/normalized_with_email.json
"""

import argparse
import json
import math
import os
import random
import re
import string
import sys
from collections import defaultdict
from urllib.request import Request, urlopen
from urllib.error import URLError, HTTPError

DATE_REGEX = re.compile(
    r"\d{2}-\d{2}-\d{4}|\d{2}-\d{2}-\d{2}|\d{2}/\d{2}/\d+|\d{2}\s*\w+,\s*\d+"
)

# Bank email templates matching the formats parsed by go-gmail's email parser.
# Credit card format: "Dear Card Member, Thank you for using your <bank> Credit Card ending <suffix> for Rs <amount> at <payee> on <date>."
# Debit (outflow): "Dear Customer, Rs.<amount> has been debited from account **<account_num> to <payee> on <date>."
# Credit (inflow): "Dear Customer, Rs.<amount> is successfully credited to your account **<account_num> by <payee> on <date>."
EMAIL_TEMPLATES = {
    "cc_debit": "Dear Card Member, Thank you for using your {bank} Credit Card ending {card_suffix} for Rs {amount:.2f} at {payee_text} on {date}.",
    "debit": "Dear Customer, Rs.{amount:.2f} has been debited from account **{account_num} to {payee_text} on {date}.",
    "credit": "Dear Customer, Rs.{amount:.2f} is successfully credited to your account **{account_num} by {payee_text} on {date}.",
}


# ---------------------------------------------------------------------------
# Fetch predictions from API
# ---------------------------------------------------------------------------


def fetch_predictions(api_url: str, budget_id: str) -> list[dict]:
    """Call GET /api/predictions with the budget ID header and return the JSON array."""
    url = f"{api_url.rstrip('/')}/api/predictions"
    req = Request(url, headers={"X-Budget-ID": budget_id})
    try:
        with urlopen(req) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except HTTPError as e:
        print(f"API error {e.code}: {e.read().decode()}", file=sys.stderr)
        sys.exit(1)
    except URLError as e:
        print(f"Connection error: {e.reason}", file=sys.stderr)
        sys.exit(1)


def extract_date_from_email(email_text: str) -> str:
    """Extract a date string from email text using the same patterns as go-gmail."""
    match = DATE_REGEX.search(email_text)
    return match.group(0) if match else ""


def predictions_to_training_data(predictions: list[dict]) -> list[dict]:
    """
    Convert prediction records from the API into the training data format
    expected by PennywiseMLP.get_labels_and_email().

    For each prediction:
    - If hasUserCorrected is true, prefer userCorrected* fields (fall back to
      original prediction if a specific field wasn't corrected).
    - If hasUserCorrected is false, use the original predicted values (they were correct).

    Output format per record:
        {email_text, payee, category, account, amount, date}
    """
    training_data = []

    for pred in predictions:
        email_text = pred.get("emailText", "")
        if not email_text:
            continue

        corrected = pred.get("hasUserCorrected", False)

        if corrected:
            payee = pred.get("userCorrectedPayee") or pred.get("payee") or ""
            category = pred.get("userCorrectedCategory") or pred.get("category") or ""
            account = pred.get("userCorrectedAccount") or pred.get("account") or ""
        else:
            payee = pred.get("payee") or ""
            category = pred.get("category") or ""
            account = pred.get("account") or ""

        print(f"Processing prediction for email: {email_text[:50]}... | Payee: {payee} | Category: {category} | Account: {account} | Corrected: {corrected}")

        amount = pred.get("amount", 0.0)
        date = extract_date_from_email(email_text)

        training_data.append(
            {
                "email_text": email_text,
                "payee": payee,
                "category": category,
                "account": account,
                "amount": amount,
                "date": date,
            }
        )

    return training_data


# ---------------------------------------------------------------------------
# Synthetic email generation (for bootstrapping new labels)
# ---------------------------------------------------------------------------


def generate_synthetic_email(
    payee_text: str,
    amount: float,
    date: str,
    account: str,
    account_config: dict[str, dict],
) -> str | None:
    """
    Generate a synthetic bank email for a single transaction.

    account_config maps account names to their properties:
        {
            "HDFC (Salary)": {"type": "debit", "num": 8936, "bank": "HDFC Bank"},
            "HDFC Credit Card": {"type": "cc", "num": 4432, "bank": "HDFC Bank"},
            ...
        }
    Returns None if the account isn't in the config (e.g. Cash, Wallet accounts
    that don't produce bank emails).
    """
    acct = account_config.get(account)
    if not acct:
        return None

    acct_type = acct.get("type", "debit")

    if acct_type == "cc":
        return EMAIL_TEMPLATES["cc_debit"].format(
            bank=acct.get("bank", "HDFC Bank"),
            card_suffix=acct.get("num", "0000"),
            amount=abs(amount),
            payee_text=payee_text,
            date=date,
        )

    # Debit/savings account
    if amount > 0:
        return EMAIL_TEMPLATES["credit"].format(
            amount=abs(amount),
            account_num=acct.get("num", "0000"),
            payee_text=payee_text,
            date=date,
        )
    return EMAIL_TEMPLATES["debit"].format(
        amount=abs(amount),
        account_num=acct.get("num", "0000"),
        payee_text=payee_text,
        date=date,
    )


def load_account_config(path: str) -> dict[str, dict]:
    """Load account config from a JSON file, or return a sensible default."""
    if os.path.exists(path):
        with open(path, "r") as f:
            return json.load(f)

    # Default config matching the existing bank templates
    return {
        "HDFC (Salary)": {"type": "debit", "num": 8936, "bank": "HDFC Bank"},
        "PNB (Savings)": {"type": "debit", "num": 2086, "bank": "PNB"},
        "Kotak (Savings)": {"type": "debit", "num": 6318, "bank": "Kotak"},
        "HDFC Credit Card": {"type": "cc", "num": 4432, "bank": "HDFC Bank"},
        "HDFC Swiggy Credit Card": {"type": "cc", "num": 8799, "bank": "HDFC Bank"},
    }


def generate_synthetic_data(
    transactions: list[dict], account_config: dict[str, dict]
) -> list[dict]:
    """
    Generate synthetic email_text for a list of transaction dicts.

    Input format per record (user-provided for bootstrapping new labels):
        {payee, payee_text (optional), category, account, amount, date}

    payee_text is the merchant text that appears in the email body (e.g. the VPA
    string or merchant name). If not provided, payee name is used directly.

    Skips records whose account isn't in account_config (no bank email template).
    """
    results = []
    for txn in transactions:
        payee_text = txn.get("payee_text", txn.get("payee", ""))
        email = generate_synthetic_email(
            payee_text=payee_text,
            amount=txn.get("amount", 0.0),
            date=txn.get("date", "01-01-2026"),
            account=txn.get("account", ""),
            account_config=account_config,
        )
        if email is None:
            continue

        results.append(
            {
                "email_text": email,
                "payee": txn.get("payee", ""),
                "category": txn.get("category", ""),
                "account": txn.get("account", ""),
                "amount": txn.get("amount", 0.0),
                "date": txn.get("date", ""),
            }
        )
    return results


# ---------------------------------------------------------------------------
# Augment: fetch non-prediction transactions and generate synthetic emails
# ---------------------------------------------------------------------------

UPI_SUFFIXES = ["oksbi", "ybl", "axl", "pz", "okaxis", "hdfcbank", "icici", "ptyes", "paytm", "ibl"]
FIRST_NAMES = [
    "RAHUL", "PRIYA", "AMIT", "NEHA", "SURESH", "DEEPAK", "POOJA", "RAJESH",
    "SUNITA", "VIVEK", "ANITA", "MANOJ", "KAVITA", "SANJAY", "REKHA",
    "VIKRAM", "MEENA", "ASHOK", "GEETA", "KIRAN", "ARUN", "PRATIK",
]
LAST_NAMES = [
    "KUMAR", "SHARMA", "SINGH", "PATEL", "GUPTA", "JOSHI", "VERMA", "YADAV",
    "CHAUHAN", "PANDEY", "MISHRA", "NEGI", "BISHT", "TOMAR", "KAPRI",
    "PATIL", "BHATT", "SAH", "DAS", "REDDY", "IYER", "MURTHY",
]


def generate_random_vpa() -> str:
    """Generate a random UPI VPA string like: VPA user123@ybl FIRSTNAME LASTNAME"""
    suffix = random.choice(UPI_SUFFIXES)
    first = random.choice(FIRST_NAMES)
    last = random.choice(LAST_NAMES)
    # VPA handle variations
    handle_type = random.choice(["phone", "name", "qr", "alphanumeric"])
    if handle_type == "phone":
        handle = f"{random.randint(6000000000, 9999999999)}"
    elif handle_type == "name":
        rand_chars = "".join(random.choices(string.ascii_lowercase, k=random.randint(2, 6)))
        handle = f"{first.lower()}{rand_chars}"
    elif handle_type == "qr":
        prefix = random.choice(["paytmqr", "gpay-", "Getepay.", "Vyapar."])
        handle = f"{prefix}{random.randint(100000, 9999999)}"
    else:
        handle = f"Q{random.randint(100000000, 999999999)}"
    return f"VPA {handle}@{suffix} {first} {last}"


def generate_merchant_text(payee_name: str) -> str:
    """Generate a plausible merchant text for a known merchant payee."""
    # For non-UPI payees, the merchant name appears directly in the email
    return payee_name.upper()


def fetch_transactions(api_url: str, budget_id: str) -> list[dict]:
    """Fetch all normalized transactions from the API."""
    url = f"{api_url.rstrip('/')}/api/transactions/normalized"
    req = Request(url, headers={"X-Budget-ID": budget_id})
    try:
        with urlopen(req) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except HTTPError as e:
        print(f"API error {e.code}: {e.read().decode()}", file=sys.stderr)
        sys.exit(1)
    except URLError as e:
        print(f"Connection error: {e.reason}", file=sys.stderr)
        sys.exit(1)


def is_upi_transaction(email_text: str) -> bool:
    """Check if an email contains a UPI/VPA payment (vs a card swipe or merchant)."""
    return "VPA " in email_text or "vpa " in email_text.lower()


def detect_upi_payees(real_emails: list[dict]) -> set[str]:
    """Detect which payees are typically UPI-based by looking at real email texts."""
    payee_upi_count: dict[str, int] = defaultdict(int)
    payee_total: dict[str, int] = defaultdict(int)
    for record in real_emails:
        payee = record.get("payee", "")
        if not payee:
            continue
        payee_total[payee] += 1
        if is_upi_transaction(record.get("email_text", "")):
            payee_upi_count[payee] += 1
    # A payee is UPI-based if >50% of its real emails contain VPA
    upi_payees = set()
    for payee, total in payee_total.items():
        if payee_upi_count.get(payee, 0) / total > 0.5:
            upi_payees.add(payee)
    return upi_payees


def format_date_for_email(iso_date: str) -> str:
    """Convert ISO date (2025-08-06) to DD-MM-YY format for email templates."""
    try:
        parts = iso_date.split("T")[0].split("-")
        if len(parts) == 3:
            return f"{parts[2]}-{parts[1]}-{parts[0][-2:]}"
    except (IndexError, ValueError):
        pass
    return "01-01-26"


def augment_from_transactions(
    transactions: list[dict],
    prediction_txn_ids: set[str],
    upi_payees: set[str],
    account_config: dict[str, dict],
    samples_per_upi_txn: int = 3,
) -> list[dict]:
    """
    Generate synthetic training data from transactions that don't have predictions.

    For each non-prediction transaction:
    - If the payee is UPI-based: generate multiple synthetic emails with random VPAs
      (because the model needs to learn that different VPAs can map to the same payee)
    - If the payee is merchant-based: generate one email with the merchant name
    - Skip transactions without payee/category/account names
    """
    augmented = []

    for txn in transactions:
        txn_id = txn.get("id", "")
        # Skip transactions that already have real prediction emails
        if txn_id in prediction_txn_ids:
            continue

        payee = txn.get("payeeName") or ""
        category = txn.get("categoryName") or ""
        account = txn.get("accountName") or ""
        amount = txn.get("amount", 0.0)
        date = format_date_for_email(txn.get("date", ""))

        # Skip if missing key labels
        if not payee or not account:
            continue

        # Skip transfer transactions (account-to-account)
        if txn.get("transferAccountId"):
            continue

        is_upi = payee in upi_payees
        n_samples = samples_per_upi_txn if is_upi else 1

        for _ in range(n_samples):
            if is_upi:
                payee_text = generate_random_vpa()
            else:
                payee_text = generate_merchant_text(payee)

            email = generate_synthetic_email(
                payee_text=payee_text,
                amount=amount,
                date=date,
                account=account,
                account_config=account_config,
            )
            if email is None:
                continue

            augmented.append({
                "email_text": email,
                "payee": payee,
                "category": category,
                "account": account,
                "amount": amount,
                "date": date,
            })

    return augmented


# ---------------------------------------------------------------------------
# Merge
# ---------------------------------------------------------------------------


def merge_datasets(*datasets: list[dict]) -> list[dict]:
    """Merge multiple training data lists into one, deduplicating by email_text."""
    seen = set()
    merged = []
    for dataset in datasets:
        for record in dataset:
            key = record.get("email_text", "")
            if key and key not in seen:
                seen.add(key)
                merged.append(record)
    return merged


# ---------------------------------------------------------------------------
# Prediction accuracy report
# ---------------------------------------------------------------------------


def print_prediction_accuracy(predictions: list[dict]) -> None:
    """Report how many predictions were correct vs. corrected by the user."""
    total = len(predictions)
    if total == 0:
        print("No predictions to analyze.")
        return

    has_corrected_flag = sum(1 for p in predictions if "hasUserCorrected" in p)
    if has_corrected_flag == 0:
        print("No correction metadata found (hasUserCorrected field missing).")
        return

    corrected = [p for p in predictions if p.get("hasUserCorrected")]
    uncorrected = [p for p in predictions if not p.get("hasUserCorrected")]
    no_email = sum(1 for p in predictions if not p.get("emailText"))

    # Per-field breakdown
    payee_corrected = sum(1 for p in corrected if p.get("userCorrectedPayee"))
    category_corrected = sum(1 for p in corrected if p.get("userCorrectedCategory"))
    account_corrected = sum(1 for p in corrected if p.get("userCorrectedAccount"))

    # Unresolved: no correction and empty predicted values
    empty_payee = sum(1 for p in predictions if not p.get("payee") and not p.get("userCorrectedPayee"))
    empty_category = sum(1 for p in predictions if not p.get("category") and not p.get("userCorrectedCategory"))

    print(f"\n{'='*60}")
    print("PREDICTION ACCURACY REPORT")
    print(f"{'='*60}")
    print(f"Total predictions:     {total}")
    print(f"Without email text:    {no_email} (skipped)")
    print(f"Correct (untouched):   {len(uncorrected)}  ({len(uncorrected)/total*100:.1f}%)")
    print(f"User corrected:        {len(corrected)}  ({len(corrected)/total*100:.1f}%)")
    print()
    print("Per-field corrections (out of corrected):")
    print(f"  Payee corrected:     {payee_corrected}")
    print(f"  Category corrected:  {category_corrected}")
    print(f"  Account corrected:   {account_corrected}")
    print()
    print("Unresolved (empty prediction, never corrected):")
    print(f"  Empty payee:         {empty_payee}")
    print(f"  Empty category:      {empty_category}")
    print(f"{'='*60}")


# ---------------------------------------------------------------------------
# Stats
# ---------------------------------------------------------------------------


def print_stats(data: list[dict]) -> None:
    """Print label distribution statistics for a training dataset."""
    total = len(data)
    with_email = sum(1 for d in data if d.get("email_text"))

    payee_counts: dict[str, int] = defaultdict(int)
    category_counts: dict[str, int] = defaultdict(int)
    account_counts: dict[str, int] = defaultdict(int)

    for d in data:
        if not d.get("email_text"):
            continue
        payee_counts[d.get("payee", "(empty)")] += 1
        category_counts[d.get("category", "(empty)")] += 1
        account_counts[d.get("account", "(empty)")] += 1

    print(f"\nTotal records: {total}  |  With email_text: {with_email}")
    print(f"Unique payees: {len(payee_counts)}  |  Unique categories: {len(category_counts)}  |  Unique accounts: {len(account_counts)}")

    for label_name, counts in [("Payees", payee_counts), ("Categories", category_counts), ("Accounts", account_counts)]:
        print(f"\n{label_name}:")
        for name, count in sorted(counts.items(), key=lambda x: -x[1]):
            print(f"  {name}: {count}")


# ---------------------------------------------------------------------------
# File I/O helpers
# ---------------------------------------------------------------------------


def load_json(path: str) -> list[dict]:
    with open(path, "r") as f:
        return json.load(f)


def save_json(data: list[dict], path: str) -> None:
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    with open(path, "w") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
    print(f"Saved {len(data)} records to {path}")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

DEFAULT_OUTPUT = "./data/normalized_with_email.json"


def extract_unresolved(predictions: list[dict]) -> list[dict]:
    """
    Extract predictions where payee or category ended up empty after resolving
    corrections. Keeps the prediction ID and raw fields for manual inspection.
    """
    unresolved = []
    for pred in predictions:
        if not pred.get("emailText"):
            continue

        corrected = pred.get("hasUserCorrected", False)
        if corrected:
            payee = pred.get("userCorrectedPayee") or pred.get("payee") or ""
            category = pred.get("userCorrectedCategory") or pred.get("category") or ""
        else:
            payee = pred.get("payee") or ""
            category = pred.get("category") or ""

        if not payee or not category:
            unresolved.append({
                "id": pred.get("id", ""),
                "transactionId": pred.get("transactionId", ""),
                "emailText": pred.get("emailText", ""),
                "amount": pred.get("amount", 0.0),
                "predictedPayee": pred.get("payee") or "",
                "predictedCategory": pred.get("category") or "",
                "predictedAccount": pred.get("account") or "",
                "payeeConfidence": pred.get("payeePrediction"),
                "categoryConfidence": pred.get("categoryPrediction"),
                "accountConfidence": pred.get("accountPrediction"),
                "hasUserCorrected": corrected,
                "userCorrectedPayee": pred.get("userCorrectedPayee") or "",
                "userCorrectedCategory": pred.get("userCorrectedCategory") or "",
                "userCorrectedAccount": pred.get("userCorrectedAccount") or "",
                "resolvedPayee": payee,
                "resolvedCategory": category,
            })
    return unresolved


def cmd_fetch(args):
    print(f"Fetching predictions from {args.api_url} (budget: {args.budget_id})...")
    predictions = fetch_predictions(args.api_url, args.budget_id)
    print(f"Received {len(predictions)} predictions")

    print_prediction_accuracy(predictions)

    # Separate out unresolved predictions for manual review
    unresolved = extract_unresolved(predictions)
    if unresolved:
        unresolved_path = os.path.join(os.path.dirname(args.output) or ".", "unresolved_predictions.json")
        save_json(unresolved, unresolved_path)
        print(f"\n⚠ {len(unresolved)} predictions have empty payee or category — saved to {unresolved_path} for review")

    training_data = predictions_to_training_data(predictions)
    print(f"\nConverted {len(training_data)} records with email text")

    save_json(training_data, args.output)
    print_stats(training_data)


def cmd_generate(args):
    transactions = load_json(args.input)
    account_config = load_account_config(args.accounts_config)

    synthetic = generate_synthetic_data(transactions, account_config)
    print(f"Generated {len(synthetic)} synthetic emails from {len(transactions)} transactions")

    save_json(synthetic, args.output)
    print_stats(synthetic)


def cmd_merge(args):
    datasets = []
    for path in args.inputs:
        data = load_json(path)
        print(f"Loaded {len(data)} records from {path}")
        datasets.append(data)

    merged = merge_datasets(*datasets)
    save_json(merged, args.output)
    print_stats(merged)


def cmd_stats(args):
    data = load_json(args.input)
    print_stats(data)


def cmd_augment(args):
    account_config = load_account_config(args.accounts_config)

    # 1. Fetch all normalized transactions
    print(f"Fetching transactions from {args.api_url}...")
    transactions = fetch_transactions(args.api_url, args.budget_id)
    print(f"Fetched {len(transactions)} transactions")

    # 2. Fetch prediction IDs so we skip those transactions (they have real emails)
    prediction_txn_ids: set[str] = set()
    if args.skip_predictions:
        print("Fetching predictions from API to skip those transactions...")
        predictions = fetch_predictions(args.api_url, args.budget_id)
        for pred in predictions:
            tid = pred.get("transactionId", "")
            if tid:
                prediction_txn_ids.add(tid)
        print(f"Found {len(prediction_txn_ids)} prediction transaction IDs to skip")

    # 3. Load real emails to detect UPI payees
    real_emails: list[dict] = []
    if args.real_data and os.path.exists(args.real_data):
        real_emails = load_json(args.real_data)
        print(f"Loaded {len(real_emails)} real emails for UPI payee detection")

    upi_payees = detect_upi_payees(real_emails)
    print(f"Detected {len(upi_payees)} UPI-based payees: {sorted(upi_payees)}")

    # 4. Generate synthetic data from non-prediction transactions
    augmented = augment_from_transactions(
        transactions=transactions,
        prediction_txn_ ids=prediction_txn_ids,
        upi_payees=upi_payees,
        account_config=account_config,
        samples_per_upi_txn=args.upi_samples,
    )
    print(f"Generated {len(augmented)} augmented records from {len(transactions)} transactions")

    if args.merge_with and os.path.exists(args.merge_with):
        existing = load_json(args.merge_with)
        print(f"Merging with {len(existing)} existing records from {args.merge_with}")
        augmented = merge_datasets(existing, augmented)

    save_json(augmented, args.output)
    print_stats(augmented)


def main():
    parser = argparse.ArgumentParser(
        description="Prepare training data for Pennywise MLP models"
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    # fetch
    fetch_parser = subparsers.add_parser(
        "fetch", help="Fetch predictions from API and convert to training data"
    )
    fetch_parser.add_argument("--api-url", required=True, help="go-pennywise-api base URL (e.g. http://localhost:5151)")
    fetch_parser.add_argument("--budget-id", required=True, help="Budget UUID for X-Budget-ID header")
    fetch_parser.add_argument("--output", default=DEFAULT_OUTPUT, help=f"Output JSON path (default: {DEFAULT_OUTPUT})")

    # generate
    gen_parser = subparsers.add_parser(
        "generate", help="Generate synthetic emails for bootstrapping new labels"
    )
    gen_parser.add_argument("--input", required=True, help="JSON file with transactions to synthesize emails for")
    gen_parser.add_argument("--accounts-config", default="accounts_config.json", help="Account config JSON (default: accounts_config.json)")
    gen_parser.add_argument("--output", default="./data/synthetic.json", help="Output JSON path")

    # merge
    merge_parser = subparsers.add_parser(
        "merge", help="Merge multiple training data files (deduplicates by email_text)"
    )
    merge_parser.add_argument("inputs", nargs="+", help="JSON files to merge")
    merge_parser.add_argument("--output", default=DEFAULT_OUTPUT, help=f"Output JSON path (default: {DEFAULT_OUTPUT})")

    # stats
    stats_parser = subparsers.add_parser(
        "stats", help="Print label distribution stats for a training data file"
    )
    stats_parser.add_argument("--input", default=DEFAULT_OUTPUT, help=f"Training data JSON (default: {DEFAULT_OUTPUT})")

    # augment
    aug_parser = subparsers.add_parser(
        "augment", help="Generate synthetic training data from existing transactions (not in predictions)"
    )
    aug_parser.add_argument("--api-url", required=True, help="go-pennywise-api base URL")
    aug_parser.add_argument("--budget-id", required=True, help="Budget UUID for X-Budget-ID header")
    aug_parser.add_argument("--skip-predictions", action="store_true", help="Fetch predictions from API and skip those transactions")
    aug_parser.add_argument("--real-data", help="Existing training data JSON (for UPI payee detection)")
    aug_parser.add_argument("--accounts-config", default="accounts_config.json", help="Account config JSON")
    aug_parser.add_argument("--upi-samples", type=int, default=3, help="Synthetic emails per UPI transaction (default: 3)")
    aug_parser.add_argument("--merge-with", help="Merge augmented data with an existing training data file")
    aug_parser.add_argument("--output", default="./data/augmented.json", help="Output JSON path")

    args = parser.parse_args()
    {"fetch": cmd_fetch, "generate": cmd_generate, "merge": cmd_merge, "stats": cmd_stats, "augment": cmd_augment}[args.command](args)


if __name__ == "__main__":
    main()
