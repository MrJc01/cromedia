package mux

// DefaultVideoHdlr generates a standard video hdlr box payload.
func DefaultVideoHdlr() []byte {
	// hdlr layout: 4 bytes version/flags (0), 4 bytes pre_defined (0), 4 bytes handler_type ("vide"), 12 bytes reserved (0), 13 bytes name ("VideoHandler\x00")
	hdlr := make([]byte, 37)
	copy(hdlr[8:12], "vide")
	copy(hdlr[24:], "VideoHandler")
	return hdlr
}

// DefaultAudioHdlr generates a standard audio hdlr box payload.
func DefaultAudioHdlr() []byte {
	// hdlr layout: 4 bytes version/flags (0), 4 bytes pre_defined (0), 4 bytes handler_type ("soun"), 12 bytes reserved (0), 13 bytes name ("AudioHandler\x00")
	hdlr := make([]byte, 37)
	copy(hdlr[8:12], "soun")
	copy(hdlr[24:], "AudioHandler")
	return hdlr
}

// DefaultVideoMediaHeader generates a standard vmhd box payload.
func DefaultVideoMediaHeader() []byte {
	// vmhd layout: 4 bytes version/flags (1), 2 bytes graphicsmode (0), 6 bytes opcolor (0)
	vmhd := make([]byte, 12)
	vmhd[3] = 1 // version/flags = 1
	return vmhd
}

// DefaultAudioMediaHeader generates a standard smhd box payload.
func DefaultAudioMediaHeader() []byte {
	// smhd layout: 4 bytes version/flags (0), 2 bytes balance (0), 2 bytes reserved (0)
	return make([]byte, 8)
}

// MakeH264Stsd constructs a valid H.264 stsd box payload containing SPS and PPS.
func MakeH264Stsd(width, height int, sps, pps []byte) []byte {
	// 1. Build avcC payload (AVCDecoderConfigurationRecord)
	avcC := new(ExcludeBuffer)
	avcC.WriteBytes([]byte{
		1,      // configurationVersion
		sps[1], // AVCProfileIndication
		sps[2], // profile_compatibility
		sps[3], // AVCLevelIndication
		0xff,   // lengthSizeMinusOne: 0xfc | (4 - 1) = 0xff (4-byte NAL length)
		0xe1,   // numOfSequenceParameterSets: 0xe0 | 1 = 0xe1
	})
	// SPS length (2 bytes) + SPS data
	spsLen := uint16(len(sps))
	avcC.WriteUint16(spsLen)
	avcC.WriteBytes(sps)
	// numOfPictureParameterSets: 1
	avcC.WriteBytes([]byte{1})
	// PPS length (2 bytes) + PPS data
	ppsLen := uint16(len(pps))
	avcC.WriteUint16(ppsLen)
	avcC.WriteBytes(pps)

	avcCAtom := &SimpleAtom{
		Type: "avcC",
		Data: avcC.Bytes(),
	}

	// 2. Build avc1 payload (78 bytes)
	avc1Data := new(ExcludeBuffer)
	avc1Data.WriteBytes(make([]byte, 6)) // reserved
	avc1Data.WriteUint16(1)              // data_reference_index = 1
	avc1Data.WriteBytes(make([]byte, 16)) // pre_defined/reserved
	avc1Data.WriteUint16(uint16(width))
	avc1Data.WriteUint16(uint16(height))
	avc1Data.WriteUint32(0x00480000)      // horizresolution
	avc1Data.WriteUint32(0x00480000)      // vertresolution
	avc1Data.WriteUint32(0)               // reserved
	avc1Data.WriteUint16(1)               // frame_count
	// compressorname: 32 bytes
	compName := make([]byte, 32)
	copy(compName, "H.264 Video Encoder")
	avc1Data.WriteBytes(compName)
	avc1Data.WriteUint16(0x0018)          // depth
	avc1Data.WriteUint16(0xffff)          // pre_defined = -1

	avc1Atom := &SimpleAtom{
		Type:     "avc1",
		Data:     avc1Data.Bytes(),
		Children: []*SimpleAtom{avcCAtom},
	}

	// 3. Build stsd payload
	payload := new(ExcludeBuffer)
	payload.WriteUint32(0) // version + flags = 0
	payload.WriteUint32(1) // entry count = 1
	payload.WriteBytes(serializeAtom(avc1Atom))

	return payload.Bytes()
}

// MakeAACStsd constructs a valid AAC stsd box payload containing AudioSpecificConfig (asc).
func MakeAACStsd(sampleRate int, channels int, asc []byte) []byte {
	// 1. Build esds payload
	// We write the descriptor tags recursively:
	// Tag 0x05 (DecoderSpecificInfo)
	decSpecific := []byte{0x05, byte(len(asc))}
	decSpecific = append(decSpecific, asc...)

	// Tag 0x04 (DecoderConfigDescriptor)
	decConfigPayload := []byte{
		0x40, // ObjectTypeIndication = Audio ISO/IEC 14496-3 AAC
		0x15, // StreamType = AudioStream
		0, 0, 0, // BufferSizeDB
		0, 1, 0xf4, 0, // MaxBitrate = 128000
		0, 1, 0xf4, 0, // AvgBitrate = 128000
	}
	decConfigPayload = append(decConfigPayload, decSpecific...)

	decConfig := []byte{0x04, byte(len(decConfigPayload))}
	decConfig = append(decConfig, decConfigPayload...)

	// Tag 0x06 (SLConfigDescriptor)
	slConfig := []byte{0x06, 1, 0x02}

	// Tag 0x03 (ES_Descriptor)
	esDescPayload := []byte{
		0, 1, // ES_ID = 1
		0,    // flags = 0
	}
	esDescPayload = append(esDescPayload, decConfig...)
	esDescPayload = append(esDescPayload, slConfig...)

	esDesc := []byte{0x03, byte(len(esDescPayload))}
	esDesc = append(esDesc, esDescPayload...)

	// esds box payload = version/flags (4 bytes) + ES_Descriptor
	esdsData := append(make([]byte, 4), esDesc...)

	esdsAtom := &SimpleAtom{
		Type: "esds",
		Data: esdsData,
	}

	// 2. Build mp4a payload (28 bytes)
	mp4aData := new(ExcludeBuffer)
	mp4aData.WriteBytes(make([]byte, 6)) // reserved
	mp4aData.WriteUint16(1)              // data_reference_index = 1
	mp4aData.WriteBytes(make([]byte, 8))  // reserved
	mp4aData.WriteUint16(uint16(channels))
	mp4aData.WriteUint16(16)             // samplesize = 16
	mp4aData.WriteUint16(0)              // pre_defined = 0
	mp4aData.WriteUint16(0)              // reserved = 0
	mp4aData.WriteUint32(uint32(sampleRate << 16))

	mp4aAtom := &SimpleAtom{
		Type:     "mp4a",
		Data:     mp4aData.Bytes(),
		Children: []*SimpleAtom{esdsAtom},
	}

	// 3. Build stsd payload
	payload := new(ExcludeBuffer)
	payload.WriteUint32(0) // version + flags = 0
	payload.WriteUint32(1) // entry count = 1
	payload.WriteBytes(serializeAtom(mp4aAtom))

	return payload.Bytes()
}

// MakeAudioStsd constructs a basic audio stsd box payload for standard audio tracks.
func MakeAudioStsd(codecTag string, sampleRate int, channels int) []byte {
	// Pad or trim codecTag to 4 characters
	tag := codecTag
	if len(tag) < 4 {
		tag = tag + "    "
	}
	tag = tag[:4]

	audioData := new(ExcludeBuffer)
	audioData.WriteBytes(make([]byte, 6)) // reserved
	audioData.WriteUint16(1)              // data_reference_index = 1
	audioData.WriteBytes(make([]byte, 8))  // reserved
	audioData.WriteUint16(uint16(channels))
	audioData.WriteUint16(16)             // samplesize = 16
	audioData.WriteUint16(0)              // pre_defined = 0
	audioData.WriteUint16(0)              // reserved = 0
	audioData.WriteUint32(uint32(sampleRate << 16))

	audioAtom := &SimpleAtom{
		Type: tag,
		Data: audioData.Bytes(),
	}

	payload := new(ExcludeBuffer)
	payload.WriteUint32(0) // version + flags = 0
	payload.WriteUint32(1) // entry count = 1
	payload.WriteBytes(serializeAtom(audioAtom))

	return payload.Bytes()
}

