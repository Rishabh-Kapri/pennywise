package main
//
// import (
// 	"bytes"
// 	"context"
// 	"encoding/base64"
// 	"encoding/json"
// 	"errors"
// 	"io"
// 	"log"
// 	"net/http"
// 	"os"
// 	"regexp"
// 	"strconv"
// 	"strings"
// 	"time"
//
// 	"cloud.google.com/go/firestore"
// 	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
// 	"github.com/cloudevents/sdk-go/v2/event"
// 	"golang.org/x/oauth2"
// 	"golang.org/x/oauth2/google"
// 	gmail "google.golang.org/api/gmail/v1"
// 	"google.golang.org/api/iterator"
// 	"google.golang.org/api/option"
// )
//
// type PubSubMessage struct {
// 	Data []byte `json:"data"`
// }
//
// type MessagePublishedData struct {
// 	Message PubSubMessage
// }
//
// type EventData struct {
// 	Email     string `json:"emailAddress"`
// 	HistoryId uint64 `json:"historyId"`
// }
//
// type EmailData struct {
// 	Headers []*gmail.MessagePartHeader
// 	Body    string
// }
//
// type ParsedTransactionData struct {
// 	Amount  float64
// 	Date    string
// 	Payee   string
// 	AccName string
// }
//
// type Transaction struct {
// 	BudgetId              string  `firestore:"budgetId"`
// 	Date                  string  `firestore:"date"`
// 	Amount                float64 `firestore:"amount"`
// 	Note                  string  `firestore:"note"`
// 	CategoryId            string  `firestore:"categoryId"`
// 	AccountId             string  `firestore:"accountId"`
// 	PayeeId               string  `firestore:"payeeId"`
// 	TransferAccountId     string  `firestore:"transferAccountId"`
// 	TransferTransactionId string  `firestore:"transferTransactionId"`
// 	Deleted               bool    `firestore:"deleted"`
// 	CreatedAt             string  `firestore:"createdAt"`
// 	UpdatedAt             string  `firestore:"updatedAt"`
// }
//
// func reverseArray(arr []string) {
// 	// Calculate the length of the array
// 	n := len(arr)
//
// 	for i := 0; i < n/2; i++ {
// 		arr[i], arr[n-i-1] = arr[n-i-1], arr[i]
// 	}
// }
//
// func getOauth2Config() *oauth2.Config {
// 	return &oauth2.Config{
// 		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
// 		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
// 		RedirectURL:  os.Getenv("CALLBACK_URL"),
// 		Endpoint:     google.Endpoint,
// 		Scopes:       []string{"https://mail.google.com/", "https://www.googleapis.com/auth/userinfo.email"},
// 	}
// }
//
// func getFirestoreClient() *firestore.Client {
// 	projectId := os.Getenv("PROJECT_ID")
// 	ctx := context.Background()
// 	opt := option.WithCredentialsFile("./serverless_function_source_code/pennywise-39654-c87f94721374.json")
// 	firestoreClient, err := firestore.NewClient(ctx, projectId, opt)
// 	if err != nil {
// 		log.Fatalf("Error while setting up firestore client: %v", err.Error())
// 	}
// 	return firestoreClient
// }
//
// func getRefreshToken(email string) (string, error) {
// 	log.Printf("Getting refresh token for email: %v", email)
// 	firestoreClient := getFirestoreClient()
// 	defer firestoreClient.Close()
//
// 	ctx := context.Background()
//
// 	collection := firestoreClient.Collection("users")
// 	query := collection.Where("email", "==", email)
// 	iter := query.Documents(ctx)
//
// 	doc, err := iter.Next()
// 	if err == iterator.Done {
// 		return "", err
// 	}
// 	refreshToken := doc.Data()["refresh_token"].(string)
// 	return refreshToken, nil
// }
//
// func getPrevHistoryId(email string) (uint64, error) {
// 	firestoreClient := getFirestoreClient()
// 	defer firestoreClient.Close()
//
// 	ctx := context.Background()
//
// 	collection := firestoreClient.Collection("gmailHistoryIds")
// 	query := collection.Where("email", "==", email)
// 	iter := query.Documents(ctx)
//
// 	doc, err := iter.Next()
// 	if err == iterator.Done {
// 		return 0, err
// 	}
// 	historyId, ok := doc.Data()["historyId"].(int64)
// 	if !ok {
// 		return 0, errors.New("Cannot convert to int64")
// 	}
// 	var uintHistoryID uint64
// 	if historyId >= 0 {
// 		uintHistoryID = uint64(historyId)
// 	} else {
// 		// Handle the case of negative values appropriately
// 		// For example, you could log an error or choose a default value
// 		return 0, errors.New("Negative value encountered for historyId")
// 	}
// 	return uintHistoryID, nil
// }
//
// func updateHistoryId(email string, historyId uint64) {
// 	firestoreClient := getFirestoreClient()
// 	defer firestoreClient.Close()
//
// 	ctx := context.Background()
//
// 	collection := firestoreClient.Collection("gmailHistoryIds")
// 	query := collection.Where("email", "==", email)
// 	documentsToUpdate := query.Documents(ctx)
// 	for {
// 		doc, err := documentsToUpdate.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			log.Fatalf("Error while saving history ID: %v", err.Error())
// 		}
// 		loc, _ := time.LoadLocation("Asia/Kolkata")
// 		updateData := []firestore.Update{
// 			{Path: "historyId", Value: int64(historyId)},
// 			{Path: "lastUpdatedAt", Value: time.Now().In(loc).Format("January 02, 2006 15:04:05")},
// 		}
// 		log.Printf("Updating historyId with data: %v", updateData)
// 		_, err = doc.Ref.Update(ctx, updateData)
// 		if err != nil {
// 			log.Fatalf("Error while updating history ID: %v", err.Error())
// 		}
// 	}
// }
//
// func getMessage(email string, historyId uint64, config *oauth2.Config, token *oauth2.Token) ([]EmailData, error) {
// 	ctx := context.Background()
// 	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx, token)))
// 	if err != nil {
// 		return []EmailData{}, err
// 	}
// 	listCall := gmailService.Users.History.List(email)
// 	listCall.StartHistoryId(historyId)
// 	historyRes, err := listCall.Do()
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	seen := make(map[string]bool)
// 	var msgData []EmailData
// 	for _, res := range historyRes.History {
// 		for _, addedMsg := range res.MessagesAdded {
// 			id := addedMsg.Message.Id
// 			if seen[id] {
// 				continue
// 			}
// 			seen[id] = true
// 			msgRes, err := gmailService.Users.Messages.Get(email, id).Do()
// 			if err != nil {
// 				log.Printf("Error while fetching message with id: %s %v", id, err.Error())
// 			}
// 			var bodyData strings.Builder
// 			for _, part := range msgRes.Payload.Parts {
// 				if part.MimeType == "text/html" {
// 					partData, err := base64.URLEncoding.DecodeString(part.Body.Data)
// 					if err != nil {
// 						log.Printf("Error while decoding part: %v", err.Error())
// 					}
// 					bodyData.Write(partData)
// 				}
// 			}
// 			headers := msgRes.Payload.Headers
// 			msgData = append(msgData, EmailData{Headers: headers, Body: bodyData.String()})
// 		}
// 	}
// 	return msgData, nil
// }
//
// func sendNtfyNotification(createdTransaction Transaction) {
// 	url := "https://ntfy.sh/" + os.Getenv("NTFY_TOPIC")
// 	requestBody, _ := json.Marshal(createdTransaction)
// 	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
// 	if err != nil {
// 		log.Printf("Error while creating ntfy request: %v", err.Error())
// 		return
// 	}
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("Title", "A new transaction has been added")
//
// 	client := &http.Client{}
// 	res, err := client.Do(req)
// 	if err != nil {
// 		log.Printf("Error while sending ntfy request: %v", err.Error())
// 		return
// 	}
// 	defer res.Body.Close()
//
// 	body, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		log.Printf("Error while reading ntfy res body: %v", err.Error())
// 		return
// 	}
// 	log.Printf("Notification sent! %s", string(body))
// }
//
// func getAllDocuments(firestoreClient *firestore.Client, collection string) ([]*firestore.DocumentSnapshot, error) {
// 	collectionRef := firestoreClient.Collection(collection)
//
// 	iter := collectionRef.Documents(context.Background())
// 	return iter.GetAll()
// }
//
// func addTransactionToFirestore(transactionData ParsedTransactionData) (bool, error) {
// 	firestoreClient := getFirestoreClient()
// 	defer firestoreClient.Close()
//
// 	budgetId := "Mm1kjyD58NQnNzOfx460"
// 	unexpectedCatId := "2ae8c166-11ae-4aed-86bf-6065c51faeb1"
// 	unexpectedPayeeId := "9a70b6f8-36e4-4f34-9452-828757031d6b"
//
// 	accountDocs, err := getAllDocuments(firestoreClient, "accounts")
// 	if err != nil {
// 		return false, err
// 	}
// 	payeeDocs, err := getAllDocuments(firestoreClient, "payees")
// 	if err != nil {
// 		return false, err
// 	}
// 	var foundAcc map[string]interface{}
// 	var foundPayee map[string]interface{}
// 	for _, acc := range accountDocs {
// 		if acc.Data()["name"].(string) == transactionData.AccName {
// 			foundAcc = acc.Data()
//             if foundAcc["id"] == nil {
// 			    foundAcc["id"] = acc.Ref.ID
// 			}
//             break
// 		}
// 	}
// 	var foundPayeeId string
// 	if len(foundAcc) > 0 {
// 		payeeArr := strings.Split(transactionData.Payee, " ")
// 		for _, payee := range payeeDocs {
// 			if len(foundPayee) > 0 {
// 				foundPayeeId = payee.Data()["id"].(string)
// 				break
// 			}
// 			for _, str := range payeeArr {
// 				strLowercase := strings.ToLower(str)
// 				payeeName := payee.Data()["name"].(string)
// 				if strings.Contains(payeeName, strLowercase) {
// 					foundPayee = payee.Data()
// 					break
// 				}
// 			}
// 		}
// 		if len(foundPayee) == 0 {
// 			foundPayeeId = unexpectedPayeeId
// 		}
// 		currenTime := time.Now()
// 		isoString := currenTime.Format("2006-01-02T15:04:05.999Z")
// 		newTransaction := Transaction{
// 			BudgetId:              budgetId,
// 			Date:                  transactionData.Date,
// 			Amount:                -transactionData.Amount,
// 			Note:                  "",
// 			CategoryId:            unexpectedCatId,
// 			AccountId:             foundAcc["id"].(string),
// 			PayeeId:               foundPayeeId,
// 			TransferAccountId:     "",
// 			TransferTransactionId: "",
// 			Deleted:               false,
// 			CreatedAt:             isoString,
// 			UpdatedAt:             isoString,
// 		}
// 		_, _, err := firestoreClient.Collection("transactions").Add(context.Background(), newTransaction)
// 		if err != nil {
// 			return false, err
// 		}
// 		sendNtfyNotification(newTransaction)
// 	}
// 	return true, nil
// }
//
// func init() {
// 	functions.CloudEvent("WatchGmailMessages", WatchGmailMessages)
// }
//
// func WatchGmailMessages(ctx context.Context, e event.Event) error {
// 	var msg MessagePublishedData
// 	if err := e.DataAs(&msg); err != nil {
// 		log.Fatalf("Error while getting event data: %v", err.Error())
// 	}
// 	data := string(msg.Message.Data)
// 	log.Printf("Event received: %v", data)
//
// 	var eventData EventData
// 	err := json.Unmarshal(msg.Message.Data, &eventData)
// 	if err != nil {
// 		log.Fatalf("Failed to unmarshal event msg data: %v", err.Error())
// 	}
//
// 	email := eventData.Email
// 	refreshToken, err := getRefreshToken(email)
// 	if err != nil {
// 		log.Fatalf("Error while fetching refresh token: %v", err.Error())
// 	}
//
// 	// get access token from refresh token
// 	config := getOauth2Config()
// 	tokenSource := config.TokenSource(context.Background(), &oauth2.Token{
// 		RefreshToken: refreshToken,
// 	})
// 	token, err := tokenSource.Token()
// 	if err != nil {
// 		log.Fatalf("Error while fetching access token: %v", err.Error())
// 	}
//
// 	// fetch saved previous history id
// 	prevHistoryId, err := getPrevHistoryId(email)
// 	log.Printf("Previous historyId: %v", prevHistoryId)
// 	if err != nil {
// 		log.Fatalf("Error while fetching previous history id: %v", err.Error())
// 	}
//
// 	updateHistoryId(email, eventData.HistoryId)
//
// 	msgData, err := getMessage(email, prevHistoryId, config, token)
// 	if err != nil {
// 		log.Fatalf("Error while fetching messages using prevHistoryId %d, %s", prevHistoryId, err.Error())
// 	}
// 	for _, data := range msgData {
// 		isTransaction := false
// 		var accName string
// 		for _, header := range data.Headers {
// 			if header.Name == "Subject" || header.Name == "From" {
// 				if strings.Contains(header.Value, "txn") ||
// 					strings.Contains(header.Value, "Alert : Update") ||
// 					strings.Contains(header.Value, "Alert :  Update") {
// 					isTransaction = true
// 				}
//
// 				if strings.Contains(header.Value, "Credit Card") {
// 					accName = "HDFC Credit Card"
// 				}
// 			}
// 		}
// 		if isTransaction && accName != "HDFC Credit Card" {
// 			accName = "HDFC (Salary)"
// 		}
// 		log.Printf("isTransaction: %v, accName: %v", isTransaction, accName)
// 		if !isTransaction {
// 			continue
// 		}
// 		var parsedData ParsedTransactionData
// 		if accName == "HDFC Credit Card" {
//             ccToAccMap := map[string]string{"8799": "HDFC Swiggy Credit Card", "4432": "HDFC Credit Card"}
//             
// 			pattern := `HDFC\sBank\sCredit\sCard\sending\s(\d+)\s\w+\sRs\.*\s*(\d+\.\d+)\s+at\s+(.+?on)*\s(\d+-\d+-\d+)`
// 			regex := regexp.MustCompile(pattern)
// 			matches := regex.FindStringSubmatch(data.Body)
//
// 			if len(matches) > 0 {
//                 ccNumber := matches[1]
//                 accName = ccToAccMap[ccNumber]
//
// 				amount := matches[2]
// 				payee := strings.Split(matches[3], "on")[0]
// 				parsedDate := matches[4]
// 				amountFloat, err := strconv.ParseFloat(amount, 10)
// 				if err != nil {
// 					log.Printf("Error while converting to int: %v", err.Error())
// 					continue
// 				}
// 				dateArr := strings.Split(parsedDate, "-")
// 				reverseArray(dateArr)
// 				date := strings.Join(dateArr, "-")
// 				parsedData = ParsedTransactionData{
// 					Amount:  amountFloat,
// 					Date:    date,
// 					Payee:   payee,
// 					AccName: accName,
// 				}
// 				log.Printf("%v", parsedData)
// 				_, err = addTransactionToFirestore(parsedData)
// 				if err != nil {
// 					log.Printf("Error while adding new transaction: %v", err.Error())
// 				}
// 			}
// 		} else if accName == "HDFC (Salary)" {
// 			pattern := `Rs.(\d+\.\d+).*on\s(\d+-\d+-\d+)`
// 			regex := regexp.MustCompile(pattern)
// 			matches := regex.FindStringSubmatch(data.Body)
// 			if len(matches) > 0 {
// 				amount := matches[1]
// 				parsedDate := matches[2]
// 				amountFloat, err := strconv.ParseFloat(amount, 10)
// 				if err != nil {
// 					log.Printf("Error while converting to int: %v", err.Error())
// 					continue
// 				}
// 				dateArr := strings.Split(parsedDate, "-")
// 				reverseArray(dateArr)
// 				date := "20" + strings.Join(dateArr, "-")
// 				parsedData = ParsedTransactionData{
// 					Amount:  amountFloat,
// 					Date:    date,
// 					Payee:   "Shop",
// 					AccName: accName,
// 				}
// 				log.Printf("%v", parsedData)
// 				_, err = addTransactionToFirestore(parsedData)
// 				if err != nil {
// 					log.Printf("Error while adding new transaction: %v", err.Error())
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }
