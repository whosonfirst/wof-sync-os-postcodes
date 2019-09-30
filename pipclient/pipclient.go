package pipclient

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/whosonfirst/go-whosonfirst-geojson-v2/feature"
)

type PIPClient struct {
	url  string
	http *http.Client
}

type PlacesResponse struct {
	Places []*feature.WOFStandardPlacesResult `json:"places"`
}

func NewPIPClient(u string) *PIPClient {
	maxConns := runtime.NumCPU()

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConnsPerHost:   maxConns,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{Transport: tr}
	return &PIPClient{http: client, url: u}
}

func (client *PIPClient) PointInPolygon(latitude string, longitude string) (*PlacesResponse, error) {
	u, err := client.requestURL(latitude, longitude)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := client.http.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	places := PlacesResponse{}
	err = json.Unmarshal(body, &places)
	if err != nil {
		return nil, err
	}

	return &places, nil
}

func (client *PIPClient) requestURL(latitude string, longitude string) (*url.URL, error) {
	u, err := url.Parse(client.url)
	if err != nil {
		return nil, err
	}

	u.Path = "/query"

	q := u.Query()
	q.Set("latitude", latitude)
	q.Set("longitude", longitude)
	q.Set("is_current", "-1,1")

	u.RawQuery = q.Encode()

	return u, nil
}
