package plugins

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"

	"cromedia/core"
)

func init() {
	gob.Register(&core.VideoFrame{})
	gob.Register(&core.AudioFrame{})
	gob.Register(&core.Packet{})
	gob.Register(&IPCResponse{})
}

// IPCResponse wraps the result of decoding sent from the sandbox subprocess.
type IPCResponse struct {
	Error      string
	VideoFrame *core.VideoFrame
	AudioFrame *core.AudioFrame
}

// InProcessVideoDecoder wraps a VideoDecoder to protect against panics and infinite loops.
type InProcessVideoDecoder struct {
	decoder core.VideoDecoder
	timeout time.Duration
}

// NewInProcessVideoDecoder wraps a VideoDecoder with timeout and panic recovery.
func NewInProcessVideoDecoder(dec core.VideoDecoder, timeout time.Duration) *InProcessVideoDecoder {
	if timeout <= 0 {
		timeout = 5 * time.Second // Default timeout
	}
	return &InProcessVideoDecoder{decoder: dec, timeout: timeout}
}

// Decode executes Decode with panic recovery and timeout protection.
func (d *InProcessVideoDecoder) Decode(pkt *core.Packet) (frame *core.VideoFrame, err error) {
	ch := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in decoder: %v", r)
			}
			close(ch)
		}()
		frame, err = d.decoder.Decode(pkt)
	}()

	select {
	case <-ch:
		return frame, err
	case <-time.After(d.timeout):
		return nil, errors.New("decoder timeout: infinite loop or deadlock detected")
	}
}

// Close closes the underlying decoder.
func (d *InProcessVideoDecoder) Close() error {
	return d.decoder.Close()
}

// InProcessAudioDecoder wraps an AudioDecoder to protect against panics and infinite loops.
type InProcessAudioDecoder struct {
	decoder core.AudioDecoder
	timeout time.Duration
}

// NewInProcessAudioDecoder wraps an AudioDecoder with timeout and panic recovery.
func NewInProcessAudioDecoder(dec core.AudioDecoder, timeout time.Duration) *InProcessAudioDecoder {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &InProcessAudioDecoder{decoder: dec, timeout: timeout}
}

// Decode executes Decode with panic recovery and timeout protection.
func (d *InProcessAudioDecoder) Decode(pkt *core.Packet) (frame *core.AudioFrame, err error) {
	ch := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in decoder: %v", r)
			}
			close(ch)
		}()
		frame, err = d.decoder.Decode(pkt)
	}()

	select {
	case <-ch:
		return frame, err
	case <-time.After(d.timeout):
		return nil, errors.New("decoder timeout: infinite loop or deadlock detected")
	}
}

// Close closes the underlying decoder.
func (d *InProcessAudioDecoder) Close() error {
	return d.decoder.Close()
}

// SubprocessVideoDecoder runs the decoder in a separate process for complete isolation.
type SubprocessVideoDecoder struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	enc        *gob.Encoder
	dec        *gob.Decoder
	mu         sync.Mutex
	pluginPath string
	name       string
}

// NewSubprocessVideoDecoder launches a subprocess worker to isolate decoding.
func NewSubprocessVideoDecoder(pluginPath, name string) (*SubprocessVideoDecoder, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(exe, "sandbox-worker", pluginPath, name, "video")
	cmd.Stderr = os.Stderr

	maxProcs := os.Getenv("CROMEDIA_PLUGIN_MAX_THREADS")
	if maxProcs == "" {
		maxProcs = "2" // default thread limit (Task 29)
	}
	heapQuota := os.Getenv("CROMEDIA_PLUGIN_HEAP_QUOTA")
	if heapQuota == "" {
		heapQuota = "512" // default heap quota in MB (Task 35)
	}
	cmd.Env = append(os.Environ(),
		"GOMAXPROCS="+maxProcs,
		"CROMEDIA_HEAP_QUOTA="+heapQuota,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}

	return &SubprocessVideoDecoder{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		enc:        gob.NewEncoder(stdin),
		dec:        gob.NewDecoder(stdout),
		pluginPath: pluginPath,
		name:       name,
	}, nil
}

// Decode sends a packet to the subprocess and receives the decoded frame.
func (d *SubprocessVideoDecoder) Decode(pkt *core.Packet) (*core.VideoFrame, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	start := time.Now()
	user1, sys1 := core.GetCPUTimes()

	if err := d.enc.Encode(pkt); err != nil {
		return nil, fmt.Errorf("failed to send packet to sandbox: %w", err)
	}

	var resp IPCResponse
	if err := d.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read frame from sandbox: %w", err)
	}

	user2, sys2 := core.GetCPUTimes()
	duration := time.Since(start)
	cpuDiff := (user2 - user1) + (sys2 - sys1)
	RecordPluginCall(d.name, duration, cpuDiff)

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return resp.VideoFrame, nil
}

// Close terminates the subprocess.
func (d *SubprocessVideoDecoder) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_ = d.stdin.Close()
	_ = d.stdout.Close()
	return d.cmd.Wait()
}

// SubprocessAudioDecoder runs the audio decoder in a separate process.
type SubprocessAudioDecoder struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	enc        *gob.Encoder
	dec        *gob.Decoder
	mu         sync.Mutex
	pluginPath string
	name       string
}

// NewSubprocessAudioDecoder launches a subprocess worker to isolate decoding.
func NewSubprocessAudioDecoder(pluginPath, name string) (*SubprocessAudioDecoder, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(exe, "sandbox-worker", pluginPath, name, "audio")
	cmd.Stderr = os.Stderr

	maxProcs := os.Getenv("CROMEDIA_PLUGIN_MAX_THREADS")
	if maxProcs == "" {
		maxProcs = "2" // default thread limit (Task 29)
	}
	heapQuota := os.Getenv("CROMEDIA_PLUGIN_HEAP_QUOTA")
	if heapQuota == "" {
		heapQuota = "512" // default heap quota in MB (Task 35)
	}
	cmd.Env = append(os.Environ(),
		"GOMAXPROCS="+maxProcs,
		"CROMEDIA_HEAP_QUOTA="+heapQuota,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}

	return &SubprocessAudioDecoder{
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		enc:        gob.NewEncoder(stdin),
		dec:        gob.NewDecoder(stdout),
		pluginPath: pluginPath,
		name:       name,
	}, nil
}

// Decode sends a packet to the subprocess and receives the decoded frame.
func (d *SubprocessAudioDecoder) Decode(pkt *core.Packet) (*core.AudioFrame, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	start := time.Now()
	user1, sys1 := core.GetCPUTimes()

	if err := d.enc.Encode(pkt); err != nil {
		return nil, fmt.Errorf("failed to send packet to sandbox: %w", err)
	}

	var resp IPCResponse
	if err := d.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read frame from sandbox: %w", err)
	}

	user2, sys2 := core.GetCPUTimes()
	duration := time.Since(start)
	cpuDiff := (user2 - user1) + (sys2 - sys1)
	RecordPluginCall(d.name, duration, cpuDiff)

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	return resp.AudioFrame, nil
}

// Close terminates the subprocess.
func (d *SubprocessAudioDecoder) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_ = d.stdin.Close()
	_ = d.stdout.Close()
	return d.cmd.Wait()
}

// RunSandboxWorker runs the worker loop. This should be invoked by the CLI subcommand.
func RunSandboxWorker(pluginPath, name, kind string) {
	// Disable logging to stdout to keep stdout clean for GOB IPC
	core.SetLogLevel(core.LogLevelError)

	if quotaStr := os.Getenv("CROMEDIA_HEAP_QUOTA"); quotaStr != "" {
		if quotaMb, err := strconv.Atoi(quotaStr); err == nil && quotaMb > 0 {
			go func() {
				limit := uint64(quotaMb) * 1024 * 1024
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				for range ticker.C {
					var m runtime.MemStats
					runtime.ReadMemStats(&m)
					if m.HeapAlloc > limit {
						fmt.Fprintf(os.Stderr, "Worker error: heap memory limit exceeded (%d bytes > %d limit)\n", m.HeapAlloc, limit)
						os.Exit(137) // Sigkill equivalent exit code
					}
				}
			}()
		}
	}

	if pluginPath != "" {
		if err := LoadPluginDynamic(pluginPath); err != nil {
			fmt.Fprintf(os.Stderr, "Worker error loading plugin: %v\n", err)
			os.Exit(1)
		}
	}

	pluginsMu.RLock()
	pluginInfo, ok := decoderPlugins[name]
	pluginsMu.RUnlock()
	if !ok {
		fmt.Fprintf(os.Stderr, "Worker error: decoder plugin %q not found\n", name)
		os.Exit(1)
	}

	rawDec, err := pluginInfo.NewDecoder()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Worker error creating decoder: %v\n", err)
		os.Exit(1)
	}

	dec := gob.NewDecoder(os.Stdin)
	enc := gob.NewEncoder(os.Stdout)

	if kind == "video" {
		vdec, ok := rawDec.(core.VideoDecoder)
		if !ok {
			fmt.Fprintf(os.Stderr, "Worker error: plugin %q does not implement VideoDecoder\n", name)
			os.Exit(1)
		}
		defer vdec.Close()

		for {
			var pkt core.Packet
			if err := dec.Decode(&pkt); err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				fmt.Fprintf(os.Stderr, "Worker decode packet error: %v\n", err)
				return
			}

			var resp IPCResponse
			frame, decodeErr := vdec.Decode(&pkt)
			if decodeErr != nil {
				resp.Error = decodeErr.Error()
			} else {
				resp.VideoFrame = frame
			}

			if err := enc.Encode(&resp); err != nil {
				fmt.Fprintf(os.Stderr, "Worker encode response error: %v\n", err)
				return
			}
		}
	} else if kind == "audio" {
		adec, ok := rawDec.(core.AudioDecoder)
		if !ok {
			fmt.Fprintf(os.Stderr, "Worker error: plugin %q does not implement AudioDecoder\n", name)
			os.Exit(1)
		}
		defer adec.Close()

		for {
			var pkt core.Packet
			if err := dec.Decode(&pkt); err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				fmt.Fprintf(os.Stderr, "Worker decode packet error: %v\n", err)
				return
			}

			var resp IPCResponse
			frame, decodeErr := adec.Decode(&pkt)
			if decodeErr != nil {
				resp.Error = decodeErr.Error()
			} else {
				resp.AudioFrame = frame
			}

			if err := enc.Encode(&resp); err != nil {
				fmt.Fprintf(os.Stderr, "Worker encode response error: %v\n", err)
				return
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "Worker error: unknown kind %q\n", kind)
		os.Exit(1)
	}
}
