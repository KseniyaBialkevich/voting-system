package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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

type User struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Adress  string `json:"adress"`
	Role    string `json: "role"`
}

type Authentication struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
	ID_User  int    `json:"id_user"`
}

var database *sql.DB

var myToken = make(map[string]int)

func cookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		path := r.RequestURI

		if path == "/authentication" {
			next.ServeHTTP(w, r)
		} else {
			cookie, err := r.Cookie("cookie-name")
			if err != nil {
				log.Println(err)
			}

			userID, isExist := myToken[cookie.Value]

			if isExist {
				var userRole string

				row_role := database.QueryRow("SELECT role FROM votingdb.users WHERE id = ?", userID)

				err := row_role.Scan(&userRole)
				if err != nil {
					log.Println(err)
					http.Error(w, http.StatusText(404), http.StatusNotFound)
				}

				fmt.Println("userRole: ", userRole)

				oldContext := r.Context()
				newContext := context.WithValue(oldContext, "role", userRole)

				if path == "/" {
					next.ServeHTTP(w, r.WithContext(newContext))
					return
				}

				if strings.HasPrefix(path, "/admin") && userRole == "admin" {
					next.ServeHTTP(w, r)
				} else {
					log.Println(err)
					http.Error(w, http.StatusText(403), http.StatusForbidden)
				}

				// switch userRole {
				// case "user":
				// 	fmt.Println("User Path: ", path)
				// 	next.ServeHTTP(w, r)
				// case "admin":
				// 	hasPrefix := strings.HasPrefix(path, "/admin")
				// 	if hasPrefix {
				// 		fmt.Println("Admin Path: ", path)
				// 		next.ServeHTTP(w, r)
				// 	}
				// default:
				// 	log.Println(err)
				// 	http.Error(w, http.StatusText(404), http.StatusNotFound)
				//}
				//next.ServeHTTP(w, r)
			} else {
				http.Redirect(w, r, "/authentication", 301)
			}
		}
	})
}

func LogOut(w http.ResponseWriter, r *http.Request) { //TODO
	cookie, _ := r.Cookie("cookie-name")
	delete(myToken, cookie.Value)
	http.Redirect(w, r, "/authentication", 301)
}

func AuthenticationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		login := r.FormValue("login")
		passwordHash := sha256.Sum256([]byte(r.FormValue("password")))

		password := fmt.Sprintf("%x", passwordHash)

		row := database.QueryRow("SELECT * FROM votingdb.authentication WHERE login = ?", login)

		authentication := Authentication{}
		err = row.Scan(&authentication.ID, &authentication.Login, &authentication.Password, &authentication.ID_User)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(404), http.StatusNotFound)
		} else {
			if password == authentication.Password {

				hash := sha256.New()
				hash.Write([]byte("hello\n" + login))
				hashKey := fmt.Sprintf("%x", hash.Sum(nil))

				myToken[hashKey] = authentication.ID_User

				cookie := http.Cookie{
					Name:  "cookie-name",
					Value: hashKey,
					// Path:     "/",
					// Expires:  time.Now().Add(3 * 24 * time.Hour),
					// Secure:   true,
					// HttpOnly: true,
				}

				http.SetCookie(w, &cookie)

				http.SetCookie(w, &cookie)

				http.Redirect(w, r, "/", 301)
			} else {
				http.Error(w, "login or password entered incorrectly", 400)
				return
			}
		}

	} else {
		http.ServeFile(w, r, "templates/authentication.html")
	}
}

// func IndexClientHandler(w http.ResponseWriter, r *http.Request) {
// 	rows, err := database.Query("SELECT * FROM votingdb.votings")
// 	if err != nil {
// 		log.Println(err)
// 	}

// 	defer rows.Close()

// 	votings := []Voting{}

// 	for rows.Next() {
// 		voting := Voting{}

// 		err := rows.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
// 		if err != nil {
// 			log.Println(err)

// 			continue
// 		}

// 		votings = append(votings, voting)
// 	}

// 	tmpl, err := template.ParseFiles("templates/index_client.html")
// 	if err != nil {
// 		log.Println(err)
// 	}

// 	tmpl.Execute(w, votings)
// }

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	type AllVotings struct {
		IsExistRole bool
		Votings     []Voting
	}

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

	context_role := r.Context().Value("role")

	role := fmt.Sprintf("%v", context_role)

	fmt.Println("context_role: ", role)

	var isExistRole bool

	if role == "user" {
		isExistRole = false
	} else if role == "admin" {
		isExistRole = true
	}

	allVotings := AllVotings{
		IsExistRole: isExistRole,
		Votings:     votings,
	}

	fmt.Println("IsExistRole: ", allVotings.IsExistRole)

	fmt.Printf("%#v \n", allVotings)

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		log.Println(err)
	}

	tmpl.Execute(w, allVotings)
	//tmpl.Execute(w, votings)
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

		http.Redirect(w, r, "/admin/voting_qa/{id_voting:[0-9]+}", 301)

	} else {
		http.ServeFile(w, r, "templates/admin_create_voting.html")
	}
}

func AdminVotingQAHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id_voting := vars["id_voting"]

	type QuAns struct {
		Question Question `json:"question"`
		Answers  []Answer `json:"answers"`
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

		answers := []Answer{}

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
			answers = append(answers, answer)
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

	tmpl, _ := template.ParseFiles("templates/admin_voting_qa.html")
	tmpl.Execute(w, votingQA)
}

func VotingQAHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id_voting := vars["id_voting"]

	type QuAns struct {
		Question Question `json:"question"`
		Answers  []Answer `json:"answers"`
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

		answers := []Answer{}

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
			answers = append(answers, answer)
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

func OpenQAHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id_question := vars["id_question"]

	type QAs struct {
		Question Question `json:"question"`
		Answers  []Answer `json:"answers"`
	}

	row := database.QueryRow("SELECT * FROM votingdb.questions WHERE id = ?", id_question)

	question := Question{}

	err := row.Scan(&question.ID, &question.Name, &question.ID_Voting)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(404), http.StatusNotFound)
	}

	rows, err := database.Query("SELECT * FROM votingdb.answers WHERE id_question = ?", id_question)
	if err != nil {
		log.Println(err)
	}

	defer rows.Close()

	answers := []Answer{}

	for rows.Next() {
		answer := Answer{}

		err := rows.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
		if err != nil {
			log.Println(err)
		}

		answers = append(answers, answer)
	}

	qas := QAs{
		Question: question,
		Answers:  answers,
	}

	tmpl, _ := template.ParseFiles("templates/admin_open_qa.html")
	tmpl.Execute(w, qas)
}

func CreateQuestionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {

		vars := mux.Vars(r)
		id_voting := vars["id_voting"]

		err := r.ParseForm()
		if err != nil {
			fmt.Println(err)
		}

		name := r.FormValue("name")

		_, err = database.Exec("INSERT INTO votingdb.questions (name, id_voting) VALUES (?, ?)", name, id_voting)
		if err != nil {
			fmt.Println(err)
		}

		http.Redirect(w, r, "/admin/voting_qa/"+id_voting, 301)

	} else {
		http.ServeFile(w, r, "templates/admin_create_question.html")
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

		name := r.FormValue("name")

		_, err = database.Exec("INSERT INTO votingdb.answers (name, id_question) VALUES (?, ?)", name, id_question)
		if err != nil {
			fmt.Println(err)
		}

		http.Redirect(w, r, "/admin/voting_qa/"+id_voting, 301)

	} else {
		http.ServeFile(w, r, "templates/admin_create_answer.html")
	}
}

func EditVotingHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {

		vars := mux.Vars(r)
		id_voting := vars["id_voting"]

		row := database.QueryRow("SELECT * FROM votingdb.votings WHERE id = ?", id_voting)

		voting := Voting{}

		err := row.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(404), http.StatusNotFound)
		} else {
			tmpl, _ := template.ParseFiles("templates/admin_edit_voting.html")
			tmpl.Execute(w, voting)
		}
	}

	if r.Method == "POST" {

		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}

		id_voting := r.FormValue("id_voting")
		name := r.FormValue("name")
		description := r.FormValue("description")
		startTime := r.FormValue("start_time")
		endTime := r.FormValue("end_time")

		_, err = database.Exec("UPDATE votingdb.votings set name = ?, description = ?, start_time = ?, end_time = ? WHERE id = ?", name, description, startTime, endTime, id_voting)
		if err != nil {
			log.Println(err)
		}

		http.Redirect(w, r, "/admin/voting_qa/"+id_voting, 301)
	}
}

func EditQuestionHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {

		vars := mux.Vars(r)
		id_question := vars["id_question"]

		row := database.QueryRow("SELECT * FROM votingdb.questions WHERE id = ?", id_question)

		question := Question{}

		err := row.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(404), http.StatusNotFound)
		} else {
			tmpl, _ := template.ParseFiles("templates/admin_edit_question.html")
			tmpl.Execute(w, question)
		}
	}

	if r.Method == "POST" {

		vars := mux.Vars(r)
		id_voting := vars["id_voting"]

		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}

		id_question := r.FormValue("id_question")
		name := r.FormValue("name")

		_, err = database.Exec("UPDATE votingdb.questions set name = ? WHERE id = ?", name, id_question)
		if err != nil {
			log.Println(err)
		}
		http.Redirect(w, r, "/admin/open_qa/"+id_voting+"/"+id_question, 301)
	}
}

func EditAnswerHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {

		vars := mux.Vars(r)
		id_answer := vars["id_answer"]

		row := database.QueryRow("SELECT * FROM votingdb.answers WHERE id = ?", id_answer)

		answer := Answer{}

		err := row.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(404), http.StatusNotFound)
		} else {
			tmpl, _ := template.ParseFiles("templates/admin_edit_answer.html")
			tmpl.Execute(w, answer)
		}
	}

	if r.Method == "POST" {

		vars := mux.Vars(r)
		id_question := vars["id_question"]

		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}

		id_answer := r.FormValue("id_answer")
		name := r.FormValue("name")

		_, err = database.Exec("UPDATE votingdb.answers set name = ? WHERE id = ?", name, id_answer)
		if err != nil {
			log.Println(err)
		}

		row := database.QueryRow("SELECT * FROM votingdb.questions WHERE id = ?", id_question)

		question := Question{}

		err = row.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(404), http.StatusNotFound)
		} else {
			id_voting := strconv.Itoa(question.ID_Voting)
			http.Redirect(w, r, "/admin/open_qa/"+id_voting+"/"+id_question, 301)
		}
	}
}

func DeleteVotingHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting := vars["id_voting"]

	DeleteQuestion(id_voting)

	_, err := database.Exec("DELETE FROM votingdb.votings WHERE id = ?", id_voting)
	if err != nil {
		log.Println(err)
	}

	http.Redirect(w, r, "/", 301)
}

func DeleteQuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_question := vars["id_question"]

	var idVoting int
	row_voting := database.QueryRow("SELECT id_voting FROM votingdb.questions WHERE id = ?", id_question)
	err := row_voting.Scan(&idVoting)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(404), http.StatusNotFound)
	}

	DeleteAnswer(id_question)

	_, err = database.Exec("DELETE FROM votingdb.questions WHERE id = ?", id_question)
	if err != nil {
		log.Println(err)
	}

	id_voting := strconv.Itoa(idVoting)

	http.Redirect(w, r, "/admin/voting_qa/"+id_voting, 301)
}

func DeleteAnswerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_answer := vars["id_answer"]

	var idQuestion int
	row_queestion := database.QueryRow("SELECT id_question FROM votingdb.answers WHERE id = ?", id_answer)
	err := row_queestion.Scan(&idQuestion)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(404), http.StatusNotFound)
	}

	var idVoting int
	row_answer := database.QueryRow("SELECT id_voting FROM votingdb.questions WHERE id = ?", idQuestion)
	err = row_answer.Scan(&idVoting)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(404), http.StatusNotFound)
	}

	_, err = database.Exec("DELETE FROM votingdb.answers WHERE id = ?", id_answer)
	if err != nil {
		log.Println(err)
	}

	id_voting := strconv.Itoa(idVoting)
	id_question := strconv.Itoa(idQuestion)

	http.Redirect(w, r, "/admin/open_qa/"+id_voting+"/"+id_question, 301)
}

func DeleteQuestion(id_voting string) {
	rowsQuestions, err := database.Query("SELECT * FROM votingdb.questions WHERE id_voting = ?", id_voting)
	if err != nil {
		log.Println(err)
	}

	defer rowsQuestions.Close()

	for rowsQuestions.Next() {

		question := Question{}

		err := rowsQuestions.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			log.Println(err)

			continue
		}

		id_question := strconv.Itoa(question.ID)

		DeleteAnswer(id_question)

		_, err = database.Exec("DELETE FROM votingdb.questions WHERE id = ?", id_question)
		if err != nil {
			log.Println(err)
		}
	}
}

func DeleteAnswer(id_question string) {
	rowsAnswers, err := database.Query("SELECT * FROM votingdb.answers WHERE id_question = ?", id_question)
	if err != nil {
		log.Println(err)
	}

	defer rowsAnswers.Close()

	for rowsAnswers.Next() {

		answer := Answer{}

		err := rowsAnswers.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
		if err != nil {
			log.Println(err)

			continue
		}

		id_answer := answer.ID

		_, err = database.Exec("DELETE FROM votingdb.answers WHERE id = ?", id_answer)
		if err != nil {
			log.Println(err)
		}
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
	router.HandleFunc("/authentication", AuthenticationHandler)
	router.HandleFunc("/logout", LogOut)

	router.HandleFunc("/", IndexHandler)
	router.HandleFunc("/admin/create_voting", CreateVotingHandler)
	router.HandleFunc("/admin/voting_qa/{id_voting:[0-9]+}", AdminVotingQAHandler)
	router.HandleFunc("/voting_qa/{id_voting:[0-9]+}", VotingQAHandler)
	router.HandleFunc("/admin/open_qa/{id_voting:[0-9]+}/{id_question:[0-9]+}", OpenQAHandler)
	router.HandleFunc("/admin/create_question/{id_voting:[0-9]+}", CreateQuestionHandler)
	router.HandleFunc("/admin/create_answer/{id_voting:[0-9]+}/{id_question:[0-9]+}", CreateAnswerHandler)
	router.HandleFunc("/admin/edit_voting/{id_voting:[0-9]+}", EditVotingHandler)
	router.HandleFunc("/admin/edit_question/{id_voting:[0-9]+}/{id_question:[0-9]+}", EditQuestionHandler)
	router.HandleFunc("/admin/edit_answer/{id_question:[0-9]+}/{id_answer:[0-9]+}", EditAnswerHandler)
	router.HandleFunc("/admin/delete_voting/{id_voting:[0-9]+}", DeleteVotingHandler)
	router.HandleFunc("/admin/delete_question/{id_question:[0-9]+}", DeleteQuestionHandler)
	router.HandleFunc("/admin/delete_answer/{id_answer:[0-9]+}", DeleteAnswerHandler)
	http.Handle("/", router)

	router.Use(cookieMiddleware)

	fmt.Println("Server is listening...")

	err = http.ListenAndServe(":8080", router)
	if err != nil {
		log.Println("HTTP Server Error - ", err)
	}
}
