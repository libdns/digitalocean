package digitalocean

import (
	"context"
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
		client := godo.NewFromToken(p.APIToken)

		if p.APIBase != "" {
			base := p.APIBase
			if !strings.HasSuffix(base, "/") {
				base += "/"
			}

			if err := godo.SetBaseURL(base)(client); err != nil {
				return err
			}
		}

		p.client = client
	}

	return nil
}

func (p *Provider) getDNSEntries(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.getClient(); err != nil {
		return nil, err
	}

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

	if err := p.getClient(); err != nil {
		return record, err
	}

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

	if err := p.getClient(); err != nil {
		return record, err
	}

	// Get ID from dns record
	id, err := idFromRecord(record)
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

	if err := p.getClient(); err != nil {
		return record, err
	}

	// Get ID from dns record
	id, err := idFromRecord(record)
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
