package main

import (
	"errors"
	"fmt"
)

var LoadBalancerContainerImageName = "load-balancer-server:latest"

func runLoadBalancerServer(lb *LoadBalancerServer, service *Service) (bool, error) {
	_, err := runCommand("start load balancer server", fmt.Sprintf("docker run -e CONTAINER_NAME=%s -e SERVICE_NAME=%s -d -p %d:4000 -p %d:3210  --network load-balancer-network --name %s %s", lb.ContainerName, service.Name, lb.Port, lb.HealthPort, lb.ContainerName, LoadBalancerContainerImageName))
	if err != nil {
		return false, errors.New("error running docker container")
	} else {
		db.Save(lb)
	}
	return true, nil
}

func stopLoadBalancerServer(lb *LoadBalancerServer) {
	_, errStop := runCommand("stop load balancer server", fmt.Sprintf("docker stop %s", lb.ContainerName))
	if errStop != nil {
		fmt.Println(errStop)
	}

	_, errDel := runCommand("delete load balancer server", fmt.Sprintf("docker rm %s", lb.ContainerName))
	if errDel != nil {
		fmt.Println(errDel)
	}
	db.Delete(&LoadBalancerServer{}, "id = ?", lb.ID)
}

func stopAllLoadBalancerServer() {
	_, errStop := runCommand("stop all load balancer servers", fmt.Sprintf("docker stop $(docker ps -q --filter ancestor=%s)", LoadBalancerContainerImageName))
	if errStop != nil {
		fmt.Println(errStop)
	}

	_, errDel := runCommand("delete all load balancer servers", fmt.Sprintf("docker rm $(docker ps -aq --filter ancestor=%s)", LoadBalancerContainerImageName))
	if errDel != nil {
		fmt.Println(errDel)
	}
	db.Delete(&LoadBalancerServer{}, "id > 0")

}

func stopAllServiceLoadBalancerServer(service *Service) {
	_, errStop := runCommand("stop all load balancer servers", fmt.Sprintf("docker stop $(docker ps --format '{{.Names}}' | grep '^lb-%s-load-balancer')", service.Name))
	if errStop != nil {
		fmt.Println(errStop)
	}

	_, errDel := runCommand("delete all load balancer servers", fmt.Sprintf("docker rm $(docker ps -a --format '{{.Names}}' | grep '^lb-%s-load-balancer')", service.Name))
	if errDel != nil {
		fmt.Println(errDel)
	}
	db.Delete(&LoadBalancerServer{}, "service_id = ?", service.ID)

}
