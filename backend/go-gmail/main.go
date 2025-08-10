package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"gmail-transactions/pkg/gmail"
	"gmail-transactions/pkg/pubsub"
)

func handler(w http.ResponseWriter, r *http.Request) {
	// body, err := io.ReadAll(r.Body)
	// defer r.Body.Close()
	// if err != nil {
	// 	fmt.Println(err)
	// 	http.Error(w, "failed to read body", http.StatusInternalServerError)
	// 	return
	// }

	w.WriteHeader(http.StatusAccepted)

	data := gmail.Init()
	log.Println(data)
	requestBody, _ := json.Marshal(data)
	log.Println("return data", string(requestBody))
	w.Write(requestBody)
}

func main() {
	go pubsub.PullMessages()
	// // go pubsub.TestMessages()
	// go gmail.Init()

	http.HandleFunc("/", handler)
	fmt.Println("Server listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
