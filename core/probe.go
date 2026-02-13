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
