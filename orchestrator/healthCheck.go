package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

func serviceHealthChecks(service *Service) {
	tickerLB := time.NewTicker(2 * time.Second)
	tickerBackend := time.NewTicker(time.Duration(service.HealthCheckInterval) * time.Second)

	const lbRequestRateThresholdUpper = 60.0
	const lbRequestRateThresholdLower = 20.0

	const backendRequestRateThresholdUpper = 20.0
	const backendRequestRateThresholdLower = 12.0
	lastFiveTotalRequestRate := []float64{0.0, 0.0, 0.0, 0.0, 0.0}
	healthCounter := 0
	maxLbCount := int(math.Ceil(float64(service.Max)/2)) + 1
	if len(service.LoadBalancers) == 0 {
		startLoadBalancerServers(service)
	}
	if len(service.Backends) == 0 {
		startBackendServers(service)
	}
	for {
		select {
		case <-service.endServiceChecks:
			{
				tickerLB.Stop()
				tickerBackend.Stop()
				return
			}
		case <-tickerLB.C:
			{
				//fmt.Println("LB Health Check ", service.Name, "Current LBs:", len(service.LoadBalancers))
				wg := sync.WaitGroup{}
				wg.Add(len(service.LoadBalancers))
				reqRateChannel := make(chan float64)

				totalLbReqRate := 0.0
				for index, _ := range service.LoadBalancers {
					go loadBalancerServerHealthCheck(index, service, reqRateChannel, &wg)
				}
				healthyLbCount := 0
				for _, lb := range service.LoadBalancers {
					reqRate := <-reqRateChannel
					totalLbReqRate += reqRate
					if lb.IsHealthy {
						healthyLbCount++
					}
				}

				healthyBackendCount := 0
				for _, b := range service.Backends {
					if b.IsHealthy {
						healthyBackendCount++
					}
				}
				//update the last ten request rates
				lastFiveTotalRequestRate = append(lastFiveTotalRequestRate[1:], totalLbReqRate)
				healthCounter++
				if healthCounter%5 == 0 {
					avgLastFiveRequestRate := (lastFiveTotalRequestRate[0] + lastFiveTotalRequestRate[1] + lastFiveTotalRequestRate[2] + lastFiveTotalRequestRate[3] + lastFiveTotalRequestRate[4]) / 5
					avgLbReqRate := avgLastFiveRequestRate / float64(healthyLbCount)

					avgBackendReqRate := avgLastFiveRequestRate / float64(healthyBackendCount)
					if avgBackendReqRate > backendRequestRateThresholdUpper {
						if service != nil {
							if healthyBackendCount < service.Max {
								backend := getNewBackendServer(service)
								service.Backends = append(service.Backends, backend)
								_, err := runBackendServer(backend, service)
								if err != nil {
									fmt.Println(err)
								}
							}
						}
					} else if avgBackendReqRate < backendRequestRateThresholdLower {
						if service != nil {
							if healthyBackendCount > service.Min {
								backend := service.Backends[len(service.Backends)-1]
								service.Backends = service.Backends[:len(service.Backends)-1]
								stopBackendServer(backend)
								go callLoadBalancerServiceUpdateEndpoints(service)
							}
						}
					}
					if avgLbReqRate > lbRequestRateThresholdUpper {
						if healthyLbCount < maxLbCount {
							lb := getNewLoadBalancer(service)
							service.LoadBalancers = append(service.LoadBalancers, lb)
							_, err := runLoadBalancerServer(lb, service)
							if err != nil {
								fmt.Println(err)
							}
						}
					} else if avgLbReqRate < lbRequestRateThresholdLower {
						if service != nil {
							if healthyLbCount > MinLBCount {
								lb := service.LoadBalancers[len(service.LoadBalancers)-1]
								service.LoadBalancers = service.LoadBalancers[:len(service.LoadBalancers)-1]
								stopLoadBalancerServer(lb)
							}
						}
					}
				}

				if len(service.LoadBalancers) < MinLBCount {
					for i := 0; i < (MinLBCount - len(service.LoadBalancers)); i++ {
						startLoadBalancerServer(service)
					}
				}
			}

		case <-tickerBackend.C:
			{
				//fmt.Println("Backend Health Check ", service.Name, "Min:", service.Min, "Max:", service.Max, "Current Backends:", len(service.Backends))
				lenBackends := len(service.Backends)
				wg := sync.WaitGroup{}
				wg.Add(lenBackends)
				for index, _ := range service.Backends {
					go backendServerHealthCheck(index, service, &wg)
				}
				wg.Wait()
				if lenBackends < service.Min {
					for i := 0; i < service.Min-lenBackends; i++ {
						startBackendServer(service)
					}
				}
			}
		}
	}

}

func backendServerHealthCheck(bIndex int, service *Service, wg *sync.WaitGroup) {
	defer wg.Done()
	if service.Backends[bIndex].unHealthyCount >= service.UnHealthyThreshold {
		fmt.Printf("\n\nBackend server on Port %d is unhealthy\n", service.Backends[bIndex].Port)
		//stop the container
		stopBackendServer(service.Backends[bIndex])
		service.Backends[bIndex] = getNewBackendServer(service)
		_, err := runBackendServer(service.Backends[bIndex], service)
		go callLoadBalancerServiceUpdateEndpoints(service)
		if err != nil {
			fmt.Println(err)
		}
		service.Backends[bIndex].unHealthyCount = 0
		return
	}
	success := backendServerHealthEndpointCall(service.Backends[bIndex], service)
	if success {
		if service.Backends[bIndex].IsHealthy == false {
			db.Model(service.Backends[bIndex]).Update("is_healthy", true)
			go callLoadBalancerServiceUpdateEndpoints(service)
		}
		if service.Backends[bIndex].unHealthyCount > 0 {
			service.Backends[bIndex].unHealthyCount--
		}
	} else {
		db.Model(service.Backends[bIndex]).Update("is_healthy", false)
		service.Backends[bIndex].unHealthyCount++
		go callLoadBalancerServiceUpdateEndpoints(service)
	}
}

func loadBalancerServerHealthCheck(lbIndex int, service *Service, c chan float64, wg *sync.WaitGroup) {
	defer wg.Done()
	if service.LoadBalancers[lbIndex].unHealthyCount >= 2 {
		fmt.Printf("\n\nLoad Balancer server on Port %d is unhealthy\n", service.LoadBalancers[lbIndex].Port)
		//stop the container
		stopLoadBalancerServer(service.LoadBalancers[lbIndex])
		service.LoadBalancers[lbIndex] = getNewLoadBalancer(service)
		_, err := runLoadBalancerServer(service.LoadBalancers[lbIndex], service)
		if err != nil {
			fmt.Println(err)
		}
		service.LoadBalancers[lbIndex].unHealthyCount = 0
		c <- 0.0
		return
	}
	lbReqRate, success := loadBalancerServerHealthEndpointCall(service.LoadBalancers[lbIndex].HealthPort)
	if success {
		if service.LoadBalancers[lbIndex].IsHealthy == false {
			db.Model(service.LoadBalancers[lbIndex]).Update("is_healthy", true)
		}
		if service.LoadBalancers[lbIndex].unHealthyCount > 0 {
			service.LoadBalancers[lbIndex].unHealthyCount = 0
		}
		c <- lbReqRate
		return

	} else {
		db.Model(service.LoadBalancers[lbIndex]).Update("is_healthy", false)
		service.LoadBalancers[lbIndex].unHealthyCount++
	}
	c <- 0.0
	return
}

func loadBalancerServerHealthEndpointCall(port int) (float64, bool) {
	httpClient := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := httpClient.Get(fmt.Sprint("http://localhost:", port, "/lb-health"))
	if err != nil {
		fmt.Println("Error:", err)
		return 0.0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0.0, false
	}

	dat, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0.0, false
	}

	var reqRate float64
	err = json.Unmarshal(dat, &reqRate)
	if err != nil {
		return 0.0, false
	}
	return reqRate, true
}

func backendServerHealthEndpointCall(backend *BackendServer, service *Service) bool {
	httpClient := http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := httpClient.Get(fmt.Sprint("http://localhost:", backend.Port, service.HealthEndpoint))
	if err != nil {
		fmt.Println("Error:", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	return true
}

func callLoadBalancerServiceUpdateEndpoints(service *Service) {
	for _, lb := range service.LoadBalancers {
		if lb.IsHealthy {
			go loadBalancerServiceUpdateEndpointCall(lb.HealthPort)
		}
	}
}
func loadBalancerServiceUpdateEndpointCall(port int) {
	httpClient := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := httpClient.Get(fmt.Sprint("http://localhost:", port, "/service-update"))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if resp.StatusCode != 200 {
		fmt.Println("SERVICE UPDATE STATUS CODE NOT 200", "PORT:", port, "STATUS CODE:", resp.StatusCode)
	}
}
