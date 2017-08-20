package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/github"
	tty "github.com/mattn/go-tty"
	uuid "github.com/satori/go.uuid"
)

func TestGetTokenFilePath(t *testing.T) {
	fp, err := getTokenFilePath()
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if !strings.Contains(fp, defaultTokenFilePath) {
		t.Fatalf("%q should be contained in output of config file path: %v",
			defaultTokenFilePath, fp)
	}
}

func TestGetClientWithToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, ``)
	}))
	defer ts.Close()

	*apiRawurl = ":"
	readUsername = func(t *tty.TTY) (string, error) { return "", nil }
	readPassword = func(t *tty.TTY) (string, error) { return "", nil }
	if _, err := getClientWithToken(context.Background(), ""); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	*isAnonymous = true
	*apiRawurl = ts.URL + "/"
	if _, err := getClientWithToken(context.Background(), ""); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}

	*isAnonymous = false
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	readUsername = func(t *tty.TTY) (string, error) { return "", io.EOF }
	readPassword = func(t *tty.TTY) (string, error) { return "", nil }
	if _, err := getClientWithToken(context.Background(), fp); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	*isInsecure = true
	readUsername = func(t *tty.TTY) (string, error) { return "", nil }
	readPassword = func(t *tty.TTY) (string, error) { return "", nil }
	if _, err := getClientWithToken(context.Background(), fp); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
}

func TestGetToken(t *testing.T) {
	canErr := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if canErr {
			canErr = false
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, `{"token":"foobar"}`)
	}))
	defer ts.Close()

	readUsername = func(t *tty.TTY) (string, error) { return "", io.EOF }
	readPassword = func(t *tty.TTY) (string, error) { return "", nil }
	if _, err := getToken(context.Background(), nil, ""); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	apiURL, err := url.Parse(ts.URL + "/")
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}

	readUsername = func(t *tty.TTY) (string, error) { return "", nil }
	readPassword = func(t *tty.TTY) (string, error) { return "", nil }
	if _, err := getToken(context.Background(), apiURL, ""); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	if err := ioutil.WriteFile(fp, []byte(""), 0600); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.Remove(fp); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	if _, err := getToken(context.Background(), apiURL, filepath.Join(fp, "foo")); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	*isInsecure = true
	token, err := getToken(context.Background(), apiURL, fp)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if token != "foobar" {
		t.Fatalf("want %q but %q", "foobar", token)
	}
}

func TestPrompt(t *testing.T) {
	readUsername = func(t *tty.TTY) (string, error) { return "foo", nil }
	readPassword = func(t *tty.TTY) (string, error) { return "bar", nil }
	u, p, err := prompt(context.Background())
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if u != "foo" {
		t.Fatalf("want %q but %q", "foo", u)
	}
	if p != "bar" {
		t.Fatalf("want %q but %q", "bar", u)
	}

	readUsername = func(t *tty.TTY) (string, error) { return "", io.EOF }
	readPassword = func(t *tty.TTY) (string, error) { return "", nil }
	if _, _, err = prompt(context.Background()); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	readUsername = func(t *tty.TTY) (string, error) { return "", nil }
	readPassword = func(t *tty.TTY) (string, error) { return "", io.EOF }
	if _, _, err = prompt(context.Background()); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = prompt(ctx); err != context.Canceled {
		t.Fatalf("should be context canceled: %v", err)
	}
}

func TestSaveToken(t *testing.T) {
	token := "foobar"
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	if err := saveToken(token, fp); err != nil {
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

	if err := saveToken("", filepath.Join(fp, "foo")); err == nil {
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
	if err := saveToken("", errFP); err == nil {
		t.Fatalf("should be fail: %v", err)
	}
}

func TestCreateGist(t *testing.T) {
	filename := uuid.NewV4().String()
	tc := "foobar"
	canErr := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if canErr {
			canErr = false
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, fmt.Sprintf(`{"files":{"%s":{"content":"%s"}}}`, filename, tc))
	}))
	defer ts.Close()

	apiURL, err := url.Parse(ts.URL + "/")
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	c := github.NewClient(nil)
	c.BaseURL = apiURL

	if _, err := createGist(context.Background(), nil, "", c.Gists); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	if _, err := createGist(context.Background(), []string{""}, "", c.Gists); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	fp := filepath.Join(os.TempDir(), filename)
	if err := ioutil.WriteFile(fp, []byte(tc), 0400); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.Remove(fp); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	g, err := createGist(context.Background(), []string{fp}, "", c.Gists)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if *g.Files[github.GistFilename(filename)].Content != tc {
		t.Fatalf("want %q but %q", tc, *g.Files[github.GistFilename(filename)].Content)
	}

	*stdinFilename = filename
	g, err = createGist(context.Background(), nil, tc, c.Gists)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if *g.Files[github.GistFilename(filename)].Content != tc {
		t.Fatalf("want %q but %q", tc, *g.Files[github.GistFilename(filename)].Content)
	}
}

func TestReadFile(t *testing.T) {
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	tc := "foobar"
	if err := ioutil.WriteFile(fp, []byte(tc), 0400); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.Remove(fp); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	content, err := readFile(fp)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if content != tc {
		t.Fatalf("want %q but %q", tc, content)
	}

	if _, err := readFile(""); err == nil {
		t.Fatalf("should be fail: %v", err)
	}
}

func TestOpenURL(t *testing.T) {
	envPath := os.Getenv("PATH")
	if err := os.Unsetenv("PATH"); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("PATH"); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
		if err := os.Setenv("PATH", envPath); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	if err := openURL("http://example.com/"); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	if err := os.Setenv("PATH", fp); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if err := os.Mkdir(fp, 0700); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	bins := []string{"xdg-open", "open", "plumb", "rundll32.exe"}
	for _, bin := range bins {
		if err := ioutil.WriteFile(filepath.Join(fp, bin), []byte("#!/bin/sh\n"), 0500); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}
	defer func() {
		if err := os.RemoveAll(fp); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	if err := openURL("http://example.com/"); err != nil {
		t.Fatalf("should be fail: %v", err)
	}
}
