// fiatconv implements CLI tool to convert between currencies.
//
// Uses https://exchangeratesapi.io/ service to fetch rates. Caches results
// locally for some time to lower the load on the service as suggested.
package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fiatconv/cache"
	"fiatconv/exchange"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
)

const (
	fiatNameLength = 3
	httpTimeout    = 10 * time.Second
	cacheLifetime  = time.Hour
)

var appFS = afero.NewOsFs()

type request struct {
	amount float64
	from   string
	to     string
}

func parseArguments(args []string) (request, error) {
	errTooFewArguments := errors.New("too few arguments")
	if len(args) < 4 {
		return request{}, errTooFewArguments
	}
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return request{}, fmt.Errorf("Invalid amount: %w", err)
	}
	if len(args[2]) != fiatNameLength {
		return request{}, fmt.Errorf("Invalid fiat: %s", args[2])
	}
	if len(args[3]) != fiatNameLength {
		return request{}, fmt.Errorf("Invalid fiat: %s", args[3])
	}
	return request{amount, strings.ToUpper(args[2]), strings.ToUpper(args[3])}, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: fiatconv <amount> <from_fiat> <to_fiat>

This utility converts "amount" of money in "from_fiat" currency to amount in
"to_fiat" currency. Both currencies are given as ISO 4217 code (eg. USD).`)
}

type appContext struct {
	args           []string
	stdout, stderr io.Writer
	convert        func(from, to string) (float64, error)
	cachePath      string
}

var timeNowFn = time.Now

func loadCache(path string, cutoff int64) *cache.Cache {
	f, err := appFS.Open(path)
	if err != nil {
		return cache.Load(bytes.NewBuffer(nil), cutoff)
	}
	defer f.Close()

	return cache.Load(f, cutoff)
}

func saveCache(c *cache.Cache, path string) {
	f, err := appFS.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	if err := c.Save(f); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save to cache: %v\n", err)
	}
}

func (app appContext) fiatConv() int {
	req, err := parseArguments(app.args)
	if err != nil {
		fmt.Fprintf(app.stderr, "%v\n\n", err)
		printUsage(app.stderr)
		return 1
	}

	type key struct {
		From, To string
	}
	gob.Register(key{})

	now := timeNowFn()
	c := loadCache(app.cachePath, now.Add(-cacheLifetime).Unix())
	var rate float64
	k := key{req.from, req.to}
	v, ok := c.Get(k)
	if ok {
		if f, ok := v.(float64); ok {
			rate = f
		}
	}

	if rate == 0 {
		// cache miss
		rate, err = app.convert(req.from, req.to)
		if err != nil {
			fmt.Fprintln(app.stderr, err)
			return 1
		}
		c.Set(k, rate, now.Unix())
		saveCache(c, app.cachePath)
	}

	fmt.Fprintf(app.stdout, "%.2f\n", rate*req.amount)
	return 0
}

func main() {
	api := exchange.New(exchange.WithClient(&http.Client{Timeout: httpTimeout}))

	var cachePath string
	if p, err := os.UserCacheDir(); err != nil {
		cachePath = "/dev/null"
	} else {
		cachePath = path.Join(p, path.Base(os.Args[0]))
	}

	app := appContext{
		os.Args,
		os.Stdout,
		os.Stderr,
		api.Convert,
		cachePath,
	}
	if code := app.fiatConv(); code != 0 {
		os.Exit(code)
	}
}
