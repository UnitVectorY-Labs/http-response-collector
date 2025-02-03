// main.go
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
)

// PubSubMessage represents the structure of a Pub/Sub push message
type PubSubMessage struct {
	Message struct {
		Data        string            `json:"data"`
		Attributes  map[string]string `json:"attributes"`
		MessageID   string            `json:"messageId"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// InputPayload represents the structure of the incoming JSON payload
type InputPayload struct {
	URL string `json:"url"`
}

// OutputPayload represents the structure of the processed data
type OutputPayload struct {
	URL          string `json:"url"`
	Error        string `json:"error,omitempty"`
	Headers      string `json:"headers,omitempty"`
	ResponseBody string `json:"responseBody,omitempty"`
	ResponseJson string `json:"responseJson,omitempty"`
	ResponseTime int64  `json:"responseTime"` // in milliseconds
	RequestTime  string `json:"requestTime"`
	StatusCode   int    `json:"statusCode"`
}

// Updated publishMessage now publishes to the Pub/Sub topic if RESPONSE_PUBSUB is set.
func publishMessage(message interface{}) {
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshalling message for publishing: %v", err)
		return
	}

	topicName := os.Getenv("RESPONSE_PUBSUB")
	if topicName == "" {
		log.Printf("Publish Message: %s", string(messageJSON))
		return
	}

	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		log.Printf("GOOGLE_CLOUD_PROJECT env variable not set, cannot publish to PubSub")
		log.Printf("Publish Message: %s", string(messageJSON))
		return
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Printf("Error creating PubSub client: %v", err)
		log.Printf("Publish Message: %s", string(messageJSON))
		return
	}

	topic := client.Topic(topicName)
	result := topic.Publish(ctx, &pubsub.Message{
		Data:       messageJSON,
		Attributes: map[string]string{"type": "request"},
	})
	id, err := result.Get(ctx)
	if err != nil {
		log.Printf("Error publishing message to PubSub: %v", err)
	} else {
		log.Printf("Published message with ID: %s", id)
	}
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
		// Log the invalid method and return 200 OK to avoid retries
		log.Printf("Invalid request method: %s", r.Method)
		publishErrorMessage("Invalid request method", "")
		w.WriteHeader(http.StatusOK)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		publishErrorMessage("Cannot read body", "")
		w.WriteHeader(http.StatusOK)
		return
	}
	defer r.Body.Close()

	var msg PubSubMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		log.Printf("Error unmarshalling JSON: %v. Body: %s", err, string(body))
		publishErrorMessage("Error unmarshalling JSON", string(body))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Decode the base64-encoded data
	data, err := decodeBase64(msg.Message.Data)
	if err != nil {
		log.Printf("Error decoding data: %v. Data: %s", err, msg.Message.Data)
		publishErrorMessage("Error decoding data", msg.Message.Data)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse the input JSON payload
	var input InputPayload
	if err := json.Unmarshal([]byte(data), &input); err != nil {
		log.Printf("Error unmarshalling input JSON: %v. Data: %s", err, data)
		publishErrorMessage("Error unmarshalling input JSON", data)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Validate URL
	if !isValidURL(input.URL) {
		log.Printf("Invalid URL: %s", input.URL)
		publishErrorMessage("Invalid URL", input.URL)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fetch the URL and process the response
	output, err := fetchURL(input.URL)
	if err != nil {
		log.Printf("Error fetching URL %s: %v", input.URL, err)
		publishErrorMessage("Error fetching URL", input.URL)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Convert OutputPayload to JSON
	outputJSON, err := json.Marshal(output)
	if err != nil {
		log.Printf("Error marshalling output JSON: %v", err)
		publishErrorMessage("Error marshalling output JSON", input.URL)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Log the output JSON to the console
	log.Printf("Processed Response: %s", string(outputJSON))

	// Optionally publish the processed message (currently just logs)
	publishMessage(output)

	w.WriteHeader(http.StatusOK)
}

// decodeBase64 decodes a base64-encoded string
func decodeBase64(encoded string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}

// fetchURL makes an HTTP GET request to the specified URL and processes the response
func fetchURL(url string) (*OutputPayload, error) {
	client := &http.Client{
		Timeout: 10 * time.Second, // Set a 10-second timeout
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set the User-Agent header
	req.Header.Set("User-Agent", "http-response-collector")

	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseTime := time.Since(startTime).Milliseconds()

	// Read the response headers
	headers := make(map[string]string)
	for key, values := range resp.Header {
		headers[key] = strings.Join(values, ", ")
	}

	// Encode headers as a JSON string
	encodedHeaders, err := json.Marshal(headers)
	if err != nil {
		encodedHeaders = []byte("{}")
	}

	// Read the response body with a limit of 10MB
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, err
	}

	var output OutputPayload
	output.URL = url
	output.Headers = string(encodedHeaders)
	output.ResponseTime = responseTime
	output.RequestTime = startTime.UTC().Format(time.RFC3339Nano)
	output.StatusCode = resp.StatusCode

	if json.Valid(bodyBytes) {
		output.ResponseJson = string(bodyBytes)
	} else {
		output.ResponseBody = string(bodyBytes)
	}

	return &output, nil
}

// isValidURL performs a basic validation of the URL format
func isValidURL(url string) bool {
	// Basic check to see if the URL starts with http or https
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// publishErrorMessage logs an error message variant
func publishErrorMessage(errorMsg string, url string) {
	errorPayload := OutputPayload{
		URL:         url,
		Error:       errorMsg,
		RequestTime: time.Now().UTC().Format(time.RFC3339Nano),
	}
	publishMessage(errorPayload)
}
