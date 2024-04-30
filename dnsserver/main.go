package main

import (
	"fmt"
	"log"

	"github.com/miekg/dns"
)

// Local DNS records
var localRecords = map[string]string{
	"example.local.": "192.168.1.100",
	"router.local.":  "192.168.0.1",
}

// DNS handler function
func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	switch r.Opcode { // if query
	case dns.OpcodeQuery:
		parseQuery(m)
	}
	w.WriteMsg(m)
}

// Parse DNS queries and respond (if locally known) or forward (if not)
func parseQuery(m *dns.Msg) {
	for _, q := range m.Question { // for each question (apparently some clients send multiple queries in one request - documentation lol)
		switch q.Qtype {
		case dns.TypeA: // if type A record is requested (local)
			if ip, found := localRecords[q.Name]; found {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip)) // create resource record with resolved IP in required format
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
			} else { // otherwise resolve from Cloudflare DNS
				c := new(dns.Client)
				in, _, err := c.Exchange(m, "1.1.1.1:53") // query cludflare DNS  TODO: why is this not working?
				log.Printf("contents of recieved message: %v\n", in)
				if err != nil {
					log.Printf("error querying Cloudflare: %v\n", err)
					return
				}
				if err == nil {
					m.Answer = append(m.Answer, in.Answer...)
				}
			}
		}
	}
	// TODO: add reverse lookup?
}

func main() {
	// Setup DNS server
	dns.HandleFunc(".", handleDNSRequest)
	server := &dns.Server{Addr: ":53", Net: "udp"}
	log.Printf("Starting DNS server on port%s\n", server.Addr)

	// Start server
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n", err.Error())
	}
}
