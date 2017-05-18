package main

import (
	"strings"
	"testing"
)

func TestGetConfigFilePath(t *testing.T) {
	configFilePath, err := getConfigFilePath()
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if !strings.Contains(configFilePath, defaultTokenFilePath) {
		t.Fatalf("%q should be contained in output of config file path: %v",
			defaultTokenFilePath, configFilePath)
	}
}
