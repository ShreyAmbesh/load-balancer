package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func main() {

	go printTotalCalls()
	go startCalls()
	go increaseThreads()
	// Block indefinitely to keep the calls running
	select {}
}

var totalCalls int
var mutex sync.Mutex

var threadMutex sync.RWMutex

func makeAPICall(url string, done chan struct{}) {
	client := http.Client{
		Timeout: time.Second * 10, // Set a timeout for the HTTP request
	}

	resp, err := client.Get(url)
	if err != nil {
		//fmt.Printf("Error making GET request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	mutex.Lock()
	totalCalls++
	mutex.Unlock()

	// Signal to the main goroutine that this call is done
	done <- struct{}{}
}

func printTotalCalls() {
	ticker := time.NewTicker(1 * time.Second)
	lastCallsCount := 0
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mutex.Lock()
			fmt.Printf("Total number of calls made: %d, Req Rate: %f/s \n", totalCalls, float64(totalCalls-lastCallsCount))
			lastCallsCount = totalCalls
			mutex.Unlock()
		}
	}
}

var threads int = 10

func increaseThreads() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mutex.Lock()
			threads++
			mutex.Unlock()
			fmt.Println("Increased threads to", threads)
		}
	}
}
func startCalls() {
	ticker1 := time.NewTicker(500000 * time.Millisecond)
	ticker2 := time.NewTicker(60 * time.Millisecond)
	defer ticker1.Stop()
	defer ticker2.Stop()

	done := make(chan struct{})

	for {
		select {
		case <-ticker1.C:
			//threadMutex.RLock()
			//threadCount := threads
			//threadMutex.RUnlock()
			//makeCalls("1", threadCount, done)
		case <-ticker2.C:
			threadMutex.RLock()
			threadCount := threads
			threadMutex.RUnlock()
			makeCalls("2", threadCount, done)
		}
	}
}

func makeCalls(serviceId string, threads int, done chan struct{}) {
	//fmt.Printf("Making %d calls to %s\n", threads, url)

	var ports []struct{ Port int }
	res, err := http.Get(fmt.Sprintf("http://localhost:3000/api/service/%s/load-balancers", serviceId))
	if err != nil {
		return
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return
	}
	dat, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(dat, &ports)
	if err != nil {
		return
	}

	for i := 0; i < threads; i++ {
		go func() {
			for {
				l := len(ports)
				if l == 0 {
					continue
				}
				makeAPICall(fmt.Sprintf("http://localhost:%d", ports[i%l].Port), done)
				<-done // Wait for the signal that the call is done before making the next call
			}
		}()
	}
}
