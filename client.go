package digitalocean

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/digitalocean/godo"
	"github.com/libdns/libdns"
)

type Client struct {
	client *godo.Client
	mutex  sync.Mutex
}

// ProviderData attach custom data to each libdns.RR
type ProviderData struct {
	ID int
}

func (p *Provider) getClient() error {
	if p.client == nil {
		p.client = godo.NewFromToken(p.APIToken)
	}

	return nil
}

func (p *Provider) getDNSEntries(ctx context.Context, zone string) ([]libdns.Record, []ProviderData, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	opt := &godo.ListOptions{}
	var records []libdns.Record
	var providerDatas []ProviderData

	for {
		domains, resp, err := p.client.Domains.Records(ctx, zone, opt)
		if err != nil {
			return records, nil, err
		}

		for _, entry := range domains {
			record := libdns.RR{
				Name: entry.Name,
				Data: entry.Data,
				Type: entry.Type,
				TTL:  time.Duration(entry.TTL) * time.Second,
			}
			records = append(records, record)
			providerDatas = append(providerDatas, ProviderData{ID: entry.ID})
		}

		// if we are at the last page, break out the for loop
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return records, nil, err
		}

		// set the page we want for the next request
		opt.Page = page + 1
	}

	return records, providerDatas, nil
}

func (p *Provider) getDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.Record, *ProviderData, error) {
	p.getClient()
	rr := record.RR()

	foundRecords, _, err := p.client.Domains.RecordsByTypeAndName(ctx, zone, rr.Type, libdns.AbsoluteName(rr.Name, zone), &godo.ListOptions{})
	if err != nil {
		return rr, nil, err
	}

	if len(foundRecords) == 0 {
		return rr, nil, fmt.Errorf("%w: %s %s %s", ErrRecordNotFound, zone, rr.Type, rr.Name)
	}

	if len(foundRecords) > 1 {
		return rr, nil, fmt.Errorf("found multiple records for %s %s %s - this is not supposed to happend", zone, rr.Type, rr.Name)
	}

	return rr, &ProviderData{ID: foundRecords[0].ID}, nil
}

func (p *Provider) addDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.RR, *ProviderData, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()
	rr := record.RR()

	entry := godo.DomainRecordEditRequest{
		Name: rr.Name,
		Data: rr.Data,
		Type: rr.Type,
		TTL:  int(rr.TTL.Seconds()),
	}

	rec, _, err := p.client.Domains.CreateRecord(ctx, zone, &entry)
	if err != nil {
		return rr, nil, err
	}

	return rr, &ProviderData{ID: rec.ID}, nil
}

func (p *Provider) removeDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.RR, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()
	rr := record.RR()

	_, providerData, err := p.getDNSEntry(ctx, zone, record)
	if err != nil {
		return rr, err
	}

	_, err = p.client.Domains.DeleteRecord(ctx, zone, providerData.ID)
	if err != nil {
		return rr, err
	}

	return rr, nil
}

func (p *Provider) updateDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.RR, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()
	rr := record.RR()

	_, providerData, err := p.getDNSEntry(ctx, zone, record)
	if err != nil {
		return rr, err
	}

	entry := godo.DomainRecordEditRequest{
		Name: rr.Name,
		Data: rr.Data,
		Type: rr.Type,
		TTL:  int(rr.TTL.Seconds()),
	}

	_, _, err = p.client.Domains.EditRecord(ctx, zone, providerData.ID, &entry)
	if err != nil {
		return rr, err
	}

	return rr, nil
}
