package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(log.Lmicroseconds)
	retCode := main_isolate()
	log.Printf("RETURN CODE = %d", retCode)
	os.Exit(retCode)
}
