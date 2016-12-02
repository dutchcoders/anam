# ANAM [![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/dutchcoders/anam?utm_source=badge&utm_medium=badge&utm_campaign=&utm_campaign=pr-badge&utm_content=badge) [![Go Report Card](https://goreportcard.com/badge/dutchcoders/anam)](https://goreportcard.com/report/dutchcoders/anam) [![Build Status](https://travis-ci.org/dutchcoders/anam.svg?branch=master)](https://travis-ci.org/dutchcoders/anam)

ANAM will scan all feeded hosts for specific paths, but using a raw tcp stack. Currently we support both http and https, but it will be easy to add additional protocols as well. The stack and anam are both written in Go.

We don't have real benchmarks but it should be possible to retrieve high benchmarks.

ANAM has done earned its miles by checking 10s of millions of sites for the specific /.git/HEAD configuration issue. More information about this project can be found at [http://internetsecure.today/](http://internetsecure.today).

## Install from source

### Install Golang

If you do not have a working Golang environment setup please follow Golang Installation Guide.

### Install ANAM

Installation of ANAM is easy.

```bash
$ go get -u github.com/dutchcoders/anam
$ ./anam
```

## Usage

Parameter | Description | Value
--- | --- | ---
prefix | comma seperated prefixes to prepend for domainname | www,portal,login
port | port to use | 80(http) or 443(https)
threads | amount of threads | 100
timeout | timeout to wait for connection | 10
interface | interface to use | eth0
resolvers | dns resolver to use | 127.0.0.1 or 8.8.8.8
user-agent | user-agent to identify scanner | anam (github.com/dutchcoders/anam)
profiler | start go profiler on port 6060 |
tls | use tls handshake |

## Alexa top 1M sites

The Alexa top 1M sites can be downloaded here:

http://s3.amazonaws.com/alexa-static/top-1m.csv.zip

## Example usage

Using a custom dns resolver is advised, use for example dnsmasq locally. 

We need to disable the RST responses first using iptables, because it will respond to (our) unknown packets with RST otherwise.

```bash
$ iptables -A OUTPUT -p tcp --tcp-flags RST RST -j DROP
````

Now we can start the scanner using: 

```bash
cat top-1m.csv | awk -F, "{ print $2 }" | go run main.go --tls --port 443 --resolver 127.0.0.1 "/.git/config" "/.svn/entries"
```

This software is alpha, expect bugs. Please report them using the issue tracker.

## Benchmarks

Should go here....

## Creators

**Remco Verhoef**
- <https://twitter.com/remco_verhoef>
- <https://twitter.com/dutchcoders>

## Disclaimer

Here should come an appropriate disclaimer, no warranties and anam shouldn't be used for malicious intent.

## Copyright and license

Code and documentation copyright 2016 Remco Verhoef.

Code released under [the Apache license](LICENSE).


