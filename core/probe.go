package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ContainerAtoms defines which atoms should be parsed recursively
var ContainerAtoms = map[string]bool{
	"moov": true,
	"trak": true,
	"mdia": true,
	"minf": true,
	"dinf": true,
	"stbl": true,
	"mvex": true,
	"edts": true,
	"udta": true,
	"moof": true,
	"traf": true,
	"meta": true,
}

// Atom represents an MP4 box/atom
type Atom struct {
	Offset   int64
	Size     int64
	Type     string
	Children []Atom
}

// String returns a formatted string representation of the Atom
func (a Atom) String() string {
	return fmt.Sprintf("[%s] @ %d (Size: %d)", a.Type, a.Offset, a.Size)
}

// FastProbe analyzes the file structure without loading payloads
func FastProbe(file *os.File) ([]Atom, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()

	return parseAtoms(file, 0, fileSize)
}

// parseAtoms is the recursive function to traverse the atom tree
func parseAtoms(file *os.File, start, end int64) ([]Atom, error) {
	var atoms []Atom
	offset := start

	for offset < end {
		// Seek to the current atom header
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return nil, err
		}

		// Read Header (8 bytes: 4 size + 4 type)
		header := make([]byte, 8)
		if _, err := file.Read(header); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		size := int64(binary.BigEndian.Uint32(header[0:4]))
		typ := string(header[4:8])

		// Handle Special Case: Size 1 means extended size (64-bit) follows
		if size == 1 {
			extendedHeader := make([]byte, 8)
			if _, err := file.Read(extendedHeader); err != nil {
				return nil, err
			}
			size = int64(binary.BigEndian.Uint64(extendedHeader))
			// Adjust offset for extended header reading
			// Note: The extended size includes the 8 bytes of the extended header + 8 bytes of standard header
		}

		if size == 0 {
			// Size 0 means "rest of the file"
			size = end - offset
		}

		if offset+size > end {
			size = end - offset
		}

		atom := Atom{
			Offset: offset,
			Size:   size,
			Type:   typ,
		}

		// Recursion for known containers
		if ContainerAtoms[typ] {
			// Payload starts after the header.
			// Standard header is 8 bytes.
			// Extended header logic is simplified here; full spec requires checking extensions.
			// For this MVP, assuming standard 8-byte header for containers unless size=1 logic is hit.
			headerSize := int64(8)
			if size == 1 {
				headerSize = 16
			}
			if typ == "meta" {
				headerSize += 4
			}

			children, err := parseAtoms(file, offset+headerSize, offset+size)
			if err != nil {
				// Don't fail completely on malformed children, just log/warn?
				// For now, return error to be strict.
				return nil, err
			}
			atom.Children = children
		}

		atoms = append(atoms, atom)
		offset += size
	}

	return atoms, nil
}

// FindAtom recursively searches for an atom of the specified type in the atom slice
func FindAtom(atoms []Atom, typ string) *Atom {
	for i := range atoms {
		if atoms[i].Type == typ {
			return &atoms[i]
		}
		if len(atoms[i].Children) > 0 {
			res := FindAtom(atoms[i].Children, typ)
			if res != nil {
				return res
			}
		}
	}
	return nil
}

// ReadMvhd reads timescale and duration from an mvhd atom
func ReadMvhd(file *os.File, mvhd *Atom) (timescale uint32, duration uint64, err error) {
	headerSize := int64(8)
	if mvhd.Size == 1 {
		headerSize = 16
	}

	payloadOffset := mvhd.Offset + headerSize
	_, err = file.Seek(payloadOffset, io.SeekStart)
	if err != nil {
		return 0, 0, err
	}

	// We need up to 32 bytes of the payload for version 1
	buf := make([]byte, 32)
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return 0, 0, err
	}
	if n < 20 {
		return 0, 0, fmt.Errorf("mvhd atom payload too short: %d bytes", n)
	}

	version := buf[0]
	if version == 0 {
		timescale = binary.BigEndian.Uint32(buf[12:16])
		duration = uint64(binary.BigEndian.Uint32(buf[16:20]))
	} else if version == 1 {
		if n < 32 {
			return 0, 0, fmt.Errorf("mvhd version 1 atom payload too short: %d bytes", n)
		}
		timescale = binary.BigEndian.Uint32(buf[20:24])
		duration = binary.BigEndian.Uint64(buf[24:32])
	} else {
		return 0, 0, fmt.Errorf("unsupported mvhd version: %d", version)
	}

	return timescale, duration, nil
}

// GetMP4Duration calculates and returns the exact duration of the file in milliseconds
func GetMP4Duration(file *os.File) (int64, error) {
	atoms, err := FastProbe(file)
	if err != nil {
		return 0, err
	}

	mvhd := FindAtom(atoms, "mvhd")
	if mvhd == nil {
		return 0, fmt.Errorf("mvhd atom not found")
	}

	timescale, duration, err := ReadMvhd(file, mvhd)
	if err != nil {
		return 0, err
	}

	if timescale == 0 {
		return 0, fmt.Errorf("invalid timescale (0)")
	}

	durationMs := (duration * 1000) / uint64(timescale)
	return int64(durationMs), nil
}
