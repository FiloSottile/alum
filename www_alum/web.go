package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"text/template"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"

	_ "github.com/mattn/go-sqlite3"
)

type TemplateContext struct {
	Alias string
	Addr  string
}

var db *sql.DB

var cookie_secret = make([]byte, 16)

func set_cookie(user_id string, w http.ResponseWriter) {
	mac := hmac.New(sha256.New, cookie_secret)
	mac.Write([]byte(user_id))

	cookie := &http.Cookie{
		Name:     "user_id",
		Value:    hex.EncodeToString(mac.Sum(nil)) + user_id,
		MaxAge:   60 * 5,
		Path:     "/",
		HttpOnly: true,
		// Secure:   true,
	}

	http.SetCookie(w, cookie)
}

func read_cookie(r *http.Request) string {
	cookie, err := r.Cookie("user_id")
	if err != nil {
		return ""
	}

	mac := hmac.New(sha256.New, cookie_secret)
	mac.Write([]byte(cookie.Value[sha256.Size*2:]))

	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))),
		[]byte(cookie.Value[:sha256.Size*2])) {
		return ""
	}

	return cookie.Value[sha256.Size*2:]
}

func get_form(c web.C, w http.ResponseWriter, r *http.Request) {
	user_id := read_cookie(r)

	t, _ := template.ParseFiles("form.html")
	context := &TemplateContext{
		Alias: user_id,
		Addr:  "$ADDR",
	}
	t.Execute(w, context)
}

func post_form(c web.C, w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("Could not parse form params")
		http.Error(w, http.StatusText(400), 400)
		return
	}

	log.Println(r.PostForm.Get("alias"))
	log.Println(r.PostForm.Get("addr"))
	http.Redirect(w, r, "/", 303)
}

func main() {
	_, err := rand.Read(cookie_secret)
	if err != nil {
		log.Fatal(err)
	}

	load_oauth()

	db, err = sql.Open("sqlite3", "./alum.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS "ALIASES" (
        "user_id" TEXT PRIMARY KEY NOT NULL,
        "alias" TEXT UNIQUE NOT NULL,
        "addr" TEXT NOT NULL
        );`)
	if err != nil {
		log.Fatal(err)
	}

	goji.Get("/", get_form)
	goji.Post("/", post_form)
	goji.Serve()
}