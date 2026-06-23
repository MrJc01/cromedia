package image

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cromedia/core"
)

// findSequenceFiles scans a directory for files matching a pattern.
// Supports both Glob (e.g. "path/to/*.png") and printf formats (e.g. "path/to/frame_%04d.jpg").
func findSequenceFiles(pattern string) ([]string, error) {
	if strings.HasPrefix(pattern, "http://") || strings.HasPrefix(pattern, "https://") {
		return []string{pattern}, nil
	}

	if strings.Contains(pattern, "*") {
		files, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
		return files, nil
	}

	if strings.Contains(pattern, "%") {
		dir := filepath.Dir(pattern)
		base := filepath.Base(pattern)

		// Regex to find "%04d", "%d", etc.
		rePattern := regexp.MustCompile(`%[0-9]*d`)
		if !rePattern.MatchString(base) {
			return nil, fmt.Errorf("invalid printf pattern: %s (must contain a digit placeholder like %%d or %%04d)", pattern)
		}

		placeholderBase := rePattern.ReplaceAllString(base, "___NUM___")
		escapedBase := regexp.QuoteMeta(placeholderBase)
		regexStr := "^" + strings.ReplaceAll(escapedBase, "___NUM___", `(\d+)`) + "$"

		matcher, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern regex: %w", err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}

		type fileItem struct {
			path string
			num  int
		}
		var items []fileItem

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matches := matcher.FindStringSubmatch(entry.Name())
			if len(matches) > 1 {
				num, _ := strconv.Atoi(matches[1])
				items = append(items, fileItem{
					path: filepath.Join(dir, entry.Name()),
					num:  num,
				})
			}
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].num < items[j].num
		})

		var result []string
		for _, item := range items {
			result = append(result, item.path)
		}
		return result, nil
	}

	// Single file fallback
	if _, err := os.Stat(pattern); err == nil {
		return []string{pattern}, nil
	}
	return nil, fmt.Errorf("no sequence matching pattern: %s", pattern)
}

type prefetchedPacket struct {
	pkt *core.Packet
	err error
}

// SequenceDemuxer implements demux.Demuxer to read image sequences as video frames.
type SequenceDemuxer struct {
	files      []string
	fps        float64
	index      int
	width      int
	height     int
	codec      string
	packetChan chan *prefetchedPacket
	once       sync.Once

	// GIF fields (Task 70)
	isGIF     bool
	gifFrames []image.Image
}

// NewSequenceDemuxer creates a SequenceDemuxer matching files to pattern.
func NewSequenceDemuxer(pattern string, fps float64) (*SequenceDemuxer, error) {
	if fps <= 0 {
		fps = 25.0
	}

	isGIF := strings.HasSuffix(strings.ToLower(pattern), ".gif") && !strings.Contains(pattern, "%") && !strings.Contains(pattern, "*")
	if isGIF {
		return &SequenceDemuxer{
			fps:   fps,
			isGIF: true,
			codec: "gif",
			files: []string{pattern},
		}, nil
	}

	files, err := findSequenceFiles(pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no sequence files found matching pattern: %s", pattern)
	}
	return &SequenceDemuxer{
		files: files,
		fps:   fps,
		index: 0,
	}, nil
}

// Probe parses the first image file config and prepares the Track structure.
func (d *SequenceDemuxer) Probe() ([]core.Track, error) {
	if d.isGIF {
		f, err := os.Open(d.files[0])
		if err != nil {
			return nil, fmt.Errorf("failed to open GIF file: %w", err)
		}
		defer f.Close()

		g, err := gif.DecodeAll(f)
		if err != nil {
			return nil, fmt.Errorf("failed to decode GIF: %w", err)
		}

		d.width = g.Config.Width
		d.height = g.Config.Height

		for _, img := range g.Image {
			d.gifFrames = append(d.gifFrames, img)
		}

		timescale := uint32(90000)
		sampleDuration := int64(float64(timescale) / d.fps)

		var samples []core.Sample
		var totalDuration int64
		for i := range d.gifFrames {
			samples = append(samples, core.Sample{
				ID:         i + 1,
				IsKeyframe: true,
				Offset:     0,
				Size:       0,
				Time:       totalDuration,
				Duration:   sampleDuration,
			})
			totalDuration += sampleDuration
		}

		track := core.Track{
			ID:        1,
			Type:      core.TrackTypeVideo,
			Timescale: timescale,
			Duration:  uint64(totalDuration),
			Samples:   samples,
			Width:     uint32(d.width),
			Height:    uint32(d.height),
			CodecTag:  "gif",
		}
		return []core.Track{track}, nil
	}

	if len(d.files) == 0 {
		return nil, errors.New("no files found in sequence")
	}

	var f io.ReadCloser
	var err error
	if strings.HasPrefix(d.files[0], "http://") || strings.HasPrefix(d.files[0], "https://") {
		resp, httpErr := http.Get(d.files[0])
		if httpErr != nil {
			return nil, httpErr
		}
		f = resp.Body
	} else {
		f, err = os.Open(d.files[0])
		if err != nil {
			return nil, fmt.Errorf("failed to open first file for probing: %w", err)
		}
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image config: %w", err)
	}

	d.width = cfg.Width
	d.height = cfg.Height
	d.codec = format

	// Map image formats to standard video codec tags
	codecTag := "mjpeg"
	if format == "png" {
		codecTag = "png"
	} else if format == "webp" {
		codecTag = "webp"
	} else if format == "bmp" {
		codecTag = "bmp"
	} else if format == "tiff" {
		codecTag = "tiff"
	}

	timescale := uint32(90000)
	sampleDuration := int64(float64(timescale) / d.fps)

	var samples []core.Sample
	var totalDuration int64
	for i := range d.files {
		samples = append(samples, core.Sample{
			ID:         i + 1,
			IsKeyframe: true,
			Offset:     0,
			Size:       0,
			Time:       totalDuration,
			Duration:   sampleDuration,
		})
		totalDuration += sampleDuration
	}

	track := core.Track{
		ID:        1,
		Type:      core.TrackTypeVideo,
		Timescale: timescale,
		Duration:  uint64(totalDuration),
		Samples:   samples,
		Width:     uint32(d.width),
		Height:    uint32(d.height),
		CodecTag:  codecTag,
	}

	return []core.Track{track}, nil
}

// startPrefetching preloads and decodes files in parallel (Task 56).
func (d *SequenceDemuxer) startPrefetching() {
	d.packetChan = make(chan *prefetchedPacket, 16)
	go func() {
		defer close(d.packetChan)

		for i := 0; i < len(d.files); i++ {
			idx := i
			path := d.files[idx]

			var data []byte
			var err error

			done := make(chan struct{})
			core.GlobalWorkerPool.Submit(func() {
				if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
					resp, httpErr := http.Get(path)
					if httpErr != nil {
						err = httpErr
					} else {
						data, err = io.ReadAll(resp.Body)
						resp.Body.Close()
					}
				} else {
					data, err = os.ReadFile(path)
				}
				close(done)
			})
			<-done

			timescale := uint32(90000)
			sampleDuration := int64(float64(timescale) / d.fps)
			pts := int64(idx) * sampleDuration

			var pkt *core.Packet
			if err == nil {
				pkt = &core.Packet{
					ID:          core.NewPacketID(),
					StreamIndex: 0,
					Data:        data,
					PTS:         pts,
					DTS:         pts,
					Duration:    sampleDuration,
					IsKeyframe:  true,
				}
			}

			d.packetChan <- &prefetchedPacket{pkt: pkt, err: err}
		}
	}()
}

// ReadPacket reads the next image file and returns it as an encoded Packet.
func (d *SequenceDemuxer) ReadPacket() (*core.Packet, error) {
	if d.isGIF {
		if d.index >= len(d.gifFrames) {
			return nil, io.EOF
		}

		var buf bytes.Buffer
		err := png.Encode(&buf, d.gifFrames[d.index])
		if err != nil {
			return nil, err
		}

		timescale := uint32(90000)
		sampleDuration := int64(float64(timescale) / d.fps)
		pts := int64(d.index) * sampleDuration

		pkt := &core.Packet{
			ID:          core.NewPacketID(),
			StreamIndex: 0,
			Data:        buf.Bytes(),
			PTS:         pts,
			DTS:         pts,
			Duration:    sampleDuration,
			IsKeyframe:  true,
		}
		d.index++
		return pkt, nil
	}

	d.once.Do(d.startPrefetching)

	res, ok := <-d.packetChan
	if !ok {
		return nil, io.EOF
	}

	if res.err != nil {
		return nil, res.err
	}

	d.index++
	return res.pkt, nil
}

// Close releases resources.
func (d *SequenceDemuxer) Close() error {
	return nil
}

// SequenceMuxer implements mux.Muxer to save video packets/frames as separate image files.
type SequenceMuxer struct {
	pattern   string
	index     int
	format    string
	quality   int
	gifFrames []*image.Paletted
	gifDelays []int
}

// NewSequenceMuxer creates a SequenceMuxer.
func NewSequenceMuxer(pattern string) (*SequenceMuxer, error) {
	ext := strings.ToLower(filepath.Ext(pattern))
	if ext == "" {
		return nil, errors.New("sequence pattern must have an image extension")
	}
	format := ext[1:]

	return &SequenceMuxer{
		pattern: pattern,
		index:   0,
		format:  format,
		quality: 85,
	}, nil
}

// SetQuality sets output quality for lossy formats (JPEG/WebP).
func (m *SequenceMuxer) SetQuality(quality int) {
	m.quality = quality
}

// WriteHeader matches interface constraints.
func (m *SequenceMuxer) WriteHeader(tracks []core.Track) error {
	return nil
}

func convertToPaletted(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	pal := image.NewPaletted(bounds, palette.Plan9)
	draw.Draw(pal, bounds, img, bounds.Min, draw.Src)
	return pal
}

// WritePacket encodes and writes a packet to disk.
func (m *SequenceMuxer) WritePacket(pkt *core.Packet) error {
	if m.format == "gif" { // Task 71: GIF encoding
		img, _, err := DecodeImage(bytes.NewReader(pkt.Data))
		if err != nil {
			return err
		}
		m.gifFrames = append(m.gifFrames, convertToPaletted(img))
		m.gifDelays = append(m.gifDelays, 10) // 100ms
		m.index++
		return nil
	}

	filename := fmt.Sprintf(m.pattern, m.index)
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	isTargetFormat := false
	switch m.format {
	case "jpeg", "jpg":
		if len(pkt.Data) > 3 && pkt.Data[0] == 0xFF && pkt.Data[1] == 0xD8 {
			isTargetFormat = true
		}
	case "png":
		if len(pkt.Data) > 8 && bytes.HasPrefix(pkt.Data, []byte("\x89PNG\r\n\x1a\n")) {
			isTargetFormat = true
		}
	case "webp":
		if len(pkt.Data) > 12 && bytes.HasPrefix(pkt.Data, []byte("RIFF")) && bytes.HasPrefix(pkt.Data[8:], []byte("WEBP")) {
			isTargetFormat = true
		}
	case "bmp":
		if len(pkt.Data) > 2 && pkt.Data[0] == 'B' && pkt.Data[1] == 'M' {
			isTargetFormat = true
		}
	}

	if isTargetFormat {
		// Fast path: write raw pre-encoded bytes directly to file
		err := os.WriteFile(filename, pkt.Data, 0644)
		if err != nil {
			return err
		}
	} else {
		// Slow path: decode and encode
		img, _, err := DecodeImage(bytes.NewReader(pkt.Data))
		if err != nil {
			return fmt.Errorf("failed to decode packet to encode to %s: %w", m.format, err)
		}

		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := EncodeImage(f, img, m.format, m.quality); err != nil {
			return err
		}
	}

	m.index++
	return nil
}

// WriteFrame directly encodes and writes a core.VideoFrame to disk.
func (m *SequenceMuxer) WriteFrame(frame *core.VideoFrame) error {
	img, err := ConvertToImage(frame)
	if err != nil {
		return err
	}

	if m.format == "gif" {
		m.gifFrames = append(m.gifFrames, convertToPaletted(img))
		m.gifDelays = append(m.gifDelays, 10)
		m.index++
		return nil
	}

	filename := fmt.Sprintf(m.pattern, m.index)
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := EncodeImage(f, img, m.format, m.quality); err != nil {
		return err
	}

	m.index++
	return nil
}

// WriteTrailer matches interface constraints.
func (m *SequenceMuxer) WriteTrailer() error {
	if m.format == "gif" && len(m.gifFrames) > 0 {
		filename := m.pattern
		if strings.Contains(filename, "%") {
			filename = fmt.Sprintf(m.pattern, 0)
			filename = strings.ReplaceAll(filename, "%d", "")
			if !strings.HasSuffix(filename, ".gif") {
				filename += ".gif"
			}
		}
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()

		return gif.EncodeAll(f, &gif.GIF{
			Image: m.gifFrames,
			Delay: m.gifDelays,
		})
	}
	return nil
}

// Close matches interface constraints.
func (m *SequenceMuxer) Close() error {
	return nil
}
