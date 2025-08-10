package gmail
//
// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"log"
// 	"net/http"
//
// 	"golang.org/x/oauth2"
// )
//
// // Redirect to the auth url
// func authInit(w http.ResponseWriter, r *http.Request) {
// 	authUrl := getOauth2Config().AuthCodeURL("state", oauth2.AccessTypeOffline)
// 	log.Printf("Redirecting to url: %v", authUrl)
// 	http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
// }
//
// // Callback method for setting token
// func setToken(w http.ResponseWriter, r *http.Request) {
// 	log.Print("Setting token flow")
// 	ctx := context.Background()
// 	q := r.FormValue("code")
// 	config := getOauth2Config()
// 	// exchange code for tokens
// 	token, err := config.Exchange(ctx, q)
// 	if err != nil {
// 		log.Fatalf("Error while fetching token: %v", err.Error())
// 	}
// 	// get userinfo
// 	userInfoUrl := "https://www.googleapis.com/oauth2/v2/userinfo?access_token="
// 	client := config.Client(ctx, token)
// 	res, err := client.Get(userInfoUrl + token.AccessToken)
// 	if err != nil {
// 		log.Fatalf("Error while fetching user info: %v", err.Error())
// 	}
// 	defer res.Body.Close()
// 	contents, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		log.Fatalf("Error while reading contents: %v", err.Error())
// 	}
// 	log.Print(string(contents))
// 	var userInfo UserInfo
// 	err = json.Unmarshal(contents, &userInfo)
// 	if err != nil {
// 		log.Fatalf("Failed to unmarshal user info json: %v", err.Error())
// 	}
// 	// save refresh token to firestore
// 	msg, err := saveTokenToFirestore(userInfo.Id, token.RefreshToken)
// 	if err != nil {
// 		log.Fatalf("Error while saving refresh token to firestore: %v", err.Error())
// 	}
// 	log.Print(msg)
// 	historyId, err := setUpGmailNotifications(userInfo.Email, config, token)
// 	if err != nil {
// 		log.Fatalf("Error while setting up gmail notifications: %v", err.Error())
// 	}
// 	_, err = updateHistoryIdToFirestore(userInfo.Email, historyId)
// 	if err != nil {
// 		log.Fatalf("Error while saving history ID: %v", err.Error())
// 	}
// 	log.Printf("Successully set up gmail push notifications: %d", historyId)
// }
//
// func testSetToken(refreshToken string) {
// 	ctx := context.Background()
// 	log.Print(ctx)
// 	config := getOauth2Config()
// 	fmt.Print(config, refreshToken)
// 	tokenSource := config.TokenSource(ctx, &oauth2.Token{
// 		RefreshToken: refreshToken,
// 	})
// 	token, err := tokenSource.Token()
// 	if err != nil {
// 		log.Fatalf("Error while fetching token: %v", err.Error())
// 	}
// 	userInfoUrl := "https://www.googleapis.com/oauth2/v2/userinfo?access_token="
// 	client := config.Client(ctx, token)
// 	res, err := client.Get(userInfoUrl + token.AccessToken)
// 	if err != nil {
// 		log.Fatalf("Error while fetching user info: %v", err.Error())
// 	}
// 	defer res.Body.Close()
// 	contents, err := io.ReadAll(res.Body)
// 	if err != nil {
// 		return
// 	}
// 	log.Print(string(contents))
// 	var userInfo UserInfo
// 	log.Printf("%v\n", userInfo)
// 	ok := json.Unmarshal(contents, &userInfo)
// 	if ok != nil {
// 		log.Printf("%v\n:", err)
// 	}
// 	log.Printf("userInfo: %v\n", userInfo)
// }

