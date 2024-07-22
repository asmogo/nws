package main

import (
	"fmt"
	"io/ioutil"
	"log/slog"
	"math/rand"
	"net/http"
	"time"
)

func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Generate random number between 1 and 10
	randomSleep := rand.Intn(10) + 1

	// Sleep for random number of seconds
	// time.Sleep(time.Duration(randomSleep) * time.Second)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "hi there, you were sleeping for %d seconds. I received your request: %s", randomSleep, string(body))
	slog.Info("Received request", "wait", randomSleep)
}

func main() {
	http.HandleFunc("/", echoHandler)
	//err := http.ListenAndServe(":3338", nil)
	err := http.ListenAndServeTLS(":3338", "localhost.crt", "localhost.key", nil)
	if err != nil {
		fmt.Println("Error while starting server:", err)
	}
}
