package mapbox_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/odeke-em/mapbox"
)

func TestLatLonPairJSONUnmarshal(t *testing.T) {
	tests := []struct {
		json string
		want *mapbox.DurationResponse
	}{
		0: {
			json: `{
		    "durations": [
			[0,    2910, null],
			[2903, 0,    5839],
			[4695, 5745, 0   ]
		      ]
		}`,
			want: &mapbox.DurationResponse{
				Durations: []*mapbox.LatLonPair{
					{0, 2910, mapbox.NoPathDuration},
					{2903, 0, 5839},
					{4695, 5745, 0},
				},
			},
		},
	}

	for i, tt := range tests {
		recv := new(mapbox.DurationResponse)
		if err := json.Unmarshal([]byte(tt.json), recv); err != nil {
			t.Errorf("#%d unmarshalErr: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tt.want, recv) {
			gotBytes, _ := json.MarshalIndent(recv, "", "  ")
			wantBytes, _ := json.MarshalIndent(tt.want, "", "  ")
			t.Errorf("#%d:\ngot: %s\nwant: %s", i, gotBytes, wantBytes)
		}
	}
}

/*
{
  "coordinates": [
    [13.41894, 52.50055],
    [14.10293, 52.50055],
    [13.50116, 53.10293]
  ]
}
*/

var durationsMap = map[string]map[string]float32{
	"[13.41894,52.50055]": map[string]float32{
		"[14.10293,52.50055]": 2910,
	},
	"[14.10293,52.50055]": map[string]float32{
		"[13.41894,52.50055]": 2903,
		"[13.50116,53.10293]": 5839,
	},
	"[13.50116,53.10293]": map[string]float32{
		"[13.41894,52.50055]": 4695,
		"[14.10293,52.50055]": 5745,
	},
}

type tBackend struct {
	route   string
	mapping map[string]map[string]float32
}

var _ http.RoundTripper = (*tBackend)(nil)

type roundTrip func(*http.Request) (*http.Response, error)
type produceRT func(*tBackend) roundTrip

var routeMatchesToRoundTripper = map[string]produceRT{
	"/distances": func(b *tBackend) roundTrip { return b.durationRoundTrip },
	"/geocoding": func(b *tBackend) roundTrip { return b.geocodeRoundTrip },
}

func (backend *tBackend) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path

	var rtFn func(*http.Request) (*http.Response, error)
	for segment, fn := range routeMatchesToRoundTripper {
		if strings.Contains(path, segment) {
			rtFn = fn(backend)
			break
		}
	}

	if rtFn == nil {
		return makeResp("Not Found", http.StatusNotFound, http.NoBody), nil
	}

	return rtFn(req)
}

var knownPlacesToCodes = map[string]string{
	"Los Angeles": "LA",
}

func (backend *tBackend) geocodeRoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "GET" {
		msg := fmt.Sprintf("only %q allowed not %q", "GET", req.Method)
		return makeResp(msg, http.StatusMethodNotAllowed, http.NoBody), nil
	}

	splits := strings.Split(req.URL.Path, "/")
	if len(splits) < 2 {
		return makeResp("/{{place}} in the URL path", http.StatusBadRequest, http.NoBody), nil
	}

	placeInfo := splits[len(splits)-1]
	place := strings.TrimSuffix(placeInfo, ".json")
	shortID := knownPlacesToCodes[place]
	if shortID == "" {
		msg := fmt.Sprintf("%q not found", shortID)
		return makeResp(msg, http.StatusNotFound, http.NoBody), nil
	}

	fullPath := geocodeResponsePath(shortID)
	return respFromFileContents(fullPath)
}

func respFromFileContents(path string) (*http.Response, error) {
	f, err := os.Open(path)
	if err != nil {
		return makeResp(err.Error(), http.StatusInternalServerError, http.NoBody), nil
	}
	return makeResp("200 OK", http.StatusOK, f), nil
}

func (backend *tBackend) durationRoundTrip(req *http.Request) (*http.Response, error) {
	slurp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	mat := new(mapbox.DurationRequest)
	if err := json.Unmarshal(slurp, mat); err != nil {
		return nil, err
	}

	// Now for the lookup
	fullMapping := make([]*mapbox.LatLonPair, len(mat.Coordinates))
	for i, row := range mat.Coordinates {
		path := make(mapbox.LatLonPair, len(mat.Coordinates))
		fullMapping[i] = &path
		rowSig, _ := json.Marshal(row)
		distanceMap := backend.mapping[string(rowSig)]

		for j, otherRow := range mat.Coordinates {
			if j == i || row == otherRow {
				// Distance here is zero
				path[j] = 0
				continue
			}
			// Otherwise look up that distance
			if distanceMap == nil || len(distanceMap) == 0 {
				path[j] = mapbox.NoPathDuration
				// No path
				continue
			}
			otherRowSig, _ := json.Marshal(otherRow)
			if retr, ok := distanceMap[string(otherRowSig)]; ok {
				path[j] = retr
			} else {
				path[j] = mapbox.NoPathDuration
			}
		}
	}

	// Next step serialize the response
	mresp := new(mapbox.DurationResponse)
	mresp.Durations = fullMapping
	blob, err := json.Marshal(mresp)
	if err != nil {
		return nil, err
	}

	prc, pwc := io.Pipe()
	go func() {
		_, err := pwc.Write(blob)
		if err != nil {
			log.Printf("writing out body, err: %v", err)
		}
		_ = pwc.Close()
	}()

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		Request:       req,
		Body:          prc,
		ContentLength: -1,
	}

	return resp, nil
}

func TestRoundtripDurationResponse(t *testing.T) {
	backend := &tBackend{
		mapping: durationsMap,
	}

	client, err := mapbox.NewClient(
		mapbox.WithHTTPClient(&http.Client{Transport: backend}),
	)

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		json string
		want *mapbox.DurationResponse
	}{
		0: {
			json: `
			{
			  "coordinates": [
			    [13.41894, 52.50055],
			    [14.10293, 52.50055],
			    [13.50116, 53.10293]
			  ]
			}`,
			want: &mapbox.DurationResponse{
				Durations: []*mapbox.LatLonPair{
					{0, 2910, mapbox.NoPathDuration},
					{2903, 0, 5839},
					{4695, 5745, 0},
				},
			},
		},
	}

	for i, tt := range tests {
		req := new(mapbox.DurationRequest)
		if err := json.Unmarshal([]byte(tt.json), req); err != nil {
			t.Errorf("#%d unmarshalErr: %v", i, err)
			continue
		}

		recv, err := client.RequestDuration(req)
		if err != nil {
			t.Errorf("#%d unmarshalErr: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tt.want, recv) {
			gotBytes, _ := json.MarshalIndent(recv, "", "  ")
			wantBytes, _ := json.MarshalIndent(tt.want, "", "  ")
			t.Errorf("#%d:\ngot: %s\nwant: %s", i, gotBytes, wantBytes)
		}
	}

}

func geocodeResponsePath(shortID string) string {
	return fmt.Sprintf("./testdata/places-%s.json", shortID)
}

func geocodeResponseFromFile(shortID string) *mapbox.GeocodeResponse {
	fullPath := geocodeResponsePath(shortID)
	blob, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil
	}

	gr := new(mapbox.GeocodeResponse)
	if err := json.Unmarshal(blob, gr); err != nil {
		return nil
	}
	return gr
}

func TestLookupPlace(t *testing.T) {
	backend := &tBackend{
		mapping: durationsMap,
	}

	client, err := mapbox.NewClient(
		mapbox.WithHTTPClient(&http.Client{Transport: backend}),
	)

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query   string
		wantErr bool
		want    *mapbox.GeocodeResponse
	}{
		0: {
			query: "Los Angeles",
			want:  geocodeResponseFromFile("LA"),
		},
	}

	for i, tt := range tests {
		gr, err := client.LookupPlace(tt.query)
		if tt.wantErr {
			if err == nil {
				t.Errorf("#%d: want non-nil err", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("#%d err: %v", i, err)
			continue
		}

		gotBlob := jsonMarshal(gr)
		wantBlob := jsonMarshal(tt.want)
		if !bytes.Equal(gotBlob, wantBlob) {
			t.Errorf("#%d\ngot:  %s\nwant: %s", i, gotBlob, wantBlob)
		}
	}
}

func jsonMarshal(v interface{}) []byte {
	blob, _ := json.Marshal(v)
	return blob
}

func makeResp(status string, code int, body io.ReadCloser) *http.Response {
	return &http.Response{
		Status:     status,
		StatusCode: code,
		Header:     make(http.Header),
		Body:       body,
	}
}
