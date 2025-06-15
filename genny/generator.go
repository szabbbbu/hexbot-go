package genny

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

var (
	charstore = [16]rune{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F'}
)

type Point struct {
	X int
	Y int
}

type Dim struct {
	W int
	H int
}

// hex code 
type HexCode struct {
	Value string `json:"value"`
	Coord *Point `json:"coordinates,omitempty"`
}

type QueryOptions struct {
	Count uint
	Dimensions *Dim
	SeedValues *[]string
}

type Option func(*QueryOptions)

func WithCount(ct uint) Option {
	return func (o *QueryOptions) {
		o.Count = ct
	}
}

func WithDim(w int, h int) Option {
	return func (o *QueryOptions) {
		if (w <= 1 && h <= 1) {  //keep as nil
			return
		}
		o.Dimensions = &Dim{
			W: w,
			H: h,
		}
	}
}

func WithClrSeed(seedClrs []string) Option {
	return func(o *QueryOptions) {
		if len( seedClrs ) < 1 {
			return
		}
		o.SeedValues = &seedClrs
	} 
}


func GenerateNTimes(ctx context.Context, opts ...Option) ([]HexCode, error) {
	defaultOpts := &QueryOptions{
		Count: uint(1),
		Dimensions: nil,
		SeedValues: nil,
	}
	result := []HexCode{}

	for _, option := range opts {
		option(defaultOpts)
	}

	for i := uint(0); i < defaultOpts.Count; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		newHexCode := generateHexCode(defaultOpts)
		result = append(result, newHexCode)
	}
	return result, nil
}



func generateHexCode(opts *QueryOptions) HexCode {
	h := HexCode{}
	p := Point{}
	var hexResult strings.Builder
	hexResult.WriteRune('#')
	
	if opts.Dimensions != nil {
		p.X = rand.Intn(opts.Dimensions.W)
		p.Y = rand.Intn(opts.Dimensions.H)
		h.Coord = &p
	}
	
	if opts.SeedValues != nil { // convert to rgb, then get a color similar to one of the seeds
		seeds := *opts.SeedValues
		randIdx := rand.Intn(len(seeds))
		takeRandClr := seeds[randIdx]
		r, _ := strconv.ParseInt(takeRandClr[0:2], 16, 0)
		g, _ := strconv.ParseInt(takeRandClr[2:4], 16, 0)
		b, _ := strconv.ParseInt(takeRandClr[4:6], 16, 0)
		seededHex := getSeededColor(float64(r), float64(g), float64(b), 30.0)
		hexResult.WriteString(seededHex)

	} else { // no seed values, make a random hex
		for i := 0; i < 6; i++ {
			randIdx := rand.Intn(len(charstore))
			hexResult.WriteRune(charstore[randIdx])
		}
	}
	h.Value = hexResult.String()
	return h
}


/*
Generates a random color similar to the given RGB values
Converts RGB to HSL (to easily change hue, without affecting other qualities)
Then converts HSL back to a hex code
*/
func getSeededColor(r float64, g float64, b float64, threshold float64) string {
	// convert HSL values back to hex (hsl -> rgb -> hex)
	randOffset := rand.Float64() * (2 * threshold) - threshold

	hslToHex := func (h, s, l float64) string {
		//// get back to RGB format
		// compute chroma
		chroma := (1 - math.Abs((2 * l) - 1)) * s

		// compute hprime and x
		hPrime := h / 60 // which face of the rgb cube we should be in 
		x := chroma * (1 - math.Abs(math.Mod(hPrime, 2) - 1))

		// place x and chroma values based on the calculated 'hPrime' face
		var r1, g1, b1 float64
		switch {
		case hPrime >= 0 && hPrime < 1: // red to yellow face
			r1, g1, b1 = chroma, x, 0
		case hPrime >= 1 && hPrime < 2: // yellow to green face
			r1, g1, b1 = x, chroma, 0
		case hPrime >= 2 && hPrime < 3: // green to cyan face
			r1, g1, b1 = 0, chroma, x
		case hPrime >= 3 && hPrime < 4: // cyan to blue face
			r1, g1, b1 = 0, x, chroma
		case hPrime >= 4 && hPrime < 5: // blue to magenta face
			r1, g1, b1 = x, 0, chroma
		case hPrime >= 5 && hPrime < 6: // magenta to red face
			r1, g1, b1 = chroma, 0, x
		}
		
		m := l - (chroma/2)
		//get r g b back to 0 - 255 range	
		rI := int64((r1 + m) * 255)
		gI := int64((g1 + m) * 255)
		bI := int64((b1 + m) * 255)

		return fmt.Sprintf("%02x%02x%02x", rI, gI, bI)
	}

	// normalize r g b
	nR := r / 255.0
	nG := g / 255.0
	nB := b / 255.0

	// get max min & delta
	max := math.Max(nR, math.Max(nG, nB))
	min := math.Min(nR, math.Min(nG, nB))
	delta := max - min

	//compute lightness
	lightness := (max + min) / 2

	// compute saturation
	var sat float64 = 0.0
	if (delta != 0.0) {
		sat = delta / (1 - math.Abs(2 * lightness - 1))
	} else { // in case of grayscale, randomize lightness and return
		invOffset := 1 / randOffset
		lightness += invOffset
		return hslToHex(0,0,lightness)
	}

	// compute hue
	var hue float64 = 0
	switch (max) {
	case r:
		hue = 60 * math.Mod(((g - b) / delta), 6.0)
	case g:
		hue = 60 * (((b - r) / delta) + 2)
	case b:
		hue = 60 * (((r - g) / delta) + 4)

	}
	// compute new hue
	newHue := math.Mod(hue + randOffset + 360, 360)
	
	// convert back to hex
	return hslToHex(newHue, sat, lightness)

}
