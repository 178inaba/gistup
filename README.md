# gistup

[![Build Status](https://travis-ci.org/178inaba/gistup.svg?branch=master)](https://travis-ci.org/178inaba/gistup)
[![Coverage Status](https://coveralls.io/repos/github/178inaba/gistup/badge.svg?branch=master)](https://coveralls.io/github/178inaba/gistup?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/178inaba/gistup)](https://goreportcard.com/report/github.com/178inaba/gistup)

Gist uploader for cli

## Usage

```console
$ gistup target.go
```

The github gist opens automatically in the browser.  
If the browser can not be opened, the URL will be displayed.

### Options

* `-a`
  * Create anonymous gist.
* `-d <description>`
  * Description of gist.
* `-n <file_name>`
  * File name when upload standard input.
* `-p`
  * Create public gist.

## Install

```console
$ go get -u github.com/178inaba/gistup
```

## License

[MIT](LICENSE)

## Author

[178inaba](https://github.com/178inaba)
