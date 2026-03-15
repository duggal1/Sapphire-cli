package main

import (
	"fmt"
	"log"
	"os"

	"gosloc-demo/internal/counter"
)

func main() {
	target := "."
	if len(os.Args) > 1 {
		target = os.Args[1]
	}

	fmt.Printf("Analyzing directory: %s\n", target)
	res, err := counter.CountLines(target)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("-------------------------------")
	fmt.Printf("Go Files Found:  %d\n", res.GoFiles)
	fmt.Printf("Total Lines:     %d\n", res.TotalLines)
	fmt.Printf("Average Lines:   %.2f\n", res.AverageLines)
	fmt.Println("-------------------------------")
}
