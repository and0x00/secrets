package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"time"

	"golang.org/x/net/http2"
	"gopkg.in/yaml.v3"
)

type Pattern struct {
	Name     string   `yaml:"name"`
	Regex    []string `yaml:"regex"`
	Severity string   `yaml:"severity"`
}

var patterns []Pattern

func init() {
	bytes, err := os.ReadFile("patterns.yaml")
	if err != nil {
		panic(err)
	}

	// Decode the YAML file into a list of patterns
	err = yaml.Unmarshal(bytes, &patterns)
	if err != nil {
		panic(err)
	}
}

func main() {
	file := flag.String("f", "", "File containing list of URLs")
	redirect := flag.Bool("redirect", false, "Allow redirect?")
	// delay := flag.String("delay", "1000", "File containing list of URLs")
	// url := flag.String("u", "", "Single URL")
	flag.Parse()

	var input io.Reader
	input = os.Stdin

	if *file != "" {
		file, err := os.Open(*file)
		if err != nil {
			fmt.Printf("failed to open file: %s\n", err)
			os.Exit(1)
		}
		input = file
	}

	sc := bufio.NewScanner(input)
	for sc.Scan() {
		body, err := getRequest(sc.Text(), *redirect)
		if err != nil {
			fmt.Printf("error [%s]\n", sc.Text())
			//panic(err)
		}
		for _, pattern := range patterns {
			// Verify each regex of the pattern
			for _, regex := range pattern.Regex {
				// Compile the regex
				r, err := regexp.Compile(regex)
				if err != nil {
					panic(err)
				}
				if r.MatchString(body) {
					matches := r.FindAllString(body, -1)
					for _, match := range matches {
						fmt.Printf("[%v] [%v] [%v] [%v]\n", sc.Text(), pattern.Severity, pattern.Name, match)
					}
				}
			}
		}
	}

	if err := sc.Err(); err != nil {
		panic(err)
	}
}

func defaultHTTPClient(checkRedirect func(req *http.Request, via []*http.Request) error) *http.Client {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Second * 60,
		}).DialContext,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 500,
		MaxConnsPerHost:     500,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true, Renegotiation: tls.RenegotiateOnceAsClient},
	}

	http2.ConfigureTransport(tr)

	return &http.Client{
		Timeout:       time.Duration(30) * time.Second,
		Transport:     tr,
		CheckRedirect: checkRedirect,
	}
}

func getRequest(URL string, redirect bool) (string, error) {
	client := defaultHTTPClient(nil)
	if redirect {
		client = defaultHTTPClient(func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		})
	}
	req, _ := http.NewRequest("GET", URL, nil)
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36")
	req.Header.Add("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
