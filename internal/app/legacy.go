package app

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrorInvalidLegacyMessage = errors.New("invalid legacy message")
)

const (
	LegacyPacket_SmallServerInfo    = uint16(50)
	LegacyPacket_BigServerInfo      = uint16(51)
	LegacyPacket_GetSmallServerInfo = uint16(52)
	LegacyPacket_GetBigServerInfo   = uint16(53)
	LegacyPacket_Logout             = uint16(7)
)

type LegacyMessage struct {
	DescriptorID uint16
	MessageID    uint16
	Length       uint16 // how many 2-byte segments in variable data (e.g len(data) == 12 so that means Length is 6)
	Data         []byte
}

func RawDataToLegacyMessage(data []byte) (*LegacyMessage, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("%v: legacy message too short", ErrorInvalidLegacyMessage)
	}

	descID := binary.BigEndian.Uint16([]byte{data[0], data[1]})
	msgID := binary.BigEndian.Uint16([]byte{data[2], data[3]})
	dataLen := binary.BigEndian.Uint16([]byte{data[4], data[5]})

	return &LegacyMessage{
		DescriptorID: descID,
		MessageID:    msgID,
		Length:       dataLen,
		Data:         data[6:],
	}, nil
}

func (s *Server) PacketLegacySmallServerInfo() []byte {
	packet := make([]byte, 0)

	// Write port as unsigned int (4 bytes / 2 shorts)
	// Both in BIG ENDIAN
	port := uint32(s.Config.Port)
	lowBytes := make([]byte, 2)
	highBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lowBytes, uint16(port&0xFFFF))            // low 16 bits
	binary.BigEndian.PutUint16(highBytes, uint16((port&0xFFFF0000)>>16)) // high 16 bits
	packet = append(packet, lowBytes...)
	packet = append(packet, highBytes...)

	// Prepare hostname with null terminator
	hostname := append([]byte(s.Config.Hostname), 0x00)

	// Write string length as unsigned short in BIG ENDIAN
	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, uint16(len(hostname)))
	packet = append(packet, lengthBytes...)

	// Encode hostname: pack 2 chars per short with signed encoding
	// This matches Armagetron's string encoding in nNetwork.cpp
	// C++ code: short(lo) + (short(hi) << 8), then converted to BIG ENDIAN with htons()
	for i := 0; i < len(hostname); i += 2 {
		// Treat chars as signed and convert to signed short (sign-extending)
		lo := int16(int8(hostname[i]))

		var hi int16
		if i+1 < len(hostname) {
			hi = int16(int8(hostname[i+1]))
		}

		// Combine: short(lo) + (short(hi) << 8) with signed arithmetic
		shortVal := uint16(lo + (hi << 8))

		shortBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(shortBytes, shortVal)
		packet = append(packet, shortBytes...)
	}

	// Write transaction number (unsigned int, 4 bytes / 2 shorts)
	// Set to 0 for non-master servers
	transactionLow := make([]byte, 2)
	transactionHigh := make([]byte, 2)
	binary.BigEndian.PutUint16(transactionLow, 0)
	binary.BigEndian.PutUint16(transactionHigh, 0)
	packet = append(packet, transactionLow...)
	packet = append(packet, transactionHigh...)

	return packet
}

func (s *Server) PacketLegacyBigServerInfo() []byte {
	packet := make([]byte, 0)

	port := uint32(s.Config.Port)
	packet = append(packet, encodeInt32(port)...)

	packet = append(packet, encodeString("")...) // empty so client uses sender IP

	packet = append(packet, encodeString(s.Config.Name)...)

	packet = append(packet, encodeInt32(0)...) // user count

	// VersionSync structure: min and max protocol versions (both int32)
	packet = append(packet, encodeInt32(1)...)  // version.min - minimum supported protocol version
	packet = append(packet, encodeInt32(25)...) // version.max - maximum supported protocol version

	packet = append(packet, encodeString("0.2.9.2.3")...) // release version string

	packet = append(packet, encodeInt32(16)...) // max players

	packet = append(packet, encodeString("")...) // usernames

	packet = append(packet, encodeString("")...) // options

	packet = append(packet, encodeString("")...) // URL

	packet = append(packet, encodeString("")...) // user global id's

	// SettingsDigest structure
	packet = append(packet, encodeInt32(0)...) // settings flags (uint32, not uint16!)

	// Three int32 fields before the REAL fields
	packet = append(packet, encodeInt32(0)...) // minPlayTimeTotal (int32)
	packet = append(packet, encodeInt32(0)...) // minPlayTimeOnline (int32)
	packet = append(packet, encodeInt32(0)...) // minPlayTimeTeam (int32)

	// REAL (float) settings
	packet = append(packet, encodeReal(0.1)...)  // cycleDelay
	packet = append(packet, encodeReal(0.5)...)  // acceleration
	packet = append(packet, encodeReal(0.0)...)  // rubberWallHump
	packet = append(packet, encodeReal(1.0)...)  // rubberHitWallRatio
	packet = append(packet, encodeReal(10.0)...) // wallsLength

	return packet
}

func BuildLegacyPacket(descriptor uint16, data []byte) []byte {
	// load descriptor into packet in BIG ENDIAN
	packet := make([]byte, 2)
	binary.BigEndian.PutUint16(packet, descriptor)

	// load message id into packet (should be 0)
	// BIG ENDIAN, so two bytes, [0, 0] == 0
	packet = append(packet, []byte{0, 0}...)

	// load length of shorts in data
	// this is signifying how many shorts are in the data, so if the length of the data slice is 11, the total amount of
	// shorts would be 6. e.g [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10] has a length of 11. If you break the shorts up...
	// [0, 1], [2, 3], [4, 5], [6, 7], [8, 9], [10, 0]
	// See how we append 0 if it's uneven?
	length := uint16(len(data) / 2)
	padded := false
	if len(data)%2 != 0 {
		length += 1
		padded = true
	}

	lengthBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lengthBytes, length)
	packet = append(packet, lengthBytes...)

	// load the actual data
	packet = append(packet, data...)

	// add padding if needed
	if padded {
		packet = append(packet, 0)
	}

	// load the sender id (short)
	packet = append(packet, []byte{0, 0}...)

	return packet
}
