package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile(t *testing.T) {
	testFilePath := "test.tmp"
	testContent := "test"
	if err := ioutil.WriteFile(testFilePath, []byte(testContent), 0600); err != nil {
		t.Fatalf("should not be nil: %v", err)
	}
	defer func() {
		if err := os.Remove(testFilePath); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	content, err := readFile(testFilePath)
	if err != nil {
		t.Fatalf("should not be nil: %v", err)
	}
	if content != testContent {
		t.Fatalf("want %q but %q", testContent, content)
	}
}

func TestSaveToken(t *testing.T) {
	token := "abcde"
	fp := "/tmp/gistup/token"
	err := saveToken(token, fp)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(filepath.Dir(fp)); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	f, err := os.Open(fp)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	mode := fi.Mode()
	if mode != 0600 {
		t.Fatalf("want %#o but %#o", 0600, mode)
	}
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if string(bs) != token {
		t.Fatalf("want %q but %q", token, string(bs))
	}

	err = saveToken("", filepath.Join(fp, "foo"))
	if err == nil {
		t.Fatalf("should be fail: %v", err)
	}
}

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
