package neversorrow

import "os"

// EnvOr returns an environment variable, or defaultValue if it's empty
func EnvOr(envVar string, defaultValue string) string {
	val := os.Getenv(envVar)

	if val == "" {
		return defaultValue
	}

	return val
}
