package digitalocean

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/libdns/libdns"
)

// mockDomainsService is a mock implementation of godo.DomainsService
type mockDomainsService struct {
	// Mock return data
	records []godo.DomainRecord
	record  *godo.DomainRecord

	// Error to return (when testing error paths)
	err error

	// IDs passed to DeleteRecord, so a test can check which record was removed
	deletedIDs []int
}

func (m *mockDomainsService) List(ctx context.Context, opts *godo.ListOptions) ([]godo.Domain, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) Get(ctx context.Context, name string) (*godo.Domain, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) Create(ctx context.Context, domainCreateRequest *godo.DomainCreateRequest) (*godo.Domain, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) Delete(ctx context.Context, name string) (*godo.Response, error) {
	// Not used in our tests
	return nil, nil
}

func (m *mockDomainsService) Record(ctx context.Context, domain string, id int) (*godo.DomainRecord, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) RecordsByType(ctx context.Context, domain string, ofType string, opt *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) RecordsByName(ctx context.Context, domain, name string, opt *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) RecordsByTypeAndName(ctx context.Context, domain, ofType, name string, opt *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error) {
	// Not used in our tests
	return nil, nil, nil
}

func (m *mockDomainsService) Records(ctx context.Context, domain string, opts *godo.ListOptions) ([]godo.DomainRecord, *godo.Response, error) {
	if m.err != nil {
		return nil, &godo.Response{Response: &http.Response{StatusCode: 500}}, m.err
	}

	resp := &godo.Response{
		Response: &http.Response{StatusCode: 200},
		Links:    &godo.Links{},
	}

	// Simulate pagination by returning an empty list for any page > 1
	if opts != nil && opts.Page > 1 {
		return []godo.DomainRecord{}, resp, nil
	}

	return m.records, resp, nil
}

func (m *mockDomainsService) CreateRecord(ctx context.Context, domain string, createRequest *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error) {
	if m.err != nil {
		return nil, &godo.Response{Response: &http.Response{StatusCode: 500}}, m.err
	}

	record := &godo.DomainRecord{
		ID:   12345,
		Type: createRequest.Type,
		Name: createRequest.Name,
		Data: createRequest.Data,
		TTL:  createRequest.TTL,
	}

	return record, &godo.Response{Response: &http.Response{StatusCode: 201}}, nil
}

func (m *mockDomainsService) DeleteRecord(ctx context.Context, domain string, id int) (*godo.Response, error) {
	m.deletedIDs = append(m.deletedIDs, id)
	if m.err != nil {
		return &godo.Response{Response: &http.Response{StatusCode: 500}}, m.err
	}

	return &godo.Response{Response: &http.Response{StatusCode: 204}}, nil
}

func (m *mockDomainsService) EditRecord(ctx context.Context, domain string, id int, editRequest *godo.DomainRecordEditRequest) (*godo.DomainRecord, *godo.Response, error) {
	if m.err != nil {
		return nil, &godo.Response{Response: &http.Response{StatusCode: 500}}, m.err
	}

	record := &godo.DomainRecord{
		ID:   id,
		Type: editRequest.Type,
		Name: editRequest.Name,
		Data: editRequest.Data,
		TTL:  editRequest.TTL,
	}

	return record, &godo.Response{Response: &http.Response{StatusCode: 200}}, nil
}

func (m *mockDomainsService) GetRecord(ctx context.Context, domain string, id int) (*godo.DomainRecord, *godo.Response, error) {
	if m.err != nil {
		return nil, &godo.Response{Response: &http.Response{StatusCode: 500}}, m.err
	}

	return m.record, &godo.Response{Response: &http.Response{StatusCode: 200}}, nil
}

// mockClient is a mock implementation of the godo.Client
type mockClient struct {
	Domains mockDomainsService
}

// setupTest creates a Provider with a mock DigitalOcean client
func setupTest(records []godo.DomainRecord, err error) *Provider {
	mock := &mockDomainsService{
		records: records,
		err:     err,
	}

	provider := &Provider{
		APIToken: "test-token",
	}

	// Set the client directly
	provider.client = &godo.Client{
		Domains: mock,
	}

	return provider
}

func TestProvider_unFQDN(t *testing.T) {
	tests := []struct {
		name string
		fqdn string
		want string
	}{
		{
			name: "with trailing dot",
			fqdn: "example.com.",
			want: "example.com",
		},
		{
			name: "without trailing dot",
			fqdn: "example.com",
			want: "example.com",
		},
		{
			name: "subdomain with trailing dot",
			fqdn: "sub.example.com.",
			want: "sub.example.com",
		},
	}

	p := &Provider{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.unFQDN(tt.fqdn); got != tt.want {
				t.Errorf("Provider.unFQDN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_GetRecords(t *testing.T) {
	// Mock domain records to return
	mockRecords := []godo.DomainRecord{
		{
			ID:   1,
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
			TTL:  3600,
		},
		{
			ID:   2,
			Type: "CNAME",
			Name: "www",
			Data: "example.com",
			TTL:  1800,
		},
	}

	// Test successful call
	p := setupTest(mockRecords, nil)
	ctx := context.Background()

	records, err := p.GetRecords(ctx, "example.com.")

	if err != nil {
		t.Errorf("Provider.GetRecords() error = %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Provider.GetRecords() returned %d records, want 2", len(records))
	}

	// Verify first record
	if records[0].RR().Type != "A" || records[0].RR().Name != "test" ||
		records[0].RR().Data != "192.168.1.1" || records[0].(DNS).ID != "1" {
		t.Errorf("Provider.GetRecords()[0] = %v, want A record", records[0])
	}

	// Verify second record
	if records[1].RR().Type != "CNAME" || records[1].RR().Name != "www" ||
		records[1].RR().Data != "example.com" || records[1].(DNS).ID != "2" {
		t.Errorf("Provider.GetRecords()[1] = %v, want CNAME record", records[1])
	}

	// Test error case
	p = setupTest(nil, errors.New("API error"))

	_, err = p.GetRecords(ctx, "example.com.")
	if err == nil {
		t.Error("Provider.GetRecords() expected error, got nil")
	}
}

func TestProvider_AppendRecords(t *testing.T) {
	// Test record to append
	testRecord := libdns.RR{
		Type: "A",
		Name: "test",
		Data: "192.168.1.1",
		TTL:  3600 * time.Second,
	}

	// Test successful call
	p := setupTest(nil, nil)
	ctx := context.Background()

	appendedRecords, err := p.AppendRecords(ctx, "example.com.", []libdns.Record{testRecord})

	if err != nil {
		t.Errorf("Provider.AppendRecords() error = %v", err)
	}

	if len(appendedRecords) != 1 {
		t.Errorf("Provider.AppendRecords() returned %d records, want 1", len(appendedRecords))
	}

	// Verify the returned record
	if appendedRecords[0].RR().Type != testRecord.RR().Type ||
		appendedRecords[0].RR().Name != testRecord.RR().Name ||
		appendedRecords[0].RR().Data != testRecord.RR().Data ||
		appendedRecords[0].(DNS).ID != "12345" {
		t.Errorf("Provider.AppendRecords() record mismatch, got = %v, want Type=%s, Name=%s, Data=%s, ID=12345",
			appendedRecords[0], testRecord.RR().Type, testRecord.RR().Name, testRecord.RR().Data)
	}

	// Test error case
	p = setupTest(nil, errors.New("API error"))

	_, err = p.AppendRecords(ctx, "example.com.", []libdns.Record{testRecord})
	if err == nil {
		t.Error("Provider.AppendRecords() expected error, got nil")
	}
}

func TestProvider_DeleteRecords(t *testing.T) {
	// Test record to delete
	testRecord := DNS{
		ID: "1",
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
			TTL:  3600 * time.Second,
		},
	}

	// Test successful call
	p := setupTest(nil, nil)
	ctx := context.Background()

	deletedRecords, err := p.DeleteRecords(ctx, "example.com.", []libdns.Record{testRecord})

	if err != nil {
		t.Errorf("Provider.DeleteRecords() error = %v", err)
	}

	if len(deletedRecords) != 1 {
		t.Errorf("Provider.DeleteRecords() returned %d records, want 1", len(deletedRecords))
	}

	// Verify the returned record
	if deletedRecords[0].(DNS).ID != testRecord.ID {
		t.Errorf("Provider.DeleteRecords() record ID mismatch, got = %v, want = %v", deletedRecords[0].(DNS).ID, testRecord.ID)
	}

	// Test error case
	p = setupTest(nil, errors.New("API error"))

	_, err = p.DeleteRecords(ctx, "example.com.", []libdns.Record{testRecord})
	if err == nil {
		t.Error("Provider.DeleteRecords() expected error, got nil")
	}

	// Test error case with invalid ID
	p = setupTest(nil, nil)

	invalidIDRecord := DNS{
		ID: "invalid", // Non-numeric ID
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
		},
	}

	_, err = p.DeleteRecords(ctx, "example.com.", []libdns.Record{invalidIDRecord})
	if err == nil {
		t.Error("Provider.DeleteRecords() expected error for invalid ID, got nil")
	}
}

func TestProvider_DeleteRecords_withoutID(t *testing.T) {
	// certmagic hands back a plain record when it cleans up an ACME challenge.
	// The provider has to find that record in the zone rather than fail on the
	// missing ID, which used to leave every _acme-challenge record behind.
	zone := []godo.DomainRecord{
		{ID: 7, Type: "TXT", Name: "_acme-challenge", Data: "other-token", TTL: 30},
		{ID: 42, Type: "TXT", Name: "_acme-challenge", Data: "the-token", TTL: 30},
	}
	p := setupTest(zone, nil)
	ctx := context.Background()

	plain := libdns.TXT{Name: "_acme-challenge", Text: "the-token", TTL: 30 * time.Second}
	deleted, err := p.DeleteRecords(ctx, "example.com.", []libdns.Record{plain})
	if err != nil {
		t.Fatalf("Provider.DeleteRecords() error = %v", err)
	}
	if len(deleted) != 1 {
		t.Errorf("Provider.DeleteRecords() returned %d records, want 1", len(deleted))
	}
	mock := p.client.Domains.(*mockDomainsService)
	if len(mock.deletedIDs) != 1 || mock.deletedIDs[0] != 42 {
		t.Errorf("Provider.DeleteRecords() deleted IDs = %v, want [42]", mock.deletedIDs)
	}

	// Quoted TXT data still matches the unquoted record in the zone
	p = setupTest(zone, nil)
	quoted := libdns.TXT{Name: "_acme-challenge", Text: "\"the-token\""}
	if _, err := p.DeleteRecords(ctx, "example.com.", []libdns.Record{quoted}); err != nil {
		t.Errorf("Provider.DeleteRecords() with quoted data error = %v", err)
	}

	// A record that is not in the zone is an error, not a silent no-op
	p = setupTest(zone, nil)
	missing := libdns.TXT{Name: "_acme-challenge", Text: "unknown-token"}
	if _, err := p.DeleteRecords(ctx, "example.com.", []libdns.Record{missing}); err == nil {
		t.Error("Provider.DeleteRecords() expected error for a record missing from the zone, got nil")
	}
}

func TestProvider_SetRecords(t *testing.T) {
	// Test record to set
	testRecord := DNS{
		ID: "1",
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
			TTL:  3600 * time.Second,
		},
	}

	// Test successful call
	p := setupTest(nil, nil)
	ctx := context.Background()

	setRecords, err := p.SetRecords(ctx, "example.com.", []libdns.Record{testRecord})

	if err != nil {
		t.Errorf("Provider.SetRecords() error = %v", err)
	}

	if len(setRecords) != 1 {
		t.Errorf("Provider.SetRecords() returned %d records, want 1", len(setRecords))
	}

	// Verify the returned record
	if setRecords[0].(DNS).ID != testRecord.ID {
		t.Errorf("Provider.SetRecords() record ID mismatch, got = %v, want = %v", setRecords[0].(DNS).ID, testRecord.ID)
	}

	// Test error case
	p = setupTest(nil, errors.New("API error"))

	_, err = p.SetRecords(ctx, "example.com.", []libdns.Record{testRecord})
	if err == nil {
		t.Error("Provider.SetRecords() expected error, got nil")
	}

	// Test error case with invalid ID
	p = setupTest(nil, nil)

	invalidIDRecord := DNS{
		ID: "invalid", // Non-numeric ID
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
		},
	}

	_, err = p.SetRecords(ctx, "example.com.", []libdns.Record{invalidIDRecord})
	if err == nil {
		t.Error("Provider.SetRecords() expected error for invalid ID, got nil")
	}
}

func TestProvider_getClient(t *testing.T) {
	// Test client initialization
	p := &Provider{
		APIToken: "test-token",
	}

	// Client should start as nil
	if p.client != nil {
		t.Error("Provider.client should be nil initially")
	}

	// Call getClient
	err := p.getClient()

	if err != nil {
		t.Errorf("Provider.getClient() error = %v", err)
	}

	// Client should now be initialized
	if p.client == nil {
		t.Error("Provider.client should be initialized after getClient()")
	}
}
