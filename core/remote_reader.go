package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// HTTPRangeReader implements io.Reader, io.Seeker, and io.Closer for HTTP remote files using HTTP Range Requests.
type HTTPRangeReader struct {
	url        string
	client     *http.Client
	size       int64
	currOffset int64
	respBody   io.ReadCloser
}

// NewHTTPRangeReader creates a new HTTPRangeReader for the given URL.
func NewHTTPRangeReader(url string, client *http.Client) (*HTTPRangeReader, error) {
	if client == nil {
		client = http.DefaultClient
	}

	reader := &HTTPRangeReader{
		url:    url,
		client: client,
	}

	// Perform a HEAD request to get the total content length
	resp, err := client.Head(url)
	if err != nil {
		return nil, fmt.Errorf("HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status error during HEAD: %s", resp.Status)
	}

	contentLength := resp.ContentLength
	if contentLength <= 0 {
		reader.size = -1
	} else {
		reader.size = contentLength
	}

	return reader, nil
}

// Size returns the total size of the remote file in bytes.
func (r *HTTPRangeReader) Size() int64 {
	return r.size
}

// Read reads up to len(p) bytes from the remote file.
func (r *HTTPRangeReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if r.size >= 0 && r.currOffset >= r.size {
		return 0, io.EOF
	}

	// If no active response stream, start a new HTTP GET request with Range header
	if r.respBody == nil {
		req, err := http.NewRequest("GET", r.url, nil)
		if err != nil {
			return 0, fmt.Errorf("failed to create request: %w", err)
		}

		// Set Range header
		rangeHeader := fmt.Sprintf("bytes=%d-", r.currOffset)
		req.Header.Set("Range", rangeHeader)

		resp, err := r.client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("HTTP request failed: %w", err)
		}

		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, fmt.Errorf("HTTP unexpected status: %s", resp.Status)
		}

		// Update size if it was unknown and Content-Range is present
		if r.size < 0 {
			cr := resp.Header.Get("Content-Range")
			if cr != "" {
				parts := strings.Split(cr, "/")
				if len(parts) == 2 {
					if total, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						r.size = total
					}
				}
			}
		}

		r.respBody = resp.Body
	}

	n, err := r.respBody.Read(p)
	r.currOffset += int64(n)

	// If EOF or read error, close and clean up stream
	if err != nil {
		r.respBody.Close()
		r.respBody = nil
	}

	return n, err
}

// Seek sets the offset for the next Read.
func (r *HTTPRangeReader) Seek(offset int64, whence int) (int64, error) {
	var target int64
	switch whence {
	case io.SeekStart:
		target = offset
	case io.SeekCurrent:
		target = r.currOffset + offset
	case io.SeekEnd:
		if r.size < 0 {
			return 0, errors.New("cannot seek from end, file size is unknown")
		}
		target = r.size + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if target < 0 {
		return 0, errors.New("negative seek offset")
	}

	// If the seek target changes, close the current request stream to force a new Range GET
	if target != r.currOffset {
		if r.respBody != nil {
			r.respBody.Close()
			r.respBody = nil
		}
		r.currOffset = target
	}

	return r.currOffset, nil
}

// Close closes the active HTTP response body.
func (r *HTTPRangeReader) Close() error {
	if r.respBody != nil {
		err := r.respBody.Close()
		r.respBody = nil
		return err
	}
	return nil
}
