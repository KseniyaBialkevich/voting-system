package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
)

type Product struct {
	ID      int
	Model   string
	Company string
	Price   int
}

var database *sql.DB

func DeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := database.Exec("DELETE FROM productdb.products WHERE id = ?", id)
	if err != nil {
		log.Println(err)
	}

	http.Redirect(w, r, "/", 301)
}

func EditPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	row := database.QueryRow("SELECT * FROM productdb.products WHERE id = ?", id)

	p := Product{}

	err := row.Scan(&p.ID, &p.Model, &p.Company, &p.Price)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(404), http.StatusNotFound)
	} else {
		tmpl, _ := template.ParseFiles("templates/edit.html")
		tmpl.Execute(w, p)
	}
}

func EditHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}

	id := r.FormValue("id")
	model := r.FormValue("model")
	company := r.FormValue("company")
	price := r.FormValue("price")

	_, err = database.Exec("UPDATE productdb.products set model = ?, company = ?, price = ? WHERE id = ?", model, company, price, id)
	if err != nil {
		fmt.Println(err)
	}

	http.Redirect(w, r, "/", 301)
}

func CreateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}

		model := r.FormValue("model")
		company := r.FormValue("company")
		price := r.FormValue("price")

		_, err = database.Exec("INSERT INTO productdb.products (model, company, price) VALUES (?, ?, ?)", model, company, price)
		if err != nil {
			log.Println(err)
		}

		http.Redirect(w, r, "/", 301)

	} else {
		http.ServeFile(w, r, "templates/create.html")
	}
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {

	rows, err := database.Query("SELECT * FROM productdb.products")
	if err != nil {
		log.Println(err)
	}

	defer rows.Close()

	pProducts := []Product{}

	for rows.Next() {
		p := Product{}

		err := rows.Scan(&p.ID, &p.Model, &p.Company, &p.Price)
		if err != nil {
			fmt.Println(err)

			continue
		}

		pProducts = append(pProducts, p)
	}

	tmpl, _ := template.ParseFiles("templates/index.html")
	tmpl.Execute(w, pProducts)
}

func main() {

	db, err := sql.Open("mysql", "root:11111111@tcp(localhost:3306)/productdb")
	if err != nil {
		log.Println(err)
	}

	database = db

	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/", IndexHandler)
	router.HandleFunc("/create", CreateHandler)
	router.HandleFunc("/edit/{id:[0-9]+}", EditPage).Methods("GET")
	router.HandleFunc("/edit/{id:[0-9]+}", EditHandler).Methods("POST")
	router.HandleFunc("/delete/{id:[0-9]+}", DeleteHandler)
	http.Handle("/", router)

	fmt.Println("Server is listening...")
	http.ListenAndServe(":8181", nil)
}
