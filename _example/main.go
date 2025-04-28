package main

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/libdns/digitalocean"
	"github.com/libdns/libdns"
)

func main() {
	token := os.Getenv("DO_AUTH_TOKEN")
	if token == "" {
		fmt.Printf("DO_AUTH_TOKEN not set\n")
		return
	}
	zone := os.Getenv("ZONE")
	if zone == "" {
		fmt.Printf("ZONE not set\n")
		return
	}
	provider := digitalocean.Provider{APIToken: token}

	records, err := provider.GetRecords(context.TODO(), zone)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
	}

	for _, rr := range records {
		fmt.Printf("%s %s %s\n", rr.RR().Type, rr.RR().Name, rr.RR().Data)
	}

	err = TXT_Test(provider, zone)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err.Error())
	}
	err = A_Test(provider, zone)
	if err != nil {
		fmt.Println("ERROR: %s\n", err.Error())
	}
}

func TXT_Test(provider digitalocean.Provider, zone string) error {
	testName := "libdns-txt-test"

	fmt.Printf("Create or update entry for %s\n", testName)
	_, err := provider.SetRecords(context.TODO(), zone, []libdns.Record{libdns.TXT{
		Name: testName,
		Text: fmt.Sprintf("Replacement test entry created by libdns %s", time.Now()),
		TTL:  time.Duration(30) * time.Second,
	}})

	if err != nil {
		return err
	}

	fmt.Printf("Creating new entry for %s\n", testName)
	_, err = provider.AppendRecords(context.TODO(), zone, []libdns.Record{libdns.TXT{
		Name: testName,
		Text: fmt.Sprintf("This is a test entry created by libdns %s", time.Now()),
		TTL:  time.Duration(30) * time.Second,
	}})

	if err != nil {
		return err
	}

	return nil
}

func A_Test(provider digitalocean.Provider, zone string) error {
	testName := "libdns-a-test"
	ip, _ := netip.ParseAddr("127.0.0.1")

	fmt.Printf("Create or Update new entry for %s\n", testName)
	_, err := provider.SetRecords(context.TODO(), zone, []libdns.Record{libdns.Address{
		Name: testName,
		IP:   ip,
		TTL:  time.Duration(30) * time.Second,
	}})
	return err
}
