package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func apis() {
	router := gin.Default()
	apis := router.Group("/api")
	{
		apis.POST("/service", func(context *gin.Context) {
			createService(context)
		})
		apis.GET("/service", func(context *gin.Context) {
			getAllServices(context)
		})
		apis.GET("/service/:id", func(context *gin.Context) {
			getService(context)
		})
		apis.PATCH("/service/:id", func(context *gin.Context) {
			updateService(context)
		})
		apis.DELETE("/service/:id", func(context *gin.Context) {
			deleteService(context)
		})
		apis.GET("/service/:id/load-balancers", func(context *gin.Context) {
			getServiceLoadBalancers(context)
		})
	}
	router.Run(":3000")
}

func createService(c *gin.Context) {
	var service Service

	if err := c.BindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	err := db.Save(&service).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "service created",
	})

	go reloadServices()
}

func getAllServices(c *gin.Context) {
	var services []Service
	db.Preload("Backends").Find(&services)
	c.JSON(http.StatusOK, services)
}

func getService(c *gin.Context) {
	id := c.Param("id")
	var service Service
	err := db.Preload("Backends").First(&service, id).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Service not found",
		})
		return
	}
	c.JSON(http.StatusOK, service)
}

func updateService(c *gin.Context) {
	idStr := c.Param("id")

	var findService Service
	err := db.First(&findService, idStr).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Service not found",
		})
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	var service Service

	if err := c.BindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	service.ID = uint(id)

	err = db.Save(&service).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "service updated",
	})
	fmt.Println("Service updated")

	go reloadServices()
}

func deleteService(c *gin.Context) {
	id := c.Param("id")
	var findService Service
	err := db.First(&findService, id).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Service not found",
		})
		return
	}
	err = db.Delete(&Service{}, id).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "service deleted",
	})

	go reloadServices()
}

func getServiceLoadBalancers(c *gin.Context) {
	id := c.Param("id")
	//find in service with id in services list
	for _, service := range services {
		if strconv.Itoa(int(service.ID)) == id {
			healthyLoadBalancers := make([]*LoadBalancerServer, 0)
			for _, lb := range service.LoadBalancers {
				if lb.IsHealthy {
					healthyLoadBalancers = append(healthyLoadBalancers, lb)
				}

			}
			c.JSON(http.StatusOK, healthyLoadBalancers)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": "Service not found",
	})
}
