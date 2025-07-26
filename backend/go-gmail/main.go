package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	pubsub "gmail-transactions/pkg"

	// "io"

	// "time"

	"github.com/joho/godotenv"
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

	// data := gmail.Init()
	// log.Println(data)
	// requestBody, _ := json.Marshal(data)
	// log.Println(string(requestBody))
	// w.Write(requestBody)
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error while loading .env: %v", err.Error())
	}
	log.Printf("project id %v", os.Getenv("PROJECT_ID"))
	go pubsub.PullMessages()
	// go pubsub.TestMessages()
	// go gmail.Init()

	// http.HandleFunc("/", handler)
	fmt.Println("Server listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
