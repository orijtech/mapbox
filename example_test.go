package mapbox_test

import (
	"fmt"
	"log"

	"github.com/odeke-em/mapbox"
)

func Example_client_LookupPlace() {
	client, err := mapbox.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	// Time for some tacos, let's lookup that new spot.
	match, err := client.LookupPlace("Tacquerias El Farolito")
	if err != nil {
		log.Fatal(err)
	}

	for i, feat := range match.Features {
		fmt.Printf("Match: #%d Name: %q Relevance: %v Center: %#v\n", i, feat.PlaceName, feat.Relevance, feat.Center)
		for j, ctx := range feat.Context {
			fmt.Printf("\tContext: #%d:: %#v\n", j, ctx)
		}
		fmt.Printf("\n\n")
	}
}

func Example_client_LookupLatLon() {
	client, err := mapbox.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.LookupLatLon(38.8971, -77.0366)
	if err != nil {
		log.Fatal(err)
	}

	for i, feat := range resp.Features {
		fmt.Printf("Match: #%d Name: %q Relevance: %v Center: %#v\n", i, feat.PlaceName, feat.Relevance, feat.Center)
		for j, ctx := range feat.Context {
			fmt.Printf("\tContext: #%d:: %#v\n", j, ctx)
		}
		fmt.Printf("\n\n")
	}
}
