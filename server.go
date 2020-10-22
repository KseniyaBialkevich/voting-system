package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
)

type User struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Adress  string `json:"adress"`
	Role    string `json:"role"`
}

type Authentication struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
	ID_User  int    `json:"id_user"`
}

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

type VotingResult struct {
	ID          int `json:"id"`
	ID_Voting   int `json:"id_voting"`
	ID_Question int `json:"id_question"`
	ID_Answer   int `json:"id_answer"`
	ID_User     int `json:"id_user"`
}

var database *sql.DB

var myToken = make(map[string]int)

func serverError(w http.ResponseWriter, err error, statusCode int) {
	stackTrace := string(debug.Stack())
	msg := fmt.Sprintf("Error: %s\n%s", err, stackTrace)
	log.Println(msg)

	http.Error(w, err.Error(), statusCode)
}

func convertInterface(event interface{}) *User {
	u := User{}
	mapstructure.Decode(event, &u)
	return &u
}

func deleteQuestions(w http.ResponseWriter, id_voting string) {
	rowsQuestions, err := database.Query("SELECT * FROM votingdb.questions WHERE id_voting = ?", id_voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	defer rowsQuestions.Close()

	for rowsQuestions.Next() {
		question := Question{}

		err := rowsQuestions.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		id_question := strconv.Itoa(question.ID)

		deleteAnswers(w, id_question)

		_, err = database.Exec("DELETE FROM votingdb.questions WHERE id = ?", id_question)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}
	}

	err = rowsQuestions.Err()
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}
}

func deleteAnswers(w http.ResponseWriter, id_question string) {
	rowsAnswers, err := database.Query("SELECT * FROM votingdb.answers WHERE id_question = ?", id_question)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	defer rowsAnswers.Close()

	for rowsAnswers.Next() {
		answer := Answer{}

		err := rowsAnswers.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		id_answer := answer.ID

		_, err = database.Exec("DELETE FROM votingdb.answers WHERE id = ?", id_answer)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}
	}

	err = rowsAnswers.Err()
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}
}

func cookieMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		path := r.RequestURI

		if path == "/authentication" {

			next.ServeHTTP(w, r)
		} else {

			cookie, err := r.Cookie("cookie-name")
			if err != nil {
				log.Println(err)
				http.Redirect(w, r, "/authentication", 302)
				return
			}

			userID, isExist := myToken[cookie.Value]

			if isExist {
				user := User{}

				row_user := database.QueryRow("SELECT * FROM votingdb.users WHERE id = ?", userID)

				err := row_user.Scan(&user.ID, &user.Name, &user.Surname, &user.Adress, &user.Role)
				if err != nil {
					serverError(w, err, http.StatusNotFound)
					return
				}

				oldContext := r.Context()
				newContext := context.WithValue(oldContext, "user", user)

				if strings.HasPrefix(path, "/admin") && user.Role == "admin" {
					next.ServeHTTP(w, r)
				} else if !strings.HasPrefix(path, "/admin") {
					next.ServeHTTP(w, r.WithContext(newContext))
				} else {
					serverError(w, err, http.StatusForbidden)
					return
				}

			} else {
				http.Redirect(w, r, "/authentication", 302)
			}
		}
	})
}

func LogOut(w http.ResponseWriter, r *http.Request) { //TODO
	cookie, _ := r.Cookie("cookie-name")
	delete(myToken, cookie.Value)
	http.Redirect(w, r, "/authentication", 302)
}

func AuthenticationTemplate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/authentication.html")
}

func AuthenticationHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	login := r.FormValue("login")
	passwordHash := sha256.Sum256([]byte(r.FormValue("password")))

	password := fmt.Sprintf("%x", passwordHash)

	row := database.QueryRow("SELECT * FROM votingdb.authentication WHERE login = ?", login)

	authentication := Authentication{}
	err = row.Scan(&authentication.ID, &authentication.Login, &authentication.Password, &authentication.ID_User)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	} else {
		if password == authentication.Password {
			hash := sha256.New()
			hash.Write([]byte("hello\n" + login))
			hashKey := fmt.Sprintf("%x", hash.Sum(nil))

			myToken[hashKey] = authentication.ID_User

			cookie := http.Cookie{
				Name:  "cookie-name",
				Value: hashKey,
				// Path:  "*",
				// Expires:  time.Now().Add(3 * 24 * time.Hour),
				// Secure:   true,
				// HttpOnly: true,
			}

			http.SetCookie(w, &cookie)

			http.Redirect(w, r, "/", 302)
		} else {
			err := fmt.Errorf("login or password entered incorrectly")
			serverError(w, err, http.StatusUnauthorized)
			return
		}
	}
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	type AllVotings struct {
		IsExistRole bool
		Votings     []Voting
	}

	rows, err := database.Query("SELECT * FROM votingdb.votings")
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	defer rows.Close()

	votings := []Voting{}

	for rows.Next() {
		voting := Voting{}

		err := rows.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		err = rows.Err()
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}

		votings = append(votings, voting)
	}

	context_user := r.Context().Value("user")

	user := convertInterface(context_user)

	var isExistRole bool

	if user.Role == "user" {
		isExistRole = false
	} else if user.Role == "admin" {
		isExistRole = true
	}

	allVotings := AllVotings{
		IsExistRole: isExistRole,
		Votings:     votings,
	}

	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, allVotings)
}

func CreateVotingTemplate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin_create_voting.html")
}

func CreateVotingHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	startTime := r.FormValue("start_time")
	endTime := r.FormValue("end_time")

	result, err := database.Exec("INSERT INTO votingdb.votings (name, description, start_time, end_time) VALUES(?, ?, ?, ?)", name, description, startTime, endTime)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	id_voting, err := result.LastInsertId()
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/votings/%d/questions/answers", id_voting), 302)
}

func VotingQAAdminHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

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
		serverError(w, err, http.StatusNotFound)
		return
	}

	resultQA := []QuAns{}

	questiosRows, err := database.Query("SELECT * FROM votingdb.questions WHERE id_voting = ?", id_voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	defer questiosRows.Close()

	for questiosRows.Next() {
		question := Question{}
		err := questiosRows.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		err = questiosRows.Err()
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}

		answers := []Answer{}

		answersRows, err := database.Query("SELECT * FROM votingdb.answers WHERE id_question = ?", question.ID)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		defer answersRows.Close()

		for answersRows.Next() {
			answer := Answer{}
			err := answersRows.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
			if err != nil {
				serverError(w, err, http.StatusNotFound)
				return
			}
			answers = append(answers, answer)
		}

		err = questiosRows.Err()
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
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

func VotingQATemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	type QuAns struct {
		Question Question `json:"question"`
		Answers  []Answer `json:"answers"`
	}

	type VotingQA struct {
		IsExistRole bool
		Voting      Voting  `json:"voting"`
		QAs         []QuAns `json:"qas"`
	}

	votingRow := database.QueryRow("SELECT * FROM votingdb.votings WHERE id = ?", id_voting)

	voting := Voting{}

	err := votingRow.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	resultQA := []QuAns{}

	questiosRows, err := database.Query("SELECT * FROM votingdb.questions WHERE id_voting = ?", id_voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	defer questiosRows.Close()

	for questiosRows.Next() {
		question := Question{}
		err := questiosRows.Scan(&question.ID, &question.Name, &question.ID_Voting)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		answers := []Answer{}

		answersRows, err := database.Query("SELECT * FROM votingdb.answers WHERE id_question = ?", question.ID)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}

		defer answersRows.Close()

		for answersRows.Next() {
			answer := Answer{}
			err := answersRows.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
			if err != nil {
				serverError(w, err, http.StatusNotFound)
				return
			}

			answers = append(answers, answer)
		}

		err = answersRows.Err()
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}

		qu_ans := QuAns{
			Question: question,
			Answers:  answers,
		}

		resultQA = append(resultQA, qu_ans)
	}

	err = questiosRows.Err()
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	context_user := r.Context().Value("user")

	user := convertInterface(context_user)

	var isExistRole bool

	if user.Role == "user" {
		isExistRole = false
	} else if user.Role == "admin" {
		isExistRole = true
	}

	votingQA := VotingQA{
		IsExistRole: isExistRole,
		Voting:      voting,
		QAs:         resultQA,
	}

	tmpl, _ := template.ParseFiles("templates/voting_qa.html")
	tmpl.Execute(w, votingQA)
}

func VotingQAHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_votingStr, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	id_voting, _ := strconv.Atoi(id_votingStr)

	context_user := r.Context().Value("user")
	user := convertInterface(context_user)

	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	result := make([][]int, 0)

	for key, values := range r.Form {
		for _, value := range values {
			id_question, _ := strconv.Atoi(key)
			id_answer, _ := strconv.Atoi(value)
			result = append(result, []int{id_voting, id_question, id_answer, user.ID})
		}
	}

	for _, value := range result {
		id_voting := value[0]
		id_question := value[1]
		id_answer := value[2]
		id_user := value[3]
		_, err := database.Exec(
			"INSERT INTO votingdb.voting_results (id_voting, id_question, id_answer, id_user) VALUES(?, ?, ?, ?)",
			id_voting, id_question, id_answer, id_user)
		if err != nil {
			serverError(w, err, http.StatusNotFound)
			return
		}
	}

	http.Redirect(w, r, "/", 302)
}

func ProgressHandler(w http.ResponseWriter, r *http.Request) { //TODO
	// vars := mux.Vars(r)
	// id_voting := vars["id_voting"]

	// type Result struct {
	// 	VotingName string
	// 	//Progress
	// }

	// var voting_name string

	// row := database.QueryRow("SELECT name FROM votingdb.votigs WHERE id = ?", id_voting)

	// err := row.Scan(&voting_name)
	// if err != nil {
	// 	log.Println(err)
	// 	http.Error(http.StatusText(404), http.StatusNotFound)
	// }

	// rows, err := database.QueryRow(
	// // "USE voingdb;
	// // SELECT vr.id_question, vr.COUNT(id_answer), vr.COUNT(id_user)
	// // FROM voting_results AS vr
	// // JOIN votings AS v
	// // ON vr.id_voting = v.id
	// // GROUP BY
	// // WHERE id_voting = ?", id_voting
	// )

	//TODO
}

func OpenQAHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_question, ok := vars["id_question"]
	if !ok {
		err := fmt.Errorf("question id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	type QAs struct {
		Question Question `json:"question"`
		Answers  []Answer `json:"answers"`
	}

	row := database.QueryRow("SELECT * FROM votingdb.questions WHERE id = ?", id_question)

	question := Question{}

	err := row.Scan(&question.ID, &question.Name, &question.ID_Voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
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
			serverError(w, err, http.StatusNotFound)
			return
		}

		answers = append(answers, answer)
	}

	err = rows.Err()
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	qas := QAs{
		Question: question,
		Answers:  answers,
	}

	tmpl, _ := template.ParseFiles("templates/admin_open_qa.html")
	tmpl.Execute(w, qas)
}

func CreateQuestionTemplate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin_create_question.html")
}

func CreateQuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	question_name := r.FormValue("name")

	_, err = database.Exec("INSERT INTO votingdb.questions (name, id_voting) VALUES (?, ?)", question_name, id_voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "/admin/votings/"+id_voting+"/questions/answers", 302)

}

func CreateAnswerTemplate(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/admin_create_answer.html")
}

func CreateAnswerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	id_question, ok := vars["id_question"]
	if !ok {
		err := fmt.Errorf("question id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	answer_name := r.FormValue("name")

	_, err = database.Exec("INSERT INTO votingdb.answers (name, id_question) VALUES (?, ?)", answer_name, id_question)
	if err != nil {
		fmt.Println(err)
	}

	http.Redirect(w, r, "/admin/votings/"+id_voting+"/questions/answers", 302)
}

func EditVotingTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	row := database.QueryRow("SELECT * FROM votingdb.votings WHERE id = ?", id_voting)

	voting := Voting{}

	err := row.Scan(&voting.ID, &voting.Name, &voting.Description, &voting.StartTime, &voting.EndTime)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	} else {
		tmpl, _ := template.ParseFiles("templates/admin_edit_voting.html")
		tmpl.Execute(w, voting)
	}
}

func EditVotingHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	id_voting := r.FormValue("id_voting")
	name := r.FormValue("name")
	description := r.FormValue("description")
	startTime := r.FormValue("start_time")
	endTime := r.FormValue("end_time")

	_, err = database.Exec(
		"UPDATE votingdb.votings set name = ?, description = ?, start_time = ?, end_time = ? WHERE id = ?",
		name, description, startTime, endTime, id_voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "/admin/votings/"+id_voting+"/questions/answers", 302)
}

func EditQuestionTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_question, ok := vars["id_question"]
	if !ok {
		err := fmt.Errorf("question id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	row := database.QueryRow("SELECT * FROM votingdb.questions WHERE id = ?", id_question)

	question := Question{}

	err := row.Scan(&question.ID, &question.Name, &question.ID_Voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	} else {
		tmpl, _ := template.ParseFiles("templates/admin_edit_question.html")
		tmpl.Execute(w, question)
	}
}

func EditQuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	id_question := r.FormValue("id_question")
	name := r.FormValue("name")

	_, err = database.Exec("UPDATE votingdb.questions set name = ? WHERE id = ?", name, id_question)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "/admin/votings/"+id_voting+"/questions/"+id_question+"/answers", 302)
}

func EditAnswerTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_answer, ok := vars["id_answer"]
	if !ok {
		err := fmt.Errorf("answer id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	row := database.QueryRow("SELECT * FROM votingdb.answers WHERE id = ?", id_answer)

	answer := Answer{}

	err := row.Scan(&answer.ID, &answer.Name, &answer.ID_Question)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	} else {
		tmpl, _ := template.ParseFiles("templates/admin_edit_answer.html")
		tmpl.Execute(w, answer)
	}
}

func EditAnswerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_question, ok := vars["id_question"]
	if !ok {
		err := fmt.Errorf("question id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	err := r.ParseForm()
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	id_answer := r.FormValue("id_answer")
	name := r.FormValue("name")

	_, err = database.Exec("UPDATE votingdb.answers set name = ? WHERE id = ?", name, id_answer)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	row := database.QueryRow("SELECT * FROM votingdb.questions WHERE id = ?", id_question)

	question := Question{}

	err = row.Scan(&question.ID, &question.Name, &question.ID_Voting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	} else {
		http.Redirect(w, r, fmt.Sprintf("/admin/votings/%d/questions/%s/answers", question.ID_Voting, id_question), 302)
	}
}

func DeleteVotingHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_voting, ok := vars["id_voting"]
	if !ok {
		err := fmt.Errorf("voting id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	deleteQuestions(w, id_voting)

	_, err := database.Exec("DELETE FROM votingdb.votings WHERE id = ?", id_voting)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", 302)
}

func DeleteQuestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_question, ok := vars["id_question"]
	if !ok {
		err := fmt.Errorf("question id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	var idVoting int
	row_voting := database.QueryRow("SELECT id_voting FROM votingdb.questions WHERE id = ?", id_question)
	err := row_voting.Scan(&idVoting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	deleteAnswers(w, id_question)

	_, err = database.Exec("DELETE FROM votingdb.questions WHERE id = ?", id_question)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/votings/%d/questions/answers", idVoting), 302)
}

func DeleteAnswerHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_answer, ok := vars["id_answer"]
	if !ok {
		err := fmt.Errorf("answer id parametr is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	var idQuestion int
	row_queestion := database.QueryRow("SELECT id_question FROM votingdb.answers WHERE id = ?", id_answer)
	err := row_queestion.Scan(&idQuestion)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	var idVoting int
	row_answer := database.QueryRow("SELECT id_voting FROM votingdb.questions WHERE id = ?", idQuestion)
	err = row_answer.Scan(&idVoting)
	if err != nil {
		serverError(w, err, http.StatusNotFound)
		return
	}

	_, err = database.Exec("DELETE FROM votingdb.answers WHERE id = ?", id_answer)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/votings/%d/questions/%d/answers", idVoting, idQuestion), 302)
}

func main() {
	db, err := sql.Open("mysql", "root:11111111@tcp(localhost:3306)/votingdb")
	if err != nil {
		panic(err)
	}

	database = db

	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/authentication", AuthenticationHandler).Methods("POST")
	router.HandleFunc("/authentication", AuthenticationTemplate).Methods("GET")
	router.HandleFunc("/logout", LogOut)

	router.HandleFunc("/", IndexHandler).Methods("GET")
	router.HandleFunc("/votings/{id_voting:[0-9]+}/questions/answers", VotingQAHandler).Methods("POST")
	router.HandleFunc("/votings/{id_voting:[0-9]+}/questions/answers", VotingQATemplate).Methods("GET")
	router.HandleFunc("/votings/{id_voting:[0-9]+}/progress", ProgressHandler).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions/answers", VotingQAAdminHandler).Methods("GET")
	router.HandleFunc("/admin/votings", CreateVotingHandler).Methods("POST")
	router.HandleFunc("/admin/votings", CreateVotingTemplate).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions/{id_question:[0-9]+}/answers", OpenQAHandler).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions", CreateQuestionHandler).Methods("POST")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions", CreateQuestionTemplate).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions/{id_question:[0-9]+}/answers", CreateAnswerHandler).Methods("POST")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions/{id_question:[0-9]+}/answers", CreateAnswerTemplate).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}", EditVotingHandler).Methods("PUT")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}", EditVotingTemplate).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions/{id_question:[0-9]+}", EditQuestionHandler).Methods("PUT")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}/questions/{id_question:[0-9]+}", EditQuestionTemplate).Methods("GET")
	router.HandleFunc("/admin/questions/{id_question:[0-9]+}/answers/{id_answer:[0-9]+}", EditAnswerHandler).Methods("PUT")
	router.HandleFunc("/admin/questions/{id_question:[0-9]+}/answers/{id_answer:[0-9]+}", EditAnswerTemplate).Methods("GET")
	router.HandleFunc("/admin/votings/{id_voting:[0-9]+}", DeleteVotingHandler).Methods("DELETE")
	router.HandleFunc("/admin/questions/{id_question:[0-9]+}", DeleteQuestionHandler).Methods("DELETE")
	router.HandleFunc("/admin/answers/{id_answer:[0-9]+}", DeleteAnswerHandler).Methods("DELETE")

	router.Use(cookieMiddleware)

	http.Handle("/", router)

	fmt.Println("Server is listening...")

	err = http.ListenAndServe(":9080", router)
	if err != nil {
		log.Println("HTTP Server Error - ", err)
	}
}
