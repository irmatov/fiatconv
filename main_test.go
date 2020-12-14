package main

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

func Test_parseArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    request
		wantErr bool
	}{
		{"no arguments", []string{"prog"}, request{}, true},
		{"only amount", []string{"prog", "1"}, request{}, true},
		{"no target", []string{"prog", "1", "USD"}, request{}, true},
		{"all given", []string{"prog", "1", "USD", "AUD"}, request{1, "USD", "AUD"}, false},
		{"wrong amount", []string{"prog", "a1", "USD", "AUD"}, request{}, true},
		{"extra ignored", []string{"prog", "1", "USD", "AUD", "blah", "blah"}, request{1, "USD", "AUD"}, false},
		{"case insensitive", []string{"prog", "1", "UsD", "aud"}, request{1, "USD", "AUD"}, false},
		{"wrong source", []string{"prog", "1", "not_valid", "AUD"}, request{}, true},
		{"wrong dest", []string{"prog", "1", "USD", "not_valid"}, request{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArguments(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseArguments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseArguments() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fiatConv(t *testing.T) {
	oldFS := appFS
	defer func() { appFS = oldFS }()
	appFS = afero.NewMemMapFs()
	const cachePath = "/tmp/cache"
	converterCalled := false
	converter := func(from string, to string) (float64, error) {
		converterCalled = true
		return 2, nil
	}
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	app := appContext{
		args:      []string{},
		stdout:    stdout,
		stderr:    stderr,
		convert:   converter,
		cachePath: cachePath,
	}

	t.Run("no arguments", func(t *testing.T) {
		if code := app.fiatConv(); code == 0 {
			t.Errorf("unexpected success")
		}
		if len(stdout.Bytes()) != 0 {
			t.Errorf("unexpected stdout: %s", stdout.String())
		}
		if len(stderr.Bytes()) == 0 {
			t.Errorf("expected to have output on stderr")
		}
		if converterCalled {
			t.Errorf("unexpected API call")
		}
	})

	t.Run("valid run", func(t *testing.T) {
		stdout.Reset()
		stderr.Reset()
		app.args = []string{"prog", "5", "USD", "AUD"}
		if code := app.fiatConv(); code != 0 {
			t.Errorf("fiatConv() = %v, want 0", code)
		}
		if got := stdout.String(); got != "10.00\n" {
			t.Errorf("wrong stdout: %s", got)
		}
		if len(stderr.Bytes()) != 0 {
			t.Errorf("unexpected stderr: %s", stderr.String())
		}
		if !converterCalled {
			t.Errorf("expected API call to be made")
		}
	})

	t.Run("second call uses cache", func(t *testing.T) {
		converterCalled = false
		stdout.Reset()
		stderr.Reset()
		app.args = []string{"prog", "5", "USD", "AUD"}
		if code := app.fiatConv(); code != 0 {
			t.Errorf("fiatConv() = %v, want 0", code)
		}
		if got := stdout.String(); got != "10.00\n" {
			t.Errorf("wrong stdout: %s", got)
		}
		if len(stderr.Bytes()) != 0 {
			t.Errorf("unexpected stderr: %s", stderr.String())
		}
		if converterCalled {
			t.Errorf("unexpected API call")
		}
	})

	t.Run("API failure is reported", func(t *testing.T) {
		converterCalled = false
		stdout.Reset()
		stderr.Reset()
		app.args = []string{"prog", "5", "USD", "GBP"}
		app.convert = func(from string, to string) (float64, error) {
			converterCalled = true
			return 0, errors.New("simulated")
		}
		if code := app.fiatConv(); code == 0 {
			t.Errorf("fiatConv() = 0, want 1")
		}
		if got := stdout.String(); len(got) != 0 {
			t.Errorf("wrong stdout: %s", got)
		}
		if got := stderr.String(); got != "simulated\n" {
			t.Errorf("unexpected stderr: %s", got)
		}
		if !converterCalled {
			t.Errorf("expected API call to be made")
		}
	})
}
