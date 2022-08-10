package pipclient

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/tidwall/sjson"
)

type PIPClient struct {
	url  string
	http *http.Client
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

func (client *PIPClient) PointInPolygon(latitude string, longitude string) ([]byte, error) {
	reqBody, err := client.requestBodyReader(latitude, longitude)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", client.url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Add("content-type", "application/json")

	res, err := client.http.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (client *PIPClient) requestBodyReader(latitude string, longitude string) (io.Reader, error) {
	reqJSON := []byte{}

	latitudeBytes := []byte(latitude)
	longitudeBytes := []byte(longitude)

	reqJSON, err := sjson.SetRawBytes(reqJSON, "latitude", latitudeBytes)
	if err != nil {
		return nil, err
	}

	reqJSON, err = sjson.SetRawBytes(reqJSON, "longitude", longitudeBytes)
	if err != nil {
		return nil, err
	}

	reqJSON, err = sjson.SetBytes(reqJSON, "is_current", []int{1, -1})
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(reqJSON), nil
}
