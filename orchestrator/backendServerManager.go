package main

import (
	"errors"
	"fmt"
	"strings"
)

func runBackendServer(backend *BackendServer, service *Service) (bool, error) {
	_, err := runCommand("start backend server", fmt.Sprintf("docker run -e CONTAINER_NAME=%s -d -p %d:%d --network load-balancer-network --name %s %s", backend.ContainerName, backend.Port, service.ContainerPort, backend.ContainerName, service.ContainerImageName))
	if err != nil {
		return false, errors.New("error running docker container")
	} else {
		db.Save(backend)
	}
	return true, nil
}

func stopBackendServer(backend *BackendServer) {
	_, errStop := runCommand("stop backend server", fmt.Sprintf("docker stop %s", backend.ContainerName))
	if errStop != nil {
		fmt.Println(errStop)
	}

	_, errDel := runCommand("delete backend server", fmt.Sprintf("docker rm %s", backend.ContainerName))
	if errDel != nil {
		fmt.Println(errDel)
	}
	db.Delete(&BackendServer{}, "id = ?", backend.ID)
}

func stopAllBackendServer(service *Service) {
	_, errStop := runCommand("stop all backend servers", fmt.Sprintf("docker stop $(docker ps --format '{{.Names}}' | grep '^lb-%s-%s')", service.Name, strings.Split(service.ContainerImageName, ":")[0]))
	if errStop != nil {
		fmt.Println(errStop)
	}

	_, errDel := runCommand("delete all backend servers", fmt.Sprintf("docker rm $(docker ps -a --format '{{.Names}}' | grep '^lb-%s-%s')", service.Name, strings.Split(service.ContainerImageName, ":")[0]))
	if errDel != nil {
		fmt.Println(errDel)
	}
	db.Delete(&BackendServer{}, "service_id = ?", service.ID)

}
