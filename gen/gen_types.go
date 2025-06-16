package gen
//this file contains common types for the package

type QueryOptions struct {
	Count uint
	Dimensions *Dim
	ClrSeeds *[]string
	Noise *string
}


type Point struct {
	X int
	Y int
}

type Dim struct {
	W int
	H int
}
