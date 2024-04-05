package main

import (
	"fmt"
	"net/http"
	"os"
)

var ContainerName string

func main() {
	ContainerName = os.Getenv("CONTAINER_NAME")
	http.HandleFunc("/", HelloHandler)
	http.HandleFunc("/health", HealthHandler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server: ", err)
		return
	}
}

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	_, err := fmt.Fprintf(w, "Hello from, %s!", ContainerName)
	if err != nil {
		return
	}
}
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
}
