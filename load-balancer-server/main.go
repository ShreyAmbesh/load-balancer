package main

import (
	"fmt"
	"gorm.io/gorm"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"time"
)

type IncomingReq struct {
	srcConn net.Conn
	reqId   string
}

type BackendServer struct {
	ID             uint `json:"id"`
	ServiceID      uint
	Host           string `json:"host"`
	IsHealthy      bool   `json:"isHealthy"`
	Port           int    `json:"port"`
	unHealthyCount int
	ContainerName  string `json:"containerName"`
	numRequests    int
}

type Service struct {
	ID                  uint             `json:"id"`
	Name                string           `json:"name" gorm:"unique"`
	Backends            []*BackendServer `json:"backends"`
	HealthEndpoint      string           `json:"healthEndpoint"`
	UnHealthyThreshold  int              `json:"unHealthyThreshold"`
	HealthCheckInterval int              `json:"healthCheckInterval"`
	Min                 int              `json:"min"`
	Max                 int              `json:"max"`
	ContainerImageName  string           `json:"containerImageName"`
	ContainerPort       int              `json:"containerPort"`
}
type ReverseProxy struct {
	reverseProxy *httputil.ReverseProxy
	origin       *url.URL
}

var reverseProxies = make(map[string]ReverseProxy)

var requestLog []time.Time

func (b *BackendServer) String() string {
	return fmt.Sprintf("%s:%d", b.Host, b.Port)
}

func addReverseProxy(service *Service, b *BackendServer) {
	origin, err := url.Parse(fmt.Sprintf("http://%s:%d", b.ContainerName, service.ContainerPort))
	if err != nil {
		panic(err)
	}
	reverseProxies[b.ContainerName] = ReverseProxy{
		reverseProxy: httputil.NewSingleHostReverseProxy(origin),
		origin:       origin,
	}
}

var nextBackendIndex int

var service Service

var mux sync.Mutex

func getService(db *gorm.DB) {
	ServiceName := os.Getenv("SERVICE_NAME")
	var localService Service
	err := db.Preload("Backends").First(&localService, "name = ?", ServiceName).Error
	if err != nil {
		panic(err)
	}
	//compare the backend lists and update the reverse proxies
	for _, backend := range localService.Backends {
		if _, ok := reverseProxies[backend.ContainerName]; !ok {
			addReverseProxy(&localService, backend)
		}

	}
	mux.Lock()
	service = localService
	mux.Unlock()
}

func getServiceJob(db *gorm.DB) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			getService(db)
		}
	}
}

func main() {
	db := getDb()
	getService(db)
	go apis(db)
	go getServiceJob(db)

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			cleanupRequestLog()
		}
	}()
	http.HandleFunc("/", proxy)
	log.Fatal(http.ListenAndServe(":4000", nil))
}

// Round-robin return only healthy backend
func getNextBackend() *BackendServer {
	mux.Lock()
	defer mux.Unlock()
	backends := service.Backends
	if len(backends) == 0 {
		return nil
	}
	backend := backends[nextBackendIndex%len(backends)]
	nextBackendIndex++
	if backend.IsHealthy {
		return backend
	} else {
		//find next healthy backend
		for i := 0; i < len(backends); i++ {
			backend = backends[nextBackendIndex%len(backends)]
			nextBackendIndex++
			if backend.IsHealthy {
				return backend
			}
		}
	}
	return nil
}

func proxy(w http.ResponseWriter, r *http.Request) {
	backend := getNextBackend()
	if backend == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	fmt.Println("Proxying request to", backend.ContainerName, backend.numRequests)
	backend.numRequests++
	requestLog = append(requestLog, time.Now())
	reverseProxies[backend.ContainerName].reverseProxy.ServeHTTP(w, r)
}

func cleanupRequestLog() {
	now := time.Now()
	sixtySecondsAgo := now.Add(-60 * time.Second)
	i := 0
	for ; i < len(requestLog); i++ {
		if requestLog[i].After(sixtySecondsAgo) {
			break
		}
	}
	// Remove old entries
	requestLog = requestLog[i:]
}
