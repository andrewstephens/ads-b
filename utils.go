package main

import (
	"log"
	"math"
)

func floor(x float64) float64 {
	return math.Round(math.Floor(x))
}

func mod(x, y float64) float64 {
	if y == 0 {
		log.Fatal("Y can't be zero.")
	}
	return x - y * math.Floor(x/y)
}