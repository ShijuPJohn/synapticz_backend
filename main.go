package main

import (
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"log"
)

func main() {
	err := util.DBConnectAndPopulateDBVar()
	if err != nil {
		fmt.Println(err.Error())
		log.Fatal("couldn't connect to the database")
	} else {
		fmt.Println("Connected to the database")
	}
	if err = util.CreateTableIfNotExists(); err != nil {
		log.Fatal("Couldn't create tables", err.Error())
	}
	fmt.Println("Tables Created")
}
