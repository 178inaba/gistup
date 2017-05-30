package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	"github.com/howeyc/gopass"
	homedir "github.com/mitchellh/go-homedir"
	uuid "github.com/satori/go.uuid"
)

const defaultTokenFilePath = ".config/gistup/token"

var (
	isAnonymous = flag.Bool("a", false, "Create anonymous gist")
	description = flag.String("d", "", "Description of gist")
	isPublic    = flag.Bool("p", false, "Create public gist")
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("gistup: ")
	flag.Parse()
	os.Exit(run())
}

func run() int {
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return 1
	}

	var c *github.Client
	ctx := context.Background()
	if !*isAnonymous {
		token, err := loadToken()
		if err != nil {
			token, err = getToken()
			if err != nil {
				log.Print(err)
				return 1
			}
		}

		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		c = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		c = github.NewClient(nil)
	}

	files := map[github.GistFilename]github.GistFile{}
	for _, fileName := range args {
		var fp string
		if filepath.IsAbs(fileName) {
			fp = fileName
		} else {
			wd, err := os.Getwd()
			if err != nil {
				log.Print(err)
				return 1
			}
			fp = filepath.Join(wd, fileName)
		}
		fileName = filepath.Base(fileName)

		content, err := readFile(fp)
		if err != nil {
			log.Print(err)
			return 1
		}

		files[github.GistFilename(fileName)] = github.GistFile{Content: github.String(content)}
	}

	g, _, err := c.Gists.Create(ctx, &github.Gist{
		Description: description,
		Files:       files,
		Public:      isPublic,
	})
	if err != nil {
		log.Print(err)
		return 1
	}

	if err := openURL(*g.HTMLURL); err != nil {
		fmt.Println(*g.HTMLURL)
	}
	return 0
}

func openURL(rawurl string) error {
	openCmd := "xdg-open"
	args := []string{rawurl}
	switch runtime.GOOS {
	case "darwin":
		openCmd = "open"
	case "plan9":
		openCmd = "plumb"
	case "windows":
		openCmd = "rundll32.exe"
		args = append([]string{"url.dll,FileProtocolHandler"}, args...)
	}
	if err := exec.Command(openCmd, args...).Run(); err != nil {
		return err
	}
	return nil
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

	configFilePath, err := getConfigFilePath()
	if err != nil {
		return "", err
	}

	token := a.GetToken()
	if err := saveToken(token, configFilePath); err != nil {
		return "", err
	}
	return token, nil
}

func saveToken(token, configFilePath string) error {
	if err := os.MkdirAll(filepath.Dir(configFilePath), 0700); err != nil {
		return err
	}

	if err := ioutil.WriteFile(configFilePath, []byte(token), 0600); err != nil {
		return err
	}

	return nil
}

func getConfigFilePath() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultTokenFilePath), nil
}
