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
	"github.com/google/uuid"
	tty "github.com/mattn/go-tty"
	homedir "github.com/mitchellh/go-homedir"
)

const tokenFileEdgePath = "gistup/token"

var (
	isAnonymous   = flag.Bool("a", false, "Create anonymous gist")
	description   = flag.String("d", "", "Description of gist")
	isInsecure    = flag.Bool("insecure", false, "Allow connections to SSL sites without certs")
	stdinFilename = flag.String("n", "", "File name when upload standard input")
	isPublic      = flag.Bool("p", false, "Create public gist")
	apiRawurl     = flag.String("url", "", "For GitHub Enterprise, specify the base URL of the API")

	// Variable function for testing.
	readUsername = func(t *tty.TTY) (string, error) { return t.ReadString() }
	readPassword = func(t *tty.TTY) (string, error) { return t.ReadPasswordNoEcho() }
	runCmd       = func(c *exec.Cmd) error { return c.Run() }
	removeFile   = func(name string) error { return os.Remove(name) }
	mkdirAll     = func(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
	writeFile    = func(filename string, data []byte, perm os.FileMode) error {
		return ioutil.WriteFile(filename, data, perm)
	}
)

func main() {
	log.SetFlags(0)
	log.SetPrefix(fmt.Sprintf("%s: ", os.Args[0]))
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-a] [-d <description>] [-insecure] [-p] [-url <api_baseurl>] <file>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       stdin | %s [-a] [-d <description>] [-n <file_name>] [-p]\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "The token is saved in the following file and omitting the user name and password entry from next time:")
		if runtime.GOOS == "windows" {
			fmt.Fprintf(os.Stderr, "%%APPDATA%%\\gistup\\token\n")
		} else {
			fmt.Fprintln(os.Stderr, "~/.config/gistup/token")
		}
		fmt.Fprintln(os.Stderr)
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

	tokenFilePath, err := getTokenFilePath()
	if err != nil {
		log.Print(err)
		return 1
	}

reAuth:
	c, err := getClientWithToken(ctx, tokenFilePath)
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
			if err := removeFile(tokenFilePath); err != nil {
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

	if err := openURL(g.GetHTMLURL()); err != nil {
		fmt.Println(g.GetHTMLURL())
	}
	return 0
}

func getTokenFilePath() (string, error) {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), tokenFileEdgePath), nil
	}
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", tokenFileEdgePath), nil
}

func getClientWithToken(ctx context.Context, tokenFilePath string) (*github.Client, error) {
	var apiURL *url.URL
	if *apiRawurl != "" {
		var err error
		apiURL, err = url.Parse(*apiRawurl)
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
		token, err = getToken(ctx, apiURL, tokenFilePath)
		if err != nil {
			return nil, err
		}
	}

	if *isInsecure {
		tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: tr})
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	c := github.NewClient(oauth2.NewClient(ctx, ts))
	if apiURL != nil {
		c.BaseURL = apiURL
	}
	return c, nil
}

func getToken(ctx context.Context, apiURL *url.URL, tokenFilePath string) (string, error) {
	username, password, err := prompt(ctx)
	if err != nil {
		return "", err
	}

	t := &github.BasicAuthTransport{Username: username, Password: password}
	if *isInsecure {
		t.Transport =
			&http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	c := github.NewClient(t.Client())
	if apiURL != nil {
		c.BaseURL = apiURL
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	a, _, err := c.Authorizations.Create(ctx, &github.AuthorizationRequest{
		Scopes:      []github.Scope{"gist"},
		Note:        github.String("gistup"),
		Fingerprint: github.String(u.String()),
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

func saveToken(token, tokenFilePath string) error {
	if err := mkdirAll(filepath.Dir(tokenFilePath), 0700); err != nil {
		return err
	}

	if err := writeFile(tokenFilePath, []byte(token), 0600); err != nil {
		return err
	}

	return nil
}

func createGist(ctx context.Context, filenames []string, stdinContent string, gists *github.GistsService) (*github.Gist, error) {
	files := map[github.GistFilename]github.GistFile{}
	if len(filenames) != 0 {
		for _, filename := range filenames {
			var fp string
			if filepath.IsAbs(filename) {
				fp = filename
			} else {
				wd, err := os.Getwd()
				if err != nil {
					return nil, err
				}
				fp = filepath.Join(wd, filename)
			}

			content, err := readFile(fp)
			if err != nil {
				return nil, err
			}

			files[github.GistFilename(filepath.Base(filename))] =
				github.GistFile{Content: github.String(content)}
		}
	} else {
		files[github.GistFilename(*stdinFilename)] =
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
	if err := runCmd(exec.Command(openCmd, args...)); err != nil {
		return err
	}
	return nil
}
