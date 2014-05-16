package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"code.google.com/p/goauth2/oauth"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
	"gopkg.in/yaml.v1"
)

const (
	redirectURL = "https://alum.hackerschool.com/oauth-redirect"
	authURL     = "https://www.hackerschool.com/oauth/authorize"
	tokenURL    = "https://www.hackerschool.com/oauth/token"
	scope       = ""
	apiURL      = "https://www.hackerschool.com/api/v1/people/me"
)

var oauth_config *oauth.Config

func login(c web.C, w http.ResponseWriter, r *http.Request) {
	url := oauth_config.AuthCodeURL("")
	http.Redirect(w, r, url, 303)
}

func callback(c web.C, w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		log.Println("Code not received")
		http.Error(w, http.StatusText(400), 400)
		return
	}

	transport := &oauth.Transport{Config: oauth_config}

	token, err := transport.Exchange(code)
	if err != nil || token == nil {
		log.Println("Exchange failed", err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	transport.Token = token

	res, err := transport.Client().Get(apiURL)
	if err != nil {
		log.Println("API call failed")
		http.Error(w, http.StatusText(500), 500)
		return
	}
	defer res.Body.Close()

	json_me, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("API read failed")
		http.Error(w, http.StatusText(500), 500)
		return
	}

	type Me struct{ Id int }
	me := &Me{}
	err = json.Unmarshal(json_me, me)
	if err != nil {
		log.Println("Malformed API response")
		http.Error(w, http.StatusText(500), 500)
		return
	}

	set_cookie(strconv.Itoa(me.Id), w)

	http.Redirect(w, r, "/", 303)
}

func load_oauth() {
	file, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		log.Fatal(err)
	}

	type Config struct {
		OAuth_clientId     string `yaml:"OAuth_clientId"`
		OAuth_clientSecret string `yaml:"OAuth_clientSecret"`
	}

	config := &Config{}
	err = yaml.Unmarshal(file, config)
	if err != nil {
		log.Fatal(err)
	}

	oauth_config = &oauth.Config{
		ClientId:     config.OAuth_clientId,
		ClientSecret: config.OAuth_clientSecret,
		RedirectURL:  redirectURL,
		Scope:        scope,
		AuthURL:      authURL,
		TokenURL:     tokenURL,
	}

	goji.Get("/login", login)
	goji.Get("/oauth-redirect", callback)
}
