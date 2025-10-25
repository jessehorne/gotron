package app

import (
	"fmt"
	"log"
	"net"
)

type ServerConfig struct {
	Name     string
	Hostname string
	Port     uint16
	Logger   *log.Logger
}

type Server struct {
	Config *ServerConfig
}

func NewServer(sc *ServerConfig) *Server {
	return &Server{
		Config: sc,
	}
}

func (s *Server) Listen() error {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", s.Config.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %s", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("error listening on UDP: %s", err)
	}
	defer conn.Close()

	s.Config.Logger.Printf("Started '%s' on port '%d'.\n", s.Config.Name, s.Config.Port)

	bufferSize := 1024
	buffer := make([]byte, bufferSize)
	for {
		n, clientAddr, readErr := conn.ReadFromUDP(buffer)
		if readErr != nil {
			s.Config.Logger.Printf("error reading from UDP: %s\n", readErr)
			continue
		}

		copyBuffer := make([]byte, n)
		copy(copyBuffer, buffer[:n])
		go s.handleLegacyMessage(conn, clientAddr, copyBuffer)
	}
}

func (s *Server) handleLegacyMessage(conn *net.UDPConn, clientAddr *net.UDPAddr, data []byte) {
	s.Config.Logger.Printf("received %d bytes from %s: %v\n", len(data), clientAddr.IP.String(), data)

	lm, err := RawDataToLegacyMessage(data)
	if err != nil {
		s.Config.Logger.Printf("invalid message: %v\n", err)
		return
	}

	//s.Config.Logger.Printf("LegacyMessage: dID: %d mID: %d l: %d rl: %d\n", lm.DescriptorID, lm.MessageID, lm.Length, len(lm.Data))

	switch lm.DescriptorID {
	case LegacyPacket_GetSmallServerInfo:
		s.Config.Logger.Printf("Received GetSmallServerInfo request\n")
		s.sendSmallServerInfo(conn, clientAddr)
	case LegacyPacket_GetBigServerInfo:
		s.Config.Logger.Printf("Received GetBigServerInfo request\n")
		s.sendBigServerInfo(conn, clientAddr)
	case LegacyPacket_Logout:
		s.Config.Logger.Printf("Client from %s logging out\n", clientAddr.String())
		// No response needed - client is disconnecting gracefully
	default:
		s.Config.Logger.Printf("Unknown descriptor ID: %d\n", lm.DescriptorID)
	}
}

func (s *Server) sendSmallServerInfo(conn *net.UDPConn, clientAddr *net.UDPAddr) {
	data := s.PacketLegacySmallServerInfo()
	packet := BuildLegacyPacket(LegacyPacket_SmallServerInfo, data)
	_, err := conn.WriteToUDP(packet, clientAddr)
	if err != nil {
		s.Config.Logger.Printf("error sending SmallServerInfo to client: %v\n", err)
	}
}

func (s *Server) sendBigServerInfo(conn *net.UDPConn, clientAddr *net.UDPAddr) {
	data := s.PacketLegacyBigServerInfo()
	s.Config.Logger.Printf("BigServerInfo data length: %d bytes\n", len(data))
	s.Config.Logger.Printf("BigServerInfo data hex: %x\n", data)
	packet := BuildLegacyPacket(LegacyPacket_BigServerInfo, data)
	s.Config.Logger.Printf("Sending BigServerInfo packet: %d bytes total\n", len(packet))
	s.Config.Logger.Printf("Full packet hex: %x\n", packet)
	n, err := conn.WriteToUDP(packet, clientAddr)
	if err != nil {
		s.Config.Logger.Printf("error sending BigServerInfo to client: %v\n", err)
	} else {
		s.Config.Logger.Printf("Successfully sent BigServerInfo: %d bytes\n", n)
	}
}
