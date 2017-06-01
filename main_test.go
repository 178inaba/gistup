package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uuid "github.com/satori/go.uuid"
)

func TestReadFile(t *testing.T) {
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	tc := "foobar"
	if err := ioutil.WriteFile(fp, []byte(tc), 0600); err != nil {
		t.Fatalf("should not be nil: %v", err)
	}
	defer func() {
		if err := os.Remove(fp); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	content, err := readFile(fp)
	if err != nil {
		t.Fatalf("should not be nil: %v", err)
	}
	if content != tc {
		t.Fatalf("want %q but %q", tc, content)
	}

	_, err = readFile("")
	if err == nil {
		t.Fatalf("should not be nil: %v", err)
	}
}

func TestSaveToken(t *testing.T) {
	token := "foobar"
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	err := saveToken(token, fp)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.Remove(fp); err != nil {
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

	errFP := filepath.Join(os.TempDir(), uuid.NewV4().String(), uuid.NewV4().String())
	if err := os.MkdirAll(errFP, 0700); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(filepath.Dir(errFP)); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	err = saveToken("", errFP)
	if err == nil {
		t.Fatalf("should be fail: %v", err)
	}
}

func TestGetConfigFilePath(t *testing.T) {
	fp, err := getConfigFilePath()
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if !strings.Contains(fp, defaultTokenFilePath) {
		t.Fatalf("%q should be contained in output of config file path: %v",
			defaultTokenFilePath, fp)
	}
}
