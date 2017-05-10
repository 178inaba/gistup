package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/go-github/github"
	"github.com/howeyc/gopass"
	homedir "github.com/mitchellh/go-homedir"
	uuid "github.com/satori/go.uuid"
)

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

	fmt.Println(token)
	return 0

	// ctx := context.Background()
	// c := github.NewClient(nil)
	// c.Authorizations.Create(ctx, &github.AuthorizationRequest{
	// 	Scopes:      []github.Scope{"gist"},
	// 	Note:        github.String("gistup"),
	// 	Fingerprint: github.String("gistup"),
	// })

	// g, _, err := c.Gists.Create(ctx, &github.Gist{
	// 	Files: map[github.GistFilename]github.GistFile{
	// 		"main.go": github.GistFile{Content: github.String("package main")},
	// 	},
	// 	Public: github.Bool(false),
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(*g.HTMLURL)
}

func loadToken() (string, error) {
	configFilePath, err := getConfigFilePath()
	if err != nil {
		return "", err
	}

	configFile, err := os.Open(configFilePath)
	if err != nil {
		return "", err
	}
	defer configFile.Close()

	tBytes, err := ioutil.ReadAll(configFile)
	if err != nil {
		return "", err
	}

	return string(tBytes), nil
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

	os.MkdirAll(filepath.Dir(configFilePath), os.ModePerm)
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

	return filepath.Join(home, ".config/gistup/config.toml"), nil
}
