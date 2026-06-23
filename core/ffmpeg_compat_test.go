package core

import (
	"os"
	"testing"
)

func TestParseFFmpegArgs(t *testing.T) {
	args := []string{
		"-y",
		"-i", "input.mp4",
		"-ss", "00:01:00",
		"-t", "10",
		"-c:v", "copy",
		"-c:a", "pcm",
		"-b:v", "2M",
		"-vf", "scale=1280:720",
		"-pix_fmt", "yuv420p",
		"--strict",
		"output.mp4",
	}

	cmd, err := ParseFFmpegArgs(args)
	if err != nil {
		t.Fatalf("failed to parse arguments: %v", err)
	}

	if !cmd.Overwrite {
		t.Error("expected Overwrite to be true")
	}
	if len(cmd.Inputs) != 1 || cmd.Inputs[0] != "input.mp4" {
		t.Errorf("expected input 'input.mp4', got %v", cmd.Inputs)
	}
	if cmd.StartTime != "00:01:00" {
		t.Errorf("expected start time '00:01:00', got '%s'", cmd.StartTime)
	}
	if cmd.Duration != "10" {
		t.Errorf("expected duration '10', got '%s'", cmd.Duration)
	}
	if cmd.VideoCodec != "copy" {
		t.Errorf("expected video codec 'copy', got '%s'", cmd.VideoCodec)
	}
	if cmd.AudioCodec != "pcm" {
		t.Errorf("expected audio codec 'pcm', got '%s'", cmd.AudioCodec)
	}
	if cmd.VideoBitrate != "2M" {
		t.Errorf("expected video bitrate '2M', got '%s'", cmd.VideoBitrate)
	}
	if cmd.VideoFilters != "scale=1280:720" {
		t.Errorf("expected video filters 'scale=1280:720', got '%s'", cmd.VideoFilters)
	}
	if cmd.PixelFormat != "yuv420p" {
		t.Errorf("expected pixel format 'yuv420p', got '%s'", cmd.PixelFormat)
	}
	if !cmd.Strict {
		t.Error("expected strict mode to be true")
	}
	if cmd.Output != "output.mp4" {
		t.Errorf("expected output 'output.mp4', got '%s'", cmd.Output)
	}
}

func TestCanExecuteNatively(t *testing.T) {
	cmdNative := &FFmpegCmd{
		Inputs:     []string{"in.mp4"},
		Output:     "out.mp4",
		VideoCodec: "copy",
		AudioCodec: "pcm",
	}
	if !CanExecuteNatively(cmdNative) {
		t.Error("expected command to be native-compatible")
	}

	cmdFallbackCodec := &FFmpegCmd{
		Inputs:     []string{"in.mp4"},
		Output:     "out.mp4",
		VideoCodec: "libx264",
	}
	if CanExecuteNatively(cmdFallbackCodec) {
		t.Error("expected command with libx264 to not be native-compatible")
	}

	cmdMultiInput := &FFmpegCmd{
		Inputs: []string{"in1.mp4", "in2.mp4"},
		Output: "out.mp4",
	}
	if CanExecuteNatively(cmdMultiInput) {
		t.Error("expected command with multiple inputs to not be native-compatible")
	}
}

func TestStrictBlocker(t *testing.T) {
	os.Setenv("CROMEDIA_STRICT", "1")
	defer os.Unsetenv("CROMEDIA_STRICT")

	args := []string{"-i", "in.mp4", "-c:v", "libx264", "out.mp4"}
	err := RunFFmpegCompat(args)
	if err == nil {
		t.Fatal("expected run to fail under strict mode, but got nil")
	}
	if err.Error() != "command requires fallback to system ffmpeg, but --strict mode is active" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseNewCompatFlags(t *testing.T) {
	args := []string{
		"-i", "input.mp4",
		"-metadata", "title=MyVideo",
		"-chapters", "ch1.txt",
		"-hls_time", "6",
		"-hls_list_size", "5",
		"-rtmp_playpath", "live_stream",
		"--benchmark",
		"output.m3u8",
	}

	cmd, err := ParseFFmpegArgs(args)
	if err != nil {
		t.Fatalf("failed parsing compat flags: %v", err)
	}

	if cmd.Metadata["title"] != "MyVideo" {
		t.Errorf("expected metadata title MyVideo, got %s", cmd.Metadata["title"])
	}
	if len(cmd.Chapters) != 1 || cmd.Chapters[0] != "ch1.txt" {
		t.Errorf("expected chapters ch1.txt, got %v", cmd.Chapters)
	}
	if cmd.HLSFlags["hls_time"] != "6" || cmd.HLSFlags["hls_list_size"] != "5" {
		t.Errorf("expected HLS flags, got %v", cmd.HLSFlags)
	}
	if cmd.RTMPFlags["playpath"] != "live_stream" {
		t.Errorf("expected RTMP flag playpath live_stream, got %s", cmd.RTMPFlags["playpath"])
	}
	if !cmd.Benchmark {
		t.Error("expected Benchmark flag to be true")
	}
}

func TestParseEscapedFilterString(t *testing.T) {
	filterStr := "scale=1280:720,drawtext=text='hello\\, world':fontcolor=white,noise=strength=5"
	parts := ParseEscapedFilterString(filterStr)

	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %v", len(parts), parts)
	}
	if parts[1] != "drawtext=text=hello, world:fontcolor=white" {
		t.Errorf("expected parsed string 'drawtext=text=hello, world:fontcolor=white', got %q", parts[1])
	}
}

func TestFallbackSimultaneous(t *testing.T) {
	// Task 155: Simulate multiple command routes
	cmd1 := &FFmpegCmd{Inputs: []string{"file1.mp4"}, Output: "out1.mp4", Metadata: map[string]string{"artist": "x"}}
	cmd2 := &FFmpegCmd{Inputs: []string{"file2.mp4"}, Output: "out2.mp4", HLSFlags: map[string]string{"hls_time": "10"}}

	if CanExecuteNatively(cmd1) {
		t.Error("expected fallback for command with metadata")
	}
	if CanExecuteNatively(cmd2) {
		t.Error("expected fallback for command with HLS flags")
	}
}

func TestResolveContainerTagDependencies(t *testing.T) {
	tag1 := ResolveContainerTagDependencies("mp4", "h264")
	if tag1 != "avc1" {
		t.Errorf("expected avc1, got %s", tag1)
	}

	tag2 := ResolveContainerTagDependencies("webm", "vp9")
	if tag2 != "VP90" {
		t.Errorf("expected VP90, got %s", tag2)
	}
}
