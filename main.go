package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
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

type gistCreator interface {
	Create(ctx context.Context, gist *github.Gist) (*github.Gist, *github.Response, error)
}

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

	c, err := newClient(ctx)
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

func newClient(ctx context.Context) (*github.Client, error) {
	if *isAnonymous {
		return github.NewClient(nil), nil
	}

	configFilePath, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}

	token, err := readFile(configFilePath)
	if err != nil {
		token, err = getToken()
		if err != nil {
			return nil, err
		}
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return github.NewClient(oauth2.NewClient(ctx, ts)), nil
}

func createGist(ctx context.Context, fileNames []string, gists gistCreator) (*github.Gist, error) {
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
