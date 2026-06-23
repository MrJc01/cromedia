package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

type Atom struct {
	Type     string
	Data     []byte
	Children []*Atom
}

func (a *Atom) Serialize() []byte {
	var childBytes []byte
	for _, c := range a.Children {
		childBytes = append(childBytes, c.Serialize()...)
	}
	totalSize := 8 + len(a.Data) + len(childBytes)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(totalSize))
	copy(buf[4:8], []byte(a.Type))
	res := append(buf, a.Data...)
	res = append(res, childBytes...)
	return res
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run generate_mock.go <output_path.mp4>")
		os.Exit(1)
	}
	outputPath := os.Args[1]

	// 1. Build ftyp
	ftypData := []byte("isom\x00\x00\x02\x00isommp41")
	ftypAtom := &Atom{Type: "ftyp", Data: ftypData}
	ftypBytes := ftypAtom.Serialize()

	// 2. Build moov skeleton to calculate its size first
	// We'll write mock data for stco initially and rewrite it with the real mdat offsets later.
	numSamples := 100
	sampleSize := 100

	// mvhd
	mvhdData := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhdData[12:16], 1000)   // timescale = 1000
	binary.BigEndian.PutUint32(mvhdData[16:20], 100000) // duration = 100000 (100 seconds)
	binary.BigEndian.PutUint32(mvhdData[76:80], 2)      // next track ID = 2

	// tkhd
	tkhdData := make([]byte, 84)
	binary.BigEndian.PutUint32(tkhdData[0:4], 3)        // flags = 3
	binary.BigEndian.PutUint32(tkhdData[12:16], 1)      // track ID = 1
	binary.BigEndian.PutUint32(tkhdData[20:24], 100000) // duration = 100000
	binary.BigEndian.PutUint32(tkhdData[76:80], 640<<16) // width = 640
	binary.BigEndian.PutUint32(tkhdData[80:84], 480<<16) // height = 480

	// mdhd
	mdhdData := make([]byte, 24)
	binary.BigEndian.PutUint32(mdhdData[12:16], 1000)   // timescale = 1000
	binary.BigEndian.PutUint32(mdhdData[16:20], 100000) // duration = 100000

	// hdlr
	hdlrData := make([]byte, 63)
	copy(hdlrData[8:12], []byte("vide"))
	copy(hdlrData[32:38], []byte("Video\x00"))

	// vmhd
	vmhdData := make([]byte, 12)

	// dref
	drefData := make([]byte, 20)
	binary.BigEndian.PutUint32(drefData[4:8], 1) // entry count = 1
	binary.BigEndian.PutUint32(drefData[8:12], 12)
	copy(drefData[12:16], []byte("url "))
	binary.BigEndian.PutUint32(drefData[16:20], 1) // flags = 1

	// stsd
	stsdData := make([]byte, 199)
	binary.BigEndian.PutUint32(stsdData[4:8], 1) // entry count = 1
	binary.BigEndian.PutUint32(stsdData[8:12], 191)
	copy(stsdData[12:16], []byte("avc1"))

	// stts
	sttsData := make([]byte, 16)
	binary.BigEndian.PutUint32(sttsData[4:8], 1)    // entry count = 1
	binary.BigEndian.PutUint32(sttsData[8:12], uint32(numSamples))
	binary.BigEndian.PutUint32(sttsData[12:16], 1000) // duration = 1000 ms per sample

	// stss: Keyframes every 10 samples (1, 11, 21, ...)
	numKeyframes := 10
	stssData := make([]byte, 8+numKeyframes*4)
	binary.BigEndian.PutUint32(stssData[4:8], uint32(numKeyframes))
	for i := 0; i < numKeyframes; i++ {
		binary.BigEndian.PutUint32(stssData[8+i*4:12+i*4], uint32(i*10+1))
	}

	// stsc
	stscData := make([]byte, 20)
	binary.BigEndian.PutUint32(stscData[4:8], 1)   // entry count = 1
	binary.BigEndian.PutUint32(stscData[8:12], 1)  // first chunk = 1
	binary.BigEndian.PutUint32(stscData[12:16], 1) // samples per chunk = 1
	binary.BigEndian.PutUint32(stscData[16:20], 1) // sample desc id = 1

	// stsz
	stszData := make([]byte, 12+numSamples*4)
	binary.BigEndian.PutUint32(stszData[8:12], uint32(numSamples))
	for i := 0; i < numSamples; i++ {
		binary.BigEndian.PutUint32(stszData[12+i*4:16+i*4], uint32(sampleSize))
	}

	// Dummy stco to calculate size (100 chunk offsets)
	stcoData := make([]byte, 8+numSamples*4)
	binary.BigEndian.PutUint32(stcoData[4:8], uint32(numSamples))

	// Assemble stbl
	stblAtom := &Atom{
		Type: "stbl",
		Children: []*Atom{
			{Type: "stsd", Data: stsdData},
			{Type: "stts", Data: sttsData},
			{Type: "stss", Data: stssData},
			{Type: "stsc", Data: stscData},
			{Type: "stsz", Data: stszData},
			{Type: "stco", Data: stcoData},
		},
	}

	// Assemble minf
	minfAtom := &Atom{
		Type: "minf",
		Children: []*Atom{
			{Type: "vmhd", Data: vmhdData},
			{Type: "dinf", Children: []*Atom{{Type: "dref", Data: drefData}}},
			stblAtom,
		},
	}

	// Assemble mdia
	mdiaAtom := &Atom{
		Type: "mdia",
		Children: []*Atom{
			{Type: "mdhd", Data: mdhdData},
			{Type: "hdlr", Data: hdlrData},
			minfAtom,
		},
	}

	// Assemble trak
	trakAtom := &Atom{
		Type: "trak",
		Children: []*Atom{
			{Type: "tkhd", Data: tkhdData},
			mdiaAtom,
		},
	}

	// Assemble moov
	moovAtom := &Atom{
		Type: "moov",
		Children: []*Atom{
			{Type: "mvhd", Data: mvhdData},
			trakAtom,
		},
	}

	moovBytes := moovAtom.Serialize()

	// Calculate absolute offsets for mdat samples
	// File layout: [ftyp] (len ftypBytes) + [moov] (len moovBytes) + [mdat header] (8 bytes) + [mdat samples]
	mdatStartOffset := int64(len(ftypBytes)) + int64(len(moovBytes))
	sampleStartOffset := mdatStartOffset + 8

	// Populate REAL stco chunk offsets
	for i := 0; i < numSamples; i++ {
		offset := uint32(sampleStartOffset + int64(i*sampleSize))
		binary.BigEndian.PutUint32(stcoData[8+i*4:12+i*4], offset)
	}

	// Re-serialize moov with corrected offsets
	moovBytes = moovAtom.Serialize()

	// Create Output File
	file, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Write ftyp
	file.Write(ftypBytes)

	// Write moov
	file.Write(moovBytes)

	// Write mdat
	mdatHeader := make([]byte, 8)
	binary.BigEndian.PutUint32(mdatHeader[0:4], uint32(8+numSamples*sampleSize))
	copy(mdatHeader[4:8], []byte("mdat"))
	file.Write(mdatHeader)

	// Write dummy sample payloads
	for i := 0; i < numSamples; i++ {
		payload := make([]byte, sampleSize)
		// Put some pattern to distinguish frames if probed
		binary.BigEndian.PutUint32(payload[0:4], uint32(i))
		file.Write(payload)
	}

	fmt.Printf("Successfully generated mock MP4 file at %s (%d bytes)\n", outputPath, len(ftypBytes)+len(moovBytes)+8+numSamples*sampleSize)
}
