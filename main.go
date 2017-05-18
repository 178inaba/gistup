package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/howeyc/gopass"
	homedir "github.com/mitchellh/go-homedir"
	uuid "github.com/satori/go.uuid"
)

const defaultTokenFilePath = ".config/gistup/token"

func main() {
	os.Exit(run())
}

func run() int {
	token, err := loadToken()
	if err != nil {
		token, err = getToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	c := github.NewClient(oauth2.NewClient(ctx, ts))

	files := map[github.GistFilename]github.GistFile{}
	for _, fileName := range os.Args[1:] {
		//		fileName := os.Args[1]
		var fp string
		if filepath.IsAbs(fileName) {
			fp = fileName
		} else {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fp = filepath.Join(wd, fileName)
		}
		fileName = filepath.Base(fileName)

		content, err := readFile(fp)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		files[github.GistFilename(fileName)] = github.GistFile{Content: github.String(content)}
	}

	g, _, err := c.Gists.Create(ctx, &github.Gist{
		Files:  files,
		Public: github.Bool(false),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(*g.HTMLURL)
	return 0
}

func readFile(fp string) (string, error) {
	f, err := os.Open(fp)
	if err != nil {
		return "", err
	}
	defer f.Close()

	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(bs), nil
}

func loadToken() (string, error) {
	configFilePath, err := getConfigFilePath()
	if err != nil {
		return "", err
	}

	return readFile(configFilePath)
}

func getToken() (string, error) {
	// Login username from stdin.
	var username string
	fmt.Print("Username: ")
	fmt.Scanln(&username)

	// Password from stdin.
	fmt.Print("Password: ")
	pBytes, err := gopass.GetPasswd()
	if err != nil {
		return "", err
	}
	password := string(pBytes)

	t := &github.BasicAuthTransport{Username: username, Password: password}
	c := github.NewClient(t.Client())
	a, _, err := c.Authorizations.Create(context.Background(), &github.AuthorizationRequest{
		Scopes:      []github.Scope{"gist"},
		Note:        github.String("gistup"),
		Fingerprint: github.String(uuid.NewV4().String()),
	})
	if err != nil {
		return "", err
	}

	// Save token.
	configFilePath, err := getConfigFilePath()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(configFilePath), os.ModePerm); err != nil {
		return "", err
	}

	configFile, err := os.Create(configFilePath)
	if err != nil {
		return "", err
	}
	defer configFile.Close()

	if _, err := configFile.WriteString(a.GetToken()); err != nil {
		return "", err
	}

	return a.GetToken(), nil
}

func getConfigFilePath() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultTokenFilePath), nil
}
