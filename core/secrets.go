package core

import (
	"fmt"
	"os"
)

func GetSecret(key string, required bool) string {
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}

	if required {
		panic(fmt.Sprintf("environment variable %s is not set", key))
	}

	return ""
}
