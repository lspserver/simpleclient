# simpleclient

[![Go Report Card](https://goreportcard.com/badge/github.com/lspserver/simpleclient)](https://goreportcard.com/report/github.com/lspserver/simpleclient)
[![License](https://img.shields.io/github/license/lspserver/simpleclient.svg)](https://github.com/lspserver/simpleclient/blob/main/LICENSE)



## Introduction

*simpleclient* is the client of [lspserver](https://github.com/lspserver) written in Go and JavaScript.



## Run

```bash
# go run main.go <command and arguments to run>
# Open http://localhost:8080/ .

# Init module
go mod init simpleclient

# Echo sent messages to the output area
go run main.go cat

# Run a shell.Try sending "ls" and "cat main.go"
go run main.go sh
```



## License

Project License can be found [here](LICENSE).



## Reference

- [gorilla-websocket](https://github.com/gorilla/websocket/tree/master/examples/command)
