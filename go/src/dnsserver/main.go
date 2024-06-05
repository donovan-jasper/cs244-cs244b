package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"raftclient"
	pb "raftprotos"

	"github.com/miekg/dns"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	AddRecord = iota
	DeleteRecord
	ReadRecord
)

type LocalDNSARecord struct {
	record dns.A
	expiry time.Time
}

// Local DNS records
var localRecords = map[string]LocalDNSARecord{
	"example.local.": {
		record: dns.A{
			Hdr: dns.RR_Header{
				Name:   "example.local.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    86400,
			},
			A: net.ParseIP("192.168.1.100"),
		},
		expiry: time.Now().Add(24 * time.Hour),
	},
	"router.local.": {
		record: dns.A{
			Hdr: dns.RR_Header{
				Name:   "router.local.",
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    86400,
			},
			A: net.ParseIP("192.168.0.1"),
		},
		expiry: time.Now().Add(24 * time.Hour),
	},
}

var raftClient raftclient.RaftClient

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
		if q.Qtype != dns.TypeA { // if query type is not A (IPv4 address)
			continue
		}

		if record, found := localRecords[q.Name]; found {
			if record.expiry.Before(time.Now()) {
				delete(localRecords, q.Name)
			} else {
				m.Answer = append(m.Answer, &record.record) // if record is found locally, respond with it
				continue
			}
		}

		if record, found := readRecordFromRaftCluster(q.Name); found { // otherwise resolve from Raft cluster
			m.Answer = append(m.Answer, &record)
			localRecords[q.Name] = LocalDNSARecord{
				record: record,
				expiry: time.Now().Add(time.Duration(record.Hdr.Ttl) * time.Second),
			}
		} else { // otherwise resolve from Cloudflare DNS
			c := new(dns.Client)
			recursiveMsg := new(dns.Msg)
			recursiveMsg.SetQuestion(q.Name, dns.TypeA)

			in, _, err := c.Exchange(recursiveMsg, "1.1.1.1:53") // query Cloudflare DNS
			if err != nil {
				slog.Error("error querying Cloudflare: %v\n", "error", err)
				return
			}

			// add the response back to the Raft cluster
			for _, answer := range in.Answer {
				if a, ok := answer.(*dns.A); ok {
					err = addRecordToRaftCluster(*a)
					if err != nil {
						slog.Error("error adding record to Raft cluster: %v\n", "error", err)
					}
				}
			}

			m.Answer = append(m.Answer, in.Answer...)
		}
	}
}

func readRecordFromRaftCluster(hostname string) (dns.A, bool) {
	raftResponse := raftClient.SendDNSCommand(pb.DNSCommand{
		CommandType: ReadRecord,
		Domain:      "saligrama.io",
	})

	if raftResponse.Success {
		return dns.A{
			Hdr: dns.RR_Header{
				Name:   hostname,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    uint32(raftResponse.DnsRecord.Ttl.Seconds),
			},
			A: net.ParseIP(raftResponse.DnsRecord.Ip),
		}, true
	} else {
		return dns.A{}, false
	}
}

func addRecordToRaftCluster(record dns.A) error {
	raftResponse := raftClient.SendDNSCommand(pb.DNSCommand{
		CommandType: AddRecord,
		Domain:      record.Hdr.Name,
		Ip:          record.A.String(),
		Ttl:         durationpb.New(time.Duration(record.Hdr.Ttl) * time.Second),
	})

	if !raftResponse.Success {
		return fmt.Errorf("failed to add record to Raft cluster: %s", record.Hdr.Name)
	}

	return nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)

	// Setup DNS server
	dns.HandleFunc(".", handleDNSRequest)
	server := &dns.Server{Addr: ":15353", Net: "udp"}
	slog.Info("Starting DNS server on port%s\n", "addr", server.Addr)

	raftClient = *raftclient.NewRaftClient(os.Args[1:], "127.0.0.1", 16353)

	// Start server
	err := server.ListenAndServe()
	if err != nil {
		slog.Error("Failed to start server", "error", err.Error())
	}
}
