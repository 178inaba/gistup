package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
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
	log.SetPrefix(fmt.Sprintf("%s: ", os.Args[0]))
	flag.Parse()
	os.Exit(run())
}

func run() int {
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return 1
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigCh
		cancel()
	}()

	tokenFilePath, err := getTokenFilePath()
	if err != nil {
		log.Print(err)
		return 1
	}

	c, err := newClient(ctx, "", tokenFilePath)
	if err != nil {
		log.Print(err)
		return 1
	}

	g, err := createGist(ctx, args, c.Gists)
	if err != nil {
		log.Print(err)
		return 1
	}

	if err := openURL(*g.HTMLURL); err != nil {
		fmt.Println(*g.HTMLURL)
	}
	return 0
}

func getTokenFilePath() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultTokenFilePath), nil
}

func newClient(ctx context.Context, apiRawurl, tokenFilePath string) (*github.Client, error) {
	var apiURL *url.URL
	if apiRawurl != "" {
		var err error
		apiURL, err = url.Parse(apiRawurl)
		if err != nil {
			return nil, err
		}
	}

	if *isAnonymous {
		c := github.NewClient(nil)
		if apiURL != nil {
			c.BaseURL = apiURL
		}
		return c, nil
	}

	token, err := readFile(tokenFilePath)
	if err != nil {
		token, err = getToken(apiURL, tokenFilePath)
		if err != nil {
			return nil, err
		}
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	c := github.NewClient(oauth2.NewClient(ctx, ts))
	if apiURL != nil {
		c.BaseURL = apiURL
	}
	return c, nil
}

func getToken(apiURL *url.URL, tokenFilePath string) (string, error) {
	username, password, err := prompt()
	if err != nil {
		return "", err
	}

	t := &github.BasicAuthTransport{Username: username, Password: password}
	c := github.NewClient(t.Client())
	if apiURL != nil {
		c.BaseURL = apiURL
	}
	a, _, err := c.Authorizations.Create(context.Background(), &github.AuthorizationRequest{
		Scopes:      []github.Scope{"gist"},
		Note:        github.String("gistup"),
		Fingerprint: github.String(uuid.NewV4().String()),
	})
	if err != nil {
		return "", err
	}

	token := a.GetToken()
	if err := saveToken(token, tokenFilePath); err != nil {
		return "", err
	}
	return token, nil
}

func prompt() (string, string, error) {
	// Login username from stdin.
	fmt.Print("Username: ")
	var username string
	fmt.Scanln(&username)

	// Password from stdin.
	fmt.Print("Password: ")
	pBytes, err := gopass.GetPasswd()
	if err != nil {
		return "", "", err
	}

	return username, string(pBytes), nil
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

func createGist(ctx context.Context, fileNames []string, gists *github.GistsService) (*github.Gist, error) {
	files := map[github.GistFilename]github.GistFile{}
	for _, fileName := range fileNames {
		var fp string
		if filepath.IsAbs(fileName) {
			fp = fileName
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			fp = filepath.Join(wd, fileName)
		}

		content, err := readFile(fp)
		if err != nil {
			return nil, err
		}

		files[github.GistFilename(filepath.Base(fileName))] =
			github.GistFile{Content: github.String(content)}
	}

	g, _, err := gists.Create(ctx, &github.Gist{
		Description: description,
		Files:       files,
		Public:      isPublic,
	})
	if err != nil {
		return nil, err
	}

	return g, nil
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
