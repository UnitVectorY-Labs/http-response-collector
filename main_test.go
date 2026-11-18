package main

import (
	"encoding/base64"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPubSubHandlerJSONValidation(t *testing.T) {
	t.Setenv("RESPONSE_PUBSUB", "")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	payload := func(input string) string {
		return fmt.Sprintf(`{"message":{"data":%q}}`, base64.StdEncoding.EncodeToString([]byte(input)))
	}
	validInput := fmt.Sprintf(`{"url":%q}`, server.URL)
	tests := []struct {
		name, body string
		wantFetch  bool
	}{
		{"valid", payload(validInput), true},
		{"duplicate envelope key", fmt.Sprintf(`{"message":{"data":%q,"data":%q}}`, base64.StdEncoding.EncodeToString([]byte(validInput)), base64.StdEncoding.EncodeToString([]byte(validInput))), false},
		{"duplicate input key", payload(fmt.Sprintf(`{"url":%q,"url":%q}`, server.URL, server.URL)), false},
		{"invalid UTF-8 input", payload("{\"url\":\"\xff\"}"), false},
		{"case sensitive input", payload(fmt.Sprintf(`{"URL":%q}`, server.URL)), false},
		{"case sensitive envelope", fmt.Sprintf(`{"Message":{"data":%q}}`, base64.StdEncoding.EncodeToString([]byte(validInput))), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := requests.Load()
			response := httptest.NewRecorder()
			pubSubHandler(response, httptest.NewRequest(http.MethodPost, "/pubsub/push", strings.NewReader(tt.body)))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", response.Code)
			}
			if got := requests.Load() > before; got != tt.wantFetch {
				t.Errorf("fetched URL = %v, want %v", got, tt.wantFetch)
			}
		})
	}
}

func TestFetchURLClassifiesJSONStrictly(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", `{"a":1}`, true},
		{"duplicate names", `{"a":1,"a":2}`, false},
		{"invalid UTF-8", "{\"a\":\"\xff\"}", false},
		{"malformed", `{`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tt.body) }))
			defer server.Close()
			output, err := fetchURL(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			if tt.valid && (output.ResponseJson != tt.body || output.ResponseBody != "") {
				t.Errorf("unexpected output: %+v", output)
			}
			if !tt.valid && (output.ResponseBody != tt.body || output.ResponseJson != "") {
				t.Errorf("unexpected output: %+v", output)
			}
		})
	}
}

func TestOutputPayloadJSON(t *testing.T) {
	data, err := json.Marshal(OutputPayload{URL: "https://example.com", RequestTime: "now"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"url":"https://example.com","requestTime":"now"}`; got != want {
		t.Errorf("output = %s, want %s", got, want)
	}
}
