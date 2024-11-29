package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
)

// Structs for Telegram and RequestData remain the same as previously defined
type Telegram struct {
	BotToken string
	UserID   string
	APIHost  string
	Proxy    string
}

type RequestData struct {
	API      string
	Headers  map[string]string
	Text     string
	Campus   string
	Source   string
	ID       int
	Build    string
	Room     string
	RoomID   string
	Lang     string
	Terminal string
}

type Config struct {
	Telegram    Telegram
	RequestData RequestData
}

// LoadConfig reads configuration from a JSON file
func LoadConfig(configPath string) (conf *Config) {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&conf)
	if err != nil {
		log.Fatalf("Failed to decode config JSON: %v", err)
	}
	return
}

// GetMsg method fetches data from the API and processes the response
func (R *RequestData) GetMsg() (msg string, err error) {
	// Create the request payload from the struct fields
	payload := map[string]interface{}{
		"text":     R.Text,
		"campus":   R.Campus,
		"source":   R.Source,
		"id":       R.ID,
		"build":    R.Build,
		"room":     R.Room,
		"roomId":   R.RoomID,
		"lang":     R.Lang,
		"terminal": R.Terminal,
	}

	// Marshal the payload into JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", R.API, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, value := range R.Headers {
		req.Header.Set(key, value)
	}

	// Create an HTTP client and perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check for a successful response
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-OK HTTP status: %d", resp.StatusCode)
	}

	// Decode the response body
	var res struct {
		Status int `json:"status"`
		Data   struct {
			UsedAmp float64 `json:"usedAmp"`
			AllAmp  float64 `json:"allAmp"`
		} `json:"data"`
		Rel bool `json:"rel"`
	}
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return "", fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Process the response
	if res.Status != 200 {
		msg = fmt.Sprintf("Failed to retrieve data. Status: %d", res.Status)
	} else {
		usedAmp := res.Data.UsedAmp
		allAmp := res.Data.AllAmp
		msg = fmt.Sprintf("Used Amp: %.2f, All Amp: %.2f", usedAmp, allAmp)

		if allAmp < usedAmp {
			msg = fmt.Sprintf("Warning: Used Amp (%.2f) exceeds All Amp (%.2f)!", usedAmp, allAmp)
		} else if allAmp < usedAmp+10 {
			msg = fmt.Sprintf("Warning: Amp usage is close to the limit. Used: %.2f, All: %.2f", usedAmp, allAmp)
		} else {
			msg = fmt.Sprintf("Amp usage is within limits. Used: %.2f, All: %.2f", usedAmp, allAmp)
		}
	}

	return msg, nil
}
func checkProxyAddr(proxyAddr string) (u *url.URL, err error) {
	if proxyAddr == "" {
		return nil, errors.New("proxy addr is empty")
	}

	host, port, err := net.SplitHostPort(proxyAddr)
	if err == nil {
		u = &url.URL{
			Host: net.JoinHostPort(host, port),
		}
		return
	}

	u, err = url.Parse(proxyAddr)
	if err == nil {
		return
	}

	return
}

// SendMsg sends a message using Telegram bot API
func (T *Telegram) SendMsg(text string) (err error) {
	params := url.Values{
		"chat_id": {T.UserID},
		"text":    {text},
	}

	posturl := fmt.Sprintf("https://%s/bot%s/sendMessage", T.APIHost, T.BotToken)

	client := http.Client{
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				u, err := checkProxyAddr(T.Proxy)
				if err != nil {
					return http.ProxyFromEnvironment(req)
				}

				return u, err
			},
		},
	}

	resp, err := client.PostForm(posturl, params)
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram Bot push failed with status code: %d", resp.StatusCode)
	}

	fmt.Println("Telegram Bot push succeeded")
	return nil
}
