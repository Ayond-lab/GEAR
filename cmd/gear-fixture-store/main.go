package main

import (
	"log"

	"gear/internal/component"
)

func main() {
	log.Fatal(component.Run("gear-fixture-store"))
}

