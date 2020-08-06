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

type Voting struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
}

type Question struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ID_Voting int    `json:"id_voting"`
}

type Answer struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ID_Question int    `json:"id_question"`
}

var database *sql.DB

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT * FROM votingdb.votings")
	if err != nil {
		log.Println(err)
	}

	defer rows.Close()

	votings := []Voting{}

	for rows.Next() {
		voting := Voting{}

		err := rows.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
		if err != nil {
			log.Println(err)

			continue
		}

		votings = append(votings, voting)
	}

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Println(err)
	}

	tmpl.Execute(w, votings)
}

func CreateVotingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {

		err := r.ParseForm()
		if err != nil {
			fmt.Println(err)
		}

		name := r.FormValue("name")
		description := r.FormValue("description")
		startTime := r.FormValue("start_time")
		endTime := r.FormValue("end_time")

		_, err = database.Exec("INSERT INTO votingdb.votings (name, description, start_time, end_time) VALUES(?, ?, ?, ?)", name, description, startTime, endTime)
		if err != nil {
			log.Println(err)
		}

		http.Redirect(w, r, "/voting_qa/{id_voting:[0-9]+}", 301)

	} else {
		http.ServeFile(w, r, "templates/create_voting.html")
	}
}

func VotingQAHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id_voting := vars["id_voting"]

	type QuAns struct {
		Question Question `json:"question"`
		Answers  []string `json:"answers"`
	}

	type VotingQA struct {
		Voting Voting  `json:"voting"`
		QAs    []QuAns `json:"qas"`
	}

	votingRow := database.QueryRow("SELECT * FROM votingdb.votings WHERE id = ?", id_voting)

	voting := Voting{}

	err := votingRow.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(404), http.StatusNotFound)

	}

	resultQA := []QuAns{}

	questiosRows, err := database.Query("SELECT * FROM votingdb.questions WHERE id_voting = ?", id_voting)
	if err != nil {
		log.Println(err)
	}

	defer questiosRows.Close()

	for questiosRows.Next() {
		question := Question{}
		err := questiosRows.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			log.Println(err)
			continue
		}

		answers := make([]string, 0)

		answersRows, err := database.Query("SELECT * FROM votingdb.answers WHERE id_question = ?", question.ID)
		if err != nil {
			log.Println(err)
		}

		defer answersRows.Close()

		for answersRows.Next() {
			answer := Answer{}
			err := answersRows.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
			if err != nil {
				fmt.Println(err)
				continue
			}
			answers = append(answers, answer.Name)
		}

		qu_ans := QuAns{
			Question: question,
			Answers:  answers,
		}

		resultQA = append(resultQA, qu_ans)
	}

	votingQA := VotingQA{
		Voting: voting,
		QAs:    resultQA,
	}

	tmpl, _ := template.ParseFiles("templates/voting_qa.html")
	tmpl.Execute(w, votingQA)
}

func CreateQuestionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {

		vars := mux.Vars(r)
		id_voting := vars["id_voting"]

		err := r.ParseForm()
		if err != nil {
			fmt.Println(err)
		}

		question_name := r.FormValue("question")

		_, err = database.Exec("INSERT INTO votingdb.questions (name, id_voting) VALUES (?, ?)", question_name, id_voting)
		if err != nil {
			fmt.Println(err)
		}

		http.Redirect(w, r, "/voting_qa/"+id_voting, 301)

	} else {
		http.ServeFile(w, r, "templates/create_question.html")
	}
}

func CreateAnswerHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {

		vars := mux.Vars(r)
		id_voting := vars["id_voting"]
		id_question := vars["id_question"]

		err := r.ParseForm()
		if err != nil {
			fmt.Println(err)
		}

		answer_name := r.FormValue("answer")

		_, err = database.Exec("INSERT INTO votingdb.answers (name, id_question) VALUES (?, ?)", answer_name, id_question)
		if err != nil {
			fmt.Println(err)
		}

		http.Redirect(w, r, "/voting_qa/"+id_voting, 301)

	} else {
		http.ServeFile(w, r, "templates/create_answer.html")
	}
}

func main() {

	db, err := sql.Open("mysql", "root:11111111@tcp(localhost:3306)/votingdb")
	if err != nil {
		log.Println(err)
	}

	database = db

	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/", IndexHandler)
	router.HandleFunc("/create_voting", CreateVotingHandler)
	router.HandleFunc("/voting_qa/{id_voting:[0-9]+}", VotingQAHandler)
	router.HandleFunc("/create_question/{id_voting:[0-9]+}", CreateQuestionHandler)
	router.HandleFunc("/create_answer/{id_voting:[0-9]+}/{id_question:[0-9]+}", CreateAnswerHandler)

	http.Handle("/", router)

	fmt.Println("Server is listening...")
	http.ListenAndServe(":8080", nil)
}
