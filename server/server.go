package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const audience = "factors"

// const audience = "vault"

type ExampleResponse struct {
	Error   error
	Factors []int64
}

// wrapper function for http logging
func logger(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer log.Printf("%s - %s", r.Method, r.URL)
		fn(w, r)
	}
}

func factor(nr int64) ([]int64, error) {
	fs := make([]int64, 1)
	if nr < 1 {
		return fs, errors.New("Factors of, 0 not computed")
	}

	fs[0] = 1
	apf := func(p int64, e int) {
		n := len(fs)
		for i, pp := 0, p; i < e; i, pp = i+1, pp*p {
			for j := 0; j < n; j++ {
				fs = append(fs, fs[j]*pp)
			}
		}
	}
	e := 0
	for ; nr&1 == 0; e++ {
		nr >>= 1
	}
	apf(2, e)
	for d := int64(3); nr > 1; d += 2 {
		if d*d > nr {
			d = nr
		}
		for e = 0; nr%d == 0; e++ {
			nr /= d
		}
		if e > 0 {
			apf(d, e)
		}
	}

	return fs, nil

}
func validateAudiences(auds []interface{}) bool {
	for _, v := range auds {
		if v == audience {
			return true
		}
		continue
	}

	return false
}
func validateToken(svcToken, bearerToken string) bool {

	reviewPayload := []byte(`{"kind": "TokenReview","apiVersion": "authentication.k8s.io/v1","spec": {"token": "` + svcToken + `"}}`)
	body := bytes.NewBuffer(reviewPayload)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Create request
	req, err := http.NewRequest("POST", "https://kubernetes.default:443/apis/authentication.k8s.io/v1/tokenreviews", body)

	// Headers
	req.Header.Add("Authorization", "Bearer "+bearerToken)
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	// Fetch Request
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("Failure : ", err)
	}

	// Read Response Body
	respBody, _ := ioutil.ReadAll(resp.Body)

	// Display Results
	// log.Println("response Status : ", resp.Status)
	// log.Println("response Headers : ", resp.Header)
	// log.Println("response Body : ", string(respBody))
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		log.Println("Error unmarshaling response", err)
	}
	log.Println(respData["status"])

	// check for authentication true
	if respData["status"].(map[string]interface{})["authenticated"] == true {

		// look for us in the array of audiences
		if validateAudiences(respData["status"].(map[string]interface{})["audiences"].([]interface{})) {
			return true
		}

	}

	return false
}
func readSvcAcctToken() []byte {
	file, err := os.Open("/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	return b
}
func factorHandler(w http.ResponseWriter, r *http.Request) {

	// validate the token from the request
	svcAcctToken := readSvcAcctToken()
	if validateToken(r.Header.Get("X-Auth-Token"), string(svcAcctToken)) != true {
		w.WriteHeader(403)
		return
	}

	uriParts := strings.Split(r.RequestURI, "/")
	// TODO: DEBUG
	log.Println(uriParts)
	factReq, _ := strconv.ParseInt(uriParts[2], 10, 64)
	factors, err := factor(factReq)
	resp := ExampleResponse{
		Error:   err,
		Factors: factors,
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		log.Println("json marshaling error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)

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

	http.HandleFunc("/factor/", logger(factorHandler))

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting...")

	s.Shutdown(context.Background())
}
