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
	containPath := ".config/gistup/token"
	if !strings.Contains(configFilePath, containPath) {
		t.Fatalf("%q should be contained in output of config file path: %v", containPath, configFilePath)
	}
}
