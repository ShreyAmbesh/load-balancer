package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

var (
	RunnerPort = 0

	NeighbourPort     = 0
	isLeader          = false
	isNeighbourLeader = false
	MaxNodes          = 0

	neighbourDownCount = 0

	isElectingLeader = false

	noNeighbour = false

	httpClient = http.Client{
		Timeout: 2 * time.Second,
	}
)

func leaderElection() {
	errorParsingEnv := false
	runnerPort, err := strconv.ParseInt(os.Getenv("RUNNER_PORT"), 10, 64)
	if err != nil {
		fmt.Println("Error parsing runner port")
		errorParsingEnv = true
	}
	RunnerPort = int(runnerPort)
	neighbourPort, errr := strconv.ParseInt(os.Getenv("NEIGHBOUR_PORT"), 10, 64)
	if errr != nil {
		fmt.Println("Error parsing neighbour port")
		errorParsingEnv = true
	}
	NeighbourPort = int(neighbourPort)
	maxNodes, errrr := strconv.ParseInt(os.Getenv("MAX_NODES"), 10, 64)
	if errrr != nil {
		fmt.Println("Error parsing max nodes")
		errorParsingEnv = true
	}
	MaxNodes = int(maxNodes)
	isLeader, err = strconv.ParseBool(os.Getenv("IS_LEADER"))
	if err != nil {
		isLeader = false
		errorParsingEnv = true
	}
	if !errorParsingEnv {
		if isLeader {
			go orchestrate()
		}
		router := gin.Default()
		apis := router.Group("")
		{
			apis.GET("/health", health)
			apis.GET("/elect-leader", electLeader)
			apis.GET("/stop-election", stopElection)
		}
		go checkNeighbours()
		fmt.Println("Leader election is running on port", RunnerPort, "and neighbour port", NeighbourPort)
		err = router.Run(fmt.Sprintf(":%d", RunnerPort))
		if err != nil {
			panic(err)
		}
	} else {
		orchestrate()
	}
}

func checkNeighbours() {
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ticker.C:
			{
				if isElectingLeader {
					continue
				}
				if noNeighbour {
					ticker.Stop()
					return
				}
				fmt.Println("Checking neighbour", NeighbourPort)
				resp, err := callNeighbour("/health")
				if err != nil || resp == nil {
					fmt.Println("Neighbour is down")
					neighbourDown()
					continue
				}
				fmt.Println("Neighbour is up")
				if neighbourDownCount > 0 {
					neighbourDownCount = 0
				}
				isNeighbourLeaderRes, ok := resp.(bool)
				if !ok {
					fmt.Println("Unable to convert to bool")
					return
				}
				if isNeighbourLeader != isNeighbourLeaderRes {
					isNeighbourLeader = isNeighbourLeaderRes
				}

			}
		}
	}
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, isLeader)
}

func stopElection(c *gin.Context) {
	isElectingLeader = false
	if isLeader {
		fmt.Println("I am the leader, stopping election")
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	fmt.Println("Stopping leader election")
	c.JSON(http.StatusOK, gin.H{})
	callNeighbour("/stop-election")
}

func neighbourDown() {
	neighbourDownCount++
	if neighbourDownCount >= 3 {
		if isNeighbourLeader {
			findNextNeighbour()
			startElectingLeader()
		} else {
			findNextNeighbour()
		}
	}
}

func findNextNeighbour() {
	var allNodes []int = make([]int, 0)
	startPort := 3010
	myPortIndex := 0
	for i := 0; i < MaxNodes; i++ {
		allNodes = append(allNodes, startPort)
		if startPort == RunnerPort {
			myPortIndex = i
		}
		startPort = startPort + 10
	}
	newNeighbourPortIndex := myPortIndex
	for {
		newNeighbourPortIndex++
		if newNeighbourPortIndex >= len(allNodes) {
			newNeighbourPortIndex = 0
		}
		if newNeighbourPortIndex == myPortIndex {
			fmt.Println("No new neighbour found")
			noNeighbour = true
			becomeLeader()
			return
		}
		NeighbourPort = allNodes[newNeighbourPortIndex]
		resp, err := callNeighbour("/health")
		if err != nil || resp == nil {
			continue
		}
		break
	}
	fmt.Println("New neighbour port is", NeighbourPort)
}

func startElectingLeader() {
	if isLeader {
		fmt.Println("I am the leader, not starting election")
		return
	}
	fmt.Println("Starting leader election")
	isElectingLeader = true
	callNeighbour(fmt.Sprintf("/elect-leader?candidateLeaderPort=%d", RunnerPort))
}

func electLeader(c *gin.Context) {
	isElectingLeader = true
	runnerPort64 := int64(RunnerPort)
	candidateLeaderPort, err := strconv.ParseInt(c.Query("candidateLeaderPort"), 10, 64)
	if err != nil {
		fmt.Println("Error parsing neighbour port")
		candidateLeaderPort = runnerPort64
	}
	if candidateLeaderPort < runnerPort64 {
		candidateLeaderPort = runnerPort64
	} else if candidateLeaderPort == runnerPort64 {
		becomeLeader()
		isElectingLeader = false
		callNeighbour("/stop-election")
		return
	}
	callNeighbour(fmt.Sprintf("/elect-leader?candidateLeaderPort=%d", candidateLeaderPort))
}

func callNeighbour(path string) (interface{}, error) {
	resp, err := httpClient.Get(fmt.Sprint("http://localhost:", NeighbourPort, path))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error calling neighbour")
	}
	dat, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var res interface{}
	err = json.Unmarshal(dat, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func becomeLeader() {
	isLeader = true
	isStart = false
	fmt.Println("I am the leader")
	go orchestrate()
}
