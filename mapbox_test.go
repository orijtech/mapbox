package mapbox_test

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
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

type durationsBackend struct {
	mapping map[string]map[string]float32
}

var _ http.RoundTripper = (*durationsBackend)(nil)

func (backend *durationsBackend) RoundTrip(req *http.Request) (*http.Response, error) {
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
	backend := &durationsBackend{
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
