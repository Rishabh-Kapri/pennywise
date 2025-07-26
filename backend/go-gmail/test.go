package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

type PayeePattern struct {
	Name    string
	Id      string
	Pattern []string
}

type AccountPattern struct {
	Name       string
	Suffix     string
	Pattern    []string
	Normalized string
}

type AccountInfo struct {
	Type    string
	Account string
}

// normalizing payee into groups for prediction
func normalizePayee(rawPayee string) string {
	payeeGroup := map[string]string{
		"unexpected":        "unexpected",
		"google cloud":      "google cloud",
		"google play store": "google play store",
		"petrol pump":       "petrol pump",
		"gasoline":          "petrol pump",
		"lavi's":            "lavi",
		"reconciliation":    "",
		"vishakha's":        "vishakha",
		"cred":              "transfer",
		"kotak":             "bank",
		"pnb":               "bank",
		"hdfc":              "bank",
		"gcp":               "google cloud",
		"protein":           "fitness",
		"gym":               "fitness",
		"nakpro":            "fitness",
		"muscleblaze":       "fitness",
		"myprotein":         "fitness",
		"ergo":              "insurance",
		"physiotherapy":     "doctor",
		"dentist":           "doctor",
		"neha sharma":       "doctor",
		"pathology":         "doctor",
		"labs":              "doctor",
		"diagnopien":        "doctor",
		"1mg":               "pharmacy",
		"amiya":             "pharmacy",
		"baneerjee":         "pharmacy",
		"medical":           "pharmacy",
		"chemist":           "pharmacy",
		"ola":               "taxi",
		"uber":              "taxi",
		"cab":               "taxi",
		"auto":              "taxi",
		"mutual":            "investment",
		"indmoney":          "investment",
		"funds":             "investment",
		"fund":              "investment",
		"sgb":               "investment",
		"stocks":            "investment",
		"bitwarden":         "subscription",
		"hostinger":         "subscription",
		"adobe":             "subscription",
		"spotify":           "subscription",
		"openapi":           "subscription",
		"udemy":             "subscription",
		"hotstart":          "streaming",
		"netflix":           "streaming",
		"jiocinema":         "streaming",
		"tata":              "streaming",
		"pvr":               "entertainment",
		"cinema":            "entertainment",
		"bookmyshow":        "entertainment",
		"airtel":            "bills",
		"shop":              "groceries",
		"big basket":        "groceries",
		"instamart":         "groceries",
		"licious":           "groceries",
		"blinkit":           "groceries",
		"facebook":          "shopping",
		"tailor":            "shopping",
		"clothes":           "shopping",
		"headphone":         "shopping",
		"zone":              "shopping",
		"redtape":           "shopping",
		"ikea":              "shopping",
		"vmart":             "shopping",
		"amazon":            "shopping",
		"nykaa":             "shopping",
		"minimalist":        "shopping",
		"titan":             "shopping",
		"conceptkart":       "shopping",
		"comet":             "shopping",
		"sneakers":          "shopping",
		"arc print":         "shopping",
		"barber":            "shopping",
		"zudio":             "shoppings",
		"plaza":             "shopping",
		"gaur":              "shopping",
		"bag":               "shopping",
		"avenue":            "shopping",
		"store":             "shopping",
		"myntra":            "shopping",
		"kart":              "shopping",
		"fnp":               "gifts",
		"computers":         "pc",
		"studio":            "pc",
		"steam":             "gaming",
		"hubtronics":        "electronics",
		"robu":              "electronics",
		"openelab":          "electronics",
		"rees52":            "electronics",
		"chattori":          "eating_out",
		"restaurant":        "eating_out",
		"street":            "eating_out",
		"lush":              "eating_out",
		"house":             "eating_out",
		"juggernaut":        "eating_out",
		"cafe":              "eating_out",
		"food":              "eating_out",
		"cart":              "eating_out",
		"burger":            "eating_out",
		"singh":             "eating_out",
		"imblotto":          "eating_out",
		"takana":            "eating_out",
		"teas":              "eating_out",
		"chai":              "eating_out",
		"point":             "eating_out",
		"1960":              "eating_out",
		"belgian":           "eating_out",
		"waffle":            "eating_out",
		"pizza":             "eating_out",
		"slice":             "eating_out",
		"mannar":            "eating_out",
		"corridor":          "eating_out",
		"court":             "eating_out",
		"mithas":            "eating_out",
		"c7":                "eating_out",
		"amigos":            "eating_out",
		"meghna":            "eating_out",
		"chandak":           "eating_out",
		"momos":             "eating_out",
		"donuteries":        "eating_out",
		"green":             "eating_out",
		"island":            "eating_out",
		"suko":              "eating_out",
		"thai":              "eating_out",
		"bharat":            "eating_out",
		"downtown":          "eating_out",
		"milan":             "eating_out",
		"khampa":            "eating_out",
		"chicken":           "eating_out",
		"dine":              "eating_out",
		"jimmy's":           "eating_out",
		"italian":           "eating_out",
		"mojo":              "eating_out",
		"cluckers":          "eating_out",
		"zomato/swiggy":     "travel",
		"highway":           "travel",
		"car":               "travel",
		"bike":              "travel",
		"rental":            "travel",
		"triund":            "travel",
		"trek":              "travel",
		"hotel":             "travel",
		"indigo":            "travel",
		"goibibo":           "travel",
		"cleartrip":         "travel",
		"airport":           "travel",
		"mmt":               "travel",
		"redbus":            "travel",
		"bus":               "travel",
		"train":             "travel",
		"station":           "travel",
		"paragliding":       "travel",
		"delivery":          "salary",
		"solutions":         "salary",
		"flytbase":          "salary",
		"pitaji":            "family",
		"papa":              "family",
		"maa":               "family",
		"mom":               "family",
		"mami":              "family",
		"brother":           "family",
		"mama":              "family",
		"basanti":           "family",
		"neha":              "family",
		"di":                "family",
		"bua":               "family",
		"hema":              "family",
		"kandpal":           "family",
		"mausi":             "family",
		"rakhi":             "family",
		"choti":             "family",
		"swati":             "family",
		"sahil":             "friend",
		"dasila":            "friend",
		"gautam":            "friend",
		"anshul":            "friend",
		"prashant_ds":       "friend",
		"rashi's":           "friend",
		"ashu's":            "friend",
		"sumit":             "friend",
		"negi":              "friend",
		"nitesh":            "friend",
		"loan":              "friend",
		"bhavesh":           "friend",
		"bhatt":             "friend",
	}

	rawPayee = strings.ToLower(rawPayee)

	re := regexp.MustCompile(`\w+`)
	tokens := re.FindAllString(rawPayee, -1)
	fmt.Println(tokens)

	if group, ok := payeeGroup[rawPayee]; ok {
		fmt.Println("Found exact match, returning")
		return group
	}

	knownPayees := []string{}
	for token := range payeeGroup {
		knownPayees = append(knownPayees, token)
	}

	for knownPayee, group := range payeeGroup {
		if strings.Contains(rawPayee, knownPayee) {
			fmt.Println("Found group", knownPayee, group, rawPayee)
			return group
		}
	}

	for _, token := range tokens {
		bestMatch := fuzzy.RankFindNormalizedFold(token, knownPayees)
		fmt.Printf("%+v\n", bestMatch)
		if len(bestMatch) > 0 {
			if bestMatch[0].Distance <= 3 {
				return payeeGroup[bestMatch[0].Target]
			}
		}
	}
	return rawPayee
}

func resolvePayee(normalizedEmailBody string) string {
	// @TODO: save these as aliases in the payee collection and show in UI for user to update
	payeePatterns := []PayeePattern{
		{Name: "Spotify", Id: "", Pattern: []string{"spotify"}},
		{Name: "Transfer", Pattern: []string{"cred.club", "transfer", "to credit card"}},
		{Name: "Google Cloud", Pattern: []string{"google cloud", "cybs"}},
		{Name: "Google Play Store", Pattern: []string{"playstore"}},
		{Name: "Adobe", Pattern: []string{"adobe"}},
		{Name: "Steam", Pattern: []string{"steam"}},
		{Name: "OpenAI", Pattern: []string{"openai"}},
		{Name: "Transfer: Mutual Funds", Pattern: []string{"zerodha", "iccl", "coin"}},
		{Name: "Transfer: Stocks", Pattern: []string{"indmoney"}},
		{Name: "Ashu", Pattern: []string{"divyansh"}},
		{Name: "OpenAI", Pattern: []string{"openai"}},
		{Name: "Gym", Pattern: []string{"fitness"}},
		{Name: "Shop", Pattern: []string{"@ybl", "@okaxis", "@pz", "@axis"}},
		{Name: "Airtel", Pattern: []string{"airtel"}},
		{Name: "Zomato/Swiggy", Pattern: []string{"zomato", "swiggy", "rapido"}},
	}

	// Layer 1: Exact pattern matching
	for _, payeePattern := range payeePatterns {
		for _, pattern := range payeePattern.Pattern {
			if strings.Contains(normalizedEmailBody, pattern) {
				return payeePattern.Name
			}
		}
	}

	knownPayees := []string{}
	for _, payee := range payeePatterns {
		knownPayees = append(knownPayees, payee.Name)
	}
	fmt.Println(knownPayees)

	// Optional: extract string after "at" or "to VPA"
	var possiblePayee string
	if i := strings.Index(normalizedEmailBody, " at "); i != -1 {
		possiblePayee = normalizedEmailBody[i+4:]
	} else if i := strings.Index(normalizedEmailBody, " to vpa "); i != -1 {
		possiblePayee = normalizedEmailBody[i+8:]
	} else {
		possiblePayee = normalizedEmailBody
	}
	fmt.Println(possiblePayee)

	// Layer 2: Fuzzy match with known payees
	bestMatch := fuzzy.RankFindNormalizedFold(possiblePayee, knownPayees)
	fmt.Println(bestMatch)
	if len(bestMatch) > 0 {
		// Return the top match if score is high enough
		if bestMatch[0].Distance <= 3 {
			return bestMatch[0].Target
		}
	}

	return "Unexpected"
}

func resolveAccount(normalizedEmailBody string, payee string) AccountInfo {
	// @TODO: save this as aliases/pattern in the accounts collection
	accountPattern := []AccountPattern{
		{Name: "HDFC (Salary)", Suffix: "8936", Pattern: []string{"account"}, Normalized: "savings"},
		{Name: "HDFC Credit Card", Suffix: "4432", Pattern: []string{"credit card ending"}, Normalized: "credit_card"},
		{Name: "HDFC Swiggy Credit Card", Suffix: "8799", Pattern: []string{"credit card ending"}, Normalized: "credit_card"},
		{Name: "Cash", Suffix: "", Pattern: []string{}, Normalized: "cash"},
		{Name: "PNB (Savings)", Suffix: "", Pattern: []string{}, Normalized: "savings"},
		{Name: "Kotak (Savings)", Suffix: "", Pattern: []string{}, Normalized: "savings"},
		{Name: "Kotak Credit Card", Suffix: "", Pattern: []string{}, Normalized: "credit_card"},
		{Name: "Steam", Suffix: "", Pattern: []string{}, Normalized: "credit_card"},
	}

	foundAccount := ""
	for _, account := range accountPattern {
		if strings.Contains(normalizedEmailBody, account.Suffix) {
			for _, pattern := range account.Pattern {
				if strings.Contains(normalizedEmailBody, pattern) {
					foundAccount = account.Name
					break
				}
			}
		}
	}

	accountInfo := AccountInfo{
		Type:    "debit",
		Account: foundAccount,
	}

	if payee == "Transfer" { // equality check with payeeId here
		// create a transfer transaction
		accountInfo.Type = "transfer"
		// @TODO: find a way to put other accounts here too
		accountInfo.Account = "HDFC Credit Card"
		return accountInfo
	}

	if strings.Contains(accountInfo.Account, "Credit Card") || strings.Contains(normalizedEmailBody, "debit") {
		accountInfo.Type = "debit"
	} else if strings.Contains(normalizedEmailBody, "credit") {
		accountInfo.Type = "credit"
	}
	return accountInfo
}

func test() {
	//
}

// func main() {
// 	// email := ` Dear Customer, Rs.1000.00 has been debited from account **8936 to VPA gpay-11259176055@okbizaxis The Fitness Hub on 24-06-25. Your UPI transaction reference number is 517532121623. `
// 	// email := `Dear Customer, Rs.1500.00 has been debited from account **8936 to VPA sbimops@sbi SBIMOPS on 22-06-25. Your UPI transaction reference number is 517321563571.`
// 	// email := `Dear Customer, Rs.29621.00 has been debited from account **8936 to VPA cred.club@axisb CRED Club on 04-06-25. Your UPI transaction reference number is 552123952380.`
// 	// email := `Dear Customer, Rs.650.00 has been debited from account **8936 to VPA playstore@axisbank Google Play on 25-06-25. Your UPI transaction reference number is 767127061765.`
// 	// email := ` Dear Customer, Rs.15.00 has been debited from account **8936 to VPA Q058470506@ybl Mr INDRA SINGH on 26-06-25. Your UPI transaction reference number is 517735021722.`
// 	email := ` Dear Card Member, Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 529.82 at Airtel Payments Ban on 12-06-2025 11:43:40. Authorization code:- 054056`
// 	normalizedEmail := strings.ToLower(email)
//
// 	payee := resolvePayee(normalizedEmail)
// 	accountInfo := resolveAccount(normalizedEmail, payee)
// 	normalizedPayee := normalizePayee(payee)
// 	fmt.Println(payee, normalizedPayee)
// 	fmt.Printf("%+v\n", accountInfo)
// }
