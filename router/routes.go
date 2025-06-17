package router

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	hex "hexbot/gen"
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

// dimensions sanity check - ensure width and height get configured as a positive int of 1 or higher
// giving width & height values of 0 or < get properly converted to some positive number
func dimSanityCheck (value int) int {
	if value < 0 {
		return int(math.Abs(float64(value)))
	} else if value == 0 {
		return 1
	}
	return value
}

func GenerateAsync(hexResp *HexbotResponse, count int, w int, h int, seedList []string) {
	var wg sync.WaitGroup


	chunkSize := count / NUM_PROCS
	chunkRemainder := count % NUM_PROCS

	ch := make(chan Chunk)
	// create a goroutine for each core on the machine
	for i := 1; i <= NUM_PROCS; i++ {
		// defer cancelFunc()
		if i == NUM_PROCS {
			chunkSize += chunkRemainder
		}
		wg.Add(1)
		go func (count uint, w int, h int, waitGroup *sync.WaitGroup) {
			ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second * 4)
			defer cancelFunc()
			defer waitGroup.Done()
			defer func() {
				if err := recover(); err != nil {
					log.Println("Error occurred in a color generation goroutine: ", err)
				}
			}()
				// log.Println("CREATING CHUNK WITH COUNT: ", count, i)
			data, err := hex.GenerateNTimes(ctx, hex.WithCount(count), hex.WithDim(w, h), hex.WithClrSeed(seedList))
			select {
			case ch <-Chunk{Result: data, Error: err}:
				log.Println("Successfully got a chunk")
			case <-ctx.Done():
				log.Println("Experienced Timeout!")

			}
		}(uint(chunkSize), w, h, &wg)
	}


	go func() {
		wg.Wait()
		close(ch)
    }()

    // Collect results
    for chunk := range ch {
        hexResp.Colors = append(hexResp.Colors, chunk.Result...)
        if chunk.Error != nil {
            log.Printf("Error generating chunk: %v", chunk.Error)
        }
    }

}

func GenerateSync(hexResp *HexbotResponse, count int, w int, h int, clrSeeds []string) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("Recovered in goroutine: %v", err)
		}
	}()
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
	var wAsInt int = 1
	var hAsInt int = 1
	
	if (hExists && wExists)  {
		wAsInt, _ = strconv.Atoi(possWidth[0])
		hAsInt, _ = strconv.Atoi(possHeight[0])
		wAsInt = dimSanityCheck(wAsInt)
		hAsInt = dimSanityCheck(hAsInt)
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
