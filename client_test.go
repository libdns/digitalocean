package digitalocean

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/libdns/libdns"
)

func TestClient_getDNSEntries(t *testing.T) {
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

	records, err := p.getDNSEntries(ctx, "example.com")

	if err != nil {
		t.Errorf("Client.getDNSEntries() error = %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Client.getDNSEntries() returned %d records, want 2", len(records))
	}

	// Verify first record
	if records[0].RR().Type != "A" || records[0].RR().Name != "test" ||
		records[0].RR().Data != "192.168.1.1" || records[0].(DNS).ID != "1" {
		t.Errorf("Client.getDNSEntries()[0] = %v, want A record", records[0])
	}

	// Verify second record
	if records[1].RR().Type != "CNAME" || records[1].RR().Name != "www" ||
		records[1].RR().Data != "example.com" || records[1].(DNS).ID != "2" {
		t.Errorf("Client.getDNSEntries()[1] = %v, want CNAME record", records[1])
	}

	// Test error case
	p = setupTest(nil, errors.New("API error"))

	_, err = p.getDNSEntries(ctx, "example.com")
	if err == nil {
		t.Error("Client.getDNSEntries() expected error, got nil")
	}
}

func TestClient_getDNSEntries_ExtraFields(t *testing.T) {
	// DigitalOcean returning  extra fields outside of .Data
	mockRecords := []godo.DomainRecord{
		{ID: 1, Type: "MX", Name: "@", Data: "mail.example.com", Priority: 10, TTL: 3600},
		{ID: 2, Type: "SRV", Name: "_sip._tcp", Data: "sip.example.com", Priority: 10, Weight: 20, Port: 5060, TTL: 3600},
		{ID: 3, Type: "CAA", Name: "@", Data: "letsencrypt.org", Flags: 0, Tag: "issue", TTL: 3600},
		{ID: 4, Type: "CAA", Name: "@", Data: "mailto:security@example.com", Flags: 128, Tag: "iodef", TTL: 3600},
		{ID: 5, Type: "TXT", Name: "txt", Data: "hello world", TTL: 3600},
	}

	p := setupTest(mockRecords, nil)
	records, err := p.getDNSEntries(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("getDNSEntries() error: %v", err)
	}

	wantData := []string{
		"10 mail.example.com",
		"10 20 5060 sip.example.com",
		`0 issue "letsencrypt.org"`,
		`128 iodef "mailto:example@example.com"`,
		"hello world",
	}
	if len(records) != len(wantData) {
		t.Fatalf("getDNSEntries() returned %d records instead of %d", len(records), len(wantData))
	}
	for i, want := range wantData {
		if got := records[i].RR().Data; got != want {
			t.Errorf("getDNSEntries()[%d].Data = %q instead of %q", i, got, want)
		}
	}

	// Rebuilt data must round-trip into libdns-typed records
	mx, err := records[0].RR().Parse()
	if err != nil {
		t.Fatalf("MX Parse() error: %v", err)
	}
	if got := mx.(libdns.MX); got.Preference != 10 || got.Target != "mail.example.com" {
		t.Errorf("MX Parse() = %+v", got)
	}

	srv, err := records[1].RR().Parse()
	if err != nil {
		t.Fatalf("SRV Parse() error: %v", err)
	}
	if got := srv.(libdns.SRV); got.Service != "sip" || got.Transport != "tcp" ||
		got.Priority != 10 || got.Weight != 20 || got.Port != 5060 || got.Target != "sip.example.com" {
		t.Errorf("SRV Parse() = %+v", got)
	}

	caa, err := records[3].RR().Parse()
	if err != nil {
		t.Fatalf("CAA Parse() error: %v", err)
	}
	if got := caa.(libdns.CAA); got.Flags != 128 || got.Tag != "iodef" || got.Value != "mailto:example@example.com" {
		t.Errorf("CAA Parse() = %+v", got)
	}
}

func TestDataFromGodo_EmptyRecords(t *testing.T) {
	// Records without data must NOT produce fields like '0 '
	for _, entry := range []godo.DomainRecord{
		{Type: "MX"},
		{Type: "SRV"},
		{Type: "CAA"},
	} {
		if got := dataFromGodo(entry); got != "" {
			t.Errorf("dataFromGodo(empty %s) = %q instead of empty string", entry.Type, got)
		}
	}
}

func TestClient_addDNSEntry(t *testing.T) {
	// Test record to add
	testRecord := libdns.RR{
		Type: "A",
		Name: "test",
		Data: "192.168.1.1",
		TTL:  3600 * time.Second,
	}

	// Test successful call
	p := setupTest(nil, nil)
	ctx := context.Background()

	resultRecord, err := p.addDNSEntry(ctx, "example.com", testRecord)

	if err != nil {
		t.Errorf("Client.addDNSEntry() error = %v", err)
	}

	// Verify the returned record
	if resultRecord.RR().Type != testRecord.Type ||
		resultRecord.RR().Name != testRecord.Name ||
		resultRecord.RR().Data != testRecord.Data ||
		resultRecord.(DNS).ID != "12345" {
		t.Errorf("Client.addDNSEntry() record mismatch, got = %v, want Type=%s, Name=%s, Data=%s, ID=12345",
			resultRecord, testRecord.RR().Type, testRecord.RR().Name, testRecord.RR().Data)
	}

	// Test error case
	p = setupTest(nil, errors.New("API error"))

	_, err = p.addDNSEntry(ctx, "example.com", testRecord)
	if err == nil {
		t.Error("Client.addDNSEntry() expected error, got nil")
	}
}

func TestClient_removeDNSEntry(t *testing.T) {
	// Test record to delete
	testRecord := DNS{
		ID: "1",
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
		},
	}

	// Test successful call
	p := setupTest(nil, nil)
	ctx := context.Background()

	resultRecord, err := p.removeDNSEntry(ctx, "example.com", testRecord)

	if err != nil {
		t.Errorf("Client.removeDNSEntry() error = %v", err)
	}

	// Verify the ID was preserved
	if resultRecord.(DNS).ID != testRecord.ID {
		t.Errorf("Client.removeDNSEntry() ID mismatch, got = %v, want = %v", resultRecord.(DNS).ID, testRecord.ID)
	}

	// Test error case - API error
	p = setupTest(nil, errors.New("API error"))

	_, err = p.removeDNSEntry(ctx, "example.com", testRecord)
	if err == nil {
		t.Error("Client.removeDNSEntry() expected error, got nil")
	}

	// Test error case - invalid ID
	p = setupTest(nil, nil)
	invalidIDRecord := DNS{
		ID: "invalid", // Non-numeric ID
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
		},
	}

	_, err = p.removeDNSEntry(ctx, "example.com", invalidIDRecord)
	if err == nil {
		t.Error("Client.removeDNSEntry() expected error for invalid ID, got nil")
	}
}

func TestClient_updateDNSEntry(t *testing.T) {
	// Test record to update
	testRecord := DNS{
		ID: "1",
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.2", // Updated IP
			TTL:  7200 * time.Second,
		},
	}

	// Test successful call
	p := setupTest(nil, nil)
	ctx := context.Background()

	resultRecord, err := p.updateDNSEntry(ctx, "example.com", testRecord)

	if err != nil {
		t.Errorf("Client.updateDNSEntry() error = %v", err)
	}

	// Verify the record was preserved
	if resultRecord.(DNS).ID != testRecord.ID ||
		resultRecord.RR().Type != testRecord.RR().Type ||
		resultRecord.RR().Name != testRecord.RR().Name ||
		resultRecord.RR().Data != testRecord.RR().Data {
		t.Errorf("Client.updateDNSEntry() record mismatch, got = %v, want = %v", resultRecord, testRecord)
	}

	// Test error case - API error
	p = setupTest(nil, errors.New("API error"))

	_, err = p.updateDNSEntry(ctx, "example.com", testRecord)
	if err == nil {
		t.Error("Client.updateDNSEntry() expected error, got nil")
	}

	// Test error case - invalid ID
	p = setupTest(nil, nil)
	invalidIDRecord := DNS{
		ID: "invalid", // Non-numeric ID
		Record: libdns.RR{
			Type: "A",
			Name: "test",
			Data: "192.168.1.1",
		},
	}

	_, err = p.updateDNSEntry(ctx, "example.com", invalidIDRecord)
	if err == nil {
		t.Error("Client.updateDNSEntry() expected error for invalid ID, got nil")
	}
}
