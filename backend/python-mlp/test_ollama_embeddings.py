"""
Test bge-m3 embeddings and LLM classification via Ollama for Indian UPI transaction classification.

Usage:
  1. Start Ollama: ollama serve (or via Docker)
  2. Pull models: ollama pull bge-m3 && ollama pull gemma4
  3. Run embeddings tests:  python test_ollama_embeddings.py
  4. Run LLM tests:         python test_ollama_embeddings.py --llm
  5. Run LLM with model:    python test_ollama_embeddings.py --llm --model gemma4

Embedding tests:
  - Understand Hindi merchant names (MISHTHAN BHANDAR = sweet shop)
  - Cluster similar merchants (two different salons → similar embeddings)
  - Distinguish different categories for random-name UPIs
  - Handle VPA handle parsing (ubuntusalons → salon)

LLM tests:
  - Classify UPI transactions into payee + category
  - Test keyword-based reasoning (SALONS → Purchases, BHANDAR → Dining)
  - Test amount-based reasoning (₹40 → auto, ₹56 → groceries)
  - Test person-to-person transfer detection (₹2200 → Family)
"""

import json
import os
import sys
import time

import numpy as np
import requests
from sentence_transformers import SentenceTransformer

model = SentenceTransformer("BAAI/bge-m3")

OLLAMA_URL = "http://192.168.1.34:11434"
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
OPENAI_URL = "https://api.openai.com/v1/chat/completions"


def get_embeddings(texts: list[str]) -> np.ndarray:
    """Get embeddings for a batch of texts via Ollama."""
    resp = requests.post(f"{OLLAMA_URL}/api/embed", json={
        # "model": "bge-m3",
        "model": "embeddinggemma:300m",
        "input": texts,
    })
    resp.raise_for_status()
    return np.array(resp.json()["embeddings"])

def get_sentence_transformer_embeddings(texts: list[str]) -> np.ndarray:
    embedding = model.encode(texts)
    return embedding.tolist()

def cosine_similarity(a: np.ndarray, b: np.ndarray) -> float:
    return float(np.dot(a, b) / (np.linalg.norm(a) * np.linalg.norm(b)))


def test_similarity_matrix(title: str, sentences: list[str]):
    """Print a cosine similarity matrix for a group of sentences."""
    print(f"\n{'='*100}")
    print(f"TEST: {title}")
    print(f"{'='*100}")
    
    embeddings = get_embeddings(sentences)
    
    # Print similarity matrix
    max_label_len = max(len(s[:50]) for s in sentences)
    header = " " * (max_label_len + 2) + "  ".join(f"[{i}]  " for i in range(len(sentences)))
    print(f"\n{header}")
    
    for i, sent in enumerate(sentences):
        label = sent[:max_label_len].ljust(max_label_len)
        scores = []
        for j in range(len(sentences)):
            sim = cosine_similarity(embeddings[i], embeddings[j])
            scores.append(f"{sim:.3f}")
        print(f"[{i}] {label}  {'  '.join(scores)}")
    print()


def test_pair(label: str, text_a: str, text_b: str, embeddings_cache: dict,
              expect: str = "high"):
    """Test similarity between two texts with expectation-aware grading.
    
    expect: "high" (same merchant), "medium" (related), "low" (different), "unknown" (no expectation)
    
    bge-m3 score ranges (calibrated from test results):
      HIGH:   > 0.55  (same/similar merchants)
      MEDIUM: 0.35 - 0.55  (same domain, different type)
      LOW:    < 0.45  (completely different categories)
    """
    for text in [text_a, text_b]:
        if text not in embeddings_cache:
            embeddings_cache[text] = get_embeddings([text])[0]
    
    sim = cosine_similarity(embeddings_cache[text_a], embeddings_cache[text_b])
    
    if expect == "high":
        indicator = "✅" if sim > 0.65 else "⚠️" if sim > 0.55 else "❌"
    elif expect == "medium":
        indicator = "✅" if 0.35 <= sim <= 0.55 else "⚠️" if sim <= 0.65 else "❌"
    elif expect == "low":
        indicator = "✅" if sim < 0.45 else "⚠️" if sim < 0.55 else "❌"
    else:  # unknown
        indicator = "•"
    
    print(f"  {indicator} {sim:.3f}  {label}")


# =========================================================================
# LLM Classification Tests
# =========================================================================

LLM_MODEL = "gemma4"

CLASSIFICATION_PROMPT = """You are a personal finance transaction classifier for an Indian user's budget app. Your goal is to predict the "payee" and "category" from bank alert text.

## PHASE 1: REASONING
Analyze the keywords first. If no keywords match, analyze the amount. Explain your logic in the "reasoning" field.

## PHASE 2: PAYEE EXTRACTION & CLEANUP
- If the text contains "SALARY TRANSFER", set payee to "Salary".
- If it contains "DMART READY", set payee to "D-Mart".
- If it contains "BLINKIT", set payee to "Blinkit".
- If it contains "INTERGLOBE AVIATION" or "INDIGO", set payee to "Indigo".
- If it contains "AIRTEL", set payee to "Airtel".
- If it contains "salon", "barber", or "parlour", set payee to "Barber".
- If "credited" or "CASHBACK" is in the text, set payee to "Cashback" (if not salary).
- For personal VPAs (random names):
    - ₹10-₹100 -> Use "Shop" or "Auto".
    - ₹100-₹500 -> Use "Shop" or "Barber" or actual name.
    - ₹500+ -> Use the Person's Actual Name.

## PHASE 3: CATEGORY SELECTION (MUST be exactly from list below)
1. SPECIAL RULES (Highest Priority):
   - If the transaction is a credit ("credited to your account" or "CASHBACK"), the category is ALWAYS "Inflow: Ready to Assign".
   - If amount is >₹10000 to a person on the 1st of the month, the category is "New Rent (HRA)".

2. KEYWORD RULES:
   - airtel, jio, vi, telecom, recharge -> 📱 Phone Bill
   - indigo, aviation, flight, booking, makemytrip -> ✈️ Travel - LT
   - zudio, westside, lifestyle, pantaloons, myntra, ajio, h&m, zara -> 👕 Clothing
   - electricity, water, bescom, online bill, bill payment -> 📑 Bills
   - salon, barber, haircut, parlour -> 🛍️ Purchases (Accesories, Equipments, etc)
   - kirana, grocery, mart, blinkit, zepto -> 🛒 Groceries
   - restaurant, cafe, dhaba, swiggy, zomato, bhandar, mithai -> 🍽️ Dining Out/Entertainment
   - medical, pharmacy, medplus, 1mg -> 💊 Meds
   - petrol, fuel, hp, bharat petroleum, iocl -> 🚗 Travel - ST
   - gym, fitness -> 🏋🏽 Gym

3. AMOUNT FALLBACKS (Only if NO keywords match):
   - ₹10-₹60 (no other clue) -> 🛒 Groceries
   - ₹20-₹80 (ONLY if round numbers like 20, 30, 40, 50) -> 🚗 Travel - ST
   - ₹80-₹500 -> 🛍️ Purchases (Accesories, Equipments, etc)
   - ₹500-₹5000 -> ❗ Unexpected expenses
   - ₹5000+ -> 👪 Family

## ALLOWED CATEGORIES:
{categories}

## EXAMPLES:
Email: "Rs.45.00 to VPA 8976543210@ybl SURESH KUMAR" 
Amount: ₹45
=> {{"reasoning": "Small amount to person, round number 45 suggests auto fare.", "payee": "Auto", "category": "🚗 Travel - ST", "confidence": 0.8}}

Email: "Rs.56.00 to VPA paytmqr123@ptyes ANUJ KUMAR"
Amount: ₹56
=> {{"reasoning": "Amount ₹56 in the ₹10-60 range with no other clue defaults to local grocery shop.", "payee": "Shop", "category": "🛒 Groceries", "confidence": 0.8}}

Email: "Rs.3500.00 credited by SALARY TRANSFER"
Amount: ₹3500
=> {{"reasoning": "Detected SALARY TRANSFER keyword.", "payee": "Salary", "category": "Inflow: Ready to Assign", "confidence": 0.99}}

## DATA
Email: {email_text}
Amount: ₹{amount}
Result (JSON ONLY):"""

# Categories passed dynamically — matches orchestrator's Classify(ctx, emailText, amount, categories)
DEFAULT_CATEGORIES = [
    "🛒 Groceries",
    "🍽️ Dining Out/Entertainment",
    "🚗 Travel - ST",
    "✈️ Travel - LT",
    "👕 Clothing",
    "💊 Meds",
    "📱 Phone Bill",
    "📑 Bills",
    "🏋🏽 Gym",
    "🛍️ Purchases (Accesories, Equipments, etc)",
    "❗ Unexpected expenses",
    "🎁 Gift",
    "🎂 Birthdays",
    "👪 Family",
    "💸 Ashu's pocket money",
    "🏖️ Vacation/Trips",
    "🏡 Home Renovation",
    "⚙️ Home Automation",
    "New Rent (HRA)",
    "Loan",
    "Inflow: Ready to Assign",
]

# Test cases: (email_text, amount, expected_payee, expected_category)
LLM_TEST_CASES = [
    # Easy — merchant name has clear keywords
    (
        "Dear Customer, Rs.150.00 has been debited from account **2086 to VPA ubuntusalons.99933697@hdfcbank UBUNTU SALONS on 14-06-26.",
        150.0,
        "Barber",
        "🛍️ Purchases (Accesories, Equipments, etc)",
    ),
    (
        "Dear Customer, Rs.2500.00 has been debited from account **8936 to VPA dmart@hdfcbank DMART READY on 12-04-26.",
        2500.0,
        "D-Mart",
        "🛒 Groceries",
    ),
    (
        "Dear Customer, Rs.350.00 has been debited from account **2086 to VPA Getepay.4523891@icici ANNAPURNA MISHTHAN BHANDAR on 09-04-26.",
        350.0,
        "Annapurna Mishthan Bhandar",
        "🍽️ Dining Out/Entertainment",
    ),

    # Medium — random person name, amount gives clue
    (
        "Dear Customer, Rs.56.00 has been debited from account **2086 to VPA paytmqr5ehfhj@ptyes ANUJ KUMAR on 07-04-26.",
        56.0,
        "Shop",
        "🛒 Groceries",
    ),
    (
        "Dear Customer, Rs.40.00 has been debited from account **2086 to VPA gpay-8976543210@okaxis DEEPAK YADAV on 03-04-26.",
        40.0,
        "Auto",
        "🚗 Travel - ST",
    ),
    (
        "Dear Customer, Rs.180.00 has been debited from account **2086 to VPA 9326934213@ptaxis RAJENDRAKUMAR BABURAOJI on 05-04-26.",
        180.0,
        "Shop",
        "🛍️ Purchases (Accesories, Equipments, etc)",
    ),

    # Hard — ambiguous, amount-based reasoning needed
    (
        "Dear Customer, Rs.12500.00 has been debited from account **8936 to VPA Q998954481@ybl VENKATESH MURTHY on 11-04-26.",
        12500.0,
        "Venkatesh Murthy",
        "👪 Family",
    ),
    (
        "Dear Customer, Rs.2200.00 has been debited from account **8936 to VPA priyasharma22@ybl PRIYA SHARMA on 14-04-26.",
        2200.0,
        "Priya Sharma",
        "❗ Unexpected expenses",
    ),

    # Edge cases
    (
        "Dear Customer, Rs.850.00 has been debited from account **8936 to VPA barberpoint@ybl LOOK SALONS on 07-04-26.",
        850.0,
        "Barber",
        "🛍️ Purchases (Accesories, Equipments, etc)",
    ),
    (
        "Dear Customer, Rs.3500.00 is successfully credited to your account **8936 by SALARY TRANSFER on 01-04-26.",
        3500.0,
        "Salary",
        "Inflow: Ready to Assign",
    ),
    # New Test Cases
    (
        "Dear Customer, Rs.450.00 has been debited from account **2086 to VPA apollopharmacy@hdfcbank APOLLO PHARMACY on 15-04-26.",
        450.0,
        "Apollo Pharmacy",
        "💊 Meds",
    ),
    (
        "Dear Customer, Rs.719.00 has been debited from account **2086 to VPA airtel.pay@axisbank AIRTEL PAYMENTS on 16-04-26.",
        719.0,
        "Airtel",
        "📱 Phone Bill",
    ),
    (
        "Dear Customer, Rs.552.00 has been debited from account **2086 to VPA zomato.order@hdfcbank ZOMATO on 17-04-26.",
        552.0,
        "Zomato",
        "🍽️ Dining Out/Entertainment",
    ),
    (
        "Dear Customer, Rs.340.00 has been debited from account **2086 to VPA blinkit.grocery@icici BLINKIT on 18-04-26.",
        340.0,
        "Blinkit",
        "🛒 Groceries",
    ),
    (
        "Dear Customer, Rs.4500.00 has been debited from account **2086 to VPA indigo.booking@hdfcbank INTERGLOBE AVIATION on 19-04-26.",
        4500.0,
        "Indigo",
        "✈️ Travel - LT",
    ),
    (
        "Dear Customer, Rs.2999.00 has been debited from account **2086 to VPA cultfit.members@hdfcbank CULT FIT on 20-04-26.",
        2999.0,
        "Cult Fit",
        "🏋🏽 Gym",
    ),
    (
        "Dear Customer, Rs.1899.00 has been debited from account **2086 to VPA zudio.store@tata ZUDIO on 21-04-26.",
        1899.0,
        "Zudio",
        "👕 Clothing",
    ),
    (
        "Dear Customer, Rs.2150.00 has been debited from account **2086 to VPA bescom.online@sbi BESCOM on 22-04-26.",
        2150.0,
        "Bescom",
        "📑 Bills",
    ),
    (
        "Dear Customer, Rs.25000.00 has been debited from account **2086 to VPA hitesh.landlord@ybl HITESH MEHTA on 01-04-26.",
        25000.0,
        "Hitesh Mehta",
        "New Rent (HRA)",
    ),
    (
        "Dear Customer, Rs.150.00 is successfully credited to your account **2086 by CASHBACK RECEIVED on 24-04-26.",
        150.0,
        "Cashback",
        "Inflow: Ready to Assign",
    ),
]


def classify_transaction(email_text: str, amount: float, model: str = LLM_MODEL,
                         categories: list[str] = DEFAULT_CATEGORIES) -> dict | None:
    """Call Ollama or OpenAI to classify a transaction using the LLM."""
    prompt = CLASSIFICATION_PROMPT.format(
        categories=", ".join(categories),
        email_text=email_text,
        amount=f"{abs(amount):.2f}",
    )

    is_openai = model.startswith("gpt-")

    try:
        start = time.time()
        if is_openai:
            if not OPENAI_API_KEY:
                print("  ❌ Error: OPENAI_API_KEY env var not set")
                return None
            resp = requests.post(OPENAI_URL, headers={
                "Authorization": f"Bearer {OPENAI_API_KEY}",
                "Content-Type": "application/json"
            }, json={
                "model": model,
                "messages": [{"role": "user", "content": prompt}],
                "response_format": {"type": "json_object"}
            }, timeout=60)
        else:
            resp = requests.post(f"{OLLAMA_URL}/api/generate", json={
                "model": model,
                "prompt": prompt,
                "format": "json",
                "stream": False,
            }, timeout=60)

        resp.raise_for_status()
        elapsed = time.time() - start

        result = resp.json()
        if is_openai:
            content = result["choices"][0]["message"]["content"]
            parsed = json.loads(content)
        else:
            parsed = json.loads(result.get("response", "{}"))

        parsed["_latency_ms"] = int(elapsed * 1000)
        return parsed
    except (requests.RequestException, json.JSONDecodeError, KeyError) as e:
        print(f"  ❌ Error: {e}")
        return None


def grade_result(result: dict, expected_payee: str, expected_category: str) -> str:
    """Grade a classification result against expected values."""
    if result is None:
        return "❌ ERROR"

    payee_match = result.get("payee", "").lower().strip() == expected_payee.lower().strip()
    category_match = result.get("category", "").strip() == expected_category.strip()

    if payee_match and category_match:
        return "✅"
    elif category_match:
        return "⚠️ payee"
    elif payee_match:
        return "⚠️ category"
    else:
        return "❌"


def run_llm_tests(model: str = LLM_MODEL):
    """Run all LLM classification test cases and print a summary."""
    print(f"\n{'='*100}")
    print(f"LLM CLASSIFICATION TESTS — model: {model}")
    print(f"{'='*100}")

    results = []
    for i, (email, amount, exp_payee, exp_category) in enumerate(LLM_TEST_CASES, 1):
        # Print a truncated version of the email
        email_short = email[:80] + "..." if len(email) > 80 else email
        print(f"\n  [{i}/{len(LLM_TEST_CASES)}] {email_short}")
        print(f"         Amount: ₹{abs(amount):.2f}")

        result = classify_transaction(email, amount, model=model)
        grade = grade_result(result, exp_payee, exp_category)

        if result:
            print(f"         Expected:  payee={exp_payee!r}  category={exp_category!r}")
            print(f"         Got:       payee={result.get('payee')!r}  category={result.get('category')!r}  conf={result.get('confidence', '?')}")
            if "reasoning" in result:
                print(f"         Reasoning: {result['reasoning']}")
            print(f"         {grade}  ({result['_latency_ms']}ms)")
        else:
            print(f"         {grade}")

        results.append(grade)

    # Summary
    correct = sum(1 for r in results if r == "✅")
    partial = sum(1 for r in results if r.startswith("⚠️"))
    wrong = sum(1 for r in results if r.startswith("❌"))

    print(f"\n{'='*100}")
    print(f"SUMMARY: {correct}/{len(results)} correct, {partial} partial, {wrong} wrong")
    print(f"{'='*100}")


def run_embedding_tests():
    """Run all embedding similarity tests."""
    # =========================================================================
    # TEST 1: Hindi merchant name understanding
    # Can the model tell that "MISHTHAN BHANDAR" = sweet shop = food?
    # =========================================================================
    test_similarity_matrix("Hindi Merchant Names — Should cluster by meaning", [
        "ANNAPURNA MISHTHAN BHANDAR",       # [0] Sweet shop
        "DELHI MISHTHAN BHANDAR",            # [1] Another sweet shop — should be very similar to [0]
        "SHARMA KIRANA STORE",               # [2] Grocery store — food-adjacent but different
        "UBUNTU SALONS",                     # [3] Salon — completely different
        "RAJESH MEDICAL STORE",              # [4] Pharmacy
        "ANNAPURNA RESTAURANT",              # [5] Restaurant — shares "ANNAPURNA" with [0]
    ])

    # =========================================================================
    # TEST 2: Full email text — similar salons should match
    # =========================================================================
    test_similarity_matrix("Similar Merchants in Full Email Context", [
        # Two different salons — should be VERY similar
        "debit 150 UBUNTU SALONS ubuntusalons",
        "debit 200 LOOK SALONS looksalon",
        # Two different grocery stores
        "debit 80 SHARMA KIRANA STORE sharmakiranastore",
        "debit 120 GUPTA KIRANA STORE guptakirana",
        # A restaurant — different from groceries
        "debit 350 BARBEQUE NATION barbequenation",
        # Random person name (the hard case)
        "debit 50 RAJENDRAKUMAR BABURAOJI 9326934213@ptaxis",
    ])

    # =========================================================================
    # TEST 3: The "Shop" problem — same VPA pattern, different amounts
    # Can amount + context help distinguish categories?
    # =========================================================================
    test_similarity_matrix("Amount Context for Random UPI Names", [
        "debit small groceries JAYANT RAMESH JOSHI",
        "debit large purchase JAYANT RAMESH JOSHI",
        "debit small groceries VENKATESH MURTHY",     # Different person, same context → should match [0]
        "debit large purchase VENKATESH MURTHY",       # Different person, same context → should match [1]
        "debit small groceries SHARMA KIRANA STORE",   # Known grocery → should match [0] and [2]
    ])

    # =========================================================================
    # TEST 4: Structured prefix impact
    # Does adding [DEBIT] [HDFC] context help?
    # =========================================================================
    test_similarity_matrix("Structured Prefix Impact", [
        "[DEBIT] [HDFC] UBUNTU SALONS",
        "[DEBIT] [HDFC] LOOK SALONS",
        "[CREDIT] [HDFC] UBUNTU SALONS",  # Same merchant but credit — should be somewhat different
        "[DEBIT] [PNB] UBUNTU SALONS",    # Same merchant, different bank — should be very similar to [0]
        "UBUNTU SALONS",                   # No prefix — baseline
    ])

    # =========================================================================
    # TEST 5: Known merchants — should the model distinguish these?
    # =========================================================================
    test_similarity_matrix("Known Merchants", [
        "AMAZON PAY INDIA PRIVA",
        "AMAZONIN",                        # Same merchant, different narration
        "WWW SWIGGY COM",
        "Cashfree*SWIGGY LIMITE",          # Same merchant, different narration
        "ZOMATO",
        "SPOTIFY SI",
    ])

    # =========================================================================
    # TEST 6: Pairwise tests — quick summary
    # bge-m3 thresholds: HIGH > 0.55, MEDIUM 0.35-0.55, LOW < 0.45
    # =========================================================================
    print(f"\n{'='*100}")
    print("PAIRWISE SIMILARITY TESTS (bge-m3 calibrated thresholds)")
    print(f"{'='*100}")

    cache = {}

    print("\n  Should be HIGH (>0.55) — same/similar merchants:")
    test_pair("Two sweet shops", "ANNAPURNA MISHTHAN BHANDAR", "DELHI MISHTHAN BHANDAR", cache, expect="high")
    test_pair("Two salons", "UBUNTU SALONS", "LOOK SALONS", cache, expect="high")
    test_pair("Two Amazon narrations", "AMAZON PAY INDIA PRIVA", "AMAZONIN", cache, expect="high")
    test_pair("Two Swiggy narrations", "WWW SWIGGY COM", "Cashfree*SWIGGY LIMITE", cache, expect="high")
    test_pair("Two kirana stores", "SHARMA KIRANA STORE", "GUPTA KIRANA STORE", cache, expect="high")

    print("\n  Should be MEDIUM (0.35-0.55) — same domain, different type:")
    test_pair("Sweet shop vs restaurant", "ANNAPURNA MISHTHAN BHANDAR", "BARBEQUE NATION RESTAURANT", cache, expect="medium")
    test_pair("Swiggy vs Zomato", "WWW SWIGGY COM", "ZOMATO", cache, expect="medium")
    test_pair("Kirana vs medical store", "SHARMA KIRANA STORE", "RAJESH MEDICAL STORE", cache, expect="medium")

    print("\n  Should be LOW (<0.45) — completely different:")
    test_pair("Salon vs grocery", "UBUNTU SALONS", "SHARMA KIRANA STORE", cache, expect="low")
    test_pair("Amazon vs Spotify", "AMAZON PAY INDIA PRIVA", "SPOTIFY SI", cache, expect="low")
    test_pair("Sweet shop vs salon", "ANNAPURNA MISHTHAN BHANDAR", "UBUNTU SALONS", cache, expect="low")

    print("\n  The HARD case — random person names:")
    test_pair("Two random UPI names", "RAJENDRAKUMAR BABURAOJI 9326934213@ptaxis", "VENKATESH MURTHY Q998954481@ybl", cache, expect="unknown")
    test_pair("Random name vs kirana", "RAJENDRAKUMAR BABURAOJI", "SHARMA KIRANA STORE", cache, expect="low")
    test_pair("Random name vs salon", "RAJENDRAKUMAR BABURAOJI", "UBUNTU SALONS", cache, expect="low")

    print()


if __name__ == "__main__":
    if "--llm" in sys.argv:
        # LLM classification tests
        model = LLM_MODEL
        if "--model" in sys.argv:
            idx = sys.argv.index("--model")
            if idx + 1 < len(sys.argv):
                model = sys.argv[idx + 1]
        run_llm_tests(model=model)
    else:
        # Embedding similarity tests (default)
        run_embedding_tests()
