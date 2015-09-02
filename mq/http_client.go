package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/arschles/gorion"
	"github.com/arschles/gorion/Godeps/_workspace/src/golang.org/x/net/context"
)

// Scheme is "http" or "https"
type Scheme string

// String converts a Scheme to a printable string
func (s Scheme) String() string {
	return string(s)
}

const (
	// SchemeHTTP represents http
	SchemeHTTP = "http"
	// SchemeHTTPS represents https
	SchemeHTTPS = "https"
)

type httpClient struct {
	endpt     string
	transport *http.Transport
	client    *http.Client
}

// NewHTTPClient returns a Client implementation that can talk to the IronMQ v3
// API documented at http://dev.iron.io/mq/3/reference/api/
func NewHTTPClient(scheme Scheme, host string, port uint16) Client {
	transport := &http.Transport{}
	client := &http.Client{Transport: transport}
	return &httpClient{
		transport: transport,
		client:    client,
		endpt:     fmt.Sprintf("%s://%s:%d", scheme, host, port),
	}
}

type enqueueReq struct {
	Messages []NewMessage `json:"messages"`
}

// Enqueue posts messages to IronMQ using the API defined at http://dev.iron.io/mq/3/reference/api/#post-messages
func (h *httpClient) Enqueue(ctx context.Context, queueName string, msgs []NewMessage) (*Enqueued, error) {
	reqBody := &bytes.Buffer{}
	if err := json.NewEncoder(reqBody).Encode(enqueueReq{Messages: msgs}); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", h.urlStr("queues/%s/messages", queueName), reqBody)
	if err != nil {
		return nil, err
	}
	ret := new(Enqueued)
	doFunc := func(resp *http.Response, err error) error {
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(ret); err != nil {
			return err
		}
		return nil
	}
	if err := gorion.HTTPDo(ctx, h.client, h.transport, req, doFunc); err != nil {
		return nil, err
	}
	return ret, nil
}

type dequeueReq struct {
	Num     int  `json:"n"`
	Timeout int  `json:"timeout"`
	Wait    int  `json:"wait"`
	Delete  bool `json:"delete"`
}

type dequeueResp struct {
	Messages []DequeuedMessage `json:"messages"`
}

// Dequeue gets messages from IronMQ using the API defined at http://dev.iron.io/mq/3/reference/api/#reserve-messages
func (h *httpClient) Dequeue(ctx context.Context, qName string, num int, timeout Timeout, wait Wait, delete bool) ([]DequeuedMessage, error) {
	if !timeoutInRange(timeout) {
		return nil, ErrTimeoutOutOfRange
	}
	if !waitInRange(wait) {
		return nil, ErrWaitOutOfRange
	}

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(dequeueReq{Num: num, Timeout: int(timeout), Wait: int(wait), Delete: delete}); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", h.urlStr("queues/%s/reservations", qName), body)
	if err != nil {
		return nil, err
	}
	ret := new(dequeueResp)
	doFunc := func(resp *http.Response, err error) error {
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(ret); err != nil {
			return err
		}
		return nil
	}
	if err := gorion.HTTPDo(ctx, h.client, h.transport, req, doFunc); err != nil {
		return nil, err
	}
	return ret.Messages, nil
}

type deleteReservedReq struct {
	ReservationID string `json:"reservation_id"`
}

func (h *httpClient) DeleteReserved(ctx context.Context, qName string, messageID int, reservationID string) (*Deleted, error) {
	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(deleteReservedReq{ReservationID: reservationID}); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("DELETE", h.urlStr("queues/%s/messages/%d", qName, messageID), body)
	if err != nil {
		return nil, err
	}
	ret := new(Deleted)
	doFunc := func(resp *http.Response, err error) error {
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(ret); err != nil {
			return err
		}
		return nil
	}
	if err := gorion.HTTPDo(ctx, h.client, h.transport, req, doFunc); err != nil {
		return nil, err
	}
	return ret, nil
}

// urlStr returns the url string resulting from appending path to h.endpt.
// pass path without a leading slash
func (h *httpClient) urlStr(pathFmt string, fmtVars ...interface{}) string {
	return h.endpt + "/" + fmt.Sprintf(pathFmt, fmtVars...)
}
