package app

import (
	"encoding/binary"
	"fmt"
)

func encodeInt32(val uint32) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint16(result[0:2], uint16(val&0xFFFF))
	binary.BigEndian.PutUint16(result[2:4], uint16((val&0xFFFF0000)>>16))
	return result
}

func encodeString(s string) []byte {
	result := make([]byte, 0)
	data := append([]byte(s), 0x00)

	// Debug: log what we're encoding
	fmt.Printf("encodeString: '%s' -> bytes: %v, len: %d\n", s, data, len(data))

	// write length
	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, uint16(len(data)))
	result = append(result, lengthBytes...)

	// encode string: pack 2 chars per short
	// C++ constructs short as: lo + (hi << 8), then uses htons() to convert to network byte order
	// Example: 'D'(68) + 'o'(111)<<8 = 0x6F44, htons() converts to big-endian bytes
	for i := 0; i < len(data); i += 2 {
		lo := int16(int8(data[i]))
		var hi int16
		if i+1 < len(data) {
			hi = int16(int8(data[i+1]))
		}
		// Construct the short value: lo + (hi << 8)
		shortVal := uint16(lo + (hi << 8))
		// Write directly as big-endian (network byte order)
		shortBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(shortBytes, shortVal)
		result = append(result, shortBytes...)
	}
	return result
}

// encodeReal encodes a REAL (float32) in armagetrons custom format
// 25-bit mantissa, 1-bit sign, 6-bit exponent
func encodeReal(f float32) []byte {
	const (
		MANT = 26
		EXP  = 6
		MS   = 25
	)

	y := f
	var negative uint32 = 0
	if y < 0 {
		y = -y
		negative = 1
	}

	var exp uint32 = 0

	// scale up by 64 for large numbers
	for y >= 64 && exp < (1<<EXP)-6 {
		exp += 6
		y /= 64
	}

	// scale up by 2 until < 1
	for y >= 1 && exp < (1<<EXP)-1 {
		exp++
		y /= 2
	}

	//convert to mantissa
	mant := uint32(y * float32(1<<MS))

	// clamp mantissa
	if mant > (1<<MS)-1 {
		mant = (1 << MS) - 1
	}

	// clamp exponent
	if exp > (1<<EXP)-1 {
		exp = (1 << EXP) - 1
		if mant > 0 {
			mant = (1 << MS) - 1
		}
	}

	// pack into 32-bit int
	trans := (mant & ((1 << MS) - 1)) | (negative << MS) | (exp << MANT)

	return encodeInt32(trans)
}
