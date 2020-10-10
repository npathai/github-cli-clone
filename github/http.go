package github

import (
	"net/http"
	"net/url"
)

type simpleClient struct {
	httpClient     *http.Client
	rootUrl        *url.URL
	PrepareRequest func(*http.Request)
	CacheTTL       int
}
