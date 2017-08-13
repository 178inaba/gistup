package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"

	"golang.org/x/oauth2"

	"github.com/google/go-github/github"
	tty "github.com/mattn/go-tty"
	homedir "github.com/mitchellh/go-homedir"
	toml "github.com/pelletier/go-toml"
	uuid "github.com/satori/go.uuid"
)

const (
	configDirName  = "gistup"
	configFileName = "config.toml"
	tokenFileName  = "token"
)

var (
	isAnonymous   = flag.Bool("a", false, "Create anonymous gist")
	description   = flag.String("d", "", "Description of gist")
	isInsecure    = flag.Bool("insecure", false, "Allow connections to SSL sites without certs")
	stdinFileName = flag.String("n", "", "File name when upload standard input")
	isPublic      = flag.Bool("p", false, "Create public gist")
	apiRawurl     = flag.String("url", "", "For GitHub Enterprise, specify the base URL of the API")

	// Variable function for testing.
	readUsername = func(t *tty.TTY) (string, error) {
		return t.ReadString()
	}
	readPassword = func(t *tty.TTY) (string, error) {
		return t.ReadPasswordNoEcho()
	}
)

type config struct {
	APIRawurl  string   `toml:"url"`
	APIURL     *url.URL `toml:"-"`
	IsInsecure bool     `toml:"insecure"`
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(fmt.Sprintf("%s: ", os.Args[0]))
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-a] [-d <description>] [-insecure] [-p] [-url <api_baseurl>] <file>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       stdin | %s [-a] [-d <description>] [-n <file_name>] [-p]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	os.Exit(run())
}

func run() int {
	args := flag.Args()
	var stdinContent string
	if len(args) == 0 {
		bs, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Print(err)
			return 1
		}
		stdinContent = string(bs)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigCh
		cancel()
	}()

	confDirPath, err := getConfigDir()
	if err != nil {
		log.Print(err)
		return 1
	}

reAuth:
	c, err := getClientWithToken(ctx, confDirPath)
	if err != nil {
		log.Print(err)
		return 1
	}

	g, err := createGist(ctx, args, stdinContent, c.Gists)
	if err != nil {
		// If bad token, Authentication again.
		if errResp, ok := err.(*github.ErrorResponse); ok &&
			errResp.Response.StatusCode == http.StatusUnauthorized {
			// Remove bad token file.
			if err := os.RemoveAll(confDirPath); err != nil {
				log.Print(err)
				return 1
			}

			// Authentication again.
			fmt.Println("Bad token. Authentication again.")
			goto reAuth
		}

		log.Print(err)
		return 1
	}

	if err := openURL(*g.HTMLURL); err != nil {
		fmt.Println(*g.HTMLURL)
	}
	return 0
}

func getConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), configDirName), nil
	}
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", configDirName), nil
}

func getClientWithToken(ctx context.Context, confDirPath string) (*github.Client, error) {
	// Read config.
	conf, err := getConfig(confDirPath)
	if err != nil {
		return nil, err
	}

	if *isAnonymous {
		c := github.NewClient(nil)
		if conf.APIURL != nil {
			c.BaseURL = conf.APIURL
		}
		return c, nil
	}

	tokenFilePath := filepath.Join(confDirPath, tokenFileName)
	bs, err := readFile(tokenFilePath)
	token := string(bs)
	if err != nil {
		token, err = getToken(ctx, conf, tokenFilePath)
		if err != nil {
			return nil, err
		}
	}

	if conf.IsInsecure {
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: tr})
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	c := github.NewClient(oauth2.NewClient(ctx, ts))
	if conf.APIURL != nil {
		c.BaseURL = conf.APIURL
	}
	return c, nil
}

func getConfig(confDirPath string) (*config, error) {
	confFilePath := filepath.Join(confDirPath, configFileName)
	var conf config
	bs, err := readFile(confFilePath)
	if err == nil {
		if err := toml.Unmarshal(bs, &conf); err != nil {
			return nil, err
		}
	}

	conf.IsInsecure = *isInsecure
	if *apiRawurl != "" {
		conf.APIRawurl = *apiRawurl
	}

	if conf.APIRawurl != "" {
		conf.APIURL, err = url.Parse(conf.APIRawurl)
		if err != nil {
			return nil, err
		}
	}

	if *apiRawurl != "" {
		bs, err := toml.Marshal(conf)
		if err != nil {
			return nil, err
		}
		if err := save(string(bs), confFilePath); err != nil {
			return nil, err
		}
	}

	return &conf, nil
}

func getToken(ctx context.Context, conf *config, tokenFilePath string) (string, error) {
	username, password, err := prompt(ctx)
	if err != nil {
		return "", err
	}

	t := &github.BasicAuthTransport{Username: username, Password: password}
	if conf.IsInsecure {
		t.Transport =
			&http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	c := github.NewClient(t.Client())
	if conf.APIURL != nil {
		c.BaseURL = conf.APIURL
	}
	a, _, err := c.Authorizations.Create(ctx, &github.AuthorizationRequest{
		Scopes:      []github.Scope{"gist"},
		Note:        github.String("gistup"),
		Fingerprint: github.String(uuid.NewV4().String()),
	})
	if err != nil {
		return "", err
	}

	token := a.GetToken()
	if err := save(token, tokenFilePath); err != nil {
		return "", err
	}
	return token, nil
}

func prompt(ctx context.Context) (string, string, error) {
	t, err := tty.Open()
	if err != nil {
		return "", "", err
	}
	defer t.Close()

	// Login username from tty.
	username, err := readString(ctx, "Username", readUsername, t)
	if err != nil {
		return "", "", err
	}

	// Password from tty.
	password, err := readString(ctx, "Password", readPassword, t)
	if err != nil {
		return "", "", err
	}

	return username, password, nil
}

func readString(ctx context.Context, hint string, readFunc func(t *tty.TTY) (string, error), t *tty.TTY) (string, error) {
	fmt.Printf("%s: ", hint)
	ch := make(chan string)
	errCh := make(chan error)
	go func() {
		s, err := readFunc(t)
		if err != nil {
			errCh <- err
		}
		ch <- s
	}()
	var s string
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case s = <-ch:
	case err := <-errCh:
		return "", err
	}
	return s, nil
}

func save(s, saveFilePath string) error {
	if err := os.MkdirAll(filepath.Dir(saveFilePath), 0700); err != nil {
		return err
	}
	if err := ioutil.WriteFile(saveFilePath, []byte(s), 0600); err != nil {
		return err
	}
	return nil
}

func createGist(ctx context.Context, fileNames []string, stdinContent string, gists *github.GistsService) (*github.Gist, error) {
	files := map[github.GistFilename]github.GistFile{}
	if len(fileNames) != 0 {
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

			bs, err := readFile(fp)
			if err != nil {
				return nil, err
			}

			files[github.GistFilename(filepath.Base(fileName))] =
				github.GistFile{Content: github.String(string(bs))}
		}
	} else {
		files[github.GistFilename(*stdinFileName)] =
			github.GistFile{Content: github.String(stdinContent)}
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

func readFile(fp string) ([]byte, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	bs, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return bs, nil
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
