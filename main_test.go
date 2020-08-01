package main

import (
	"fmt"
	"testing"
)

func TestParsing(t *testing.T) {

	var msg0 Message
	var msg1 Message
	var t0 int64
	var t1 int64

	for _, df := range loadTestData() {
		msg := parseDataFrame(df)

		// Odd/Even Flag Check
		oeFlag := oeFlagCheck(msg.data)
		if oeFlag == 1 {
			msg1 = msg
			t1 = df.ts
		} else {
			msg0 = msg
			t0 = df.ts
		}

		// Aircraft Type
		at := airplaneLookup(msg.icao)

		// Aircraft Ident
		ai := aircraftIdent(msg.data).ident

		// Aircraft Position (LAT/LON)
		if msg0 != (Message{}) && msg1 != (Message{}) {
			if msg0.icao == msg1.icao && msg0.df == msg1.df && msg0.tc == msg1.tc {
				msg0PosData := parseAirbornePosition([]rune(msg0.binstring))
				msg1PosData := parseAirbornePosition([]rune(msg1.binstring))
				lat, lng := parseLatLng(msg0PosData, msg1PosData, t0, t1)
				fmt.Println(lat, lng)
			}
		}

		// Aircraft Heading
		// Aircraft Elevation

		fmt.Println(oeFlag)
		fmt.Println(at)
		fmt.Println(ai)
		fmt.Println("")
	}
}
