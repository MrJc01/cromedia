package benchmark1

// GetHellcases retrieves all 100 hellcase test definitions
func GetHellcases() []Hellcase {
	var all []Hellcase
	all = append(all, GetArea1Cases()...)
	all = append(all, GetArea2Cases()...)
	all = append(all, GetArea3Cases()...)
	all = append(all, GetArea4Cases()...)
	all = append(all, GetArea5Cases()...)
	all = append(all, GetArea6Cases()...)
	all = append(all, GetArea7Cases()...)
	all = append(all, GetArea8Cases()...)
	all = append(all, GetArea9Cases()...)
	all = append(all, GetArea10Cases()...)
	return all
}
