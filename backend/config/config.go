package config

import (
	"fmt"

	"github.com/joho/godotenv"
)

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error loading .env file")
	}
}
