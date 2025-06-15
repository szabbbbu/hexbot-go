package router

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	
	hex "hexbot/genny"
)

type Chunk struct {
	Result []hex.HexCode
	Error error
}

type HexbotResponse struct {
	Colors []hex.HexCode `json:"colors"`
	Warning *string
}


var (
	NUM_PROCS int = runtime.NumCPU()
)

var validHex = regexp.MustCompile(`(?:[0-9a-fA-F]{3}){1,2}$`)


func GenerateAsync(hexResp *HexbotResponse, count int, w int, h int, seedList []string) {

	chunkSize := count / NUM_PROCS
	chunkRemainder := count % NUM_PROCS

	ch := make(chan Chunk)

	for i := 1; i <= NUM_PROCS; i++ {
		// defer cancelFunc()
		if i == NUM_PROCS {
			chunkSize += chunkRemainder
		}
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second * 10)
		go func (count uint, w int, h int) {
			// log.Println("CREATING CHUNK WITH COUNT: ", count, i)
			defer cancelFunc()
			data, err := hex.GenerateNTimes(ctx, hex.WithCount(count), hex.WithDim(w, h), hex.WithClrSeed(seedList))
			ch <- Chunk{
				Result: data,
				Error: err,
			}
		}(uint(chunkSize), w, h)
	}

	for i := 0; i < NUM_PROCS; i++ {
		res := <-ch
		if (res.Error != nil) {
			log.Printf("Worker %d failed: %v", i, res.Error)
			s := "One or more workers experienced a timeout failure. You might be requesting too many points. Sorry!"
			hexResp.Warning = &s
        	continue
		}
		log.Printf("Worker %d succeeded", i)
    	hexResp.Colors = append(hexResp.Colors, res.Result...)
	}
}

func GenerateSync(hexResp *HexbotResponse, count int, w int, h int, clrSeeds []string) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancelFunc()
	colors, err := hex.GenerateNTimes(ctx, hex.WithCount(uint(count)), hex.WithDim(w, h), hex.WithClrSeed(clrSeeds))

	if (err != nil) {
		s := "Error: the request timed out"
		hexResp.Warning = &s
		hexResp.Colors = nil
	} else {
		hexResp.Colors = colors
	}
}

func ApiHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("REQ RECEIVED: ", r.URL.RequestURI())
	var hexResp HexbotResponse = HexbotResponse{}
	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query()
	possCount := r.URL.Query().Get("count")
	possWidth, wExists := query["width"]
	possHeight, hExists := query["height"]
	seedsRaw, seedsExist := query["seed"]
	
	gz := gzip.NewWriter(w)
	encoder := json.NewEncoder(gz)
	defer gz.Close()
	
	var sepSeedList []string = []string{}
	var wAsInt int = 0
	var hAsInt int = 0
	
	if (hExists && wExists)  {
		wAsInt, _ = strconv.Atoi(possWidth[0])
		hAsInt, _ = strconv.Atoi(possHeight[0])
	}
	
	if (seedsExist) { // parse and remove invalid hex codes
		sepSeedList = strings.Split(seedsRaw[0], ",")
		for i := 0; i < len(sepSeedList); { // remove invalid hex code, add warning to response
			if (!validHex.MatchString(sepSeedList[i])) {
				sepSeedList = append(sepSeedList[:i], sepSeedList[i+1:]...) // removes item at 'i'
			} else {
				i++
			}
		}
	}
		var countAsNum int = 1
		var err error
		if len(possCount) > 0 {
			countAsNum, _ = strconv.Atoi(possCount)
		}
		if (countAsNum < 500) {
			GenerateSync(&hexResp, countAsNum, wAsInt, hAsInt, sepSeedList)
		} else {
			GenerateAsync(&hexResp, countAsNum, wAsInt,hAsInt,sepSeedList)
		}
	w.Header().Set("Content-Encoding", "gzip") // set only if we know we're sending back json

	log.Println("STARTING ENCODE")

	err = encoder.Encode(hexResp) //place json resp in request body

	if err != nil {
		panic(err)
	}

	// fmt.Fprint(w, string(jsonBytes))
	log.Println("RESPONSE SENT!")
}
