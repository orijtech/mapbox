package mapbox

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
)

type GeocodeMode string

const (
	GeocodePlaces          GeocodeMode = "mapbox.places"
	GeocodePermanentPlaces GeocodeMode = "mapbox.places-permanent"
)

func (gm GeocodeMode) String() string {
	if gm == "" {
		gm = GeocodePlaces
	}
	return string(gm)
}

// LookupPlace looks up the coordinates and information of a place
// for example "Los Angeles" or "Edmonton".
func (c *Client) LookupPlace(query string) (*GeocodeResponse, error) {
	return c.doGeoCodingRequest(&ReverseGeocodeRequest{
		Query: query,
	})
}

// LookupLatLon is a helper to reverse geocoding
// lookup a latitude and longitude pair.
func (c *Client) LookupLatLon(lat, lon float64) (*GeocodeResponse, error) {
	return c.ReverseGeocoding(&ReverseGeocodeRequest{
		Query: fmt.Sprintf("%f,%f", lon, lat),
	})
}

// ReverseGeocoding Converts coordinates to place names
// -77.036,38.897 -> 1600 Pennsylvania Ave NW.
func (c *Client) ReverseGeocoding(req *ReverseGeocodeRequest) (*GeocodeResponse, error) {
	return c.doGeoCodingRequest(req)
}

// Request format:
// GET /geocoding/v5/{mode}/{query}.json
func (c *Client) doGeoCodingRequest(req *ReverseGeocodeRequest) (*GeocodeResponse, error) {
	asURLValues, err := toURLValues(req.Request)
	if err != nil {
		return nil, err
	}

	asURLValues.Add("access_token", c.APIKey())

	// GET /geocoding/v5/{mode}/{query}.json
	outURL := fmt.Sprintf("%s/geocoding/v5/%s/%s.json?%s",
		baseURL, req.Mode, req.Query, asURLValues.Encode())

	httpClient := c._httpClient()
	res, err := httpClient.Get(outURL)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if !statusOK(res.StatusCode) {
		return nil, fmt.Errorf("%s", res.Status)
	}

	blob, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	gres := new(GeocodeResponse)
	if err := json.Unmarshal(blob, gres); err != nil {
		return nil, err
	}
	return gres, nil
}

func toURLValues(v interface{}) (url.Values, error) {
	// First JSON serialize it
	blob, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	recv := make(map[string]interface{})
	if err := json.Unmarshal(blob, &recv); err != nil {
		return nil, err
	}

	outValues := make(url.Values)
	for key, ival := range recv {
		if ival == nil {
			continue
		}
		switch typ := ival.(type) {
		case string:
			outValues.Add(key, typ)
		case uint:
			outValues.Add(key, fmt.Sprintf("%d", typ))
		case bool:
			outValues.Add(key, fmt.Sprintf("%v", typ))
		case []float32:
			for _, fV := range typ {
				outValues.Add(key, fmt.Sprintf("%f", fV))
			}
		case []string:
			for _, strV := range typ {
				outValues.Add(key, strV)
			}
		case *LatLonPair:
			for _, fV := range *typ {
				outValues.Add(key, fmt.Sprintf("%sf", fV))
			}
			outValues.Add(key, fmt.Sprintf("%v", typ))
		default:
		}
	}

	return outValues, nil
}

type ReverseGeocodeRequest struct {
	Query   string      `json:"query"`
	Mode    GeocodeMode `json:"mode"`
	Request *GeocodeRequest
}

type GeocodeType string

const (
	GTypeRegion       GeocodeType = "region"
	GTypePostcode     GeocodeType = "postcode"
	GTypePlace        GeocodeType = "place"
	GTypeLocality     GeocodeType = "locality"
	GTypeNeighborhood GeocodeType = "neighborhood"
	GTypeAddress      GeocodeType = "address"
	GTypePOI          GeocodeType = "poi"
	GTypePOILandmark  GeocodeType = "poi.landmark"
)

type GeocodeRequest struct {
	// Country is a set of one or more countries
	// specified with ISO 3166 alpha 2 country codes.
	Country []string `json:"country,omitempty"`

	Limit uint          `json:"limit,omitempty"`
	Types []GeocodeType `json:"types,omitempty"`

	Proximity    *LatLonPair `json:"proximity,omitempty"`
	BoundingBox  []float32   `json:"bbox,omitempty"`
	AutoComplete bool        `json:"autocomplete,omitempty"`
}

type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float32 `json:"coordinates"`
}

type GeocodeContext struct {
	Id        string `json:"id"`
	Text      string `json:"text"`
	ShortCode string `json:"short_code"`
	Wikidata  string `json:"wikidata"`
}

type GeocodeFeature struct {
	Id        string  `json:"id"`
	Type      string  `json:"type"`
	Text      string  `json:"text"`
	PlaceName string  `json:"place_name"`
	Relevance float32 `json:"relevance"`

	Properties *GeocodeProperty  `json:"properties"`
	Context    []*GeocodeContext `json:"context"`

	BoundingBox []float32 `json:"bbox"`
	Center      []float32 `json:"center"`
	Geometry    *Geometry `json:"geometry"`
	Attribution string    `json:"attribution"`
}

type GeocodeProperty map[string]interface{}

type GeocodeResponse struct {
	Type     string            `json:"type,omitempty"`
	Query    *LatLonPair       `json:"query,omitempty"`
	Features []*GeocodeFeature `json:"features,omitempty"`
}
