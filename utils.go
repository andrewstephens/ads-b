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

func nl(lat float64) float64 {
	if lat == 0 {
		return 59
	}

	if lat > 87 || lat < -87 {
		return 1
	}

	pi2 := math.Pi * 2
	cosdiv := (1 - math.Cos(math.Pi/(2*NZ))) / math.Pow(math.Cos((math.Pi/180)*lat), 2)
	final := math.Floor(pi2 / math.Acos(1-cosdiv))
	return final
}