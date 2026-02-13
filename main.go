package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"cromedia/core"
)

// Helper to print atom tree structure
func printTree(atoms []core.Atom, indent string) {
	for _, atom := range atoms {
		fmt.Printf("%s[%s] @ %d (Size: %d)\n", indent, atom.Type, atom.Offset, atom.Size)
		if len(atom.Children) > 0 {
			printTree(atom.Children, indent+"  ")
		}
	}
}

func getAllAtomTypes(atoms []core.Atom) []string {
	var types []string
	for _, a := range atoms {
		types = append(types, a.Type)
		types = append(types, getAllAtomTypes(a.Children)...)
	}
	return types
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: cromedia <command> [args]")
		fmt.Println("Commands: probe, cut")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "probe":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cromedia probe <file.mp4>")
			os.Exit(1)
		}
		filePath := os.Args[2]
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		atoms, err := core.FastProbe(file)
		if err != nil {
			fmt.Printf("Error probing file: %v\n", err)
			os.Exit(1)
		}
		printTree(atoms, "")

		allTypes := getAllAtomTypes(atoms)
		fmt.Printf("\nAll Found Atoms: %v\n", allTypes)

		hasCtts := false
		hasEdts := false
		for _, t := range allTypes {
			if t == "ctts" {
				hasCtts = true
			}
			if t == "edts" {
				hasEdts = true
			}
		}
		fmt.Printf("Critical Check: ctts=%v, edts=%v\n", hasCtts, hasEdts)

	case "cut":
		if len(os.Args) < 5 {
			fmt.Println("Usage: cromedia cut <input.mp4> <start_sec> <end_sec> <output.mp4>")
			os.Exit(1)
		}

		inputFile := os.Args[2]
		startSec, _ := strconv.ParseFloat(os.Args[3], 64)
		endSec, _ := strconv.ParseFloat(os.Args[4], 64)
		outputFile := os.Args[5]

		file, err := os.Open(inputFile)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		fmt.Println("[Main] Probing file...")
		atoms, err := core.FastProbe(file)
		if err != nil {
			panic(err)
		}

		// Helper to find atom
		var findAtom func(atoms []core.Atom, typ string) *core.Atom
		findAtom = func(atoms []core.Atom, typ string) *core.Atom {
			for i := range atoms {
				if atoms[i].Type == typ {
					return &atoms[i]
				}
			}
			return nil
		}

		moov := findAtom(atoms, "moov")
		if moov == nil {
			fmt.Println("Error: 'moov' atom not found")
			os.Exit(1)
		}

		demuxer := core.NewDemuxer(file)

		// 1. Extract All Tracks
		fmt.Println("[Main] Extracting Tracks...")
		tracks, err := demuxer.ExtractTracks(*moov)
		if err != nil {
			fmt.Printf("Error extracting tracks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d tracks.\n", len(tracks))
		for _, t := range tracks {
			fmt.Printf("  - Track %d (%s): TimeScale %d, Samples %d\n", t.ID, t.Type, t.Timescale, len(t.Samples))
		}

		// 2. Cut Multi-Track
		fmt.Printf("[Main] Calculating cut points (%.2f to %.2f sec)...\n", startSec, endSec)
		cutter := core.NewMultiTrackCutter(tracks)
		cutTracks, err := cutter.Cut(time.Duration(startSec*float64(time.Second)), time.Duration(endSec*float64(time.Second)))
		if err != nil {
			fmt.Printf("Error cutting: %v\n", err)
			os.Exit(1)
		}

		for _, t := range cutTracks {
			fmt.Printf("  -> Track %s will have %d samples\n", t.Type, len(t.Samples))
		}

		// 3. Perform the Surgery (Remux)
		fmt.Println("[Main] Initializing Multi-Track Remuxer...")
		remuxer := &core.Remuxer{InputFile: file}

		err = remuxer.WriteMultiTrackFile(outputFile, cutTracks)
		if err != nil {
			fmt.Printf("Error remuxing: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Surgery Complete. Created valid Multi-Track MP4: %s\n", outputFile)

	default:
		fmt.Println("Unknown command")
	}
}
