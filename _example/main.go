package main

import (
	"context"
	"fmt"
	"os"

	"github.com/onaci/digitalocean"
)

func main() {
	provider := digitalocean.Provider{APIToken: os.Getenv("DO_AUTH_TOKEN")}

	records, err := provider.GetRecords(context.TODO(), os.Getenv("STACKDOMAIN"))
	if err != nil {
		fmt.Println("ERROR")
		fmt.Println(err.Error())
	}

	for _, record := range records {
		fmt.Println(record.Name)
		fmt.Println(record.Value)
	}
}
