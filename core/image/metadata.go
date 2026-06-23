package image

import (
	"bytes"
	"encoding/binary"
	"image"
)

// RotateImageByOrientation rotates/flips an image based on EXIF orientation (1-8).
func RotateImageByOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Determine new dimensions
	var rotatedW, rotatedH int
	if orientation >= 5 && orientation <= 8 {
		rotatedW, rotatedH = h, w
	} else {
		rotatedW, rotatedH = w, h
	}

	dst := image.NewRGBA(image.Rect(0, 0, rotatedW, rotatedH))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcColor := img.At(x+bounds.Min.X, y+bounds.Min.Y)
			var nx, ny int

			switch orientation {
			case 2: // Flip horizontal
				nx, ny = w-1-x, y
			case 3: // Rotate 180
				nx, ny = w-1-x, h-1-y
			case 4: // Flip vertical
				nx, ny = x, h-1-y
			case 5: // Flip vertical + Rotate 270
				nx, ny = y, x
			case 6: // Rotate 90 CW
				nx, ny = h-1-y, x
			case 7: // Flip vertical + Rotate 90
				nx, ny = h-1-y, w-1-x
			case 8: // Rotate 270 CW
				nx, ny = y, w-1-x
			default:
				nx, ny = x, y
			}

			dst.Set(nx, ny, srcColor)
		}
	}

	return dst
}

// ParseJPEGMetadata parses JPEG bytes to find EXIF orientation and XMP XML string.
func ParseJPEGMetadata(data []byte) (orientation int, xmp string, err error) {
	orientation = 1 // Default normal
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return orientation, "", nil // Not a JPEG or too short
	}

	idx := 2
	for idx < len(data)-4 {
		if data[idx] != 0xFF {
			idx++
			continue
		}
		marker := data[idx+1]
		if marker == 0xD9 || marker == 0xDA { // End of image or Start of scan
			break
		}
		if marker == 0x00 { // Stuffed FF
			idx += 2
			continue
		}

		length := int(binary.BigEndian.Uint16(data[idx+2 : idx+4]))
		if idx+2+length > len(data) {
			break
		}

		payload := data[idx+4 : idx+2+length]

		// APP1 Marker (EXIF or XMP)
		if marker == 0xE1 {
			if len(payload) >= 6 && bytes.Equal(payload[:6], []byte("Exif\x00\x00")) {
				orientation = parseEXIF(payload[6:])
			} else if len(payload) >= 29 && bytes.Equal(payload[:29], []byte("http://ns.adobe.com/xap/1.0/\x00")) {
				xmp = string(payload[29:])
			}
		}

		idx += 2 + length
	}

	return orientation, xmp, nil
}

// ParsePNGMetadata parses PNG bytes to find EXIF orientation and XMP chunk.
func ParsePNGMetadata(data []byte) (orientation int, xmp string, err error) {
	orientation = 1
	if len(data) < 8 || !bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return orientation, "", nil
	}

	idx := 8
	for idx < len(data)-12 {
		length := int(binary.BigEndian.Uint32(data[idx : idx+4]))
		if idx+12+length > len(data) {
			break
		}
		chunkType := string(data[idx+4 : idx+8])
		payload := data[idx+8 : idx+8+length]

		if chunkType == "eXIf" {
			orientation = parseEXIF(payload)
		} else if chunkType == "iTXt" || chunkType == "tEXt" {
			// Look for XMP keyword
			if bytes.HasPrefix(payload, []byte("XML:com.adobe.xmp\x00")) {
				parts := bytes.SplitN(payload, []byte{0}, 2)
				if len(parts) == 2 {
					xmp = string(parts[1])
				}
			}
		}

		idx += 12 + length
	}

	return orientation, xmp, nil
}

func parseEXIF(exifData []byte) int {
	if len(exifData) < 8 {
		return 1
	}

	var byteOrder binary.ByteOrder = binary.BigEndian
	if exifData[0] == 'I' && exifData[1] == 'I' {
		byteOrder = binary.LittleEndian
	} else if exifData[0] != 'M' || exifData[1] != 'M' {
		return 1 // Invalid TIFF header
	}

	if byteOrder.Uint16(exifData[2:4]) != 42 {
		return 1 // Invalid signature
	}

	ifdOffset := int(byteOrder.Uint32(exifData[4:8]))
	if ifdOffset < 8 || ifdOffset >= len(exifData) {
		return 1
	}

	return parseIFD(exifData, ifdOffset, byteOrder)
}

func parseIFD(exifData []byte, offset int, byteOrder binary.ByteOrder) int {
	if offset+2 > len(exifData) {
		return 1
	}
	numEntries := int(byteOrder.Uint16(exifData[offset : offset+2]))
	offset += 2

	for i := 0; i < numEntries; i++ {
		if offset+12 > len(exifData) {
			break
		}
		tag := byteOrder.Uint16(exifData[offset : offset+2])
		tagType := byteOrder.Uint16(exifData[offset+2 : offset+4])
		count := byteOrder.Uint32(exifData[offset+4 : offset+8])

		// Orientation Tag is 0x0112 (274)
		if tag == 0x0112 {
			if tagType == 3 && count == 1 { // SHORT
				return int(byteOrder.Uint16(exifData[offset+8 : offset+10]))
			}
		}

		offset += 12
	}
	return 1
}
