package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/github"
	uuid "github.com/satori/go.uuid"
)

type gistCreatorMock struct {
	isErr bool
}

func (m *gistCreatorMock) Create(ctx context.Context, gist *github.Gist) (*github.Gist, *github.Response, error) {
	if m.isErr {
		return nil, nil, errors.New("mock error")
	}
	return gist, nil, nil
}

func TestNewClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, ``)
	}))
	defer ts.Close()

	if _, err := newClient(context.Background(), ":", ""); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	*isAnonymous = true
	if _, err := newClient(context.Background(), ts.URL, ""); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}

	*isAnonymous = false
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	if _, err := newClient(context.Background(), ts.URL, fp); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	tmpStdin := os.Stdin
	os.Stdin = pr
	if _, err := pw.WriteString("\n\n"); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		os.Stdin = tmpStdin
		if err := pr.Close(); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
		if err := pw.Close(); err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	if _, err := newClient(context.Background(), ts.URL, fp); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
}

func TestCreateGist(t *testing.T) {
	if _, err := createGist(context.Background(), nil, &gistCreatorMock{isErr: true}); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	if _, err := createGist(context.Background(), []string{""}, &gistCreatorMock{}); err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	fileName := uuid.NewV4().String()
	fp := filepath.Join(os.TempDir(), fileName)
	tc := "foobar"
	if err := ioutil.WriteFile(fp, []byte(tc), 0500); err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	g, err := createGist(context.Background(), []string{fp}, &gistCreatorMock{})
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	if *g.Files[github.GistFilename(fileName)].Content != tc {
		t.Fatalf("want %q but %q", tc, *g.Files[github.GistFilename(fileName)].Content)
	}
}

func TestReadFile(t *testing.T) {
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	tc := "foobar"
	if err := ioutil.WriteFile(fp, []byte(tc), 0600); err != nil {
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
