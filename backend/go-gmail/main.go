package main

import (
	"encoding/json"
	"fmt"
	// "io"
	"log"
	"net/http"
	"os"

	// "time"

	// mlp "gmail-transactions/pkg"

	"gmail-transactions/pkg/gmail"

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
	data := gmail.Init()
	log.Println(data)
	requestBody, _ := json.Marshal(data)
	log.Println(string(requestBody))
	w.Write(requestBody)

	// go func(body []byte) {
	// 	time.Sleep(5 * time.Second)
	// 	fmt.Println("Background processing done")
	// 	println(string(body))
	// }(body)
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error while loading .env: %v", err.Error())
	}
	log.Printf("project id %v", os.Getenv("PROJECT_ID"))
	// go mlp.PullMessages()
	// go gmail.Init()

	http.HandleFunc("/", handler)
	fmt.Println("Server listening on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
