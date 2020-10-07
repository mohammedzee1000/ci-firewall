package util

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string, failifnotfound bool) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if failifnotfound {
		log.Fatalf("required env %s not set", key)
	}
	return defaultVal
}

func GenerateEnvFile(path string, values map[string]string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("failed to open env file %w", err)
	}
	defer f.Close()
	dw := bufio.NewWriter(f)
	for k, v := range values {
		dw.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return dw.Flush()
}
