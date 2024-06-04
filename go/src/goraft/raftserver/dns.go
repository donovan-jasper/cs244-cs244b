package raftserver

import (
	"fmt"
	sync "sync"
	"time"

	pb "raftprotos"

	"google.golang.org/protobuf/proto"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

type DNSModule struct {
	dnsRecords map[string]pb.DNSRecord
	dnsMu      sync.Mutex
}

const (
	AddRecord = iota
	DeleteRecord
	ReadRecord
)

func NewDNSModule() *DNSModule {
	return &DNSModule{
		dnsRecords: make(map[string]pb.DNSRecord),
	}
}

func (dm *DNSModule) Apply(command string) []byte {
	dm.dnsMu.Lock()
	defer dm.dnsMu.Unlock()

	dnsResponse := &pb.DNSResponse{}

	dnsCommand := &pb.DNSCommand{}
	if err := proto.Unmarshal([]byte(command), dnsCommand); err != nil {
		fmt.Println("Failed to unmarshal log entry:", err)
		dnsResponse.Success = false
		return serializeDNSResponse(dnsResponse)
	}

	// Lazy timeout mechanism: delete expired records
	for domain := range dm.dnsRecords {
		if time.Since(dm.dnsRecords[domain].Added.AsTime()) > dm.dnsRecords[domain].Ttl.AsDuration() {
			delete(dm.dnsRecords, domain)
		}
	}

	switch dnsCommand.CommandType {
	case AddRecord:
		dm.dnsRecords[dnsCommand.Domain] = pb.DNSRecord{
			Hostname: dnsCommand.Hostname,
			Ip:       dnsCommand.Ip,
			Ttl:      dnsCommand.Ttl,
			Added:    timestamppb.New(time.Now()),
		}
		dnsResponse.Success = true
	case DeleteRecord:
		delete(dm.dnsRecords, dnsCommand.Domain)
		dnsResponse.Success = true
	case ReadRecord:
		record, ok := dm.dnsRecords[dnsCommand.Domain]
		// If the key exists
		if ok {
			dnsResponse.DnsRecord = &record
			dnsResponse.Success = true
		} else {
			dnsResponse.Success = false
		}

	}

	return serializeDNSResponse(dnsResponse)
}

func serializeDNSResponse(response *pb.DNSResponse) []byte {
	serializedResponse, err := proto.Marshal(response)
	if err != nil {
		fmt.Println(err)
	}
	return serializedResponse
}
