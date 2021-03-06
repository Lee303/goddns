package main

import (
	"fmt"
	"log"

	"github.com/0x4c6565/goddns/pkg/model"
	"github.com/miekg/dns"
)

type Server struct {
	storage   Storage
	Zone      string
	dnsServer *dns.Server
	Protocol  string
}

func NewServer(zone string, storage Storage, protocol string) *Server {
	return &Server{Zone: zone, storage: storage, Protocol: protocol}
}

func (s *Server) parseQuery(m *dns.Msg) {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeA:
			record, exists := s.storage.Get(q.Name, model.A)
			if exists {
				rr, err := dns.NewRR(fmt.Sprintf("%s 60 A %s", q.Name, record.IPAddress))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
		case dns.TypeAAAA:
			record, exists := s.storage.Get(q.Name, model.AAAA)
			if exists {
				rr, err := dns.NewRR(fmt.Sprintf("%s 60 AAAA %s", q.Name, record.IPAddress))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			}
		}
	}
}

func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		s.parseQuery(m)
	}

	w.WriteMsg(m)
}

func (s *Server) Start() {
	dns.HandleFunc(s.Zone, s.handleDNSRequest)

	s.dnsServer = &dns.Server{Addr: ":53", Net: s.Protocol}
	log.Printf("Starting server at %s 53\n", s.Protocol)
	err := s.dnsServer.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err)
	}
}
