package benchmark1

// Hellcase represents a single comparative test case between CroMedia and FFmpeg
type Hellcase struct {
	ID          int
	Name        string
	Category    string
	Run         func() (croMs int, croMem float64, ffMs int, ffMem float64, status string)
}
