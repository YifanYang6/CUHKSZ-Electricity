package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/YifanYang6/CUHKSZ-Electricity/utils"
)

func main() {
	// Load the config file path from command-line arguments
	var configPath string
	flag.StringVar(&configPath, "c", "config/config.json", "config.json file path")
	flag.Parse()

	// Load the configuration from the JSON file
	conf := utils.LoadConfig(configPath)

	// Retry logic parameters
	count, maxRetries, sleepSeconds := 0, 5, 5
	var msg string
	var err error

	// Retry loop to get the message
	for count < maxRetries {
		msg, err = conf.RequestData.GetMsg() // Get the message from the API
		if err != nil || msg == "Failed to retrieve data" {
			count++
			fmt.Printf("Attempt %d failed, retrying... Error: %v\n", count, err)
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
		} else {
			break
		}
	}

	// Handle failure after maximum retries
	if count == maxRetries {
		errMsg := "Error: Maximum retry limit reached."
		conf.Telegram.SendMsg(errMsg)
		// Send email for critical errors
		if emailErr := conf.Email.SendEmail(errMsg); emailErr != nil {
			log.Printf("Failed to send email notification: %v", emailErr)
		}
		log.Fatal(errMsg)
	} else {
		// Send the successful message via Telegram
		err = conf.Telegram.SendMsg(msg)
		if err != nil {
			log.Printf("Failed to send Telegram message: %v", err)
		} else {
			fmt.Println("Telegram message sent successfully:", msg)
		}

		// Only send email for warning messages
		if isWarning(msg) {
			emailErr := conf.Email.SendEmail(msg)
			if emailErr != nil {
				log.Printf("Failed to send email: %v", emailErr)
			} else {
				fmt.Println("Email sent successfully:", msg)
			}
		}

		// Only exit with error if Telegram failed (email is optional for non-warnings)
		if err != nil {
			log.Fatal("Telegram delivery failed")
		}
	}
}

// isWarning checks if the message contains warning information
func isWarning(msg string) bool {
	return len(msg) >= 7 && msg[:7] == "Warning"
}
