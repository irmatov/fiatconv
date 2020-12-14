package exchange

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	customClient := &http.Client{Timeout: 10 * time.Second}
	customBase := "https://example.com"
	tests := []struct {
		name string
		opts []Option
		want *API
	}{
		{"default", nil, &API{http.DefaultClient, defaultBase}},
		{
			"base",
			[]Option{WithBase(customBase)},
			&API{http.DefaultClient, customBase},
		},
		{
			"client",
			[]Option{WithClient(customClient)},
			&API{customClient, defaultBase},
		},
		{
			"base and client",
			[]Option{WithBase(customBase), WithClient(customClient)},
			&API{customClient, customBase},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.opts...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_makeURL(t *testing.T) {
	type args struct {
		base string
		from string
		to   string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"invalid base", args{":", "USD", "AUD"}, "", true},
		{"valid", args{"https://example.com", "USD", "AUD"}, "https://example.com/latest?base=USD&symbols=AUD", false},
		{"valid with trailing slash", args{"https://example.com/", "USD", "GBP"}, "https://example.com/latest?base=USD&symbols=GBP", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := makeURL(tt.args.base, tt.args.from, tt.args.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("makeURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("makeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeRate(t *testing.T) {
	type args struct {
		r    io.Reader
		from string
		to   string
	}
	tests := []struct {
		name    string
		args    args
		want    float64
		wantErr bool
	}{
		{"invalid JSON", args{strings.NewReader(""), "", ""}, 0, true},
		{
			"valid response",
			args{
				strings.NewReader(`{"rates":{"AUD":1.5},"base":"USD","date":"2020-11-20"}`),
				"USD",
				"AUD",
			},
			1.5,
			false,
		},
		{
			"invalid base",
			args{
				strings.NewReader(`{"rates":{"AUD":1.5},"base":"XYZ","date":"2020-11-20"}`),
				"USD",
				"AUD",
			},
			0,
			true,
		},
		{
			"missing target",
			args{
				strings.NewReader(`{"rates":{"XYZ":1.5},"base":"USD","date":"2020-11-20"}`),
				"USD",
				"AUD",
			},
			0,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeRate(tt.args.r, tt.args.from, tt.args.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeRate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("decodeRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPI_Convert(t *testing.T) {
	goodSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"rates":{"AUD":2},"base":"USD","date":"2020-11-20"}`))
	}))
	defer goodSrv.Close()

	// same as goodSrv, but gives 404
	notFoundSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"rates":{"AUD":2},"base":"USD","date":"2020-11-20"}`))
	}))
	defer notFoundSrv.Close()

	type args struct {
		from, to string
	}
	tests := []struct {
		name    string
		args    args
		baseURL string
		want    float64
		wantErr bool
	}{
		{
			"success",
			args{"USD", "AUD"},
			goodSrv.URL,
			2,
			false,
		},
		{
			"wrong source currency",
			args{"XYZ", "AUD"},
			goodSrv.URL,
			0,
			true,
		},
		{
			"missing target currency",
			args{"USD", "XYZ"},
			goodSrv.URL,
			0,
			true,
		},
		{
			"bad http status code",
			args{"USD", "AUD"},
			notFoundSrv.URL,
			0,
			true,
		},
		{
			"invalid base URL",
			args{"USD", "AUD"},
			":",
			0,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := New(WithBase(tt.baseURL))
			got, err := api.Convert(tt.args.from, tt.args.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("Convert() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Convert() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("API error reported", func(t *testing.T) {
		const errMsg = "Feeling bad today"
		badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error": "%s"}`, errMsg)
		}))
		defer badSrv.Close()

		api := New(WithBase(badSrv.URL))
		_, err := api.Convert("USD", "AUD")
		if err == nil {
			t.Errorf("Convert() must fail, but it doesn't")
		} else if err.Error() != errMsg {
			t.Errorf("Convert() error = %v, want %v", err, errMsg)
		}
	})
}
