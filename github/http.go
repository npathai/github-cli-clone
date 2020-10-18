package github

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/npathai/github-cli-clone/version"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const apiPayloadVersion = "application/vnd.github.v3+json;charset=utf-8"
const draftsType = "application/vnd.github.shadow-cat-preview+json;charset=utf-8"
const cacheVersion = 2

var UserAgent = "Hub " + version.Version

type simpleClient struct {
	httpClient     *http.Client
	rootUrl        *url.URL
	PrepareRequest func(*http.Request)
	CacheTTL       int
}

type simpleResponse struct {
	*http.Response
}

type errorInfo struct {
	Message  string       `json:"message"`
	Errors   []fieldError `json:"errors"`
	Response *http.Response
}

func (e *errorInfo) Error() string {
	return e.Message
}

type errorInfoSimple struct {
	Message string   `json:"message"`
	Errors  []string `json:"errors"`
}

type fieldError struct {
	Resource string `json:"resource"`
	Message  string `json:"message"`
	Code     string `json:"code"`
	Field    string `json:"field"`
}

type verboseTransport struct {
	Transport *http.Transport
	Verbose bool
	OverrideURL *url.URL
	Out io.Writer
	Colorized bool
}

// An implementation of http.ProxyFromEnvironment that isn't broken
func proxyFromEnvironment(req *http.Request) (*url.URL, error) {
	proxy := os.Getenv("http_proxy")
	if proxy == "" {
		proxy = os.Getenv("HTTP_PROXY")
	}
	if proxy == "" {
		return nil, nil
	}

	proxyURL, err := url.Parse(proxy)
	if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
		if proxyURL, err := url.Parse("http://" + proxy); err == nil {
			return proxyURL, nil
		}
	}

	if err != nil {
		return nil, fmt.Errorf("invalid proxy address %q: %v", proxy, err)
	}

	return proxyURL, nil
}

func newHttpClient(testHost string, verbose bool, unixSocket string) *http.Client {
	var testURL *url.URL
	if testHost != "" {
		testURL, _ = url.Parse(testHost)
	}
	var httpTransport *http.Transport
	if unixSocket != "" {
		dialFunc := func(network, addr string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		}
		dialContext := func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		}
		httpTransport = &http.Transport{
			DialContext:           dialContext,
			DialTLS:               dialFunc,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
		}
	} else {
		httpTransport = &http.Transport{
			Proxy: proxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	}
	tr := &verboseTransport{
		Transport:   httpTransport,
		Verbose:     verbose,
		OverrideURL: testURL,
		// FIXME add these once we incorporate ui package
		Out:         nil,
		Colorized:   nil,
	}

	return &http.Client{
		Transport: tr,
	}
}

func (client *simpleClient) GetFile(path string, mimeType string) (*simpleResponse, error) {
	return client.PerformRequest("GET", path, nil, func(req *http.Request) {
		req.Header.Set("Accept", mimeType)
	})
}

func (c *simpleClient) PerformRequest(method string, path string, body io.Reader, configure func(r *http.Request)) (*simpleResponse, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	u = c.rootUrl.ResolveReference(u)
	return c.performRequestUrl(method, u, body, configure)
}

func (client *simpleClient) performRequestUrl(method string, url *url.URL, body io.Reader, configure func(r *http.Request)) (res *simpleResponse, err error) {
	req, err := http.NewRequest(method, url.String(), body)
	if err != nil {
		return nil, err
	}
	if client.PrepareRequest != nil {
		client.PrepareRequest(req)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", apiPayloadVersion)

	if configure != nil {
		configure(req)
	}

	key := cacheKey(req)
	if cachedResponse := client.cacheRead(key, req); cachedResponse != nil {
		res = &simpleResponse{cachedResponse}
		return
	}

	httpResponse, err := client.httpClient.Do(req)
	if err != nil {
		return
	}

	client.cacheWrite(key, httpResponse)
	res = &simpleResponse{httpResponse}

	return
}

func cacheKey(req *http.Request) string {
	path := strings.Replace(req.URL.EscapedPath(), "/", "-", -1)
	if len(path) > 1 {
		path = strings.TrimPrefix(path, "-")
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	hash := md5.New()
	fmt.Fprintf(hash, "%d:", cacheVersion)
	io.WriteString(hash, req.Header.Get("Accept"))
	io.WriteString(hash, req.Header.Get("Authorization"))
	queryParts := strings.Split(req.URL.RawQuery, "&")
	sort.Strings(queryParts)
	for _, q := range queryParts {
		fmt.Fprintf(hash, "%s&", q)
	}
	if isGraphQL(req) && req.Body != nil {
		if b, err := ioutil.ReadAll(req.Body); err == nil {
			req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
			hash.Write(b)
		}
	}
	return fmt.Sprintf("%s/%s_%x", host, path, hash.Sum(nil))
}

func (c *simpleClient) cacheRead(key string, req *http.Request) (res *http.Response) {
	if c.CacheTTL > 0 && canCache(req) {
		f := cacheFile(key)
		cacheInfo, err := os.Stat(f)
		if err != nil {
			return
		}
		if time.Since(cacheInfo.ModTime()).Seconds() > float64(c.CacheTTL) {
			return
		}
		cf, err := os.Open(f)
		if err != nil {
			return
		}
		defer cf.Close()

		cb, err := ioutil.ReadAll(cf)
		if err != nil {
			return
		}
		parts := strings.SplitN(string(cb), "\r\n\r\n", 2)
		if len(parts) < 2 {
			return
		}

		res = &http.Response{
			Body:   ioutil.NopCloser(bytes.NewBufferString(parts[1])),
			Header: http.Header{},
		}
		headerLines := strings.Split(parts[0], "\r\n")
		if len(headerLines) < 1 {
			return
		}
		if proto := strings.SplitN(headerLines[0], " ", 3); len(proto) >= 3 {
			res.Proto = proto[0]
			res.Status = fmt.Sprintf("%s %s", proto[1], proto[2])
			if code, _ := strconv.Atoi(proto[1]); code > 0 {
				res.StatusCode = code
			}
		}
		for _, line := range headerLines[1:] {
			kv := strings.SplitN(line, ":", 2)
			if len(kv) >= 2 {
				res.Header.Add(kv[0], strings.TrimLeft(kv[1], " "))
			}
		}
	}
	return
}

type readCloserCallback struct {
	Callback func()
	Closer   io.Closer
	io.Reader
}

func (rc *readCloserCallback) Close() error {
	err := rc.Closer.Close()
	if err == nil {
		rc.Callback()
	}
	return err
}

func (client *simpleClient) cacheWrite(key string, res *http.Response) {
	if client.CacheTTL > 0 && canCache(res.Request) && res.StatusCode < 500 && res.StatusCode != 403 {
		bodyCopy := &bytes.Buffer{}
		bodyReplacement := readCloserCallback{
			Reader: io.TeeReader(res.Body, bodyCopy),
			Closer: res.Body,
			Callback: func() {
				f := cacheFile(key)
				err := os.MkdirAll(filepath.Dir(f), 0771)
				if err != nil {
					return
				}
				cf, err := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
				if err != nil {
					return
				}
				defer cf.Close()
				fmt.Fprintf(cf, "%s %s\r\n", res.Proto, res.Status)
				res.Header.Write(cf)
				fmt.Fprintf(cf, "\r\n")
				io.Copy(cf, bodyCopy)
			},
		}
		res.Body = &bodyReplacement
	}
}

func (res *simpleResponse) ErrorInfo() (msg *errorInfo, err error) {
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	msg = &errorInfo{}
	err = json.Unmarshal(body, msg)
	if err != nil {
		msgSimple := &errorInfoSimple{}
		if err = json.Unmarshal(body, msgSimple); err == nil {
			msg.Message = msgSimple.Message
			for _, errMsg := range msgSimple.Errors {
				msg.Errors = append(msg.Errors, fieldError{
					Code:    "custom",
					Message: errMsg,
				})
			}
		}
	}
	if err == nil {
		msg.Response = res.Response
	}

	return
}

func isGraphQL(req *http.Request) bool {
	return req.URL.Path == "/graphql"
}

func canCache(req *http.Request) bool {
	return strings.EqualFold(req.Method, "GET") || isGraphQL(req)
}

func cacheFile(key string) string {
	return path.Join(os.TempDir(), "hub", "api", key)
}

func (res *simpleResponse) Link(name string) string {
	linkVal := res.Header.Get("Link")
	re := regexp.MustCompile(`<([^>]+)>; rel="([^"]+)"`)
	for _, match := range re.FindAllStringSubmatch(linkVal, -1) {
		if match[2] == name {
			return match[1]
		}
	}
	return ""
}

func (res *simpleResponse) Unmarshal(dest interface{}) (err error) {
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	return json.Unmarshal(body, dest)
}

func (client *simpleClient) PostJSON(path string, payload interface{}) (*simpleResponse, error) {
	return client.jsonRequest("POST", path, payload, nil)
}

func (c *simpleClient) Get(path string) (*simpleResponse, error) {
	return c.performRequest("GET", path, nil, nil)
}