package main

import (
	"database/sql"
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

func get_form(c web.C, w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("form.html")
	context := &TemplateContext{
		Alias: "$ALIAS",
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
	load_oauth()

	var err error
	db, err = sql.Open("sqlite3", "./alum.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS "ALIASES" (
        "user_id" INTEGER PRIMARY KEY NOT NULL,
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
