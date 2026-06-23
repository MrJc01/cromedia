package benchmark2

type SyncTest struct {
	ID          int
	Name        string
	Description string
	Run         func() (croMs int, croMem float64, status string, err error)
}
