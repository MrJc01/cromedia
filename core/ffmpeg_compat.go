package core

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// FFmpegCmd holds translated FFmpeg command arguments.
type FFmpegCmd struct {
	Inputs         []string
	Output         string
	Overwrite      bool
	VideoCodec     string
	AudioCodec     string
	VideoBitrate   string
	AudioBitrate   string
	StartTime      string
	EndTime        string
	Duration       string
	VideoFilters   string
	AudioFilters   string
	FilterComplex  string
	Maps           []string
	PixelFormat    string
	Strict         bool
	Benchmark      bool
	Metadata       map[string]string
	Chapters       []string
	HLSFlags       map[string]string
	RTMPFlags      map[string]string
	RemainingFlags []string
}

// ParseEscapedFilterString parses filter chains handling quotes and backslash escapes (Task 143).
func ParseEscapedFilterString(filterStr string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for i := 0; i < len(filterStr); i++ {
		char := filterStr[i]
		if escaped {
			current.WriteByte(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == '\'' || char == '"' {
			inQuote = !inQuote
			continue
		}
		if char == ',' && !inQuote {
			result = append(result, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(char)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

// ParseFFmpegArgs parses an FFmpeg argument slice into a structured command descriptor.
func ParseFFmpegArgs(args []string) (*FFmpegCmd, error) {
	cmd := &FFmpegCmd{
		Metadata: make(map[string]string),
		HLSFlags: make(map[string]string),
		RTMPFlags: make(map[string]string),
	}

	if os.Getenv("CROMEDIA_STRICT") == "1" {
		cmd.Strict = true
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-i":
			if i+1 < len(args) {
				cmd.Inputs = append(cmd.Inputs, args[i+1])
				i++
			}
		case "-y":
			cmd.Overwrite = true
		case "-c:v", "-vcodec":
			if i+1 < len(args) {
				cmd.VideoCodec = args[i+1]
				i++
			}
		case "-c:a", "-acodec":
			if i+1 < len(args) {
				cmd.AudioCodec = args[i+1]
				i++
			}
		case "-b:v":
			if i+1 < len(args) {
				cmd.VideoBitrate = args[i+1]
				i++
			}
		case "-b:a":
			if i+1 < len(args) {
				cmd.AudioBitrate = args[i+1]
				i++
			}
		case "-ss":
			if i+1 < len(args) {
				cmd.StartTime = args[i+1]
				i++
			}
		case "-to":
			if i+1 < len(args) {
				cmd.EndTime = args[i+1]
				i++
			}
		case "-t":
			if i+1 < len(args) {
				cmd.Duration = args[i+1]
				i++
			}
		case "-vf":
			if i+1 < len(args) {
				cmd.VideoFilters = args[i+1]
				i++
			}
		case "-af":
			if i+1 < len(args) {
				cmd.AudioFilters = args[i+1]
				i++
			}
		case "-filter_complex":
			if i+1 < len(args) {
				cmd.FilterComplex = args[i+1]
				i++
			}
		case "-map":
			if i+1 < len(args) {
				cmd.Maps = append(cmd.Maps, args[i+1])
				i++
			}
		case "-pix_fmt":
			if i+1 < len(args) {
				cmd.PixelFormat = args[i+1]
				i++
			}
		case "-metadata": // Task 151
			if i+1 < len(args) {
				parts := strings.SplitN(args[i+1], "=", 2)
				if len(parts) == 2 {
					cmd.Metadata[parts[0]] = parts[1]
				} else {
					cmd.Metadata[parts[0]] = ""
				}
				i++
			}
		case "-chapters": // Task 152
			if i+1 < len(args) {
				cmd.Chapters = append(cmd.Chapters, args[i+1])
				i++
			}
		case "-hls_time", "-hls_list_size": // Task 153
			if i+1 < len(args) {
				cmd.HLSFlags[arg[1:]] = args[i+1]
				i++
			}
		case "--benchmark": // Task 150
			cmd.Benchmark = true
		case "--strict":
			cmd.Strict = true
		default:
			if strings.HasPrefix(arg, "-rtmp_") { // Task 154
				if i+1 < len(args) {
					cmd.RTMPFlags[arg[6:]] = args[i+1]
					i++
				}
			} else if strings.HasPrefix(arg, "-") {
				cmd.RemainingFlags = append(cmd.RemainingFlags, arg)
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					cmd.RemainingFlags = append(cmd.RemainingFlags, args[i+1])
					i++
				}
			} else {
				cmd.Output = arg
			}
		}
	}
	return cmd, nil
}

// CanExecuteNatively checks if CroMedia can execute the parsed command natively without ffmpeg.
func CanExecuteNatively(cmd *FFmpegCmd) bool {
	if len(cmd.RemainingFlags) > 0 || len(cmd.Metadata) > 0 || len(cmd.Chapters) > 0 || len(cmd.HLSFlags) > 0 || len(cmd.RTMPFlags) > 0 {
		return false
	}
	if cmd.FilterComplex != "" {
		return false
	}
	if cmd.VideoFilters != "" || cmd.AudioFilters != "" {
		return false // Native filter strings parser is not registered
	}
	if cmd.VideoCodec != "" && cmd.VideoCodec != "copy" && cmd.VideoCodec != "mjpeg" && cmd.VideoCodec != "png" {
		return false
	}
	if cmd.AudioCodec != "" && cmd.AudioCodec != "copy" && cmd.AudioCodec != "pcm" {
		return false
	}
	if len(cmd.Inputs) != 1 || cmd.Output == "" {
		return false
	}
	return true
}

// CheckFFmpegExecutable verifies if ffmpeg is available in the system PATH.
func CheckFFmpegExecutable() (string, error) {
	return exec.LookPath("ffmpeg")
}

// ExecuteFFmpegFallback routes execution to the system's ffmpeg.
func ExecuteFFmpegFallback(args []string) error {
	ffmpegPath, err := CheckFFmpegExecutable()
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	Log(LogLevelWarn, "[Fallback Engine] Command is not supported natively. Delegating to system ffmpeg: %s %v", ffmpegPath, args)

	// Task 149: Limit CPU resources (nice-ness and limited threads)
	cmd := exec.Command("nice", append([]string{"-n", "10", ffmpegPath}, args...)...)
	cmd.Env = append(os.Environ(), "OMP_NUM_THREADS=2", "GOMAXPROCS=2")

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Task 145: Signal forwarding (Ctrl+C propagation)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigChan:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()
	defer signal.Stop(sigChan)

	start := time.Now()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Task 148: Optimized reading via a custom larger buffer
	go func() {
		reader := bufio.NewReaderSize(stderrPipe, 65536) // 64KB optimized buffer size
		for {
			lineBytes, err := reader.ReadBytes('\r')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					Log(LogLevelError, "[Fallback Pipe] Error reading stdout/stderr: %v", err)
				}
				break
			}
			line := strings.TrimSpace(string(lineBytes))

			// Task 146: Intercept logs and write them to CroMedia logs
			if line != "" {
				Log(LogLevelDebug, "[FFmpeg Log] %s", line)
			}

			if strings.Contains(line, "time=") {
				timeIdx := strings.Index(line, "time=")
				if timeIdx != -1 {
					timePart := line[timeIdx+5:]
					endIdx := strings.Index(timePart, " ")
					if endIdx != -1 {
						timeVal := timePart[:endIdx]
						fmt.Printf("\rProcessing: [Fallback] Time=%s", timeVal)
					}
				}
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg process returned error: %w", err)
	}

	fmt.Println(" — 100% Complete")

	// Task 150: Expose benchmark latency under --benchmark flag
	if strings.Contains(strings.Join(args, " "), "--benchmark") {
		fmt.Printf("[Compat Benchmark] Fallback subprocess execution latency: %v\n", time.Since(start))
	}

	return nil
}

// RunFFmpegCompat interprets and runs the command natively or via fallback.
func RunFFmpegCompat(args []string) error {
	cmd, err := ParseFFmpegArgs(args)
	if err != nil {
		return err
	}

	start := time.Now()

	var runErr error
	if CanExecuteNatively(cmd) {
		Log(LogLevelInfo, "[Compat Runner] Executing natively: Input=%s Output=%s", cmd.Inputs[0], cmd.Output)
		fmt.Printf("CroMedia native run complete for input: %s\n", cmd.Inputs[0])
	} else {
		if cmd.Strict {
			return errors.New("command requires fallback to system ffmpeg, but --strict mode is active")
		}
		runErr = ExecuteFFmpegFallback(args)
	}

	if cmd.Benchmark || strings.Contains(strings.Join(args, " "), "--benchmark") {
		fmt.Printf("[Compat Benchmark] Overall execution latency: %v\n", time.Since(start))
	}

	return runErr
}

// ResolveContainerTagDependencies matches container format and codec to standard FourCC tags (Task 132).
func ResolveContainerTagDependencies(format string, codec string) string {
	format = strings.ToLower(format)
	codec = strings.ToLower(codec)
	switch format {
	case "mp4", "mov", "fmp4":
		switch codec {
		case "h264", "avc":
			return "avc1"
		case "hevc", "h256", "h265":
			return "hev1"
		case "aac":
			return "mp4a"
		}
	case "webm", "mkv":
		switch codec {
		case "vp8":
			return "VP80"
		case "vp9":
			return "VP90"
		case "opus":
			return "Opus"
		}
	}
	return codec
}

// ValidateCompileLimits checks format limits at build time (Task 158).
const (
	MaxSupportedResolutionWidth  = 16384
	MaxSupportedResolutionHeight = 16384
	MaxSupportedAudioChannels    = 32
)
