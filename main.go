package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

// PubSubMessage represents the structure of a Pub/Sub message
type PubSubMessage struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func main() {
	http.HandleFunc("/pubsub/push", pubSubHandler)

	port := ":8080"
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// pubSubHandler handles incoming Pub/Sub push requests
func pubSubHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var msg PubSubMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("Error unmarshalling JSON: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Decode the base64-encoded data
	data, err := decodeBase64(msg.Message.Data)
	if err != nil {
		log.Printf("Error decoding data: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf("Received message ID: %s", msg.Message.MessageID)
	log.Printf("Publish Time: %s", msg.Message.PublishTime)
	log.Printf("Data: %s", data)
	log.Printf("Attributes: %v", msg.Message.Attributes)

	w.WriteHeader(http.StatusOK)
}

// decodeBase64 decodes a base64-encoded string
func decodeBase64(encoded string) (string, error) {
	decodedBytes, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded)))
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}
