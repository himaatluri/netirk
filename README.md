# Netirk

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fhimasagaratluri%2Fnetirk.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fhimasagaratluri%2Fnetirk?ref=badge_shield) [![Go Report Card](https://goreportcard.com/badge/github.com/himasagaratluri/netirk)](https://goreportcard.com/report/github.com/himasagaratluri/netirk)

[dev.to - Introducing NetIrk](https://dev.to/himaatluri/introducing-netirk-a-lightweight-cli-tool-for-high-level-network-insights-5e4p)

```(shell)
               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
```

A CLI that can run some network analysis.

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
➜  netirk git:(tls) ✗ ./netirk check --target google.com --verify-ssl
               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
                                  

Getting server certs...

➥ Cert: 0 
 ￫ CA: false
 ￫ Issuer: WR2
 ￫ Expiry: Monday, 13-Jan-25 08:36:56 UTC
 ￫ PublicKey: 
   -----BEGIN CERTIFICATE-----
   
```

### Trace

```(shell)
➜  netirk git:(tls) ✗ ./netirk trace --host https://amazon.com --port 8080

               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
                                  

DNS Resolution done: 7.618718ms
Connect Done: 26.686553ms
Connect Done: 28.837579ms
Connect Done: 28.027495ms
Request failed: dial tcp 54.239.28.85:8080: connect: connection refused
➜  netirk git:(tls) ✗ ./netirk trace --host https://amazon.com --port 443 

               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
                                  

DNS Resolution done: 7.553307ms
Connect Done: 27.578134ms
TLS Handshake Done: 83.745488ms
Time to first byte: 147.988115ms
```

### Server

```(shell)
➜  netirk git:(tls) ✗ curl localhost:8080/host
hostname-prints/returned

➜  netirk git:(tls) ✗ curl localhost:8080/health
healthy          
```

```(shell)
➜  netirk git:(tls) ✗ ./netirk server                                   
               _    _        _ 
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
                                  

Starting a simple http server on port:  8080
2024/11/25 23:46:37 request:  GET /host
2024/11/25 23:46:44 request:  GET /health
```
