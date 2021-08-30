package tftp

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/pin/tftp"
)

type Server struct {
	configs  map[string][]byte
	configMu *sync.Mutex

	listeners   map[string]*tftp.Server
	listenersMu *sync.Mutex
}

func New() *Server {
	return &Server{
		configs:     make(map[string][]byte),
		configMu:    new(sync.Mutex),
		listeners:   make(map[string]*tftp.Server),
		listenersMu: new(sync.Mutex),
	}
}

func (s *Server) handler(filename string, wt io.WriterTo) error {
	buf := new(bytes.Buffer)
	if _, err := wt.WriteTo(buf); err != nil {
		return fmt.Errorf("could not write file (%s) to buffer: %w", filename, err)
	}
	s.configMu.Lock()
	s.configs[filename] = buf.Bytes()
	s.configMu.Unlock()
	return nil
}

func (s *Server) Handle(ip net.IP) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()

	str := ip.String()
	if _, ok := s.listeners[str]; ok {
		return
	}

	log.Println("INFO: Registering TFTP listener:", str)

	svr := tftp.NewServer(nil, s.handler)
	s.listeners[str] = svr

	go func(str string) {
		if err := svr.ListenAndServe(str + ":69"); err != nil {
			log.Printf("WARNING: Could not start tftp listener on %s:69: %v\n", str, err)
		}
		s.listenersMu.Lock()
		delete(s.listeners, str)
		s.listenersMu.Unlock()
	}(str)
}

func (s *Server) Shutdown() map[string][]byte {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	for _, l := range s.listeners {
		if l != nil {
			l.Shutdown()
		}
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	return s.configs
}
