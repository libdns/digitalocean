# DigitalOcean for `libdns`

This package implements the libdns interfaces for the [DigitalOcean API](https://developers.digitalocean.com/documentation/v2/#domains) (using the Go implementation from: https://github.com/digitalocean/godo)

## Authenticating

To authenticate you need to supply a DigitalOcean API token.

## Example

Here's a minimal example of how to get all your DNS records using this `libdns` provider

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/onaci/digitalocean"
)

func main() {
	provider := digitalocean.Provider{APIToken: os.Getenv("DO_AUTH_TOKEN")}
	provider.NewSession()

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
```
