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
			fmt.Println("Attempt failed, retrying...")
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
		} else {
			break
		}
	}

	// Handle failure after maximum retries
	if count == maxRetries {
		errMsg := "Error: Maximum retry limit reached."
		conf.Telegram.SendMsg(errMsg)
		log.Fatal(errMsg)
	} else {
		// Send the successful message via Telegram
		err = conf.Telegram.SendMsg(msg)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Message sent successfully:", msg)
	}
}
