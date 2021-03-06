# gistup

[![Build Status](https://travis-ci.org/178inaba/gistup.svg?branch=master)](https://travis-ci.org/178inaba/gistup)
[![Coverage Status](https://coveralls.io/repos/github/178inaba/gistup/badge.svg?branch=master)](https://coveralls.io/github/178inaba/gistup?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/178inaba/gistup)](https://goreportcard.com/report/github.com/178inaba/gistup)

Gist uploader for cli.

## Usage

```console
$ gistup target.go

# Or upload from standard input.
$ stdin | gistup
```

The github gist opens automatically in the browser.  
If the browser can not be opened, the URL will be displayed.

The token is saved in the following file and omitting the user name and password entry from next time:

* macOS, Linux
  * `~/.config/gistup/token`
* Windows
  * `%APPDATA%\gistup\token`

### Options

* `-a`
  * Create anonymous gist.
* `-d <description>`
  * Description of gist.
* `-insecure`
  * Allow connections to SSL sites without certs.
* `-n <file_name>`
  * File name when upload standard input.
* `-p`
  * Create public gist.
* `-url <api_baseurl>`
  * For GitHub Enterprise, specify the base URL of the API.

## Install

```console
$ go get -u github.com/178inaba/gistup
```

## License

[MIT](LICENSE)

## Author

[178inaba](https://github.com/178inaba)
