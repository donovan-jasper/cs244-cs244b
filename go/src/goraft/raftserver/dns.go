package raftserver

import (
	"fmt"
	"log/slog"
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

	slog.Info("Before apply, DNS record contents", "records", dm.dnsRecords)

	switch dnsCommand.CommandType {
	case AddRecord:
		fmt.Println("Adding record with domain", dnsCommand.Domain)
		dm.dnsRecords[dnsCommand.Domain] = pb.DNSRecord{
			Hostname: dnsCommand.Hostname,
			Ip:       dnsCommand.Ip,
			Ttl:      dnsCommand.Ttl,
			Added:    timestamppb.New(time.Now()),
		}
		dnsResponse.Success = true
	case DeleteRecord:
		fmt.Println("Deleting record with domain", dnsCommand.Domain)
		delete(dm.dnsRecords, dnsCommand.Domain)
		dnsResponse.Success = true
	case ReadRecord:
		fmt.Println("Reading record with domain", dnsCommand.Domain)
		record, ok := dm.dnsRecords[dnsCommand.Domain]
		// If the key exists
		if ok {
			dnsResponse.DnsRecord = &record
			dnsResponse.Success = true
		} else {
			fmt.Println("Record not found, read failed")
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
