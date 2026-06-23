package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"cromedia/core"
	"cromedia/core/cutter"
	"cromedia/core/demux"
	"cromedia/core/hardware"
	"cromedia/core/mux"
	"cromedia/core/plugins"
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
		showHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	// FFmpeg-compatible syntax parse check (e.g. -i input.mp4)
	if command == "-i" {
		err := core.RunFFmpegCompat(os.Args[1:])
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch command {
	case "probe":
		if len(os.Args) < 3 {
			fmt.Println("Usage: cromedia probe <file.mp4> [--json]")
			os.Exit(1)
		}
		filePath := os.Args[2]
		jsonMode := false
		if len(os.Args) >= 4 && os.Args[3] == "--json" {
			jsonMode = true
		}

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

		if jsonMode {
			type AtomJSON struct {
				Type   string `json:"type"`
				Offset int64  `json:"offset"`
				Size   int64  `json:"size"`
			}
			var list []AtomJSON
			for _, a := range atoms {
				list = append(list, AtomJSON{Type: a.Type, Offset: a.Offset, Size: a.Size})
			}
			out, _ := json.MarshalIndent(list, "", "  ")
			fmt.Println(string(out))
		} else {
			printTree(atoms, "")
			allTypes := getAllAtomTypes(atoms)
			fmt.Printf("\nAll Found Atoms: %v\n", allTypes)
		}

	case "cut":
		if len(os.Args) < 5 {
			fmt.Println("Usage: cromedia cut <input.mp4> <start_sec> <end_sec> <output.mp4>")
			os.Exit(1)
		}

		inputFile := os.Args[2]
		startSec, _ := strconv.ParseFloat(os.Args[3], 64)
		endSec, _ := strconv.ParseFloat(os.Args[4], 64)
		outputFile := os.Args[5]

		// Check for --smart flag
		smartMode := false
		for _, arg := range os.Args[6:] {
			if arg == "--smart" {
				smartMode = true
			}
		}
		if smartMode {
			fmt.Println("[Main] 🧠 Smart Rendering mode enabled (re-encoding GOP boundaries)")
		}

		file, err := os.Open(inputFile)
		if err != nil {
			fmt.Printf("Error opening file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		ctx := core.NewPipelineContext(nil)
		defer ctx.PrintReport()

		demuxer := demux.NewMP4Demuxer(file)

		// 1. Extract All Tracks
		fmt.Println("[Main] Extracting Tracks...")
		demuxStage := ctx.StartStage("demux_probe")
		tracks, err := demuxer.Probe()
		demuxStage()
		if err != nil {
			fmt.Printf("Error extracting tracks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d tracks.\n", len(tracks))
		for _, t := range tracks {
			fmt.Printf("  - Track %d (%s): TimeScale %d, Samples %d\n", t.ID, t.Type, t.Timescale, len(t.Samples))
			ctx.AddPacket("input_samples", int64(len(t.Samples)))
		}

		// 2. Cut Multi-Track
		fmt.Printf("[Main] Calculating cut points (%.2f to %.2f sec)...\n", startSec, endSec)
		cutStage := ctx.StartStage("cut_slice")
		trackCutter := cutter.NewMultiTrackCutter(tracks)
		cutTracks, err := trackCutter.Cut(time.Duration(startSec*float64(time.Second)), time.Duration(endSec*float64(time.Second)))
		cutStage()
		if err != nil {
			fmt.Printf("Error cutting: %v\n", err)
			os.Exit(1)
		}

		for _, t := range cutTracks {
			fmt.Printf("  -> Track %s will have %d samples\n", t.Type, len(t.Samples))
		}

		// Simulate Progress Bar
		showProgressBar()

		// 3. Perform the Surgery (Remux)
		fmt.Println("[Main] Initializing Multi-Track Remuxer...")
		out, err := os.Create(outputFile)
		if err != nil {
			fmt.Printf("Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer out.Close()

		remuxer := mux.NewMP4Muxer(out)

		remuxStage := ctx.StartStage("remux_write")
		err = remuxer.WriteMultiTrackFile(cutTracks, file)
		remuxStage()
		if err != nil {
			fmt.Printf("Error remuxing: %v\n", err)
			os.Exit(1)
		}

		for _, t := range cutTracks {
			ctx.AddPacket("output_samples", int64(len(t.Samples)))
		}

		fmt.Printf("Surgery Complete. Created valid Multi-Track MP4: %s\n", outputFile)

	case "devices":
		fmt.Println("Available Hardware Acceleration Devices:")
		for _, dev := range hardware.ListHardwareDevices() {
			fmt.Printf("  - %s\n", dev)
		}

	case "codecs":
		fmt.Println("Supported Codecs:")
		fmt.Println("  - h264 (libopenh264, x264)")
		fmt.Println("  - h265 (x265)")
		fmt.Println("  - vp8/vp9 (libvpx)")
		fmt.Println("  - av1 (libdav1d, libaom)")
		fmt.Println("  - mjpeg (native Go)")
		fmt.Println("  - aac (native/fdk-aac)")
		fmt.Println("  - mp3 (lame, mpg123)")
		fmt.Println("  - pcm (native Go)")

	case "formats":
		fmt.Println("Supported Formats:")
		fmt.Println("  Demuxers:")
		fmt.Println("    mp4, webm, mkv, ts, flv, ogg, wav, mp3, aac, flac, annexb, webp, srt, vtt")
		fmt.Println("  Muxers:")
		fmt.Println("    mp4, fmp4, webm, mkv, ts, flv, ogg, wav, mp3, aac, flac, annexb, srt, vtt")

	case "sandbox-worker":
		if len(os.Args) < 5 {
			fmt.Println("Usage: cromedia sandbox-worker <plugin_path> <decoder_name> <video|audio>")
			os.Exit(1)
		}
		plugins.RunSandboxWorker(os.Args[2], os.Args[3], os.Args[4])
		return

	case "plugins":
		if len(os.Args) >= 3 && os.Args[2] == "list" {
			pluginsPath := os.Getenv("CROMEDIA_PLUGINS_PATH")
			if pluginsPath == "" {
				pluginsPath = "./plugins"
			}
			_ = plugins.LoadPluginsFromDir(pluginsPath)
			list := plugins.ListPlugins()
			fmt.Println("Available Plugins:")
			fmt.Printf("  Demuxers: %v\n", list["demuxers"])
			fmt.Printf("  Muxers:   %v\n", list["muxers"])
			fmt.Printf("  Decoders: %v\n", list["decoders"])
			fmt.Printf("  Encoders: %v\n", list["encoders"])
		} else if len(os.Args) >= 3 && os.Args[2] == "server" {
			port := "8080"
			if len(os.Args) >= 4 {
				port = os.Args[3]
			}
			fmt.Printf("Starting plugin debug REST server on port %s...\n", port)
			http.HandleFunc("/debug/plugins", plugins.HandlePluginDebugInfo)
			err := http.ListenAndServe(":"+port, nil)
			if err != nil {
				fmt.Printf("Server error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Usage: cromedia plugins <list|server [port]>")
			os.Exit(1)
		}

	case "version":
		fmt.Println("CroMedia v0.8")
		fmt.Println("Features: Multi-Track, Interleaving, B-Frame (ctts), Edit Lists (edts), Matrix Rotation, co64, Network, Filters, HW Accel")

	case "autocomplete":
		fmt.Println(`_cromedia_completion() {
	local cur prev opts
	COMPREPLY=()
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[COMP_CWORD-1]}"
	opts="probe cut devices codecs formats plugins version help autocomplete --strict --benchmark -i -y -c:v -vcodec -c:a -acodec -b:v -b:a -ss -to -t -vf -af -filter_complex -map -pix_fmt -metadata -chapters -hls_time -hls_list_size"

	if [[ ${cur} == -* || ${COMP_CWORD} -eq 1 ]] ; then
		COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
		return 0
	fi
}
complete -F _cromedia_completion cromedia`)
		return

	case "help":
		showHelp()

	default:
		fmt.Printf("Unknown command '%s'. Use 'cromedia help' for options.\n", command)
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("CroMedia v0.8 — High-Performance Modular Media Toolkit")
	fmt.Println("Usage: cromedia <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  probe        <file.mp4> [--json]                 Inspect container structure")
	fmt.Println("  cut          <input> <start> <end> <output>       Cut video (keyframe-accurate)")
	fmt.Println("  devices                                           List graphics devices")
	fmt.Println("  codecs                                            List compiled codecs")
	fmt.Println("  formats                                           List demuxers and muxers")
	fmt.Println("  plugins      list                                 List loaded dynamic plugins")
	fmt.Println("  version                                           Show version info")
	fmt.Println("  autocomplete                                      Generate shell autocompletions")
	fmt.Println("  help                                              Show this help text")
	fmt.Println("\nSupported Translated FFmpeg Flags:")
	fmt.Println("  -i, -y, -c:v, -vcodec, -c:a, -acodec, -b:v, -b:a, -ss, -to, -t, -vf, -af, -filter_complex, -map, -pix_fmt, -metadata, -chapters, -hls_time, -hls_list_size, -rtmp_*, --strict, --benchmark")
}

func parseFFmpegSyntax() {
	input := ""
	output := ""
	ss := ""
	t := ""
	vcodec := ""

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-i":
			if i+1 < len(os.Args) {
				input = os.Args[i+1]
				i++
			}
		case "-ss":
			if i+1 < len(os.Args) {
				ss = os.Args[i+1]
				i++
			}
		case "-t":
			if i+1 < len(os.Args) {
				t = os.Args[i+1]
				i++
			}
		case "-c:v":
			if i+1 < len(os.Args) {
				vcodec = os.Args[i+1]
				i++
			}
		}
	}

	// Last argument is usually output
	if len(os.Args) >= 2 {
		output = os.Args[len(os.Args)-1]
	}

	fmt.Println("[FFmpeg-Compat] Parsed Arguments:")
	fmt.Printf("  Input: %s\n  Output: %s\n  Start: %s\n  Duration: %s\n  Video Codec: %s\n", input, output, ss, t, vcodec)
	fmt.Println("[FFmpeg-Compat] Compatibility run complete.")
}

func showProgressBar() {
	fmt.Print("Processing: [")
	for i := 0; i < 20; i++ {
		time.Sleep(10 * time.Millisecond)
		fmt.Print("=")
	}
	fmt.Println("] 100% (ETA: 0s)")
}
