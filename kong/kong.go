package kong

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/go-querystring/query"
)

const defaultBaseURL = "http://localhost:8001"

type service struct {
	client *Client
}

// Client talks to the Admin API or control plane of a
// Kong cluster
type Client struct {
	client  *http.Client
	baseURL string
	common  service
	Sample  *SampleService
}

// Response is a Kong Admin API response. It wraps http.Response.
type Response struct {
	*http.Response
	//other Kong specific fields
}

// Status respresents current status of a Kong node.
type Status struct {
	Database struct {
		Reachable bool `json:"reachable"`
	} `json:"database"`
	Server struct {
		ConnectionsAccepted int `json:"connections_accepted"`
		ConnectionsActive   int `json:"connections_active"`
		ConnectionsHandled  int `json:"connections_handled"`
		ConnectionsReading  int `json:"connections_reading"`
		ConnectionsWaiting  int `json:"connections_waiting"`
		ConnectionsWriting  int `json:"connections_writing"`
		TotalRequests       int `json:"total_requests"`
	} `json:"server"`
}

func (c *Client) newRequest(method, endpoint string, qs interface{},
	body interface{}) (*http.Request, error) {
	// TODO introduce method as a type, method as string
	// in http package is doomsday
	if endpoint == "" {
		return nil, errors.New("endpoint can't be nil")
	}
	//TODO verify endpoint is preceded with /

	//body to be sent in JSON
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	//Create a new request
	req, err := http.NewRequest(method, c.baseURL+endpoint,
		bytes.NewBuffer(buf))

	if err != nil {
		return nil, err
	}
	//Add query string if any
	if qs != nil {
		values, err := query.Values(qs)
		if err != nil {
			return nil, err
		}
		req.URL.RawQuery = values.Encode()
	}
	return req, nil
}

func hasError(res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return nil
	}
	body, _ := ioutil.ReadAll(res.Body) // TODO error in error?
	return errors.New(res.Status + " " + string(body))
}

// NewClient returns a Client which talks to Admin API of Kong
func NewClient(baseURL *string, client *http.Client) (*Client, error) {
	if client == nil {
		client = http.DefaultClient
	}
	kong := new(Client)
	kong.client = client
	if baseURL != nil {
		//TODO validate URL
		kong.baseURL = *baseURL
	} else {
		kong.baseURL = defaultBaseURL
	}
	kong.common.client = kong
	kong.Sample = (*SampleService)(&kong.common)

	return kong, nil
}

func newResponse(res *http.Response) *Response {
	return &Response{Response: res}
}

// Do executes a HTTP request and returns a response
func (c *Client) Do(ctx context.Context, req *http.Request,
	v interface{}) (*Response, error) {
	var err error
	if req == nil {
		return nil, errors.New("Request object cannot be nil")
	}
	req = req.WithContext(ctx)
	//Make the request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	///check for API errors
	if err = hasError(resp); err != nil {
		return nil, err
	}
	// Call Close on exit
	defer func() {
		var e error
		e = resp.Body.Close()
		if e != nil {
			err = e
		}
	}()
	response := newResponse(resp)

	// response
	if v != nil {
		if writer, ok := v.(io.Writer); ok {
			_, err = io.Copy(writer, resp.Body)
			if err != nil {
				return nil, err
			}
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
			if err != nil {
				return nil, err
			}
		}
	}
	return response, err
}

// Status returns the status of a Kong node
func (c *Client) Status() (*Status, error) {

	req, err := c.newRequest("GET", "/status", nil, nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	ctx := context.Background()

	var s Status
	_, err = c.Do(ctx, req, &s)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &s, nil
}
