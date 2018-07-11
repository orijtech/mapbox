package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/orijtech/mapbox"
)

func main() {
	var lat, lon float64
	flag.Float64Var(&lat, "lat", 38.8971, "latitude")
	flag.Float64Var(&lon, "lon", -77.0366, "longitude")
	flag.Parse()

	client, err := mapbox.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.LookupLatLon(lat, lon)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("#%v\n", resp)
	for i, feature := range resp.Features {
		fmt.Printf("Feature: #%d: %#v\n", i, feature)
	}
}
