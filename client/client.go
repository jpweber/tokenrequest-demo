package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// wrapper function for http logging
func logger(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer log.Printf("%s - %s", r.Method, r.URL)
		fn(w, r)
	}
}

func readToken() []byte {
	file, err := os.Open("/var/run/secrets/tokens/factor-token")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	return b
}

func reqWithToken(w http.ResponseWriter, r *http.Request) {
	// read the svc token
	token := readToken()

	// submit simple get request with token
	host := "test-service:8080"
	URI := "/factor/" + r.RequestURI

	client := &http.Client{}
	req, _ := http.NewRequest("GET", "http://"+host+URI, nil)
	req.Header.Set("X-Auth-Token", string(token))
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error in http request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		w.Write([]byte("The HTTP request was not authenticated. Downstream service responded with 403"))
		return
	}

	if resp.StatusCode == http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		w.Write(body)
	}

}

func main() {
	log.Println("Starting application...")

	s := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		log.Fatal(s.ListenAndServe())
	}()

	http.HandleFunc("/", logger(reqWithToken))

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting...")

	s.Shutdown(context.Background())
}
