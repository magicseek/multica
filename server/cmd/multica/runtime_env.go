package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

const runtimeEnvFileEnv = "MULTICA_RUNTIME_ENV_FILE"

var (
	runtimeEnvOnce   sync.Once
	runtimeEnvValues map[string]string
)

func runtimeEnvValue(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	runtimeEnvOnce.Do(loadRuntimeEnv)
	return runtimeEnvValues[key]
}

func loadRuntimeEnv() {
	runtimeEnvValues = map[string]string{}
	path := strings.TrimSpace(os.Getenv(runtimeEnvFileEnv))
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		return
	}
	runtimeEnvValues = values
}
