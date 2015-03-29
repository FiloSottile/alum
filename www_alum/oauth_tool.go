// +build tool

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"code.google.com/p/goauth2/oauth"
)

var (
	clientId     = ""
	clientSecret = ""
	redirectURL  = "https://alum.hackerschool.com/oauth-redirect"
	authURL      = "https://www.recurse.com/oauth/authorize"
	tokenURL     = "https://www.recurse.com/oauth/token"
	scope        = ""
	apiURL       = "https://www.recurse.com/api/v1/people/me"
)

var config = &oauth.Config{
	ClientId:     clientId,
	ClientSecret: clientSecret,
	RedirectURL:  redirectURL,
	Scope:        scope,
	AuthURL:      authURL,
	TokenURL:     tokenURL,
}

func main() {
	// Set up a Transport using the config.
	transport := &oauth.Transport{Config: config}

	url := config.AuthCodeURL("")
	fmt.Println("Visit this URL to get a code")
	fmt.Println(url)

	bio := bufio.NewReader(os.Stdin)
	code, hasMoreInLine, err := bio.ReadLine()
	if err != nil || hasMoreInLine == true {
		log.Fatal("Failed to read code")
	}

	// Exchange the authorization code for an access token.
	// ("Here's the code you gave the user, now give me a token!")
	token, err := transport.Exchange(string(code))
	if err != nil {
		log.Fatal("Exchange:", err)
	}

	// Make the actual request using the cached token to authenticate.
	// ("Here's the token, let me in!")
	transport.Token = token

	// Make the request.
	r, err := transport.Client().Get(apiURL)
	if err != nil {
		log.Fatal("Get:", err)
	}
	defer r.Body.Close()

	// Write the response to standard output.
	io.Copy(os.Stdout, r.Body)

	// Send final carriage return, just to be neat.
	fmt.Println()
}
