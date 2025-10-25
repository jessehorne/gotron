package main

import (
	"github.com/jessehorne/gotron/internal/app"
	"log"
)

const (
	port       = "4534"
	bufferSize = 1024
)

func main() {
	s := app.NewServer(&app.ServerConfig{
		Name:     "Dock's Test GoTron Server",
		Hostname: "",  // Empty string so client uses sender IP
		Port:     4534,
		Logger:   log.Default(),
	})

	err := s.Listen()
	if err != nil {
		s.Config.Logger.Printf("error while listening: %s", err)
	}
}
