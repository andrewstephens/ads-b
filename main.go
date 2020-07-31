package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
)

const (
	NZ = 15
)

type BinaryString []rune

type AircraftIdent struct {
	tc int64
	ec int64
	ident string
}

func main() {
	//hexstringExample := "8D4840D6202CC371C32CE0576098"

	testData := loadTestData()

	var aircraftPositionBuffer []AirbornePosition

	DataLoop:
	for _, d := range testData {
		binString := hs2bs(d)

		bsr := []rune(binString)
		df := extractBits(bsr, 1, 5)     // Downlink Format
		ca := extractBits(bsr, 6, 8)     // Capability
		icao := extractBits(bsr, 9, 32)  // ICAO Aircraft Address
		data := extractBits(bsr, 33, 88) // Data
		tc := extractBits(bsr, 33, 37)   // Type Code
		pi := extractBits(bsr, 89, 112)  // Parity / Interrogator ID

		if bin2int(df) != 17 && bin2int(df) != 18 {
			log.Fatal("Not a correct aircraft ID downlink format.")
		}

		// If Aircraft position type code(s)
		if bin2int(tc) >= 9 && bin2int(tc) <= 18 {
			pos := parseAirbornePosition(bsr)
			// TODO: NEED TO ADD LOGIC FOR ORDER IN WHICH DATA COMES IN (FLAG IS EVEN OR ODD?)
			if len(aircraftPositionBuffer) == 0 {
				aircraftPositionBuffer = append(aircraftPositionBuffer, pos)
			} else if len(aircraftPositionBuffer) == 1 {
				ac := aircraftPositionBuffer[0]
				if ac.tc == tc && ac.icao == icao && ac.df == df && pos.cprFlg == 1 {
					aircraftPositionBuffer = append(aircraftPositionBuffer, pos)

					// Globally Unambiguous position
					latitude, err := calculateLatitude(pos, ac)
					if err != nil {
						continue DataLoop
					}
					//longitude := calculateLongitude(pos, ac)

					fmt.Println("Latitude: ", latitude)
					fmt.Println("Woo!")
				}
			} else if len(aircraftPositionBuffer) == 2 {
				aircraftPositionBuffer = []AirbornePosition{}
			}
		}

		icaoLookup := airplaneLookup(bin2hex(icao))
		aircraftIdent := aircraftIdent(data)

		fmt.Println("Downlink Format: ", bin2int(df))
		fmt.Println("Capability: ", bin2int(ca))
		fmt.Println("Flight Ident: ", aircraftIdent)
		fmt.Println("ICAO Hex: ", bin2hex(icao))
		fmt.Println("ICAO: ", icaoLookup)
		fmt.Println("Data: ", data)
		tcInt := bin2int(tc)
		fmt.Println("TC: ", tcInt, typeCodeLookup(tcInt))
		fmt.Println("PI: ", pi)
	}
}

func calculateLatitude(ac, pos AirbornePosition) (float64, error) {
	cprLatEven := float64(bin2int(ac.latCPR)) / float64(131072)
	cprLatOdd := float64(bin2int(pos.latCPR)) / float64(131072)

	// Calculate Latitude Index "j"
	j := math.Floor((58 * cprLatEven) - (60 * cprLatOdd) + float64(1) / float64(2))

	// Calculate Latitude
	dLatEven := float64(360) / float64(4 * NZ)
	dLatOdd := float64(360) / float64(4 * NZ - 1)
	latEven := dLatEven * (math.Mod(j, float64(60)) + cprLatEven)
	latOdd := dLatOdd * (math.Mod(j, float64(59)) + cprLatOdd)

	if latEven >= 270 {
		latEven = latEven - 360
	}

	if latOdd >= 270 {
		latOdd = latOdd - 360
	}

	// Compute NL(latE) and NL(latOdd)
	// if not the same, the two positions are located at different latitude zones.
	// Exit this calculation and wait for more next messages.
	nlLatEven := nl(latEven)
	nlLatOdd := nl(latOdd)
	if nlLatEven != nlLatOdd {
		return 0, errors.New("the two positions are not in same latitude zones")
	}

	var latitude float64
	if pos.time >= ac.time {
		latitude = latEven
	} else {
		latitude = latOdd
	}

	return latitude, nil
}

func calculateLongitude(ac, pos AirbornePosition) (float64, error) {
	cprLatEven := float64(bin2int(ac.latCPR)) / float64(131072)
	cprLatOdd := float64(bin2int(pos.latCPR)) / float64(131072)

	// Calculate Latitude Index "j"
	j := math.Floor((58 * cprLatEven) - (60 * cprLatOdd) + float64(1) / float64(2))

	// Calculate Latitude
	dLatEven := float64(360) / float64(4 * NZ)
	dLatOdd := float64(360) / float64(4 * NZ - 1)
	latEven := dLatEven * (math.Mod(j, float64(60)) + cprLatEven)
	latOdd := dLatOdd * (math.Mod(j, float64(59)) + cprLatOdd)

	if latEven >= 270 {
		latEven = latEven - 360
	}

	if latOdd >= 270 {
		latOdd = latOdd - 360
	}

	// Compute NL(latE) and NL(latOdd)
	// if not the same, the two positions are located at different latitude zones.
	// Exit this calculation and wait for more next messages.
	nlLatEven := nl(latEven)
	nlLatOdd := nl(latOdd)
	if nlLatEven != nlLatOdd {
		return 0, errors.New("the two positions are not in same latitude zones")
	}

	var latitude float64
	if pos.time >= ac.time {
		latitude = latEven
	} else {
		latitude = latOdd
	}

	return latitude, nil
}

func aircraftIdent(data string) AircraftIdent {
	alphaNums := "#ABCDEFGHIJKLMNOPQRSTUVWXYZ#####_###############0123456789######"
	alphaNumericLookup := strings.Split(alphaNums, "")
	fmt.Println(alphaNumericLookup)

	bsr := []rune(data)
	tc := bin2int(extractBits(bsr, 1, 5))
	ec := bin2int(extractBits(bsr, 5, 8))

	var ident []string
	var i int64
	for i = 8; i < 56; i = i + 6 {
		bits := extractBits(bsr, i+1, i+6)
		indx := bin2int(bits)
		ident = append(ident, alphaNumericLookup[indx])
	}

	identStr := strings.Join(ident, "")

	return AircraftIdent{
		tc: tc,
		ec: ec,
		ident: identStr,
	}

}

func airplaneLookup(icao string) string {
	in, err := os.Open("data/aircraft_db.csv")
	if err != nil {
		log.Fatal("Whoops")
	}

	defer in.Close()

	r := csv.NewReader(in)

	for {
		row, err := r.Read()
		if err == io.EOF {
			return "End of file"
		}

		if row[0] == icao {
			return row[3]
		} else {
			continue
		}
	}
}

func checksum(bs string) (int, error) {

	if len(bs) < 1 {
		return 1, errors.New("binary string is empty")
	}

	g := []int{
		0b11111111, 0b11111010, 0b00000100, 0b10000000,
	}

	var bitSlices []int
	for i := range bs {
		if (i % 8) == 0 {
			byteString, err := strconv.ParseUint(bs[i:i+8], 2, 16)
			if err != nil {
				log.Fatal("Not able to convert bit string to int")
			}

			bitSlices = append(bitSlices, int(byteString))
		}
	}

	bitSliceLen := len(bitSlices) - 3

	for i := 0; i < bitSliceLen; i++ {
		for j := 0; j < 8; j++ {

			mask := 0x80 >> j
			bits := bitSlices[i] & mask

			if bits > 0 {
				opt1 := bitSlices[i] ^ (g[0] >> j)
				bitSlices[i] = opt1

				opt2 := bitSlices[i+1] ^ (0xFF & ((g[0] << (8 - j)) | (g[1] >> j)))
				bitSlices[i+1] = opt2

				opt3 := bitSlices[i+2] ^ (0xFF & ((g[1] << (8 - j)) | (g[2] >> j)))
				bitSlices[i+2] = opt3

				opt4 := bitSlices[i+3] ^ (0xFF & ((g[2] << (8 - j)) | (g[3] >> j)))
				bitSlices[i+3] = opt4
			}
		}
	}

	result := (bitSlices[len(bitSlices)-3] << 16) | (bitSlices[len(bitSlices)-2] << 8) | bitSlices[len(bitSlices)-1]

	return result, nil

}

func extractBits(bs BinaryString, start, end int64) string {
	return string(bs[start-1 : end])
}

func bin2int(bin string) int64 {
	result, err := strconv.ParseInt(bin, 2, 64)
	if err != nil {
		log.Println(err)
		log.Fatal("Failed to convert binary string to integer.")
	}

	return result
}

func bin2hex(bin string) string {
	return fmt.Sprintf("%x", bin2int(bin))
}

func hs2bs(hs string) string {
	if (len(hs) % 2) != 0 {
		log.Fatal("Hex string must be a multiple of 2.")
	}

	var binSlice []string
	hsSplit := strings.Split(hs, "")
	for i := range hsSplit {
		if (i % 2) != 0 {
			continue
		}

		hexCombined := hsSplit[i] + hsSplit[i+1]

		bin, err := strconv.ParseUint(hexCombined, 16, 64)
		if err != nil {
			log.Fatal("Couldn't parse hex string.")
		}

		binSlice = append(binSlice, fmt.Sprintf("%08b", bin))
	}

	return strings.Join(binSlice, "")
}

func typeCodeLookup(tc int64) string {
	switch {
	case tc >= 1 && tc <= 4:
		return "Aircraft Identification"
	case tc >= 5 && tc <= 8:
		return "Surface Position"
	case tc >= 9 && tc <= 18:
		return "Airborne Position (w/ Baro Altitude)"
	case tc == 19:
		return "Airborne Velocities"
	case tc >= 20 && tc <= 22:
		return "Airborne Position (w/ GNSS Height)"
	case tc >= 23 && tc <= 27:
		return "Reserved"
	case tc == 28:
		return "Aircraft Status"
	case tc == 29:
		return "Target State and Status Information"
	case tc == 31:
		return "Aircraft Operation Status"
	default:
		return "Invalid Code"
	}
}

func loadTestData() []string {
	var data []string

	in, err := os.Open("data/sample_data_adsb.csv")
	if err != nil {
		log.Fatal("Whoops")
	}

	defer in.Close()

	r := csv.NewReader(in)

	i := 0
	for {
		if i == 0 {
			i++
			continue
		}
		row, err := r.Read()
		if err == io.EOF {
			break
		}

		data = append(data, row[1])
	}

	return data
}

type AirbornePosition struct {
	df string			// Downlink Format
	icao string			// ICAO ident
	tc string  			// Type code
	ss string 			// Surveillance status
	nicsb string 		// NIC Supplement-B
	alt string 			// Altitude
	time string 		// Time
	cprFlg int64		// CPR odd/even frame flag
	latCPR string		// Latitude in CPR Format
	lonCPR string		// Longitude in CPR Format
}

// parseAirbornePosition
func parseAirbornePosition(data []rune) AirbornePosition {
	df := extractBits(data, 1, 5)
	icao := extractBits(data, 9, 32)
	tc := extractBits(data, 33, 37)
	ss := extractBits(data, 38, 39)
	nicsb := extractBits(data, 40, 40)
	alt := extractBits(data, 41, 52)
	time := extractBits(data, 53, 53)
	cprFlg := bin2int(extractBits(data, 54, 54))
	latCPR := extractBits(data, 55, 71)
	lonCPR := extractBits(data, 72, 88)

	return AirbornePosition{
		df: df,
		icao: icao,
		tc: tc,
		ss: ss,
		nicsb: nicsb,
		alt: alt,
		time: time,
		cprFlg: cprFlg,
		latCPR: latCPR,
		lonCPR: lonCPR,
	}
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