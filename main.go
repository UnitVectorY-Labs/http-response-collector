// main.go
package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
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
	Headers      map[string]string      `json:"headers"`
	ResponseBody map[string]interface{} `json:"responseBody,omitempty"`
	Body         string                 `json:"body,omitempty"`
	ResponseTime int64                  `json:"responseTime"` // in milliseconds
	RequestTime  time.Time              `json:"requestTime"`
	StatusCode   int                    `json:"statusCode"`
}

// ErrorPayload represents the structure of an error message
type ErrorPayload struct {
	Error        string    `json:"error"`
	OriginalData string    `json:"originalData,omitempty"`
	RequestTime  time.Time `json:"requestTime"`
}

// publishMessage currently logs the message. This can be extended in the future.
func publishMessage(message interface{}) {
	messageJSON, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshalling message for publishing: %v", err)
		return
	}
	log.Printf("Publish Message: %s", string(messageJSON))
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

	// Read the response body with a limit of 10MB
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, err
	}

	var output OutputPayload
	output.Headers = headers
	output.ResponseTime = responseTime
	output.RequestTime = startTime
	output.StatusCode = resp.StatusCode

	// Attempt to parse the body as JSON
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
		output.ResponseBody = jsonBody
	} else {
		// If not JSON, store the raw body as a string
		output.Body = string(bodyBytes)
	}

	return &output, nil
}

// isValidURL performs a basic validation of the URL format
func isValidURL(url string) bool {
	// Basic check to see if the URL starts with http or https
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// publishErrorMessage logs an error message variant
func publishErrorMessage(errorMsg string, originalData string) {
	errorPayload := ErrorPayload{
		Error:        errorMsg,
		OriginalData: originalData,
		RequestTime:  time.Now(),
	}
	publishMessage(errorPayload)
}
