package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var variables = []string{"PORT"}

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error loading .env file")
	}

	for _, env := range variables {
		_, envExists := os.LookupEnv(env)
		if !envExists {
			panic(fmt.Sprintf("Missing env variable '%s'. Aborting...\n", env))
		}
	}
}
