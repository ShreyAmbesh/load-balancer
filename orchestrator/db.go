package main

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func getDb() *gorm.DB {
	dsn := "host=localhost user=shreyambesh password=postgres dbname=load_balancer port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	fmt.Println("Connected to database")
	//Migrate the schema
	err = db.AutoMigrate(&Service{}, &BackendServer{}, &LoadBalancerServer{})
	if err != nil {
		fmt.Println("Error migrating schema", err)
	}
	fmt.Println("Migration completed")
	return db
}
