// Package exchange implements (part of) currency converter API provided by exchangeratesapi.io.
package exchange

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const defaultBase = "https://api.exchangeratesapi.io"

// API is exchange rate client.
type API struct {
	client *http.Client
	base   string
}

// Option configure API.
type Option func(*API)

// WithClient returns Option telling API to use given HTTP client.
func WithClient(client *http.Client) Option {
	return func(a *API) { a.client = client }
}

// WithBase returns Option telling API to use given HTTP base.
func WithBase(base string) Option {
	return func(a *API) { a.base = base }
}

func makeURL(base, from, to string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	q := make(url.Values)
	q.Set("base", from)
	q.Set("symbols", to)
	u.RawQuery = q.Encode()
	u.Path = "/latest"
	return u.String(), nil
}

func decodeRate(r io.Reader, from, to string) (float64, error) {
	var response struct {
		Rates map[string]float64
		Base  string
	}
	if err := json.NewDecoder(r).Decode(&response); err != nil {
		return 0, err
	}
	if response.Base != from {
		return 0, fmt.Errorf("unexpected base in response: %s", response.Base)
	}
	if v, ok := response.Rates[to]; ok {
		return v, nil
	}
	return 0, errors.New("target code not found in response")
}

// New returns new exchange rate API client.
func New(opts ...Option) *API {
	api := API{http.DefaultClient, defaultBase}
	for _, option := range opts {
		option(&api)
	}
	return &api
}

// Convert returns exchange rate using "from" currency as base and "to" as target.
func (api *API) Convert(from, to string) (float64, error) {
	url, err := makeURL(api.base, from, to)
	if err != nil {
		return 0, err
	}

	resp, err := api.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var r struct {
			Error string
		}
		if err = json.NewDecoder(resp.Body).Decode(&r); err == nil && len(r.Error) > 0 {
			return 0, errors.New(r.Error)
		}
		return 0, fmt.Errorf("unexpected HTTP status code: %v", resp.StatusCode)
	}
	return decodeRate(resp.Body, from, to)
}
