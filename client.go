package digitalocean

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/digitalocean/godo"
	"github.com/libdns/libdns"
)

// normalizeName converts a libdns record name to the DigitalOcean record-name
// form used in API responses. DO stores names RELATIVE to the zone (e.g. "@" for
// the apex, "_acme-challenge" for _acme-challenge.<zone>), and may return them
// without the trailing dot. libdns may hand us an absolute or zone-suffixed name,
// so strip a trailing dot and the ".<zone>" suffix to match DO's representation.
func normalizeName(name, zone string) string {
	z := strings.TrimSuffix(zone, ".")
	n := strings.TrimSuffix(name, ".")
	if n == z {
		return "@"
	}
	n = strings.TrimSuffix(n, "."+z)
	return n
}

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

// listDNSEntries fetches all DNS records for a zone. UNLOCKED: callers that
// already hold p.mutex (removeDNSEntry/updateDNSEntry) use this to avoid a
// re-entrant deadlock; getDNSEntries is the locked public path.
func (p *Provider) listDNSEntries(ctx context.Context, zone string) ([]godo.DomainRecord, error) {
	p.getClient()

	opt := &godo.ListOptions{}
	var all []godo.DomainRecord
	for {
		records, resp, err := p.client.Domains.Records(ctx, zone, opt)
		if err != nil {
			return all, err
		}
		all = append(all, records...)
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			return all, err
		}
		opt.Page = page + 1
	}
	return all, nil
}

// resolveID returns the DigitalOcean numeric record ID for a record. It uses the
// ID carried on the record when present (the common case for records we returned
// from Append/Get), and otherwise LOOKS IT UP by matching name+type+data against
// the zone. The lookup path is essential for ACME DNS-01: the solver deletes the
// temporary _acme-challenge TXT by passing a plain libdns.RR with NO ID, so the
// upstream `idFromRecord` did strconv.Atoi("") and failed — leaving the TXT record
// behind and breaking the challenge. (Upstream is unmaintained; this is our fix.)
func (p *Provider) resolveID(ctx context.Context, zone string, record libdns.Record) (int, error) {
	if id, err := idFromRecord(record); err == nil && id != 0 {
		return id, nil
	}

	rr := record.RR()
	name := normalizeName(rr.Name, zone)
	entries, err := p.listDNSEntries(ctx, zone)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.Type == rr.Type && e.Name == name && e.Data == rr.Data {
			return e.ID, nil
		}
	}
	// Fall back to matching on name+type only (data can differ slightly, e.g. TXT
	// quoting), so cleanup still finds a unique challenge record.
	for _, e := range entries {
		if e.Type == rr.Type && e.Name == name {
			return e.ID, nil
		}
	}
	return 0, fmt.Errorf("digitalocean: no record found to resolve ID for %s %s in zone %s", rr.Type, rr.Name, zone)
}

func (p *Provider) removeDNSEntry(ctx context.Context, zone string, record libdns.Record) (libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.getClient()

	// Resolve the DO record ID (by ID on the record, else by name/type/data) —
	// the ACME solver deletes by name with no ID, which the upstream code couldn't.
	id, err := p.resolveID(ctx, zone, record)
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

	// Resolve the DO record ID (by ID on the record, else by name/type/data).
	id, err := p.resolveID(ctx, zone, record)
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
