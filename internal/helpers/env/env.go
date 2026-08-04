package env

import "github.com/joho/godotenv"

// LoadEnvVars loads development environment variables from .env.
// The caller decides whether a missing or malformed file is fatal.
func LoadEnvVars() error {
	return godotenv.Load()
}
