package main

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

func apis(db *gorm.DB) {
	http.HandleFunc("/lb-health", HealthHandler)
	http.HandleFunc("/service-update", func(w http.ResponseWriter, r *http.Request) {
		ServiceUpdateHandler(w, r, db)
	})
	err := http.ListenAndServe(":3210", nil)
	if err != nil {
		fmt.Println("Error starting lb server: ", err)
		return
	}
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(calculateRequestRate())
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(200)
	_, errWrite := w.Write(dat)
	if errWrite != nil {
		log.Printf("Error writing health response: %s", errWrite)
		return
	}
}

func calculateRequestRate() float64 {
	now := time.Now()
	thirtySecondsAgo := now.Add(-30 * time.Second)
	count := 0
	for i := len(requestLog) - 1; i >= 0; i-- {
		if requestLog[i].Before(thirtySecondsAgo) {
			break
		}
		count++
	}
	return float64(count) / 30.0
}

func ServiceUpdateHandler(w http.ResponseWriter, r *http.Request, db *gorm.DB) {
	w.WriteHeader(200)
	getService(db)
}
