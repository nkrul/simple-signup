package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gocarina/gocsv"
)

//go:embed index.html
var IndexFile []byte

func main() {
	addr := ":8081"
	fileName := "./signups.csv"
	indexFileName := "./index.html"

	args := os.Args[1:]
	for len(args) > 0 {
		if strings.EqualFold(args[0], "--port") {
			addr = fmt.Sprintf(":%v", args[1])
			args = args[2:]
			continue
		}
		if strings.EqualFold(args[0], "--file") {
			fileName = args[1]
			args = args[2:]
			continue
		}
		if strings.EqualFold(args[0], "--index") {
			indexFileName = args[1]
			args = args[2:]
			continue
		}
		break
	}

	file, err := os.ReadFile(indexFileName)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Fatal(err)
		}
	} else {
		IndexFile = file
	}

	handler := &SignupHandler{fileName: fileName}
	handler.init()

	server := &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1024,
	}
	err = server.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("%v\n", err)
	}
}

var _ http.Handler = (*SignupHandler)(nil)

type SignupHandler struct {
	mu       sync.Mutex
	fileName string
}

func (obj *SignupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseForm()
		if err == nil {
			form := r.Form
			emails := form["email"]
			if len(emails) == 1 {
				email := emails[0]
				obj.signupEmail(strings.TrimSpace(email))
			}
		} else {
			log.Fatal(err)
		}
		sendRedirectAfterPost(w)
		return
	}
	if r.Method == "GET" {
		w.WriteHeader(200)
		w.Write(IndexFile)
		return
	}
	sendRedirectViaHeader(w)

}

func sendRedirectViaHeader(w http.ResponseWriter) {
	w.Header().Add("Location", "/")
	w.WriteHeader(302)
}
func sendRedirectAfterPost(w http.ResponseWriter) {
	w.Header().Add("Location", "/")
	w.WriteHeader(303)
}

func (obj *SignupHandler) isEmailValid(email string) bool {
	if !strings.Contains(email, "@") {
		return false
	}
	parts := strings.Split(email, "@")

	// make sure we (probably) have some kind of username
	if len(parts[0]) < 1 {
		return false
	}

	// make sure we (probably) have some kind of server name
	if strings.HasPrefix(parts[1], ".") {
		return false
	}
	if strings.HasSuffix(parts[1], ".") {
		return false
	}
	if strings.Contains(parts[1], "..") {
		return false
	}
	return true
}
func (obj *SignupHandler) isEmailNewAndUnique(email string) bool {
	c := make(chan SignupRecord)

	f, err := os.OpenFile(obj.fileName, os.O_RDONLY, 0644)
	if err != nil {
		panic(err)
	}

	go func() {
		err := gocsv.UnmarshalToChan(f, c)
		if err != nil {
			log.Fatal(err)
		}
	}()
	for r := range c {
		if r.Email == email {
			return false
		}
	}
	return true
}
func (obj *SignupHandler) signupEmail(email string) {
	if !obj.isEmailValid(email) {
		return
	}
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.isEmailNewAndUnique(email) {

		f, err := os.OpenFile(obj.fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}

		csvWriter := gocsv.DefaultCSVWriter(f)
		csvWriter.Write([]string{
			email,
			"true",
			"false",
			time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
		csvWriter.Flush()

		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}
}

func (obj *SignupHandler) init() {
	obj.mu.Lock()
	defer obj.mu.Unlock()

	_, err := os.OpenFile(obj.fileName, os.O_RDWR, 0644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.Create(obj.fileName)
			if err != nil {
				log.Fatal(err)
			}
			f.Write(([]byte)("Email,Valid,Emailed,UtcSignedUpAt\n"))
		} else {
			log.Fatal(err)
		}
	}
}

type SignupRecord struct {
	Email         string
	Valid         bool
	Emailed       bool
	UtcSignedUpAt DateTime
}

type DateTime struct {
	time.Time
}

func (date *DateTime) UnmarshalCSV(csv string) (err error) {
	date.Time,
		err = time.Parse(time.RFC3339, csv)
	return err
}
