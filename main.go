package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type headerFlags map[string]string

var cfAuthorizationCookie *http.Cookie

func (h headerFlags) String() string {
	var headers []string
	for key, value := range h {
		headers = append(headers, key+": "+value)
	}
	return strings.Join(headers, ", ")
}

func (h headerFlags) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid header format (expected 'Key: Value')")
	}
	h[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	return nil
}

func main() {
	var headers headerFlags = make(map[string]string)

	targetPtr := flag.String("target", "", "URL of the Kubernetes API server (e.g., https://kubernetes.default.svc.cluster.local)")
	portPtr := flag.Int("port", 8080, "Port for the proxy server to listen on")
	flag.Var(&headers, "header", "Custom headers to add in requests (format: 'Key: Value')")

	flag.Parse()

	if *targetPtr == "" {
		fmt.Println("No target URL provided. Please provide a target URL.")
		os.Exit(1)
	}

	proxyUrl, err := url.Parse(*targetPtr)
	if err != nil {
		fmt.Printf("Error parsing target URL: %s\n", err)
		os.Exit(1)
	}
	if proxyUrl.Scheme == "" || proxyUrl.Host == "" {
		fmt.Println("Invalid target URL format. Please ensure it is in the format 'https://<host>'")
		os.Exit(1)
	}
	fmt.Printf("Proxy URL: %s\n", proxyUrl.String())

	reverseProxy := httputil.NewSingleHostReverseProxy(proxyUrl)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Original Request URL: %s, Host: %s\n", r.URL.String(), r.Host)

		for key, value := range headers {
			r.Header.Add(key, value)
		}

		// Attach the CF_Authorization cookie to the request if it's available
		if cfAuthorizationCookie != nil {
			r.AddCookie(cfAuthorizationCookie)
		}

		r.URL.Host = proxyUrl.Host
		r.URL.Scheme = proxyUrl.Scheme
		r.Host = proxyUrl.Host

		fmt.Printf("Modified Request URL: %s, Host: %s\n", r.URL.String(), r.Host)
		// Print headers
		for name, values := range r.Header {
			// Loop over all values for the name.
			for _, value := range values {
				fmt.Println(name, value)
			}
		}

		// ModifyResponse function to capture the CF_Authorization cookie
		reverseProxy.ModifyResponse = func(response *http.Response) error {
			// Capture the cookie if it's not already set
			if cfAuthorizationCookie == nil {
				for _, cookie := range response.Cookies() {
					if cookie.Name == "CF_Authorization" {
						cfAuthorizationCookie = cookie
						break
					}
				}
			}
			return nil
		}

		reverseProxy.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", *portPtr)
	fmt.Printf("Starting server on %s\n", addr)
	fmt.Printf("Target URL: %s\n", *targetPtr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
		os.Exit(1)
	}
}
