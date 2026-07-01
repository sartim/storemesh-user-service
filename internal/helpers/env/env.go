package env

import (
	"log"

	"github.com/joho/godotenv"
)

func LoadEnvVars() {
	err := godotenv.Load()
	if err != nil {
		msg := "Error loading .env file"
		log.Panicf("%s: %s", msg, err)
	}
}
