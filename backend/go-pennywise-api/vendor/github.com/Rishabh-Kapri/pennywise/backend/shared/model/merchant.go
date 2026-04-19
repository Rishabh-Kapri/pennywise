package model

import (
	"time"

	"github.com/google/uuid"
)

type GlobalMCCTag string

const (
	// 🍔 Food & Dining
	FoodDelivery GlobalMCCTag = "FOOD_DELIVERY"
	FastFood     GlobalMCCTag = "FAST_FOOD"
	DiningOut    GlobalMCCTag = "DINING_OUT"
	CoffeeShop   GlobalMCCTag = "COFFEE_SHOP"

	// 🛒 Groceries & Daily Needs
	Groceries     GlobalMCCTag = "GROCERIES"
	QuickCommerce GlobalMCCTag = "QUICK_COMMERCE"
	Pharmacy      GlobalMCCTag = "PHARMACY"

	// 🛍️ Shopping & Retail
	E_Commerce          GlobalMCCTag = "E_COMMERCE"
	ShoppingClothing    GlobalMCCTag = "SHOPPING_CLOTHING"
	ShoppingElectronics GlobalMCCTag = "SHOPPING_ELECTRONICS"
	ShoppingFurniture   GlobalMCCTag = "SHOPPING_FURNITURE"
	ShoppingGeneral     GlobalMCCTag = "SHOPPING_GENERAL"

	// 🏡 Housing & Utilities
	RentMortgage       GlobalMCCTag = "RENT_MORTGAGE"
	UtilityElectricity GlobalMCCTag = "UTILITY_ELECTRICITY"
	UtilityWater       GlobalMCCTag = "UTILITY_WATER"
	UtilityGas         GlobalMCCTag = "UTILITY_GAS"
	UtilityBroadband   GlobalMCCTag = "UTILITY_BROADBAND"
	TelecomMobile      GlobalMCCTag = "TELECOM_MOBILE"
	HomeMaintenance    GlobalMCCTag = "HOME_MAINTENANCE"

	// 🚗 Transit & Travel
	TransportLocal GlobalMCCTag = "TRANSPORT_LOCAL"
	TransitPublic  GlobalMCCTag = "TRANSIT_PUBLIC"
	TravelFlights  GlobalMCCTag = "TRAVEL_FLIGHTS"
	TravelTrains   GlobalMCCTag = "TRAVEL_TRAINS"
	TravelHotels   GlobalMCCTag = "TRAVEL_HOTELS"

	// 🍿 Subscriptions & Entertainment
	SubscriptionVideo    GlobalMCCTag = "SUBSCRIPTION_VIDEO"
	SubscriptionAudio    GlobalMCCTag = "SUBSCRIPTION_AUDIO"
	SubscriptionSoftware GlobalMCCTag = "SUBSCRIPTION_SOFTWARE"
	SubscriptionDigital  GlobalMCCTag = "SUBSCRIPTION_DIGITAL"
	EntertainmentMovies  GlobalMCCTag = "ENTERTAINMENT_MOVIES"
	EntertainmentEvents  GlobalMCCTag = "ENTERTAINMENT_EVENTS"
	Gaming               GlobalMCCTag = "GAMING"

	// 🧘🏽 Health & Wellness
	MedicalHospital GlobalMCCTag = "MEDICAL_HOSPITAL"
	FitnessGym      GlobalMCCTag = "FITNESS_GYM"
	Sports          GlobalMCCTag = "SPORTS"
	GroomingSalon   GlobalMCCTag = "GROOMING_SALON"

	// 💳 Financial & Obligations
	BillCreditCard   GlobalMCCTag = "BILL_CREDIT_CARD"
	BillEmi          GlobalMCCTag = "BILL_EMI"
	Tax              GlobalMCCTag = "TAX"
	InsuranceLife    GlobalMCCTag = "INSURANCE_LIFE"
	InsuranceHealth  GlobalMCCTag = "INSURANCE_HEALTH"
	InsuranceVehicle GlobalMCCTag = "INSURANCE_VEHICLE"

	// 📈 Wealth & Investments
	InvestmentMutualFund GlobalMCCTag = "INVESTMENT_MUTUAL_FUND"
	InvestmentStocks     GlobalMCCTag = "INVESTMENT_STOCKS"
	InvestmentCrypto     GlobalMCCTag = "INVESTMENT_CRYPTO"
	InvestmentGold       GlobalMCCTag = "INVESTMENT_GOLD"
	InvestmentFdRd       GlobalMCCTag = "INVESTMENT_FD_RD"
	InvestmentNppPf      GlobalMCCTag = "INVESTMENT_NPS_PPF"

	// 👪 Life & Family
	EducationFees   GlobalMCCTag = "EDUCATION_FEES"
	PetCare         GlobalMCCTag = "PET_CARE"
	Children        GlobalMCCTag = "CHILDREN"
	CharityDonation GlobalMCCTag = "CHARITY_DONATION"
	Gift            GlobalMCCTag = "GIFTS"

	// 💵 Income
	IncomeSalary           GlobalMCCTag = "INCOME_SALARY"
	IncomeFreelance        GlobalMCCTag = "INCOME_FREELANCE"
	IncomeBusiness         GlobalMCCTag = "INCOME_BUSINESS"
	IncomeRewardCashback   GlobalMCCTag = "INCOME_REWARD_CASHBACK"
	IncomeRefund           GlobalMCCTag = "INCOME_REFUND"
	IncomeInterestDividend GlobalMCCTag = "INCOME_INTEREST_DIVIDEND"

	// 🔄 System & Transfers
	TransferSelf   GlobalMCCTag = "TRANSFER_SELF"
	TransferPp     GlobalMCCTag = "TRANSFER_P2P"
	CashWithdrawal GlobalMCCTag = "CASH_WITHDRAWAL"
	WalletTopup    GlobalMCCTag = "WALLET_TOPUP"
	ChargesFees    GlobalMCCTag = "CHARGES_FEES"
)

type GlobalMerchant struct {
	ID            uuid.UUID    `json:"id"`
	CanonicalName string       `json:"canonical_name"`
	MCCTag        GlobalMCCTag `json:"mcc_tag"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type GlobalMerchantMapping struct {
	CleanedRawText string `json:"cleaned_raw_text"`
	MerchantID     uuid.UUID
	CreatedAt      time.Time
}
