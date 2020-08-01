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

type AircraftIdent struct {
	tc int64
	ec int64
	ident string
}
type DataFrame struct {
	ts int64
	data string
}

type Message struct {
	binstring string
	df int64
	ca int64
	icao string
	data string
	tc int64
	pi int64
}

type Altitude struct {
	alt int64
	accuracy int32
}

type Aircraft struct {
	aircraftType string
	ident string
	heading string
	pos string
	elevation int64
}

func (m Message) verifyChecksum() bool {
	result, err := checksum(m.binstring)
	if err != nil {
		log.Fatal("Couldn't verify checksum")
	}

	return result == 0
}

func parseDataFrame(df DataFrame) Message {
	ts := df.ts
	bs := []rune(hs2bs(df.data))

	fmt.Println(ts)
	fmt.Println(bs)

	dlFmt := extractBits(bs, 1, 5)     // Downlink Format
	ca := extractBits(bs, 6, 8)     // Capability
	icao := extractBits(bs, 9, 32)  // ICAO Aircraft Address
	data := extractBits(bs, 33, 88) // Data
	tc := extractBits(bs, 33, 37)   // Type Code
	pi := extractBits(bs, 89, 112)  // Parity / Interrogator ID

	msg := Message{
		binstring: hs2bs(df.data),
		df: bin2int(dlFmt),
		ca: bin2int(ca),
		icao: bin2hex(icao),
		data: data,
		tc: bin2int(tc),
		pi: bin2int(pi),
	}

	if !msg.verifyChecksum() {
		return Message{}
	}

	return msg
}

func main() {
	testData := loadTestData()

	var aircraftPositionBuffer []AircraftPosition

	DataLoop:
	for _, d := range testData {
		binString := hs2bs(d.data)
	// TODO: Change out logic to take struct of data with timestamp, then parse positions
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
			if len(aircraftPositionBuffer) == 0 {
				aircraftPositionBuffer = append(aircraftPositionBuffer, pos)
			} else if len(aircraftPositionBuffer) == 1 {
				ac := aircraftPositionBuffer[0]
				var eoFlag int64
				if ac.cprFlg == 0 {
					eoFlag = 1
				} else {
					eoFlag = 0
				}

				if ac.tc == tc && ac.icao == icao && ac.df == df && pos.cprFlg == eoFlag {
					aircraftPositionBuffer = append(aircraftPositionBuffer, pos)

					// Globally Unambiguous position
					latitude, err := calculateLatitude(pos, ac)
					if err != nil {
						continue DataLoop
					}

					longitude, err := calculateLongitude(latitude, aircraftPositionBuffer)
					if err != nil {

					}


					fmt.Println("Latitude: ", latitude)
					fmt.Println("Latitude: ", longitude)
					//fmt.Println("Latitude: ", longitude)
					fmt.Println("Woo!")
				}
			} else if len(aircraftPositionBuffer) == 2 {
				aircraftPositionBuffer = []AircraftPosition{}
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

type LatitudeData struct {
	latEven float64
	latOdd float64
	lat float64
}


// parseAirbornePosition
func parseAirbornePosition(data []rune) AircraftPosition {
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

	return AircraftPosition{
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

func calculateLatitude(ac, pos AircraftPosition) (LatitudeData, error) {
	// TODO: Make sure it's actually even and odd (0 == even, 1==odd)
	var even AircraftPosition
	var odd AircraftPosition

	if ac.cprFlg == 0 {
		even = ac
		odd = pos
	} else {
		even = pos
		odd = ac
	}

	cprLatEven := float64(bin2int(even.latCPR)) / float64(131072)
	cprLatOdd := float64(bin2int(odd.latCPR)) / float64(131072)

	// Calculate Latitude Index "j"
	j := math.Floor((59 * cprLatEven) - (60 * cprLatOdd) + float64(1) / float64(2))

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
		return LatitudeData{}, errors.New("the two positions are not in same latitude zones")
	}

	var latitude float64
	if bin2int(pos.time) >= bin2int(ac.time) {
		latitude = latEven
	} else {
		latitude = latOdd
	}

	latitudeData := LatitudeData{
		latEven: latEven,
		latOdd: latOdd,
		lat: latitude,
	}

	return latitudeData, nil
}

func parseLatLng(msg0 AircraftPosition, msg1 AircraftPosition, ts0 int64, ts1 int64) (float64, float64) {

	if msg0.cprFlg == 0 && msg1.cprFlg == 1 {
		//
	} else if msg0.cprFlg == 1 && msg1.cprFlg == 0 {
		msg0, msg1 = msg1, msg0
		ts0, ts1 = ts1, ts0
	} else {
		log.Fatal("Both even and odd CPR frames are required.")
	}

	cprLatEven := float64(bin2int(msg0.latCPR)) / float64(131072)
	cprLatOdd := float64(bin2int(msg1.latCPR)) / float64(131072)
	cprLonEven := float64(bin2int(msg0.lonCPR)) / float64(131072)
	cprLonOdd := float64(bin2int(msg1.lonCPR)) / float64(131072)

	// Calculate Latitude Index "j"
	j := floor((59 * cprLatEven) - (60 * cprLatOdd) + float64(1) / float64(2))

	// Calculate Latitude
	dLatEven := float64(360) / float64(4 * NZ)
	dLatOdd := float64(360) / float64((4 * NZ) - 1)
	latEven := dLatEven * (mod(j, float64(60)) + cprLatEven)
	latOdd := dLatOdd * (mod(j, float64(59)) + cprLatOdd)

	if latEven >= 270 {
		latEven = latEven - 360
	}

	if latOdd >= 270 {
		latOdd = latOdd - 360
	}

	var latitude float64
	if ts0 <= ts1 {
		latitude = latEven
	} else {
		latitude = latOdd
	}

	// Compute NL(latE) and NL(latOdd)
	// if not the same, the two positions are located at different latitude zones.
	// Exit this calculation and wait for more next messages.
	nlLatEven := nl(latEven)
	nlLatOdd := nl(latOdd)
	if nlLatEven != nlLatOdd {
		log.Fatal("the two positions are not in same latitude zones")
	}

	// Calculate Longitude
	var longitude float64

	if ts0 <= ts1 {
		nlResult := nl(latEven)
		ni := math.Max(nlResult, 1)
		dLon := float64(360) / ni
		a := cprLonEven * (nlResult - 1) - (cprLonOdd * nlResult) + 0.5
		m := floor(a)
		longitude = dLon * (mod(m, ni) + cprLonEven)
	} else {
		nlResult := nl(latOdd)
		ni := math.Max(nlResult, 1)
		dLon := float64(360) / ni
		a := cprLonEven * (nlResult - 1) - (cprLonOdd * nlResult) + 0.5
		m := floor(a)
		longitude = dLon * (mod(m, ni) + cprLonOdd)
	}

	if longitude > 180 {
		longitude = longitude - 360
	}

	// TODO: Implement Float truncation for lat/lon
	return latitude, longitude
}

func calculateLongitude(latData LatitudeData, ap []AircraftPosition) (float64, error) {
	fmt.Println(latData)
	fmt.Println(ap)

	var even AircraftPosition
	var odd AircraftPosition
	if ap[0].cprFlg == 0 {
		even = ap[0]
		odd = ap[1]
	} else {
		even = ap[1]
		odd = ap[0]
	}

	cprLonEven := float64(bin2int(even.lonCPR)) / float64(131072)
	cprLonOdd := float64(bin2int(odd.lonCPR)) / float64(131072)

	var longitude float64

	if even.time > odd.time {
		ni := math.Max(nl(latData.latEven), 1)
		dLon := float64(360) / ni
		m := math.Floor(cprLonEven * (nl(latData.latEven) - 1) - cprLonOdd * (nl(latData.latEven) + (float64(1) / float64(2))))
		longitude = dLon * (math.Mod(m, ni) + cprLonEven)
	} else {
		ni := math.Max(nl(latData.latOdd), 1)
		dLon := float64(360) / ni
		m := math.Floor(cprLonEven * (nl(latData.latOdd) - 1) - cprLonOdd * (nl(latData.latOdd) + (float64(1) / float64(2))))
		longitude = dLon * (math.Mod(m, ni) + cprLonOdd)
	}

	if longitude >= 180 {
		longitude = longitude - 360
	}

	fmt.Println(longitude)

	return 0, nil
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

func aircraftAltitude(data string) Altitude {
	ds := []rune(data)
	qbit := bin2int(extractBits(ds, 48, 48))
	n := extractBits(ds, 41, 47) + extractBits(ds, 49, 52)
	var ftMultiple int32
	if qbit == 0 {
		ftMultiple = 100
	} else {
		ftMultiple = 25
	}

	alt := bin2int(n) * 25 - 1000

	return Altitude{
		alt: alt,
		accuracy: ftMultiple,
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

func extractBits(bs []rune, start, end int64) string {
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

func loadTestData() []DataFrame {
	var data []DataFrame

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

		ts, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			log.Fatal("Can't parse timestamp")
		}

		df := DataFrame{
			ts: ts,
			data: row[1],
		}

		data = append(data, df)
	}

	ex1 := DataFrame{
		ts: 1596231450,
		data: "8D40621D58C382D690C8AC2863A7",
	}
	ex2 := DataFrame{
		ts: 1596231481,
		data: "8D40621D58C386435CC412692AD6",
	}

	data = []DataFrame{ex1, ex2}

	return data
}

type AircraftPosition struct {
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


func nl(lat float64) float64 {
	if lat == 0 {
		return 59
	}

	if 87 == math.Abs(lat) {
		return 2
	}

	if lat > 87 || lat < -87 {
		return 1
	}

	a := 1 - math.Cos(math.Pi / (2 * NZ))
	b := math.Pow(math.Cos((math.Pi / 180) * lat), 2)
	final := floor((2 * math.Pi) / (math.Acos(1 - (a / b))))
	return final
}

func floor(x float64) float64 {
	return math.Round(math.Floor(x))
}

func mod(x, y float64) float64 {
	if y == 0 {
		log.Fatal("Y can't be zero.")
	}
	result := x - y * math.Floor(x/y)
	return result
}

func oeFlagCheck(bs string) int64 {
	return bin2int(extractBits([]rune(bs), 54, 54))
}