package config

import (
	"log"
	"os"
)

func env(variableName, defaultVal string) string {
	if val := os.Getenv(variableName); val != "" {
		return val
	}

	if defaultVal == "" {
		log.Panicf("Require %s variable in env", variableName)
		os.Exit(1)
	}

	return defaultVal
}
