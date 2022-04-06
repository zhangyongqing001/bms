package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type Book struct {
	Id          int     `db:"id"`
	Title       string  `db:"title"`
	Price       float64 `db:"price"`
	PublisherId int     `db:"publisher_id"`
	PublishDate string  `db:"publish_date"`
	Authors     []int
}

type Author struct {
	Id   int    `db:"id"`
	Name string `db:"name"`
}

type Publisher struct {
	Id   int    `db:"id"`
	Name string `db:"name"`
}

var db *sqlx.DB

func init() {
	dsn := "root:123@tcp(mysql:3306)/bms"
	db, _ = sqlx.Connect("mysql", dsn)
	err := db.Ping()
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	fmt.Println("BMS2 Server start ...")
	log.Println("BMS2 Server start ...")
	http.HandleFunc("/", home)
	http.HandleFunc("/login", login)
	http.HandleFunc("/book_list", list)
	http.HandleFunc("/book_add", loginAuth(add))
	http.HandleFunc("/book_edit/", loginAuth(edit))
	http.HandleFunc("/book_delete/", loginAuth(delete))
	http.Handle("/statics/", http.FileServer(http.Dir(".")))

	http.ListenAndServe(":8080", nil)
}

var Temps = make(map[string]*template.Template, 5)

func init() {
	logFile, err := os.OpenFile("./log/runLog.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		log.SetPrefix("[running]")
		log.SetFlags(log.Lshortfile | log.Lmicroseconds | log.Ldate)
	} else {
		fmt.Println("Open file error: ", err)
	}

	isIn := func(a int, list []int) bool {

		for _, s := range list {
			if s == a {
				return true
			}
		}
		return false
	}

	Temps["home"] = template.Must(template.ParseFiles("./statics/htmls/home.html"))
	Temps["login"] = template.Must(template.ParseFiles("./statics/htmls/home.html", "./statics/htmls/login.html"))
	Temps["list"] = template.Must(template.New("book_list.html").Funcs(template.FuncMap{"isIn": isIn}).ParseFiles("./statics/htmls/home.html", "./statics/htmls/book_list.html"))
	Temps["add"] = template.Must(template.ParseFiles("./statics/htmls/home.html", "./statics/htmls/book_add.html"))
	Temps["edit"] = template.Must(template.New("book_edit.html").Funcs(template.FuncMap{"isIn": isIn}).ParseFiles("./statics/htmls/home.html", "./statics/htmls/book_edit.html"))
}

// 全局检验装饰器
func loginAuth(fn func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, _ := r.Cookie("admin")
		if cookie == nil || cookie.Value != "adminvalue" {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		}
		fn(w, r) // add(w, r)
	}

}

func login(w http.ResponseWriter, r *http.Request) {
	tmp := Temps["login"]
	switch r.Method {
	case "GET":
		tmp.Execute(w, nil)
	case "POST":
		// 登录 校验逻辑
		name := r.PostFormValue("username")
		pawd := r.PostFormValue("password")
		if name == "jack" && pawd == "123" {
			// 设置cookie
			cookie := &http.Cookie{Name: "admin", Value: "adminvalue"}
			http.SetCookie(w, cookie)
			http.Redirect(w, r, "/book_list", http.StatusTemporaryRedirect)
		} else {
			tmp.Execute(w, "用户名或密码错误")
		}

	}

}

func home(w http.ResponseWriter, r *http.Request) {
	// tmp, _ := template.ParseFiles("home.html")
	tmp := Temps["home"]
	tmp.Execute(w, nil)
}

func list(w http.ResponseWriter, r *http.Request) {
	sqlBook := "select id,title,price,publish_date,publisher_id from book"
	sqlPublishers := "select id,name from publisher"
	sqlAuthors := "select id,name from author"
	sqlBook2Author := "select author_id from book2author where book_id=?"

	var books []Book
	var publishers []Publisher
	var authors []Author

	db.Select(&books, sqlBook)
	db.Select(&publishers, sqlPublishers)
	db.Select(&authors, sqlAuthors)

	for i, book := range books {
		rows, _ := db.Query(sqlBook2Author, book.Id)
		for rows.Next() {
			var authorId int
			rows.Scan(&authorId)
			book.Authors = append(book.Authors, authorId)
		}
		books[i] = book
	}
	tmp := Temps["list"]
	Info := map[string]interface{}{"books": books, "publishers": publishers, "authors": authors}
	tmp.Execute(w, Info)
}

func add(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		sqlPublishers := "select id,name from publisher"
		sqlAuthors := "select id,name from author"
		var publishers []Publisher
		var authors []Author
		db.Select(&publishers, sqlPublishers)
		db.Select(&authors, sqlAuthors)

		tmp := Temps["add"]
		info := map[string]interface{}{"authors": authors, "publishers": publishers}
		tmp.Execute(w, info)
	case "POST":
		title := r.FormValue("title")
		price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
		publishDate := r.FormValue("publish_date")
		publisherId, _ := strconv.Atoi(r.FormValue("publisher_id"))

		// db
		bookSql := "insert book(title, price, publish_date, publisher_id) values(?,?,?,?)"
		ret, err := db.Exec(bookSql, title, price, publishDate, publisherId)
		if err != nil {
			fmt.Println("Exec error: ", err)
		} else {
			bookId, _ := ret.LastInsertId()
			for _, authorId := range r.PostForm["authors_id"] {
				authorId, _ := strconv.Atoi(authorId)
				sqlBook2Author := "insert book2author(book_id,author_id) values(?,?)"
				db.Exec(sqlBook2Author, bookId, authorId)
			}
		}
		http.Redirect(w, r, "/book_list", http.StatusTemporaryRedirect)
	}
}

func edit(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/book_edit/"):]
	id, _ := strconv.Atoi(idStr)
	var book Book
	bookSql := "select id,title,price,publish_date,publisher_id from book where id=?"
	err := db.Get(&book, bookSql, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// 查作者id
	sqlBook2Author := "select author_id from book2author where book_id=?"
	rows, _ := db.Query(sqlBook2Author, book.Id)
	for rows.Next() {
		var authodId int
		rows.Scan(&authodId)
		book.Authors = append(book.Authors, authodId)
	}
	switch r.Method {
	case "GET":
		sqlPublishers := "select id,name from publisher"
		sqlAuthors := "select id,name from author"
		var publishers []Publisher
		var authors []Author
		db.Select(&publishers, sqlPublishers)
		db.Select(&authors, sqlAuthors)
		tmp := Temps["edit"]
		info := map[string]interface{}{"authors": authors, "publishers": publishers, "book": book}
		tmp.Execute(w, info)
	case "POST":
		title := r.FormValue("title")
		price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
		publishDate := r.FormValue("publish_date")
		publisherId, _ := strconv.Atoi(r.FormValue("publisher_id"))
		// db
		bookSql := "update book set title=?,price=?,publish_date=?,publisher_id=? where id=?"
		db.Exec(bookSql, title, price, publishDate, publisherId, book.Id)
		sqlBook2Author := "delete from book2author where book_id=?"
		db.Exec(sqlBook2Author, book.Id)
		for _, authorId := range r.PostForm["author_ids"] {
			authorId, _ := strconv.Atoi(authorId)
			sqlBook2Author2 := "insert book2author(book_id, author_id) values(?,?)"
			db.Exec(sqlBook2Author2, book.Id, authorId)
		}

		http.Redirect(w, r, "/book_list", http.StatusTemporaryRedirect)
	}
}

func delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/book_delete/"):]
	id, _ := strconv.Atoi(idStr)
	// 从db中删除book
	sqlBookDelete := "delete from book where id=?"
	sqlBook2AuthorDelete := "delete from book2author where book_id=?"
	db.Exec(sqlBookDelete, id)
	db.Exec(sqlBook2AuthorDelete, id)

	// 自己试试？
	// sqlDelete := "delete from book where id=?; delete from book2author where book_id=?;"
	// _, err := db.Exec(sqlDelete, id, id)
	// fmt.Println(err)
	http.Redirect(w, r, "/book_list", http.StatusTemporaryRedirect)

}
