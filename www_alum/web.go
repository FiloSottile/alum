package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"

	_ "github.com/mattn/go-sqlite3"
)

const CHARSET = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-._@+"

var db *sql.DB

var cookie_secret = make([]byte, 16)

func validate_charset(s, charset string) bool {
	for _, c := range s {
		if strings.Index(charset, string(c)) == -1 {
			return false
		}
	}
	return true
}

func set_cookie(user_id string, w http.ResponseWriter) {
	mac := hmac.New(sha256.New, cookie_secret)
	mac.Write([]byte(user_id))

	cookie := &http.Cookie{
		Name:     "user_id",
		Value:    hex.EncodeToString(mac.Sum(nil)) + user_id,
		MaxAge:   60 * 5,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
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

	if user_id == "" {
		http.Redirect(w, r, "/login", 302)
		return
	}

	var alias string
	var addr string

	err := db.QueryRow(`SELECT alias, addr FROM "ALIASES" WHERE user_id = ?`,
		user_id).Scan(&alias, &addr)
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	csrf_token := make([]byte, 16)
	_, err = rand.Read(csrf_token)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	cookie := &http.Cookie{
		Name:     "csrf_token",
		Value:    hex.EncodeToString(csrf_token),
		MaxAge:   60,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	}
	http.SetCookie(w, cookie)

	t, _ := template.ParseFiles("form.html")

	type TemplateContext struct {
		Alias      string
		Addr       string
		Csrf_token string
	}
	context := &TemplateContext{
		Alias:      alias,
		Addr:       addr,
		Csrf_token: hex.EncodeToString(csrf_token),
	}

	err = t.Execute(w, context)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func post_form(c web.C, w http.ResponseWriter, r *http.Request) {
	user_id := read_cookie(r)

	err := r.ParseForm()
	if err != nil {
		log.Println("Could not parse form params")
		http.Error(w, http.StatusText(400), 400)
		return
	}

	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(403), 403)
		return
	}
	if cookie.Value != r.PostForm.Get("csrf_token") {
		http.Error(w, http.StatusText(403), 403)
		return
	}

	alias := r.PostForm.Get("alias")
	addr := r.PostForm.Get("addr")

	if !validate_charset(alias, CHARSET[:len(CHARSET)-2]) || !validate_charset(addr, CHARSET) {
		http.Error(w, "Unallowed characters", 403)
		return
	}

	if alias == "postmaster" || alias == "webmaster" || alias == "root" ||
		alias == "abuse" || alias == "hackerschool" {
		http.Error(w, "Stop it ;)", 403)
		return
	}

	_, err = db.Exec(`DELETE FROM "ALIASES" WHERE user_id = ?`, user_id)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	_, err = db.Exec(`INSERT INTO "ALIASES" (user_id, alias, addr)
					  VALUES (?, ?, ?)`, user_id, alias, addr)
	if err != nil {
		log.Println(err)
		file, err := ioutil.ReadFile("./error.html")
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(500), 500)
			return
		}
		w.Write(file)
		return
	}

	// Recreate the postfix file.
	rows, err := db.Query(`SELECT alias, addr FROM "ALIASES"`)
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	virtual, err := os.Create("/etc/postfix/virtual")
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	for rows.Next() {
		var alias string
		var addr string
		err = rows.Scan(&alias, &addr)
		fmt.Fprintf(virtual, "%s@alum.hackerschool.com %s\n", alias, addr)
	}

	virtual.Close()

	exec.Command("postmap", "/etc/postfix/virtual").Run()
	exec.Command("postfix", "reload").Run()

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
