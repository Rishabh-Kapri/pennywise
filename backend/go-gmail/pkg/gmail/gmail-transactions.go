package gmail

import (

	// "regexp"
	// "strconv"

	// "github.com/joho/godotenv"

	// "github.com/joho/godotenv"

	gmail "google.golang.org/api/gmail/v1"
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
	CategoryId            string  `firestore:"categoryId,omitempty"`
	AccountId             string  `firestore:"accountId"`
	PayeeId               string  `firestore:"payeeId"`
	TransferAccountId     string  `firestore:"transferAccountId,omitempty"`
	TransferTransactionId string  `firestore:"transferTransactionId,omitempty"`
	Deleted               bool    `firestore:"deleted"`
	CreatedAt             string  `firestore:"createdAt"`
	UpdatedAt             string  `firestore:"updatedAt"`
}

type PennywiseTxn struct {
	Date                  string  `json:"date"`
	PayeeId               string  `json:"payeeId"`
	CategoryId            string  `json:"categoryId"`
	AccountId             string  `json:"accountId"`
	Amount                float64 `json:"amount"`
	Note                  string  `json:"note"`
	Source                string  `json:"source"` // MLP for prediction, PENNYWISE for frontend
	TransferAccountId     string  `json:"transferAccountId,omitempty"`
	TransferTransactionId string  `json:"transferTransactionId,omitempty"`
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
	Account         string  `json:"account"`
	Payee           string  `json:"payee"`
	Type            string  `json:"type"`
}

// // Creates a new transaction to firestore
// func addTransactionToFirestore(transactionData ParsedTransactionData) (bool, error) {
// 	firestoreClient := utils.GetFirestoreClient()
// 	defer firestoreClient.Close()
//
// 	// ctx := context.Background()
// 	// budgetId := "Mm1kjyD58NQnNzOfx460"
//
// 	accountDocs, err := getAllDocuments(firestoreClient, "accounts")
// 	if err != nil {
// 		return false, err
// 	}
// 	payeeDocs, err := getAllDocuments(firestoreClient, "payees")
// 	if err != nil {
// 		return false, err
// 	}
// 	categoryDocs, err := getAllDocuments(firestoreClient, "categories")
// 	if err != nil {
// 		return false, err
// 	}
// 	var foundAcc map[string]any
// 	var foundPayee map[string]any
// 	var foundCategory map[string]any
//
// 	for _, acc := range accountDocs {
// 		if strings.Contains(acc.Data()["name"].(string), transactionData.AccName) {
// 			foundAcc = acc.Data()
// 		}
// 	}
// 	if transactionData.Category != "null" {
// 		for _, cat := range categoryDocs {
// 			if strings.Contains(cat.Data()["name"].(string), transactionData.Category) {
// 				foundCategory = cat.Data()
// 				break
// 			}
// 		}
// 	}
// 	if len(foundAcc) > 0 {
// 		for _, payee := range payeeDocs {
// 			if strings.Contains(payee.Data()["name"].(string), transactionData.Payee) {
// 				foundPayee = payee.Data()
// 				break
// 			}
// 		}
// 		// var newTransaction Transaction
//
// 		// currentTime := time.Now()
// 		// isoString := currentTime.Format("2006-01-02T15:04:05.999Z")
// 		log.Printf("%v\n", foundPayee)
// 		log.Printf("%v\n", foundCategory)
//
// 		// pennywiseTxn := PennywiseTxn{
// 		// 	BudgetId:              "2166418d-3fa2-4acc-b92c-ab9f36c18d76",
// 		// 	Date:                  transactionData.Date,
// 		// 	Amount:                transactionData.Amount,
// 		// 	Note:                  "",
// 		// 	CategoryId:            "bfba16ed-3680-4a12-bda3-0290f7686762",
// 		// 	AccountId:             "20ee5327-51d6-4d02-b863-1bfd4ee47722",
// 		// 	PayeeId:               "b18da7e2-15c8-485e-819b-4ce217c812b1",
// 		// 	TransferAccountId:     "",
// 		// 	TransferTransactionId: "",
// 		// 	Deleted:               false,
// 		// 	CreatedAt:             isoString,
// 		// 	UpdatedAt:             isoString,
// 		// }
// 		// makePennywiseApiRequest("/api/transactions", "POST", pennywiseTxn)
//
// 		// payeeId := foundPayee["id"].(string)
// 		// accountId := foundAcc["id"].(string)
// 		//
// 		// if transactionData.Category != "null" && foundCategory != nil {
// 		// 	categoryId := foundCategory["id"].(string)
// 		// 	newTransaction = Transaction{
// 		// 		BudgetId:              budgetId,
// 		// 		Date:                  transactionData.Date,
// 		// 		Amount:                transactionData.Amount,
// 		// 		Note:                  "",
// 		// 		CategoryId:            categoryId,
// 		// 		AccountId:             accountId,
// 		// 		PayeeId:               payeeId,
// 		// 		TransferAccountId:     "",
// 		// 		TransferTransactionId: "",
// 		// 		Deleted:               false,
// 		// 		CreatedAt:             isoString,
// 		// 		UpdatedAt:             isoString,
// 		// 	}
// 		// } else {
// 		// 	newTransaction = Transaction{
// 		// 		BudgetId:              budgetId,
// 		// 		Date:                  transactionData.Date,
// 		// 		Amount:                transactionData.Amount,
// 		// 		Note:                  "",
// 		// 		AccountId:             accountId,
// 		// 		PayeeId:               payeeId,
// 		// 		TransferAccountId:     "",
// 		// 		TransferTransactionId: "",
// 		// 		Deleted:               false,
// 		// 		CreatedAt:             isoString,
// 		// 		UpdatedAt:             isoString,
// 		// 	}
// 		// }
// 		// log.Printf("New transaction: %v", newTransaction)
// 		// _, _, err := firestoreClient.Collection("transactions").Add(ctx, newTransaction)
// 		// if err != nil {
// 		// 	return false, err
// 		// }
// 		// sendNtfyNotification(newTransaction)
// 	}
// 	return true, nil
// }
//
// // Send a push notification using ntfy
// func sendNtfyNotification(createdTransaction Transaction) {
// 	url := "https://ntfy.sh/" + os.Getenv("NTFY_TOPIC")
// 	requestBody, _ := json.Marshal(createdTransaction)
// 	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
// 	if err != nil {
// 		log.Printf("Error while creating ntfy request: %v", err.Error())
// 		return
// 	}
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("Title", "A new transaction has been added!")
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

func Init() *EmailDetails {
	// WatchGmailMessages()

	// text1 := `<div dir="ltr"><table align="center" width="550" border="0" cellspacing="0" cellpadding="0"><tbody><tr><td width="550" valign="top" align="center"><table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:550px"><tbody><tr><td height=""></td></tr><tr><td align="left" valign="middle" style="font-family:Arial;font-size:16px;line-height:22px;color:rgb(0,0,0)">Dear Customer, Rs.200.00 has been debited from account **8936 to VPA divyanshkapri12-1@okaxis DIVYANSH KAPRI on 07-07-25. Your UPI transaction reference number is . If you did not authorize this transaction, please report it immediately by calling 18002586161 Or SMS BLOCK UPI to 00000000. Warm Regards, HDFC Bank</td></tr></tbody></table></td></tr></tbody></table></div>`
	// text2 := `<div dir="ltr"><table align="center" width="550" border="0" cellspacing="0" cellpadding="0"><tbody><tr><td width="550" valign="top" align="center"><table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:550px"><tbody><tr><td height=""></td></tr><tr><td align="left" valign="middle" style="font-family:Arial;font-size:16px;line-height:22px;color:rgb(0,0,0)">Dear Customer, Thank you for using HDFC Bank Credit Card 4432 for Rs. 22.63 at GOOGLE CLOUD CYBS SI on 03-07-2025 22:40:35 Authorization code:- XXXXXX Please note that this transaction was conducted without OTP /PIN. If you have not authorized the above transaction, please call on 18002586161 Warm regards, HDFC Bank (This is a system generated mail and should not be replied to)</td></tr></tbody></table></td></tr></tbody></table></div>`
	// text3 := `<div dir="ltr"><table align="center" width="550" border="0" cellspacing="0" cellpadding="0"><tbody><tr><td width="550" valign="top" align="center"><table align="center" width="100%" border="0" cellspacing="0" cellpadding="0" style="width:550px"><tbody><tr><td height=""></td></tr><tr><td align="left" valign="middle" style="font-family:Arial;font-size:16px;line-height:22px;color:rgb(0,0,0)"> Dear Customer, Rs.15000.00 is successfully credited to your account **8936 by VPA 9458306660@ybl SHEELA KAPRI on 02-07-25. Your UPI transaction reference number is 887393696385. Thank you for banking with us. Warm Regards, HDFC Bank </td></tr></tbody></table></td></tr></tbody></table></div>`
	// text4 := `Dear Card Member, Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 529.82 at Bharti Airtel Limited on 21-05-2025 13:01:46. Authorization code:- 062756 After the above transaction, the available balance on your card is Rs 401279.18 and the total outstanding is Rs 81720.82. For more details on this transaction please visit HDFC Bank MyCards. If you have not done this transaction, please immediately call on 18002586161 to report this transaction. Explore HDFC Bank MyCards: your one stop platform to manage Credit Card ON THE GO.`
	// text5 := ``
		// text6 := ``
	// text7 := ``
// 	text8 := "Dear Customer, Thank you for using HDFC Bank Card XX4432 for Rs. 1196.52 at Adobe Systems Software on 02-08-2025 15:55:53"
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

	// parsedDetails6, _ := parseEmail(text6)
	// log.Print(parsedDetails6)
	// parsedDetails6.Type = "account"
	// predictedFields6 := getPredictedFields(parsedDetails6, "Test 6")
	// log.Println(predictedFields6)
	// transactionData6 := ParsedTransactionData{
	// 	Amount:   parsedDetails6.Amount,
	// 	Date:     parsedDetails6.Date,
	// 	Payee:    predictedFields6.Payee,
	// 	AccName:  predictedFields6.Account,
	// 	Category: predictedFields6.Category,
	// }
	// log.Print(transactionData6)

	// parsedDetails7, _ := parseEmail(text7)
	// log.Print(parsedDetails7)
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

	// parsedDetails8, _ := parseEmail(text8)
	// log.Print(parsedDetails8)
	// parsedDetails8.Type = "account"
	// predictedFields8 := getPredictedFields(parsedDetails8, "Test 6")
	// log.Println(predictedFields8)
	// transactionData8 := ParsedTransactionData{
	// 	Amount:   parsedDetails8.Amount,
	// 	Date:     parsedDetails8.Date,
	// 	Payee:    predictedFields8.Payee,
	// 	AccName:  predictedFields8.Account,
	// 	Category: predictedFields8.Category,
	// }
	// transactionData := ParsedTransactionData{
	// 	Amount:   1196.52,
	// 	Date:     "2025-08-08",
	// 	Payee:    "Adobe",
	// 	AccName:  "HDFC Credit Card",
	// 	Category: " 🅰️ Adobe",
	// }
	// log.Print(transactionData)
	// // _, err := addTransactionToFirestore(transactionData8)
	// // if err != nil {
	// // 	log.Printf("Error while adding new transaction: %v", err.Error())
	// // }
	// addTransactionToPostgres(transactionData)
	//
	// return parsedDetails7

	// getFirestoreClient()
	// watchGmailMessages()
	return nil
}
