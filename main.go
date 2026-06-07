package main

import (
	"flag"
	"log"
	"net"
	"os"

	tea "charm.land/bubbletea/v2"
)

var dataFile = flag.String("file", "", "anime TOML file (default: XDG data dir)")
var configFile = flag.String("config", "", "config TOML file (default: XDG config dir)")

func main() {
	flag.Parse()

	listener, err := net.Listen("tcp", "127.0.0.1:63219")
	if err != nil {
		log.Fatal("another instance is already running")
	}
	defer listener.Close()

	cfg, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	store, err := NewStore(*dataFile)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if os.Getenv("DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
	}

	if _, err := tea.NewProgram(NewRootModel(store, cfg)).Run(); err != nil {
		log.Fatal(err)
	}
}
