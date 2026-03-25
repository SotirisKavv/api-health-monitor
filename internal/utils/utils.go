package utils

import "os"

func GetEnv(key, fallback string) string {

	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return fallback
}
