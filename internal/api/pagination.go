package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// DefaultPageLen is the page size requested when no limit is given.
const DefaultPageLen = 50

// Page is the Bitbucket paginated envelope.
type Page[T any] struct {
	Size     int    `json:"size"`
	Page     int    `json:"page"`
	PageLen  int    `json:"pagelen"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Values   []T    `json:"values"`
}

// RawPage is Page with values left as raw JSON (used by `bb api --paginate`).
type RawPage = Page[json.RawMessage]

// ListOptions controls a paginated listing.
type ListOptions struct {
	// Limit is the maximum number of items to return; <= 0 means no limit.
	Limit int
	// Fields, when set, is passed as the `fields` query parameter. Callers
	// listing large objects should always narrow the response.
	Fields string
	// Query is the BBQL filter (q=).
	Query string
	// Sort is the sort field (sort=), e.g. "-updated_on".
	Sort string
	// Extra query parameters.
	Extra url.Values
	// Headers are sent with every page request.
	Headers map[string]string
}

func (o ListOptions) pageLen() int {
	if o.Limit > 0 && o.Limit < DefaultPageLen {
		return o.Limit
	}
	return DefaultPageLen
}

func (o ListOptions) query() url.Values {
	q := url.Values{}
	for k, vs := range o.Extra {
		q[k] = append([]string(nil), vs...)
	}
	if q.Get("pagelen") == "" {
		q.Set("pagelen", strconv.Itoa(o.pageLen()))
	}
	if o.Fields != "" {
		q.Set("fields", o.Fields)
	}
	if o.Query != "" {
		q.Set("q", o.Query)
	}
	if o.Sort != "" {
		q.Set("sort", o.Sort)
	}
	return q
}

// ErrStop can be returned from a Paginate callback to stop early without error.
type errStop struct{}

func (errStop) Error() string { return "stop" }

// ErrStop stops pagination.
var ErrStop error = errStop{}

// Paginate walks all pages of path, following `next` links, and calls fn for
// each value until opts.Limit is reached or fn returns ErrStop.
func Paginate[T any](ctx context.Context, c *Client, path string, opts ListOptions, fn func(T) error) error {
	next := path
	query := opts.query()
	count := 0
	for next != "" {
		var page Page[T]
		if _, err := c.Do(ctx, Request{Path: next, Query: query, Headers: opts.Headers}, &page); err != nil {
			return err
		}
		for _, v := range page.Values {
			if err := fn(v); err != nil {
				if err == ErrStop {
					return nil
				}
				return err
			}
			count++
			if opts.Limit > 0 && count >= opts.Limit {
				return nil
			}
		}
		// `next` already embeds the query string; don't add it again.
		next, query = page.Next, nil
	}
	return nil
}

// List collects up to opts.Limit values from path.
func List[T any](ctx context.Context, c *Client, path string, opts ListOptions) ([]T, error) {
	var out []T
	err := Paginate(ctx, c, path, opts, func(v T) error {
		out = append(out, v)
		return nil
	})
	return out, err
}
