package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type BackendServer struct {
	ID             uint `json:"id"`
	ServiceID      uint
	IsHealthy      bool `json:"isHealthy"`
	Port           int  `json:"port"`
	unHealthyCount int
	ContainerName  string `json:"containerName"`
}

type LoadBalancerServer struct {
	ID             uint `json:"-"`
	ServiceID      uint `json:"-"`
	IsHealthy      bool `json:"-"`
	Port           int  `json:"port"`
	HealthPort     int  `json:"-"`
	unHealthyCount int
	ContainerName  string `json:"-"`
}

type Service struct {
	ID                  uint                  `json:"id"`
	Name                string                `json:"name" gorm:"unique"`
	Backends            []*BackendServer      `json:"backends" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	HealthEndpoint      string                `json:"healthEndpoint"`
	UnHealthyThreshold  int                   `json:"unHealthyThreshold"`
	HealthCheckInterval int                   `json:"healthCheckInterval"`
	Min                 int                   `json:"min"`
	Max                 int                   `json:"max"`
	ContainerImageName  string                `json:"containerImageName"`
	ContainerPort       int                   `json:"containerPort"`
	LoadBalancers       []*LoadBalancerServer `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	endServiceChecks    chan bool
}

var (
	lbHealthPortCounter = 3201
	lbPortCounter       = 5001
	backendPortCounter  = 7001

	isStart = true

	MinLBCount = 2
)

func increaseLoadBalancerPortCounter() {
	lbPortCounter++
	lbHealthPortCounter++
}

func increaseBackendPortCounter() {
	backendPortCounter++
}

var services []*Service

var db = getDb()

func main() {
	fmt.Println("Welcome to the load balancer!")
	//go quit()
	leaderElection()
}

func orchestrate() {
	go apis()
	db.Preload("Backends").Preload("LoadBalancers").Find(&services)
	if isStart {
		//stop all backend and load balancer servers
		_, errStop := runCommand("stop all containers", "docker stop $(docker ps --format '{{.Names}}' | grep '^lb-')")
		if errStop != nil {
			fmt.Println(errStop)
		}

		_, errDel := runCommand("delete all containers", "docker rm $(docker ps --format '{{.Names}}' | grep '^lb-')")
		if errDel != nil {
			fmt.Println(errDel)
		}
		db.Delete(&LoadBalancerServer{}, "id > 0")
		db.Delete(&BackendServer{}, "id > 0")
		db.Preload("Backends").Preload("LoadBalancers").Find(&services)
	}
	if !checkIfDockerNetworkExists() {
		createDockerNetwork()
	}

	//choose port numbers
	for _, service := range services {
		for _, backend := range service.Backends {
			if backend.Port > backendPortCounter {
				backendPortCounter = backend.Port + 1
			}
		}
		for _, lb := range service.LoadBalancers {
			if lb.Port > lbPortCounter {
				lbPortCounter = lb.Port + 1
			}
			if lb.HealthPort > lbHealthPortCounter {
				lbHealthPortCounter = lb.HealthPort + 1
			}
		}
	}
	//run backend servers
	for _, service := range services {
		startService(service, false)
	}
	isStart = false
	for {
		select {}
	}
}

func startService(service *Service, serviceUpdated bool) {
	if isStart || serviceUpdated {
		startBackendServers(service)
		// run load balancer servers
		startLoadBalancerServers(service)
	}
	if !serviceUpdated {
		service.endServiceChecks = make(chan bool)
		fmt.Printf("Starting health checks for service %s\n", service.Name)
		go serviceHealthChecks(service)
	}
}

func startBackendServers(service *Service) {
	stopAllBackendServer(service)
	for i := 0; i < service.Min; i++ {
		startBackendServer(service)
	}
}
func startBackendServer(service *Service) {
	backend := getNewBackendServer(service)
	service.Backends = append(service.Backends, backend)
	_, err := runBackendServer(backend, service)
	if err != nil {
		fmt.Println(err)
	}
}

func startLoadBalancerServers(service *Service) {
	stopAllServiceLoadBalancerServer(service)
	for i := 0; i < MinLBCount; i++ {
		startLoadBalancerServer(service)
	}
}

func startLoadBalancerServer(service *Service) {
	lb := getNewLoadBalancer(service)
	service.LoadBalancers = append(service.LoadBalancers, lb)
	_, err := runLoadBalancerServer(lb, service)
	if err != nil {
		fmt.Println(err)
	}
}

func getNewBackendServer(service *Service) *BackendServer {
	defer increaseBackendPortCounter()
	return &BackendServer{
		ServiceID:     service.ID,
		Port:          backendPortCounter,
		IsHealthy:     false,
		ContainerName: fmt.Sprintf("lb-%s-%s-%d", service.Name, strings.Split(service.ContainerImageName, ":")[0], backendPortCounter),
	}
}

func getNewLoadBalancer(service *Service) *LoadBalancerServer {
	defer increaseLoadBalancerPortCounter()
	return &LoadBalancerServer{
		ServiceID:     service.ID,
		Port:          lbPortCounter,
		HealthPort:    lbHealthPortCounter,
		IsHealthy:     false,
		ContainerName: fmt.Sprintf("lb-%s-load-balancer-%d", service.Name, lbPortCounter),
	}
}

func quit() {
	fmt.Println("\n" +
		"Press 'q' and 'ENTER' to quit")
	reader := bufio.NewReader(os.Stdin)

	for {
		text, _ := reader.ReadString('\n')

		if text == "q\n" || text == "\033\n" {
			fmt.Println("Exiting program.")
			for _, service := range services {
				stopAllBackendServer(service)
			}
			stopAllLoadBalancerServer()
			removeDockerNetwork()
			os.Exit(0)
		}
		//fmt.Println("You entered: ", text)
	}
}

func checkIfDockerNetworkExists() bool {
	_, err := runCommand("check docker network", "docker network inspect load-balancer-network")
	if err != nil {
		return false
	}
	return true
}
func createDockerNetwork() {
	_, err := runCommand("create docker network", "docker network create --subnet=10.0.0.0/16 load-balancer-network")
	if err != nil {
		return
	}
}

func removeDockerNetwork() {
	_, err := runCommand("remove docker network", "docker network rm load-balancer-network")
	if err != nil {
		return
	}
}

func reloadServices() {
	var updatedServices []*Service
	db.Preload("Backends").Preload("LoadBalancers").Find(&updatedServices)
	// stop backends and load balancers for services that are not in the updated services
	for _, service := range services {
		found := false
		for _, updatedService := range updatedServices {
			if service.ID == updatedService.ID && service.Name == updatedService.Name {
				found = true
				break
			}
		}
		if !found {
			service.endServiceChecks <- true
			stopAllServiceLoadBalancerServer(service)
			stopAllBackendServer(service)
		}
	}

	// start services that are not in the services
	for _, updatedService := range updatedServices {
		found := false
		for _, service := range services {
			if updatedService.ID == service.ID && updatedService.Name == service.Name {
				found = true
				break
			}
		}
		if !found {
			startService(updatedService, true)
		}
	}
	for _, service := range services {
		service.endServiceChecks <- true
	}
	services = updatedServices
	for _, service := range services {
		service.endServiceChecks = make(chan bool)
		go serviceHealthChecks(service)
	}
}
