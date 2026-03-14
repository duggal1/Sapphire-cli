//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"math/rand"
)

func broken() int {
	var x int = 10 // Fixed type mismatch
	y := 10
	{
		z := 20 // Fixed shadowing
		fmt.Println(z)
	}
	
	// Fixed missing import
	r := rand.Intn(10)
	fmt.Println(r)

	return 0 // Fixed return type
}
