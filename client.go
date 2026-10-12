package digitalocean

import (
	"context"
	"fmt"
	"strconv"
	"sync"

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

func (p *Provider) getDNSEntries(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	return p.listDNSEntries(ctx, zone)
}

// listDNSEntries pages through every record in the zone. The caller holds the mutex.
func (p *Provider) listDNSEntries(ctx context.Context, zone string) ([]libdns.Record, error) {
	opt := &godo.ListOptions{}
	var records []libdns.Record
	for {
		domains, resp, err := p.client.Domains.Records(ctx, zone, opt)
		if err != nil {
			return records, err
		}

		for _, entry := range domains {
			record := fromGodo(entry)
			records = append(records, record)
		}

		// if we are at the last page, break out the for loop
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return records, err
		}

		// set the page we want for the next request
		opt.Page = page + 1
	}

	return records, nil
}

// idFromRecord returns the DigitalOcean ID of a record. Records this provider
// created carry it, but callers do not always hand those back: certmagic passes
// a plain RR when it cleans up an ACME challenge, which used to fail here with
// strconv.Atoi: parsing "" and leave the _acme-challenge record behind. Without
// an ID, match the record against the zone by name, type and data. The caller
// holds the mutex.
func (p *Provider) idFromRecord(ctx context.Context, zone string, record libdns.Record) (int, error) {
	if dns, ok := record.(DNS); ok && dns.ID != "" {
		return strconv.Atoi(dns.ID)
	}
	want := record.RR()
	entries, err := p.listDNSEntries(ctx, zone)
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		dns, ok := entry.(DNS)
		if !ok {
			continue
		}
		have := dns.Record
		if have.Name == want.Name && have.Type == want.Type && unquote(have.Data) == unquote(want.Data) {
			return strconv.Atoi(dns.ID)
		}
	}
	return 0, fmt.Errorf("no %s record %q with data %q in zone %s", want.Type, want.Name, want.Data, zone)
}

// unquote strips one pair of surrounding double quotes, which TXT data may or
// may not carry depending on who produced it.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func (p *Provider) addDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	entry := recordToGoDo(record)

	rec, _, err := p.client.Domains.CreateRecord(ctx, zone, &entry)
	if err != nil {
		return record, err
	}

	return fromRecord(record, strconv.Itoa(rec.ID)), nil
}

func (p *Provider) removeDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	id, err := p.idFromRecord(ctx, zone, record)
	if err != nil {
		return record, err
	}

	_, err = p.client.Domains.DeleteRecord(ctx, zone, id)
	if err != nil {
		return record, err
	}

	return record, nil
}

func (p *Provider) updateDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	id, err := p.idFromRecord(ctx, zone, record)
	if err != nil {
		return record, err
	}

	entry := recordToGoDo(record)

	_, _, err = p.client.Domains.EditRecord(ctx, zone, id, &entry)
	if err != nil {
		return record, err
	}

	return record, nil
}
