package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

const (
	ADDR_CHARSET  = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-._@+"
	ALIAS_CHARSET = "0123456789abcdefghijklmnopqrstuvwxyz-._"
)

var db *sql.DB

var cookie_secret = make([]byte, 16)

var virtual_mutex = &sync.Mutex{}

var form_template = load_template("form.html")

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

	if len(cookie.Value) <= sha256.Size*2 {
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

func load_template(filename string) *template.Template {
	var file_content, err = ioutil.ReadFile(filename)
	if err != nil {
		log.Panic(err)
	}
	return template.Must(template.New(filename).Parse(string(file_content)))
}

func get_form(w http.ResponseWriter, r *http.Request) {
	user_id := read_cookie(r)
	if user_id == "" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	var alias string
	var addr string

	err := db.QueryRow(`SELECT alias, addr FROM "ALIASES" WHERE user_id = ?`,
		user_id).Scan(&alias, &addr)
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	csrf_token := make([]byte, 16)
	_, err = rand.Read(csrf_token)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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

	if err = form_template.Execute(w, context); err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func post_form(w http.ResponseWriter, r *http.Request) {
	user_id := read_cookie(r)
	if user_id == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	err := r.ParseForm()
	if err != nil {
		log.Println("Could not parse form params")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if cookie.Value != r.PostForm.Get("csrf_token") {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	alias := r.PostForm.Get("alias")
	addr := r.PostForm.Get("addr")

	if !validate_charset(alias, ALIAS_CHARSET) || !validate_charset(addr, ADDR_CHARSET) {
		http.Error(w, "Unallowed characters", http.StatusForbidden)
		return
	}

	if alias == "postmaster" || alias == "webmaster" || alias == "root" ||
		alias == "abuse" || alias == "hackerschool" || alias == "admin" ||
		alias == "mailer-daemon" || alias == "founders" || alias == "faculty" {
		http.Error(w, "Stop it ;)", http.StatusForbidden)
		return
	}

	if len(alias) > 200 {
		http.Error(w, "Stop it ;)", http.StatusForbidden)
		return
	}

	_, err = db.Exec(`DELETE FROM "ALIASES" WHERE user_id = ?`, user_id)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	_, err = db.Exec(`INSERT INTO "ALIASES" (user_id, alias, addr)
					  VALUES (?, ?, ?)`, user_id, alias, addr)
	if err != nil {
		log.Println(err)
		file, err := ioutil.ReadFile("./error.html")
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Write(file)
		return
	}

	// Recreate the postfix file.
	rows, err := db.Query(`SELECT alias, addr FROM "ALIASES"`)
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	virtual_mutex.Lock()
	virtual, err := os.Create("/etc/postfix/virtual")
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for rows.Next() {
		var alias string
		var addr string
		err = rows.Scan(&alias, &addr)
		fmt.Fprintf(virtual, "%s@alum.hackerschool.com %s\n", alias, addr)
		fmt.Fprintf(virtual, "%s@alum.recurse.com %s\n", alias, addr)
	}

	virtual.Close()

	exec.Command("postmap", "/etc/postfix/virtual").Run()
	exec.Command("postfix", "reload").Run()

	virtual_mutex.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			get_form(w, r)
		} else if r.Method == "POST" {
			post_form(w, r)
		} else {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		}
	})
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}
