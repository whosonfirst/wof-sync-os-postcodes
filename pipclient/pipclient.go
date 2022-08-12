package pipclient

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/tidwall/sjson"
)

type PIPClient struct {
	url  string
	http *http.Client
	wp   *workerpool.WorkerPool
}

func NewPIPClient(u string) *PIPClient {
	maxConns := runtime.NumCPU()

	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{Transport: tr}

	wp := workerpool.New(maxConns)

	return &PIPClient{http: client, url: u, wp: wp}
}

type result struct {
	bytes []byte
	err   error
}

func (client *PIPClient) PointInPolygon(latitude string, longitude string) ([]byte, error) {
	c := make(chan result)

	client.wp.Submit(func() {
		reqBody, err := client.requestBodyReader(latitude, longitude)
		if err != nil {
			c <- result{nil, err}
			return
		}

		req, err := http.NewRequest("POST", client.url, reqBody)
		if err != nil {
			c <- result{nil, err}
			return
		}

		req.Close = true
		req.Header.Add("content-type", "application/json")

		res, err := client.http.Do(req)
		if err != nil {
			c <- result{nil, err}
			return
		}

		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		c <- result{body, err}

	})

	result := <-c

	return result.bytes, result.err
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
