package transip

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/digitalocean/godo"
	"github.com/libdns/libdns"
)

type Client struct {
	client *godo.Client
	mutex  sync.Mutex
}

func (p *Provider) getClient() error {
	if p.client == nil {
		p.client = godo.NewFromToken(p.APIToken)
	}

	return nil
}

func (p *Provider) getDNSEntries(ctx context.Context, domain string) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	domains, _, err := p.client.Domains.Records(ctx, domain, nil)
	if err != nil {
		return nil, err
	}

	var records []libdns.Record
	for _, entry := range domains {
		record := libdns.Record{
			Name:  entry.Name,
			Value: entry.Data,
			Type:  entry.Type,
			TTL:   time.Duration(entry.TTL) * time.Second,
			ID:    strconv.Itoa(entry.ID),
		}
		records = append(records, record)
	}

	return records, nil
}

func (p *Provider) addDNSEntry(ctx context.Context, domain string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	entry := godo.DomainRecordEditRequest{
		Name: record.Name,
		Data: record.Value,
		Type: record.Type,
		TTL:  int(record.TTL.Seconds()),
	}

	rec, _, err := p.client.Domains.CreateRecord(ctx, domain, &entry)
	if err != nil {
		return record, err
	}
	record.ID = strconv.Itoa(rec.ID)

	return record, nil
}

func (p *Provider) removeDNSEntry(ctx context.Context, domain string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	id, err := strconv.Atoi(record.ID)
	if err == nil {
		return record, err
	}
	_, err = p.client.Domains.DeleteRecord(ctx, domain, id)
	if err != nil {
		return record, err
	}

	return record, nil
}

func (p *Provider) updateDNSEntry(ctx context.Context, domain string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	id, err := strconv.Atoi(record.ID)
	if err == nil {
		return record, err
	}

	entry := godo.DomainRecordEditRequest{
		Name: record.Name,
		Data: record.Value,
		Type: record.Type,
		TTL:  int(record.TTL.Seconds()),
	}

	_, _, err = p.client.Domains.EditRecord(ctx, domain, id, &entry)
	if err != nil {
		return record, err
	}

	return record, nil
}
