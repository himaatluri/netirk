# Netirk
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fhimasagaratluri%2Fnetirk.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fhimasagaratluri%2Fnetirk?ref=badge_shield)

A Cli that can run some network analysis.

## Instructions

### Usage

```(shell)
go install github.com/himasagaratluri/netirk

netirk check
```

### output

### About

```(shell)
⚡  netirk help check
verify if host resp is OK

Usage:
  netirk check [flags]

Flags:
  -h, --help   help for check
```

### In action

```(shell)
2023/04/05 23:35:10 ------
2023/04/05 23:35:10 Netirk
2023/04/05 23:35:10 ------
2023/04/05 23:35:10 
Testing the url:        https://google.com:443
2023/04/05 23:35:11 Success, the endpoint is reachable!  
```

### Development

```(shell)
go mod init github.com/himasagaratluri/netirk

go install github.com/spf13/cobra

cobra init --pkg-name netirk
```
