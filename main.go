package main

import (
	"fmt"
	"github.com/ShijuPJohn/synapticz_backend/util"
	"log"
)

func main() {
	err := util.DBConnectAndPopulateDBVar()
	if err != nil {
		log.Fatal("couldn't connect to the database")
	} else {
		fmt.Println("Connected to the database")
	}
}
