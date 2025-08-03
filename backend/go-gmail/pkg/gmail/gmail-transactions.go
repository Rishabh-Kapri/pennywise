package gmail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	// "regexp"
	// "strconv"
	"strings"
	"time"

	// "github.com/joho/godotenv"

	// "github.com/joho/godotenv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"cloud.google.com/go/firestore"
)

const (
	CONFIDENCE_THRESHOLD = 0.7
)

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type MessagePublishedData struct {
	Message PubSubMessage
}

type UserInfo struct {
	Id      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type EventData struct {
	Email     string `json:"emailAddress"`
	HistoryId uint64 `json:"historyId"`
}

type EmailMessageData struct {
	Headers []*gmail.MessagePartHeader
	Parts   []*gmail.MessagePart
}

type EmailData struct {
	Headers []*gmail.MessagePartHeader
	Body    string
}

type ParsedTransactionData struct {
	Amount   float64
	Date     string
	Payee    string
	AccName  string
	Category string
}

type Transaction struct {
	BudgetId              string  `firestore:"budgetId"`
	Date                  string  `firestore:"date"`
	Amount                float64 `firestore:"amount"`
	Note                  string  `firestore:"note"`
	CategoryId            string  `firestore:"categoryId"`
	AccountId             string  `firestore:"accountId"`
	PayeeId               string  `firestore:"payeeId"`
	TransferAccountId     string  `firestore:"transferAccountId"`
	TransferTransactionId string  `firestore:"transferTransactionId"`
	Deleted               bool    `firestore:"deleted"`
	CreatedAt             string  `firestore:"createdAt"`
	UpdatedAt             string  `firestore:"updatedAt"`
}

type PredictionRes struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

type EmailDetails struct {
	Text            string  `json:"email_text"`
	Date            string  `json:"date"`
	Amount          float64 `json:"amount"`
	TransactionType string  `json:"transaction_type"`
	Type            string  `json:"type"`
	Account         string  `json:"account"`
	Payee           string  `json:"payee"`
}

// Get the oauth2 config
func getOauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("CALLBACK_URL"),
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
	}
}

// Get the firestore client using the service account credentials
func getFirestoreClient() *firestore.Client {
	// @TODO: return single instance
	projectId := os.Getenv("PROJECT_ID")
	log.Printf("Setting up firestore client %v", projectId)
	ctx := context.Background()
	credsFile := os.Getenv("GCLOUD_SECRETS_FILE")
	opt := option.WithCredentialsFile(credsFile)
	firestoreClient, err := firestore.NewClient(ctx, projectId, opt)
	if err != nil {
		log.Fatalf("Error while setting up firestore client: %v", err.Error())
	}
	return firestoreClient
}

// Saves the refresh token to the firestore users collection
func saveTokenToFirestore(id string, refreshToken string) (string, error) {
	firestoreClient := getFirestoreClient()
	defer firestoreClient.Close()

	ctx := context.Background()

	collection := firestoreClient.Collection("users")
	updateData := []firestore.Update{
		{Path: "refresh_token", Value: refreshToken},
	}
	_, err := collection.Doc(id).Update(ctx, updateData)
	if err != nil {
		return "", err
	}
	return "Successfully updated refresh token", nil
}

// Add a gmail watch request
func setUpGmailNotifications(email string, config *oauth2.Config, token *oauth2.Token) (uint64, error) {
	projectId := os.Getenv("PROJECT_ID")
	pubsubTopic := os.Getenv("PUBSUB_TOPIC")
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token)))
	if err != nil {
		return 0, err
	}
	watchRequest := &gmail.WatchRequest{
		LabelIds:          []string{"INBOX"},
		LabelFilterAction: "include",
		TopicName:         fmt.Sprintf("projects/%s/topics/%s", projectId, pubsubTopic),
	}
	gmailUserService := gmail.NewUsersService(gmailService)
	res, err := gmailUserService.Watch(email, watchRequest).Do()
	if err != nil {
		return 0, err
	}
	return res.HistoryId, nil
}

// Returns the email message data after the specified historyId
func getMessage(email string, historyId uint64, config *oauth2.Config, token *oauth2.Token) ([]EmailData, error) {
	ctx := context.Background()
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token)))
	if err != nil {
		return nil, err
	}

	listCall := gmailService.Users.History.List(email)
	listCall.StartHistoryId(historyId)
	historyRes, err := listCall.Do()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var msgData []EmailData
	for _, res := range historyRes.History {
		for _, addedMsg := range res.MessagesAdded {
			id := addedMsg.Message.Id
			if seen[id] {
				continue
			}
			seen[id] = true
			msgRes, err := gmailService.Users.Messages.Get(email, id).Do()
			if err != nil {
				log.Printf("Error while fetching message with id: %s %v", id, err.Error())
				return nil, err
			}
			// body, err := base64.URLEncoding.DecodeString(msgRes.Payload.Body.Data)
			// if err != nil {
			// 	log.Printf("Error while decoding body: %v", err.Error())
			// }
			var bodyData strings.Builder
			for _, part := range msgRes.Payload.Parts {
				if part.MimeType == "text/html" {
					partData, err := base64.URLEncoding.DecodeString(part.Body.Data)
					if err != nil {
						log.Printf("Error while decoding part: %v", err.Error())
					}
					bodyData.Write(partData)
				}
			}
			headers := msgRes.Payload.Headers
			// parts := msgRes.Payload.Parts
			msgData = append(msgData, EmailData{Headers: headers, Body: bodyData.String()})
		}
	}
	return msgData, nil
}

// Save the history ID from gmail watch to firestore
func updateHistoryIdToFirestore(email string, historyId uint64) (string, error) {
	firestoreClient := getFirestoreClient()
	defer firestoreClient.Close()

	ctx := context.Background()

	collection := firestoreClient.Collection("gmailHistoryIds")
	query := collection.Where("email", "==", email)
	documentsToUpdate := query.Documents(ctx)
	for {
		doc, err := documentsToUpdate.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", err
		}
		loc, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			log.Printf("Error while loading timezone :%v", err.Error())
			return "", err
		}
		updateData := []firestore.Update{
			{Path: "historyId", Value: int64(historyId)},
			{Path: "lastUpdatedAt", Value: time.Now().In(loc).Format("January 02, 2006 15:04:05")},
		}
		log.Printf("Updating historyId with data: %v", updateData)
		_, err = doc.Ref.Update(ctx, updateData)
		if err != nil {
			log.Printf("Error while updating history ID: %v", err.Error())
			return "", err
		}
		log.Printf("Updated %s historyId", doc.Ref.ID)
	}
	return "Successfully update historyId", nil
}

func getAllDocuments(firestoreClient *firestore.Client, collection string) ([]*firestore.DocumentSnapshot, error) {
	collectionRef := firestoreClient.Collection(collection)

	iter := collectionRef.Documents(context.Background())
	return iter.GetAll()
}

// Creates a new transaction to firestore
func addTransactionToFirestore(transactionData ParsedTransactionData) (bool, error) {
	firestoreClient := getFirestoreClient()
	defer firestoreClient.Close()

	ctx := context.Background()
	budgetId := "Mm1kjyD58NQnNzOfx460"
	// @TODO: Add functionality to assign categories based on payee
	// unexpectedCatId := ""
	// unexpectedPayeeId := ""

	accountDocs, err := getAllDocuments(firestoreClient, "accounts")
	if err != nil {
		return false, err
	}
	payeeDocs, err := getAllDocuments(firestoreClient, "payees")
	if err != nil {
		return false, err
	}
	categoryDocs, err := getAllDocuments(firestoreClient, "categories")
	if err != nil {
		return false, err
	}
	var foundAcc map[string]any
	var foundPayee map[string]any
	var foundCategory map[string]any

	for _, acc := range accountDocs {
		if acc.Data()["name"].(string) == transactionData.AccName {
			foundAcc = acc.Data()
		}
	}
	if transactionData.Category != "null" {
		for _, cat := range categoryDocs {
			if cat.Data()["name"].(string) == transactionData.Category {
				foundCategory = cat.Data()
				break
			}
		}
	}
	if len(foundAcc) > 0 {
		for _, payee := range payeeDocs {
			if payee.Data()["name"].(string) == transactionData.Payee {
				foundPayee = payee.Data()
				break
			}
		}
		var newTransaction Transaction

		currentTime := time.Now()
		isoString := currentTime.Format("2006-01-02T15:04:05.999Z")

		payeeId := foundPayee["id"].(string)
		accountId := foundAcc["id"].(string)

		if transactionData.Category != "null" && foundCategory != nil {
			categoryId := foundCategory["id"].(string)
			newTransaction = Transaction{
				BudgetId:              budgetId,
				Date:                  transactionData.Date,
				Amount:                transactionData.Amount,
				Note:                  "",
				CategoryId:            categoryId,
				AccountId:             accountId,
				PayeeId:               payeeId,
				TransferAccountId:     "",
				TransferTransactionId: "",
				Deleted:               false,
				CreatedAt:             isoString,
				UpdatedAt:             isoString,
			}
		} else {
			newTransaction = Transaction{
				BudgetId:              budgetId,
				Date:                  transactionData.Date,
				Amount:                transactionData.Amount,
				Note:                  "",
				AccountId:             accountId,
				PayeeId:               payeeId,
				TransferAccountId:     "",
				TransferTransactionId: "",
				Deleted:               false,
				CreatedAt:             isoString,
				UpdatedAt:             isoString,
			}
		}
		log.Printf("New transaction: %v", newTransaction)
		_, _, err := firestoreClient.Collection("transactions").Add(ctx, newTransaction)
		if err != nil {
			return false, err
		}
		sendNtfyNotification(newTransaction)
	}
	return true, nil
}

// Send a push notification using ntfy
func sendNtfyNotification(createdTransaction Transaction) {
	url := "https://ntfy.sh/" + os.Getenv("NTFY_TOPIC")
	requestBody, _ := json.Marshal(createdTransaction)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error while creating ntfy request: %v", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Title", "A new transaction has been added!")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error while sending ntfy request: %v", err.Error())
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error while reading ntfy res body: %v", err.Error())
		return
	}
	log.Printf("Notification sent! %s", string(body))
}

// Returns the saved refresh token saved in firestore
func getRefreshToken(email string) (string, error) {
	firestoreClient := getFirestoreClient()
	defer firestoreClient.Close()

	ctx := context.Background()

	collection := firestoreClient.Collection("users")
	query := collection.Where("email", "==", email)
	iter := query.Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return "", err
	}
	refreshToken := doc.Data()["refresh_token"].(string)
	return refreshToken, nil
}

// Returns the previous historyId saved in firestore
func getPrevHistoryId(email string) (uint64, error) {
	firestoreClient := getFirestoreClient()
	defer firestoreClient.Close()

	ctx := context.Background()

	collection := firestoreClient.Collection("gmailHistoryIds")
	query := collection.Where("email", "==", email)
	iter := query.Documents(ctx)

	doc, err := iter.Next()
	if err == iterator.Done {
		return 0, err
	}
	historyId, ok := doc.Data()["historyId"].(int64)
	if !ok {
		return 0, errors.New("Cannot convert to int64")
	}
	var uintHistoryID uint64
	if historyId >= 0 {
		uintHistoryID = uint64(historyId)
	} else {
		// Handle the case of negative values appropriately
		// For example, you could log an error or choose a default value
		return 0, errors.New("Negative value encountered for historyId")
	}
	return uintHistoryID, nil
}

func parseEmail(html string) (*EmailDetails, error) {
	re := regexp.MustCompile(`(?i)(Dear\s+(Customer|Card Member|Card Holder).*?)\,*(\s|\S)+(\d{2}-\d{2}-\d{4}|\d{2}-\d{2}-\d{2})`)
	match := re.FindStringSubmatch(html)
	// log.Println("match 0", match[0])
	// log.Println("match 1", match[1])
	// log.Println("match 2", match[2])
	// log.Println("match 3", match[3])
	var emailDetails EmailDetails
	if len(match) > 0 {
		newlineRe := regexp.MustCompile(`\n`)
		trimmedText := newlineRe.ReplaceAllString(match[0], " ") + "."
		emailDetails.Text = trimmedText

		log.Println("📝 Email Text: ", trimmedText)

		amountRegex := regexp.MustCompile(`(?i)(Rs\.?\s?)([\d,]+\.\d{2})`)
		typeRegex := regexp.MustCompile(`(?i)(credited|debited)`)
		dateRegex := regexp.MustCompile(`\d{2}-\d{2}-\d{4}|\d{2}-\d{2}-\d{2}`)
		date := dateRegex.FindString(trimmedText)
		if len(date) > 0 {
			formattedDate := ""
			var parsed time.Time
			var err error
			if len(date) == 10 {
				parsed, err = time.Parse("02-01-2006", date) // DD-MM-YYYY
			} else if len(date) == 8 {
				parsed, err = time.Parse("02-01-06", date) // DD-MM-YY
			}
			if err == nil {
				formattedDate = parsed.Format("2006-01-02") // YYYY-MM-DD
			}
			log.Println("📅 Date: ", formattedDate)
			emailDetails.Date = formattedDate

			emailDetails.TransactionType = "debited"

			// Extract credited/debited type
			typeMatch := typeRegex.FindString(trimmedText)
			if len(typeMatch) > 0 {
				log.Println("📌 Type: ", typeMatch)
				emailDetails.TransactionType = typeMatch
			}
			// Extract amount
			amountMatch := amountRegex.FindStringSubmatch(trimmedText)
			if len(date) > 0 {
				log.Println("💰 Amount: ", amountMatch[2])
				amount, err := strconv.ParseFloat(amountMatch[2], 10)
				if err != nil {
					log.Fatalf("Error while converting amount to float: %v", err.Error())
					return nil, err
				}
				if emailDetails.TransactionType == "debited" {
					amount = -amount
				}
				emailDetails.Amount = amount
			}
		}
	}
	return &emailDetails, nil
}

func callPredictApi(emailDetails *EmailDetails) (PredictionRes, error) {
	url := os.Getenv("MLP_API") + "/predict"
	requestBody, _ := json.Marshal(&emailDetails)

	var prediction PredictionRes
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error while creating predict request: %v", err.Error())
		return prediction, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("Error while sending predict request: %v", err.Error())
		return prediction, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("Error while reading predict res body: %v", err.Error())
		return prediction, err
	}
	log.Printf("%s prediction done! %s", emailDetails.Type, string(body))
	err = json.Unmarshal((body), &prediction)
	if err != nil {
		log.Printf("Error while parsing predicted data: %v", err.Error())
		return prediction, err
	}
	return prediction, nil
}

type Predicted struct {
	Account  string
	Payee    string
	Category string
}

func getPredictedFields(parsedDetails *EmailDetails, fallbackAccount string) Predicted {
	// @TODO: handle error better
	// @TODO: log predicted fields in database along with their confidence
	predicted := Predicted{
		Account:  fallbackAccount,
		Payee:    "Unexpected",            // default payee when confidence is below CONFIDENCE_THRESHOLD
		Category: "❗ Unexpected expenses", // default category when confidence is below CONFIDENCE_THRESHOLD
	}
	// predict account first
	accountPrediction, err := callPredictApi(parsedDetails)
	if err != nil {
		log.Fatalf("Error in callPredictApi for account: %v", err)
	}
	log.Printf("Predicted account %v with Confidence %v\n", accountPrediction.Label, accountPrediction.Confidence*100)
	if accountPrediction.Confidence < CONFIDENCE_THRESHOLD {
		// Use fallback account and return unexpected for category and payee on low confidence
		return predicted
	}
	// if confidence is high enough then only move forward other fall back to other method
	predicted.Account = accountPrediction.Label
	payeeEmailDetails := &EmailDetails{
		Text:    parsedDetails.Text,
		Date:    parsedDetails.Date,
		Amount:  parsedDetails.Amount,
		Account: accountPrediction.Label,
		Type:    "payee",
	}
	payeePrediction, err := callPredictApi(payeeEmailDetails)
	if err != nil {
		log.Fatalf("Error in callPredictApi for payee: %v", err)
	}
	log.Printf("Predicted payee %v with Confidence %v\n", payeePrediction.Label, payeePrediction.Confidence*100)
	if payeePrediction.Confidence < CONFIDENCE_THRESHOLD {
		return predicted
	}
	// if confidence is high enough then only move forward
	predicted.Payee = payeePrediction.Label
	categoryEmailDetails := &EmailDetails{
		Text:    parsedDetails.Text,
		Date:    parsedDetails.Date,
		Amount:  parsedDetails.Amount,
		Account: accountPrediction.Label,
		Payee:   payeePrediction.Label,
		Type:    "category",
	}
	categoryPrediction, err := callPredictApi(categoryEmailDetails)
	if err != nil {
		log.Fatalf("Error in callPredictApi for category: %v", err)
	}
	log.Printf("Predicted category %v with Confidence %v\n", categoryPrediction.Label, categoryPrediction.Confidence*100)
	if categoryPrediction.Confidence >= CONFIDENCE_THRESHOLD {
		predicted.Category = categoryPrediction.Label
	}
	return predicted
}

func processEmail() {}

// Receives pubsub event from gmail_watch topic
func ProcessGmailHistoryId(eventData EventData) (bool, error) {
	log.Println("Processing event data:", eventData)
	email := eventData.Email
	// log.Printf("EMAIL: %v", email)
	refreshToken, err := getRefreshToken(email)
	if err != nil {
		log.Printf("Error while fetching refresh token: %v", err.Error())
		return false, err
	}

	// get access token from refresh token
	config := getOauth2Config()
	tokenSource := config.TokenSource(context.Background(), &oauth2.Token{
		RefreshToken: refreshToken,
	})
	token, err := tokenSource.Token()
	if err != nil {
		log.Printf("Error while fetching access token: %v", err.Error())
		return false, err
	}
	prevHistoryId, err := getPrevHistoryId(email)
	// prevHistoryId := uint64(3609237)
	log.Print("prevHistoryId:", prevHistoryId)
	if err != nil {
		log.Printf("Error while fetching previous history id: %v", err.Error())
		return false, err
	}

	_, err = updateHistoryIdToFirestore(email, eventData.HistoryId)
	if err != nil {
		log.Printf("Error while updating history id: %v", err.Error())
		return false, err
	}

	msgData, err := getMessage(email, prevHistoryId, config, token)
	log.Print("Email Data fetched:", len(msgData))
	if err != nil {
		log.Printf("Error while fetching messages using prevHistoryId %d, %s", prevHistoryId, err.Error())
		return false, err
	}
	// log.Printf("%v", msgData)
	for _, data := range msgData {
		// log.Print(data)
		isTransaction := false
		var accName string
		for _, header := range data.Headers {
			if header.Name != "Subject" && header.Name != "From" {
				continue
			}
			valueLower := strings.ToLower(header.Value)
			log.Print(valueLower)
			if strings.Contains(valueLower, "txn") ||
				strings.Contains(valueLower, "alert : update") ||
				strings.Contains(valueLower, "alert :  update") ||
				strings.Contains(valueLower, "view: account") {
				isTransaction = true
			}
			if strings.Contains(valueLower, "credit card") {
				accName = "HDFC Credit Card"
			}
		}
		if isTransaction && accName != "HDFC Credit Card" {
			accName = "HDFC (Salary)"
		}
		if !isTransaction {
			log.Println("Not a transaction, skipping.")
			continue
		}

		// log.Println("Email body:", data.Body)
		parsedDetails, err := parseEmail(data.Body)
		parsedDetails.Type = "account"

		if err != nil {
			log.Printf("Error while parsing email %v", err.Error())
			return false, err
		}
		log.Println("Parsed email details: ", parsedDetails)
		if parsedDetails.Amount == 0 {
			log.Println("Amount is zero, skipping.")
			continue
		}
		predictedFields := getPredictedFields(parsedDetails, accName)
		log.Println("Predicted fields: ", predictedFields)
		transactionData := ParsedTransactionData{
			Amount:   parsedDetails.Amount,
			Date:     parsedDetails.Date,
			Payee:    predictedFields.Payee,
			AccName:  predictedFields.Account,
			Category: predictedFields.Category,
		}
		log.Println("Transaction data: ", transactionData)
		_, err = addTransactionToFirestore(transactionData)
		if err != nil {
			log.Printf("Error while adding new transaction: %v", err.Error())
			return false, err
		}
	}
	return true, nil
}

func Init() *EmailDetails {
	// WatchGmailMessages()

	// text1 := `<div dir="ltr"><table align="center" width="550" border="0" cellspacing="0" cellpadding="0"><tbody><tr><td width="550" valign="top" align="center"><table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:550px"><tbody><tr><td height=""></td></tr><tr><td align="left" valign="middle" style="font-family:Arial;font-size:16px;line-height:22px;color:rgb(0,0,0)">Dear Customer, Rs.200.00 has been debited from account **8936 to VPA divyanshkapri12-1@okaxis DIVYANSH KAPRI on 07-07-25. Your UPI transaction reference number is . If you did not authorize this transaction, please report it immediately by calling 18002586161 Or SMS BLOCK UPI to 00000000. Warm Regards, HDFC Bank</td></tr></tbody></table></td></tr></tbody></table></div>`
	// text2 := `<div dir="ltr"><table align="center" width="550" border="0" cellspacing="0" cellpadding="0"><tbody><tr><td width="550" valign="top" align="center"><table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:550px"><tbody><tr><td height=""></td></tr><tr><td align="left" valign="middle" style="font-family:Arial;font-size:16px;line-height:22px;color:rgb(0,0,0)">Dear Customer, Thank you for using HDFC Bank Credit Card 4432 for Rs. 22.63 at GOOGLE CLOUD CYBS SI on 03-07-2025 22:40:35 Authorization code:- XXXXXX Please note that this transaction was conducted without OTP /PIN. If you have not authorized the above transaction, please call on 18002586161 Warm regards, HDFC Bank (This is a system generated mail and should not be replied to)</td></tr></tbody></table></td></tr></tbody></table></div>`
	// text3 := `<div dir="ltr"><table align="center" width="550" border="0" cellspacing="0" cellpadding="0"><tbody><tr><td width="550" valign="top" align="center"><table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:550px"><tbody><tr><td height=""></td></tr><tr><td align="left" valign="middle" style="font-family:Arial;font-size:16px;line-height:22px;color:rgb(0,0,0)"> Dear Customer, Rs.15000.00 is successfully credited to your account **8936 by VPA 9458306660@ybl SHEELA KAPRI on 02-07-25. Your UPI transaction reference number is 887393696385. Thank you for banking with us. Warm Regards, HDFC Bank </td></tr></tbody></table></td></tr></tbody></table></div>`
	// text4 := `Dear Card Member, Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 529.82 at Bharti Airtel Limited on 21-05-2025 13:01:46. Authorization code:- 062756 After the above transaction, the available balance on your card is Rs 401279.18 and the total outstanding is Rs 81720.82. For more details on this transaction please visit HDFC Bank MyCards. If you have not done this transaction, please immediately call on 18002586161 to report this transaction. Explore HDFC Bank MyCards: your one stop platform to manage Credit Card ON THE GO.`
	// text5 := `<!doctype html> <html> <head> <meta http-equiv="Content-Type" content="text/html; charset=utf-8"> <meta name="viewport" content="target-densitydpi=device-dpi"> <meta name="viewport" content="width=device-width; initial-scale=1.0; maximum-scale=1.0; user-scalable=0;"> <meta name="apple-mobile-web-app-capable" content="yes"> <meta name="HandheldFriendly" content="true"> <meta name="MobileOptimized" content="width"> <title>HDFC BANK</title><!-- Facebook sharing information tags --> <!--<meta property="og:title" content="HDFC BANK" />--> <style type="text/css"> @media screen and (min-device-width: 320px) and (max-device-width: 768px) { table { width: 100%; } .td { padding: 0px 25px; text-align: left; } .heading { text-align: center; padding: 0px 25px; } .cta { padding: 0px 25px; text-align: center; } } </style> </head> <body data-new-gr-c-s-loaded="14.1101.0"> <table width="600" border="0" cellspacing="0" cellpadding="0" align="center"> <tbody> <tr> <td style="border-bottom:1px solid #cccccc; border-top:1px solid #cccccc; border-left:1px solid #cccccc;border-right:1px solid #cccccc;"> <table width="100%" border="0" cellspacing="0" cellpadding="0"> <tbody> <tr> <td valign="top" bgcolor="#dcddde" style="line-height:0px;background-color:#dcddde; border-left:1px solid #dcddde;"><a href="https://trkt.aclmails.in/v1/r/ejgFR%2FF%2FW97QdNXuLdGC%2Ff%2FhZV1DI%2FZGEs5XYG3Jw0J4NIFducnPlRxDjaBzTtbKAzS2Sw1Wwagzh9ucT42mm7mb30vTtNx2Isjmr2V%2BbJDzl9nGdqFHzTEdIg%2B9dRigHRJC77FFZ5KamZPUaXHImyMPInosEFrCj5fa%2BRe6VRD4I2NMMMfVRPMYWbNOCyotZYBDylRUrymBrc%2FIGf%2FhDY4cYMaUTVOWyfP1PF%2B4QEgPpsAvK%2BIkhx%2BBcqgQnTIxKG7AmVYAITMa8EiFVdwJpZZ2l3xOAcyPevtkwC1qgzQ099iwcRaXrP73cnjlIF7ocLOR9dLaf8BtZLC6lyRuLYmtNuc2pZUZBkXC8TBAT%2BPRL0TQeMEPYsiSu%2BzGhhmmwbcyXz23%2B8U%3D" target="_blank"><img src="https://img.pinchappmails.com/hdfc/images/2024/march/D3.jpg" style="display: block; vertical-align: top" alt width="100%" border="0"></a></td> </tr> </tbody> </table> </td> </tr> <tr> <td height="20"></td> </tr> <tr> <td valign="top" bgcolor="#dcddde" style="line-height:0px;background-color:#dcddde; border-left:1px solid #dcddde;"><img src="https://wgjpss.stripocdn.email/content/guids/CABINET_d17330eb5cbe5af68cf444e0d9b181ad116d88b22fb8b3fd1c93e1846c59af76/images/nmbandnew.jpg" alt width="100%" border="0"></td> </tr> <tr> <td> <table width="100%" border="0" cellspacing="0" cellpadding="0"> <tbody> <tr> <td valign="top" align="center"> <table align="center" width="550" border="0" cellspacing="0" cellpadding="0"> <tbody> <tr> <td width="550" valign="top" align="center"> <table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:100% !important; text-align: center"> <tbody> <tr> <td height="15"></td> </tr> </tbody> </table> </td> </tr> </tbody> </table> </td> </tr> </tbody> </table> </td> </tr> <tr> <td> <table width="100%" border="0" cellspacing="0" cellpadding="0"> <tbody> <tr> <td valign="top" align="center"> <table align="center" width="550" border="0" cellspacing="0" cellpadding="0"> <tbody> <tr> <td width="550" valign="top" align="center"> <table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:100% !important; text-align: center"> <tbody> <tr> <td height></td> </tr> <tr> <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left" class="td esd-text">Dear Customer, Rs. 3000.00 is successfully credited to your account **8936 by VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP on 12-07-25. Your UPI transaction reference number is 020037866914. Thank you for banking with us. Warm Regards, HDFC Bank </td> </tr> <tr> <td height="25"></td> </tr> <tr> <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left" class="td esd-text"> </td> </tr> <tr> <td height="25"></td> </tr> <tr> <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left; border-bottom: 2px solid #cccccc; padding-bottom: 0px" class="td esd-text"></td> </tr> <tr> <td height="15"></td> </tr> </tbody> </table> </td> </tr> </tbody> </table> </td> </tr> </tbody> </table> </td> </tr> <tr> <td valign="top"  style="line-height:0px; border-left:0px solid #dcddde; padding-bottom: 5px"><img src="https://img.pinchappmails.com/hdfc/images/2024/july/Footerbanner_NEW_Aug.jpg" alt width="100%" border="0"></td> </tr> <tr> <td style="font-family:Arial,Helvetica,sans-serif;color:#000000;padding-bottom:5px;padding-top:5px;text-align:left;font-size:0.5rem;border:none;line-height:12px;padding-left:15px;padding-right:15px" align="left" class="esd-text">For more details on Service charges and Fees, <a href="https://trkt.aclmails.in/v1/r/zt2jFrPtlEFz7NWo%2FszuZybT2gt2ow5O%2Bixh8E1tIwRlbfGO1xKx3NzXCa%2BDv4FGSCmzD6UdPnNr%2FO92prG4ahOg9oSExjJdNcPBLrwoYvpDXlJAxP2O30hEZpGMQc%2BfhHbhcRlUpzmSFYJzIfnFe2stdXjXrCzp%2FGRek3OJcEtOG0xV0G%2BFYJkRKMZV6QreLXQMVsNo2WepptDDEh5tVCJ2jwDUYjuVO6LDI4GI2WGhOzpjZdxgwW1ZDnF6A%2Fy9Ihws7Uxp2LIId1lwPSXKZeEsPuLrQI7CIRUpYxCG91zSegbHjS7CIvZ9" style="text-decoration:underline;color:#004b8d;outline:none" target="_blank"><strong> click here.</strong></a></td> </tr> <tr> <td style="font-family:Arial,Helvetica,sans-serif;color:#000000;text-align:left;font-size:0.5rem;border:none;padding-left:15px" align="left" width="30%" class="esd-text">© HDFC Bank <span style="font-size: 3px"></span></td> </tr> </tbody> </table> </body> </html><img src="https://trkt.aclmails.in/v1/w/x2Jmu%2BA91wsNcN9uVwTQHBTtXYAJpx7Mifv%2FZh0u3tNswiHPhne45dY%2BlgNvScLsUv00q1HFN8vaForbsaGnq0LCgk0dDYjLaYPRdpmJlq7IYlskUuW1K8sf%2Fkan6lO56tiLlqlBM0%2F38NLwnDB82c5vLebnYgXV5hSa%2FWHgBaF9A9l56fGJXU%2B%2BHd85Dar%2FAtPESq1gGIOAqQp57PAOnQXrHm06Rw%3D%3D"  width="1" height="1" border="0">}`
	text6 := `<!doctype html>
<html>

<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
    <meta name="viewport" content="target-densitydpi=device-dpi">
    <meta name="viewport" content="width=device-width; initial-scale=1.0; maximum-scale=1.0; user-scalable=0;">
    <meta name="apple-mobile-web-app-capable" content="yes">
    <meta name="HandheldFriendly" content="true">
    <meta name="MobileOptimized" content="width">
    <title>HDFC BANK</title><!-- Facebook sharing information tags -->
    <!--<meta property="og:title" content="HDFC BANK" />-->
    <style type="text/css">
        @media screen and (min-device-width: 320px) and (max-device-width: 768px) {
            table {
                width: 100%;
            }

            .td {
                padding: 0px 25px;
                text-align: left;
            }

            .heading {
                text-align: center;
                padding: 0px 25px;
            }

            .cta {
                padding: 0px 25px;
                text-align: center;
            }
        }
    </style>
</head>

<body data-new-gr-c-s-loaded="14.1101.0">
    <table width="600" border="0" cellspacing="0" cellpadding="0" align="center">
        <tbody>
                          <tr>
                <td style="border-bottom:1px solid #cccccc; border-top:1px solid #cccccc; border-left:1px solid #cccccc;border-right:1px solid #cccccc;">
                    <table width="100%" border="0" cellspacing="0" cellpadding="0">
                        <tbody>
                            <tr>
                                <td valign="top" bgcolor="#dcddde" style="line-height:0px;background-color:#dcddde; border-left:1px solid #dcddde;"><a href="https://trkt.aclmails.in/v1/r/RpAI0iplMke1u4UTdKfW3pYkYxO29n%2FTA50xD7JN3ZXQYoAgl5MLyeMoKwHE5szKBYWEkBxk21s%2BoDXCfijyQ3lMKqJjYgLEo1MJfMUW9MZVMmyfBna0bSXtl5KxkGmmujVX3ga7azPp%2FWBhmoEP3PF8V7AemfGE8m1srZUjRQC0q0WkalBM8th7moHZgZgQaVh9kBzpUGE%2B%2F4sf594ndqKWtsXojfxcjg1RE0Oo0Xq5XNE9SZmMnrNADnXLE3fLk9UOTGMkYPOTw9FBhnus8R5Je4iO%2BvGTSnPcWZWHOOr8PrZsY3YD3Mv3bVE5SEeEYlpjq4V0u77pPTVnhyLqXiNglPQ%2FZpzxSNO8n863r5Ib0rnq89ekao9amH5p1JkaEi%2BbyN0QBM8%3D" target="_blank"><img src="https://img.pinchappmails.com/hdfc/images/2024/oct/Banner_600X130.jpg" style="display: block; vertical-align: top" alt width="100%" border="0"></a></td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
            <tr>
                <td height="20"></td>
            </tr>
            <tr>
                <td valign="top" bgcolor="#dcddde" style="line-height:0px;background-color:#dcddde; border-left:1px solid #dcddde;"><img src="https://wgjpss.stripocdn.email/content/guids/CABINET_d17330eb5cbe5af68cf444e0d9b181ad116d88b22fb8b3fd1c93e1846c59af76/images/nmbandnew.jpg" alt width="100%" border="0"></td>
            </tr>
            <tr>
                <td>
                    <table width="100%" border="0" cellspacing="0" cellpadding="0">
                        <tbody>
                            <tr>
                                <td valign="top" align="center">
                                    <table align="center" width="550" border="0" cellspacing="0" cellpadding="0">
                                        <tbody>
                                            <tr>
                                                <td width="550" valign="top" align="center">
                                                    <table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:100% !important; text-align: center">
                                                        <tbody>
                                                            <tr>
                                                                <td height="15"></td>
                                                            </tr>
                                                        </tbody>
                                                    </table>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
            <tr>
                <td>
                    <table width="100%" border="0" cellspacing="0" cellpadding="0">
                        <tbody>
                            <tr>
                                <td valign="top" align="center">
                                    <table align="center" width="550" border="0" cellspacing="0" cellpadding="0">
                                        <tbody>
                                            <tr>
                                                <td width="550" valign="top" align="center">
                                                    <table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:100% !important; text-align: center">
                                                        <tbody>
                                                            <tr>
                                                                <td height></td>
                                                            </tr>
                                                            <tr>
                                                                <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left" class="td esd-text">Dear Customer,
Rs. 100.00 is successfully credited to your account **8936 by VPA 9997167687@ibl RISHABH KAPRI SO GOKUL CHANDRA KAP on 14-07-25.
Your UPI transaction reference number is 962866801888.
Thank you for banking with us.
Warm Regards,
HDFC Bank



</td>
                                                            </tr>
                                                            <tr>
                                                                <td height="25"></td>
                                                            </tr>
                                                                                                                        <tr>
                                                                <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left" class="td esd-text">
</td>
                                                            </tr>
                                                           
                                                            <tr>
                                                                <td height="25"></td>
                                                            </tr>
                                                                                                                        <tr>
                                                                <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left; border-bottom: 2px solid #cccccc; padding-bottom: 0px" class="td esd-text"></td>
                                                            </tr>
                                                            <tr>
                                                                <td height="15"></td>
                                                            </tr>
                                                        </tbody>
                                                    </table>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
          <tr>
                <td valign="top"  style="line-height:0px; border-left:0px solid #dcddde; padding-bottom: 5px"><img src="https://img.pinchappmails.com/hdfc/images/2024/july/Footerbanner_NEW_Aug.jpg" alt width="100%" border="0"></td>
            </tr>
            
            <tr>
                <td style="font-family:Arial,Helvetica,sans-serif;color:#000000;padding-bottom:5px;padding-top:5px;text-align:left;font-size:0.5rem;border:none;line-height:12px;padding-left:15px;padding-right:15px" align="left" class="esd-text">For more details on Service charges and Fees, <a href="https://trkt.aclmails.in/v1/r/JOpD5b%2BQfYzCpiyGDwW7bi5DbOLq1YjO6IVD8ifwlDdWroDdChXfNh%2BWSntePvakCL7j6ELKgeVcAuKEd2n1ck8JOX2ToHHnEt3UXjeY4pwEbhvmXUmcdoo1x%2FydzlUU8CVDLHTX5RcqviD%2BIHBJwkhlGHd8BGh%2FwpZo8JRtSxccBLk6K8GrsYtGSmvH8ZeJflx%2B9mDaIoIuwrFKksHoFvwcIn7tE645eYDghstldipH32xELX1KI0hv5Nb%2BLrCO%2FsnSITtrFivkK8EIMMSHLjJM%2B3qysrkPh9x0POCcJYsCtOa27lGbzF9e" style="text-decoration:underline;color:#004b8d;outline:none" target="_blank"><strong> click here.</strong></a></td>
            </tr>
            
            
            <tr>
                <td style="font-family:Arial,Helvetica,sans-serif;color:#000000;text-align:left;font-size:0.5rem;border:none;padding-left:15px" align="left" width="30%" class="esd-text">© HDFC Bank <span style="font-size: 3px"></span></td>
            </tr>
                          
           
        </tbody>
    </table>
</body>

</html><img src="https://trkt.aclmails.in/v1/w/OZ9DHeQnU%2BA0ky%2Fiy7LSsQFw0qfOnsRtC6vPTx%2FsP8Ykh8ic7h10D1w%2F%2FhCxGnLQ3gnJoVuMhrFGeeXNKeeqL8NcOewtWE27AJsec4itjv3yF9002oygXINxsaUuWVbKv8nUQUVStnOOtFU78qdVmsXXbygvUA1s%2FKH1Qy%2B70zol%2F2m%2Bc1A5b1NcF%2FEHjxDrwAdJt1YGFUXh78cgGCQHMug9CYeR9w%3D%3D"  width="1" height="1" border="0">`
	text7 := `<!doctype html>
<html>

<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
    <meta name="viewport" content="target-densitydpi=device-dpi">
    <meta name="viewport" content="width=device-width; initial-scale=1.0; maximum-scale=1.0; user-scalable=0;">
    <meta name="apple-mobile-web-app-capable" content="yes">
    <meta name="HandheldFriendly" content="true">
    <meta name="MobileOptimized" content="width">
    <title>HDFC BANK</title><!-- Facebook sharing information tags -->
    <!--<meta property="og:title" content="HDFC BANK" />-->
    <style type="text/css">
        @media screen and (min-device-width: 320px) and (max-device-width: 768px) {
            table {
                width: 100%;
            }

            .td {
                padding: 0px 25px;
                text-align: left;
            }

            .heading {
                text-align: center;
                padding: 0px 25px;
            }

            .cta {
                padding: 0px 25px;
                text-align: center;
            }
        }
    </style>
</head>

<body data-new-gr-c-s-loaded="14.1101.0">
    <table width="600" border="0" cellspacing="0" cellpadding="0" align="center">
        <tbody>
                          <tr>
                <td style="border-bottom:1px solid #cccccc; border-top:1px solid #cccccc; border-left:1px solid #cccccc;border-right:1px solid #cccccc;">
                    <table width="100%" border="0" cellspacing="0" cellpadding="0">
                        <tbody>
                            <tr>
                                <td valign="top" bgcolor="#dcddde" style="line-height:0px;background-color:#dcddde; border-left:1px solid #dcddde;"><a href="https://trkt.aclmails.in/v1/r/JaGAb%2F%2B71gkQM%2FwAgfsM4HGJjZkn%2Ffo0qaHQxR5y6t9iH%2F9RJ5RJgZ84SBtmzJjt4wBotfIea1DSgRtt2JrDXZ%2FRfRRmTnrV7YvujMfA%2BVqmU3ToFHItsyFGSbHZBkZgrzcqBZxvhNJCRH2O4knZ1I4aH7njVLc6AXC1CVvFzNF6juT6F0rWif9EgI1AKgF3wXTJNBPPKFLLgjZ1QSJvkjfpZUKTuOR7gGswp4uLAHnHxjAHHVQ8xWCiDtJ%2Bw8eKCcYm3uNksUgyANtnNQxH9B4jQr1lBwLZvYgHy2OiGu7WVTP0FUDaTKTkHWoz%2BZdBIBSw9xB2SrsRWztEcDBh0jNjQs5tvCW2YzWvfW4WLJcowzg3pX1JL%2BGy5AR6bkLiSak23%2F%2FjDTeHsQtfPsW2roa1XJe%2B%2BnfSuoKPGp5Z2Tk0k3pF5ks9bH27MkFjIYvKd5W995sP3Uaz%2BmtmmCR0Ko37T2O73v8Y7e4Vz1pbI0pMmZgVyJAfZ1J7DPyOKvW4KFEYBlGD" target="_blank"><img src="https://img.pinchappmails.com/hdfc/images/2023/nov/AL.jpg" style="display: block; vertical-align: top" alt width="100%" border="0"></a></td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
            <tr>
                <td height="20"></td>
            </tr>
            <tr>
                <td valign="top" bgcolor="#dcddde" style="line-height:0px;background-color:#dcddde; border-left:1px solid #dcddde;"><img src="https://wgjpss.stripocdn.email/content/guids/CABINET_d17330eb5cbe5af68cf444e0d9b181ad116d88b22fb8b3fd1c93e1846c59af76/images/nmbandnew.jpg" alt width="100%" border="0"></td>
            </tr>
            <tr>
                <td>
                    <table width="100%" border="0" cellspacing="0" cellpadding="0">
                        <tbody>
                            <tr>
                                <td valign="top" align="center">
                                    <table align="center" width="550" border="0" cellspacing="0" cellpadding="0">
                                        <tbody>
                                            <tr>
                                                <td width="550" valign="top" align="center">
                                                    <table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:100% !important; text-align: center">
                                                        <tbody>
                                                            <tr>
                                                                <td height="15"></td>
                                                            </tr>
                                                        </tbody>
                                                    </table>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
            <tr>
                <td>
                    <table width="100%" border="0" cellspacing="0" cellpadding="0">
                        <tbody>
                            <tr>
                                <td valign="top" align="center">
                                    <table align="center" width="550" border="0" cellspacing="0" cellpadding="0">
                                        <tbody>
                                            <tr>
                                                <td width="550" valign="top" align="center">
                                                    <table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:100% !important; text-align: center">
                                                        <tbody>
                                                            <tr>
                                                                <td height></td>
                                                            </tr>
                                                            <tr>
                                                                <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left" class="td esd-text">Dear Customer,
Rs.500.00 has been debited from account 8936 to VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP on 14-07-25.
Your UPI transaction reference number is 519527171609.
If you did not authorize this transaction, please report it immediately by calling 18002586161 Or SMS BLOCK UPI to 7308080808.
Warm Regards,
HDFC Bank


</td>
                                                            </tr>
                                                            <tr>
                                                                <td height="25"></td>
                                                            </tr>

                                                                                                                        <tr>
                                                                <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left" class="td esd-text">

</td>
                                                            </tr>
                                                            <tr>
                                                                <td height="25"></td>
                                                            </tr>
                                                                                                                        <tr>
                                                                <td align="left" valign="middle" style="font-family:Arial; font-size:16px; line-height:22px; color:#000; font-weight: normal; text-align: left; border-bottom: 2px solid #cccccc; padding-bottom: 0px" class="td esd-text">
</td>
                                                            </tr>
                                                            <tr>
                                                                <td height="15"></td>
                                                            </tr>
                                                        </tbody>
                                                    </table>
                                                </td>
                                            </tr>
                                        </tbody>
                                    </table>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </td>
            </tr>
          <tr>
                <td valign="top"  style="line-height:0px; border-left:0px solid #dcddde; padding-bottom: 5px"><img src="https://img.pinchappmails.com/hdfc/images/2024/july/Footerbanner_NEW_Aug.jpg" alt width="100%" border="0"></td>
            </tr>
            
            <tr>
                <td style="font-family:Arial,Helvetica,sans-serif;color:#000000;padding-bottom:5px;padding-top:5px;text-align:left;font-size:0.5rem;border:none;line-height:12px;padding-left:15px;padding-right:15px" align="left" class="esd-text">For more details on Service charges and Fees, <a href="https://trkt.aclmails.in/v1/r/ZowjtV31xNGbgDeSjAoxZrvoCOItY4pdYEb5FjTp1DsAYhS866g8XZoMeaTj1oJnEmewhJEUhlMPXygXShn%2B9enTQ5Swq5on%2FDSPSXIY454ie%2BNLoy35pcTzC6aO%2FIqcLswSeyUswZCjrkGPW0fm%2FgrfarYCgQH%2FrVpQHmRCNuEjk4%2BpwD34%2F0XrILyxHAIAw1%2B0XVa59R8%2Bnlc7FaDRLWLr950wI8Ac1fpUhJ1vJELxxJj0FzNl94Mr4aC9buCHZEMJQhPxssdqdo9gkGmylJSj1B1e%2Fl5eIBVaxmjv4iypifWiHhP9DkXv" style="text-decoration:underline;color:#004b8d;outline:none" target="_blank"><strong> click here.</strong></a></td>
            </tr>
            
            
            <tr>
                <td style="font-family:Arial,Helvetica,sans-serif;color:#000000;text-align:left;font-size:0.5rem;border:none;padding-left:15px" align="left" width="30%" class="esd-text">© HDFC Bank <span style="font-size: 3px"></span></td>
            </tr>
                          
           
        </tbody>
    </table>
</body>

</html><img src="https://trkt.aclmails.in/v1/w/TDEA0JsMC9Leq45iCML%2Fw9HuReba5c%2BS9wrmzbnIgRh%2B0mH%2BwlEwTuMgwO14dgWEWYd2%2FPKhR9FdG2476aK2HYHv2vBdInNmD2lCS55XzKex2feC6qooIPefsamFs9f0uy1ae3LR4vaVuavny1yWS8jvVA%2BZcD2jLH4Qu0XInJC8tbLW3D7jIeD%2FMqHkVoT0XIUyX5yJUy6Ij%2F0AXRyc8tQ%2BxA6lnQ%3D%3D"  width="1" height="1" border="0">`
	//
	// parsedDetails1, _ := parseEmail(text1)
	// parsedDetails1.Type = "account"
	// predictedFields1 := getPredictedFields(parsedDetails1, "Test 1")
	// log.Println(predictedFields1)
	// transactionData1 := ParsedTransactionData{
	// 	Amount:   parsedDetails1.Amount,
	// 	Date:     parsedDetails1.Date,
	// 	Payee:    predictedFields1.Payee,
	// 	AccName:  predictedFields1.Account,
	// 	Category: predictedFields1.Category,
	// }
	// log.Print(transactionData1)
	//
	// parsedDetails2, _ := parseEmail(text2)
	// parsedDetails2.Type = "account"
	// predictedFields2 := getPredictedFields(parsedDetails2, "Test 2")
	// log.Println(predictedFields2)
	// transactionData2 := ParsedTransactionData{
	// 	Amount:   parsedDetails2.Amount,
	// 	Date:     parsedDetails2.Date,
	// 	Payee:    predictedFields2.Payee,
	// 	AccName:  predictedFields2.Account,
	// 	Category: predictedFields2.Category,
	// }
	// log.Print(transactionData2)
	//
	// parsedDetails3, _ := parseEmail(text3)
	// parsedDetails3.Type = "account"
	// predictedFields3 := getPredictedFields(parsedDetails3, "Test 3")
	// log.Println(predictedFields3)
	// transactionData3 := ParsedTransactionData{
	// 	Amount:   parsedDetails3.Amount,
	// 	Date:     parsedDetails3.Date,
	// 	Payee:    predictedFields3.Payee,
	// 	AccName:  predictedFields3.Account,
	// 	Category: predictedFields3.Category,
	// }
	// log.Print(transactionData3)
	//
	// parsedDetails4, _ := parseEmail(text4)
	// log.Print(parsedDetails4)
	// parsedDetails4.Type = "account"
	// predictedFields4 := getPredictedFields(parsedDetails4, "Test 4")
	// log.Println(predictedFields4)
	// transactionData4 := ParsedTransactionData{
	// 	Amount:   parsedDetails4.Amount,
	// 	Date:     parsedDetails4.Date,
	// 	Payee:    predictedFields4.Payee,
	// 	AccName:  predictedFields4.Account,
	// 	Category: predictedFields4.Category,
	// }
	// log.Print(transactionData4)
	//
	// parsedDetails5, _ := parseEmail(text5)
	// log.Print(parsedDetails5)
	// parsedDetails5.Type = "account"
	// predictedFields5 := getPredictedFields(parsedDetails5, "Test 5")
	// log.Println(predictedFields5)
	// transactionData5 := ParsedTransactionData{
	// 	Amount:   parsedDetails5.Amount,
	// 	Date:     parsedDetails5.Date,
	// 	Payee:    predictedFields5.Payee,
	// 	AccName:  predictedFields5.Account,
	// 	Category: predictedFields5.Category,
	// }
	// log.Print(transactionData5)

	// parseEmail(text1)
	// parseEmail(text2)
	// parseEmail(text3)

	parsedDetails6, _ := parseEmail(text6)
	log.Print(parsedDetails6)
	parsedDetails6.Type = "account"
	predictedFields6 := getPredictedFields(parsedDetails6, "Test 6")
	log.Println(predictedFields6)
	transactionData6 := ParsedTransactionData{
		Amount:   parsedDetails6.Amount,
		Date:     parsedDetails6.Date,
		Payee:    predictedFields6.Payee,
		AccName:  predictedFields6.Account,
		Category: predictedFields6.Category,
	}
	log.Print(transactionData6)

	parsedDetails7, _ := parseEmail(text7)
	log.Print(parsedDetails7)
	// parsedDetails7.Type = "account"
	// predictedFields7 := getPredictedFields(parsedDetails7, "Test 7")
	// log.Println("PREDICTED", predictedFields7)
	// transactionData := ParsedTransactionData{
	// 	Amount:   parsedDetails7.Amount,
	// 	Date:     parsedDetails7.Date,
	// 	Payee:    predictedFields7.Payee,
	// 	AccName:  predictedFields7.Account,
	// 	Category: predictedFields7.Category,
	// }
	// log.Println(transactionData)
	// addTransactionToFirestore(transactionData)
	// return transactionData
	return parsedDetails7

	// getFirestoreClient()
	// watchGmailMessages()
}
