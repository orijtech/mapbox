package mapbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
)

type Client struct {
	sync.RWMutex
	version    string
	apiKey     string
	httpClient *http.Client
}

func (c *Client) SetAPIKey(key string) {
	c.Lock()
	defer c.Unlock()

	c.apiKey = key
}

var defaultEnvAPIKey = os.Getenv("MAPBOX_API_KEY")

func (c *Client) APIKey() string {
	c.RLock()
	defer c.RUnlock()

	if c.apiKey != "" {
		return c.apiKey
	}
	return defaultEnvAPIKey
}

type LatLonPair []float32
type LatLonMatrix [][]float32

var NoPathDuration = float32(-1)

func (llp *LatLonPair) UnmarshalJSON(b []byte) error {
	var irecv []interface{}
	if err := json.Unmarshal(b, &irecv); err != nil {
		return err
	}

	var recv []float32
	for _, v := range irecv {
		if v == nil { // They sent back `null` so no path
			recv = append(recv, NoPathDuration)
		} else {
			switch t := v.(type) {
			case float32:
				recv = append(recv, t)
			case float64:
				recv = append(recv, float32(t))
			case int:
				recv = append(recv, float32(t))
			case int32:
				recv = append(recv, float32(t))
			case int64:
				recv = append(recv, float32(t))
			case uint:
				recv = append(recv, float32(t))
			case uint32:
				recv = append(recv, float32(t))
			case uint64:
				recv = append(recv, float32(t))
			default:
				recv = append(recv, NoPathDuration)
			}
		}
	}

	*llp = recv
	return nil
}

type DurationResponse struct {
	Durations []*LatLonPair `json:"durations,omitempty"`
}

var errUnimplemented = errors.New("unimplemented")

type DurationRequest struct {
	Coordinates []*LatLonPair `json:"coordinates"`
}

const defaultAPIVersion = "v1"

func (c *Client) APIVersion() string {
	c.RLock()
	defer c.RUnlock()

	if version := c.version; version != "" {
		return version
	} else {
		return defaultAPIVersion
	}
}

var baseURL = "https://api.mapbox.com"

func (c *Client) durationsURL() string {
	return fmt.Sprintf("%s/distances/%s/mapbox/driving?access_token=%s",
		baseURL, c.APIVersion(), c.APIKey())
}

func (c *Client) _httpClient() *http.Client {
	c.RLock()
	defer c.RUnlock()

	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func statusOK(c int) bool { return c >= 200 && c <= 299 }

func (c *Client) RequestDuration(dreq *DurationRequest) (*DurationResponse, error) {
	blob, err := json.Marshal(dreq)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("POST", c.durationsURL(), bytes.NewReader(blob))
	httpClient := c._httpClient()
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	if !statusOK(res.StatusCode) {
		return nil, fmt.Errorf("%s", res.Status)
	}
	slurp, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	dres := new(DurationResponse)
	if err := json.Unmarshal(slurp, dres); err != nil {
		return nil, err
	}

	return dres, nil
}

func NewClient(opts ...Option) (*Client, error) {
	c := new(Client)
	for _, opt := range opts {
		opt.apply(c)
	}

	return c, nil
}
