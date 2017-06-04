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

	_, err := newClient(context.Background(), ":", "")
	if err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	*isAnonymous = true
	_, err = newClient(context.Background(), ts.URL, "")
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}

	*isAnonymous = false
	getPassword = func() ([]byte, error) { return nil, errors.New("test error") }
	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	_, err = newClient(context.Background(), ts.URL, fp)
	if err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	getPassword = func() ([]byte, error) { return []byte{}, nil }
	_, err = newClient(context.Background(), ts.URL, fp)
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
}

func TestCreateGist(t *testing.T) {
	_, err := createGist(context.Background(), nil, &gistCreatorMock{isErr: true})
	if err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	_, err = createGist(context.Background(), []string{""}, &gistCreatorMock{})
	if err == nil {
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

	_, err = readFile("")
	if err == nil {
		t.Fatalf("should be fail: %v", err)
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
	err := os.Unsetenv("PATH")
	if err != nil {
		t.Fatalf("should not be fail: %v", err)
	}
	defer func() {
		err := os.Unsetenv("PATH")
		if err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
		err = os.Setenv("PATH", envPath)
		if err != nil {
			t.Fatalf("should not be fail: %v", err)
		}
	}()
	err = openURL("http://example.com/")
	if err == nil {
		t.Fatalf("should be fail: %v", err)
	}

	fp := filepath.Join(os.TempDir(), uuid.NewV4().String())
	err = os.Setenv("PATH", fp)
	if err != nil {
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
	err = openURL("http://example.com/")
	if err != nil {
		t.Fatalf("should be fail: %v", err)
	}
}
