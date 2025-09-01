package digitalocean

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
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

func (p *Provider) removeDNSEntry(ctx context.Context, zone string, id int, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	_, err := p.client.Domains.DeleteRecord(ctx, zone, id)
	if err != nil {
		return record, err
	}

	return fromRecord(record, strconv.Itoa(id)), nil
}

func (p *Provider) updateDNSEntry(ctx context.Context, zone string, id int, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	entry := recordToGoDo(record)

	_, _, err := p.client.Domains.EditRecord(ctx, zone, id, &entry)
	if err != nil {
		return record, err
	}

	return fromRecord(record, strconv.Itoa(id)), nil
}

func (p *Provider) idFromRecord(ctx context.Context, zone string, record libdns.Record) (int, error) {
	// Try to get id from underlying DNS struct
	id, err := idFromRecord(record)
	if err == nil {
		return id, err
	}

	// Failed to get ID from record, query the zone instead
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	// RecordsByTypeAndName requires the name to be a fqdn https://docs.digitalocean.com/reference/api/digitalocean/#tag/Domain-Records/operation/domains_list_records
	fqdn := strings.Join([]string{record.RR().Name, zone}, ".")
	records, _, err := p.client.Domains.RecordsByTypeAndName(ctx, zone, record.RR().Type, fqdn, &godo.ListOptions{})
	if err != nil {
		return id, err
	}
	if len(records) == 0 {
		return id, fmt.Errorf("record not found: %s %s %s", zone, record.RR().Type, record.RR().Name)
	}
	if len(records) > 1 {
		log.Printf("libdns/digitalocean: warning: found multiple records with the same name and type, only the first record will be used")
	}

	return records[0].ID, nil
}
