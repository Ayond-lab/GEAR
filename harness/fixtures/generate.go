package main

import (
	"encoding/json"
	"fmt"
	"os"

	"gear/internal/cvdemo"
)

func main() {
	applications := generate()
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(applications); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() []cvdemo.Application {
	return cvdemo.GenerateApplications()
}
