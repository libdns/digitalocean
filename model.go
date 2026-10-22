package digitalocean

import (
	"strconv"
	"time"

	"github.com/digitalocean/godo"
	"github.com/libdns/libdns"
)

// DNS custom struct that implements the libdns.Record interface and keeps the ID field used internally
type DNS struct {
	Record libdns.RR
	ID     string
}

func (d DNS) RR() libdns.RR {
	return d.Record
}

// fromRecord creates a dns struct from a libdns.Record, with an optional ID
func fromRecord(record libdns.Record, id string) DNS {
	rr := record.RR()
	return DNS{
		Record: rr,
		ID:     id,
	}
}

// fromGodo creates a dns struct from godo.DomainRecord
func fromGodo(entry godo.DomainRecord) DNS {
	return DNS{
		Record: libdns.RR{
			Name: entry.Name,
			Data: dataFromGodo(entry),
			Type: entry.Type,
			TTL:  time.Duration(entry.TTL) * time.Second,
		},
		ID: strconv.Itoa(entry.ID),
	}
}

// dataFromGodo rebuilds the zone file representation of the record data because DO returns priority, weight, port etc. in separate fields
func dataFromGodo(entry godo.DomainRecord) string {
	switch entry.Type {
	case "MX":
		return libdns.MX{
			Preference: uint16(entry.Priority),
			Target:     entry.Data,
		}.RR().Data
	case "SRV":
		return libdns.SRV{
			Priority: uint16(entry.Priority),
			Weight:   uint16(entry.Weight),
			Port:     uint16(entry.Port),
			Target:   entry.Data,
		}.RR().Data
	case "CAA":
		return libdns.CAA{
			Flags: uint8(entry.Flags),
			Tag:   entry.Tag,
			Value: entry.Data,
		}.RR().Data
	default:
		return entry.Data
	}
}

// recordToGoDo converts a libdns.RR to the DigitalOcean API format
func recordToGoDo(record libdns.Record) godo.DomainRecordEditRequest {
	rr := record.RR()
	return godo.DomainRecordEditRequest{
		Name: rr.Name,
		Data: rr.Data,
		Type: rr.Type,
		TTL:  int(rr.TTL.Seconds()),
	}
}

// idFromRecord get the ID from a libdns.Record
func idFromRecord(record libdns.Record) (int, error) {
	var raw string
	if dns, ok := record.(DNS); ok {
		raw = dns.ID
	}
	id, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return id, nil
}
