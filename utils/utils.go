package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
)

// Structs for Telegram and RequestData remain the same as previously defined
type Telegram struct {
	BotToken string
	UserID   string
	APIHost  string
	Proxy    string
}

// Email holds Gmail API credential files and user info
type Email struct {
	CredentialsFile string // path to credentials.json
	TokenFile       string // path to token.json
	User            string // email address of the authenticated user
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
	Email       Email
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

	// Create an HTTP client with more permissive TLS configuration
	// Create HTTP client with Go 1.24 compatible TLS configuration
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				},
			},
			ForceAttemptHTTP2: false,
		},
	}
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

	// Process the response with remaining-based logic
	usedAmp := res.Data.UsedAmp
	allAmp := res.Data.AllAmp
	remaining := allAmp - usedAmp
	const warningThreshold = 20.0
	if remaining < 0 {
		msg = fmt.Sprintf("Warning: Exceeded limit by %.2f!", -remaining)
	} else if remaining <= warningThreshold {
		msg = fmt.Sprintf("Warning: Remaining electricity is low: %.2f", remaining)
	} else {
		msg = fmt.Sprintf("Remaining electricity: %.2f", remaining)
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

// getTokenFromWeb requests a token from the web, then returns the retrieved token
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	return tok, nil
}

// saveToken saves a token to a file path
func saveToken(path string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// getClient reads token file or performs OAuth flow to get HTTP client
func getClient(ctx context.Context, config *oauth2.Config, tokenFile string) (*http.Client, error) {
	b, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		// Token file doesn't exist, get token from web
		token, err := getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokenFile, token); err != nil {
			return nil, err
		}
		return config.Client(ctx, token), nil
	}

	token := &oauth2.Token{}
	if err := json.Unmarshal(b, token); err != nil {
		return nil, fmt.Errorf("unable to parse token file: %w", err)
	}
	return config.Client(ctx, token), nil
}

// SendEmail sends a message via Gmail API
func (E *Email) SendEmail(body string) error {
	ctx := context.Background()
	b, err := ioutil.ReadFile(E.CredentialsFile)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w", err)
	}
	cfg, err := google.ConfigFromJSON(b, gmail.GmailSendScope)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file: %w", err)
	}
	client, err := getClient(ctx, cfg, E.TokenFile)
	if err != nil {
		return err
	}
	srv, err := gmail.New(client)
	if err != nil {
		return fmt.Errorf("unable to retrieve Gmail client: %w", err)
	}
	// create RFC822 email message
	msgStr := fmt.Sprintf("To: %s\r\nSubject: Electricity Alert\r\n\r\n%s", E.User, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(msgStr))
	msg := &gmail.Message{Raw: encoded}
	_, err = srv.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return fmt.Errorf("unable to send email via Gmail API: %w", err)
	}
	log.Println("Gmail API push succeeded")
	return nil
}
