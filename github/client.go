package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	GitHubHost string = "github.com"
	OAuthAppURL string = "https://hub.github.com/"
)

type User struct {
	Login string `json:"login"`
}

type Repository struct {
	Name          string                 `json:"name"`
	FullName      string                 `json:"full_name"`
	Parent        *Repository            `json:"parent"`
	Owner         *User                  `json:"owner"`
	Private       bool                   `json:"private"`
	HasWiki       bool                   `json:"has_wiki"`
	Permissions   *RepositoryPermissions `json:"permissions"`
	HtmlUrl       string                 `json:"html_url"`
	DefaultBranch string                 `json:"default_branch"`
}

type RepositoryPermissions struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

type PullRequestSpec struct {
	Label string      `json:"label"`
	Ref   string      `json:"ref"`
	Sha   string      `json:"sha"`
	Repo  *Repository `json:"repo"`
}

type IssueLabel struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Milestone struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type Team struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Issue struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	User   *User  `json:"user"`

	PullRequest *PullRequest     `json:"pull_request"`
	Head        *PullRequestSpec `json:"head"`
	Base        *PullRequestSpec `json:"base"`

	MergeCommitSha      string `json:"merge_commit_sha"`
	MaintainerCanModify bool   `json:"maintainer_can_modify"`
	Draft               bool   `json:"draft"`

	Comments  int          `json:"comments"`
	Labels    []IssueLabel `json:"labels"`
	Assignees []User       `json:"assignees"`
	Milestone *Milestone   `json:"milestone"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	MergedAt  time.Time    `json:"merged_at"`

	RequestedReviewers []User `json:"requested_reviewers"`
	RequestedTeams     []Team `json:"requested_teams"`

	ApiUrl  string `json:"url"`
	HtmlUrl string `json:"html_url"`

	ClosedBy *User `json:"closed_by"`
}

type PullRequest Issue

func NewClient(host string) *Client {
	return newClientWithHost(&Host{Host: host})
}

func newClientWithHost(host *Host) *Client {
	return &Client{Host: host}
}

type Client struct {
	Host *Host
	cachedClient *simpleClient
}

func (client *Client) ensureAccessToken() error {
	if client.Host.AccessToken == "" {
		host, err := CurrentConfig().PromptForHost(client.Host.Host)
		if err != nil {
			return err
		}
		client.Host = host
	}
	return nil
}

func (client *Client) apiClient() *simpleClient {
	unixSocket := os.ExpandEnv(client.Host.UnixSocket)
	httpClient := newHttpClient(os.Getenv("HUB_TEST_HOST"), os.Getenv("HUB_VERBOSE") != "", unixSocket)
	apiRoot := client.absolute(normalizeHost(client.Host.Host))
	if !strings.HasPrefix(apiRoot.Host, "api.github.") {
		apiRoot.Path = "/api/v3/"
	}

	return &simpleClient{
		httpClient: httpClient,
		rootUrl: apiRoot,
	}
}

func (client *Client) absolute(host string) *url.URL {
	u, err := url.Parse("https://" + host + "/")
	if err != nil {
		panic(err)
	} else if client.Host != nil && client.Host.Protocol != "" {
		u.Scheme = client.Host.Protocol
	}
	return u
}

func normalizeHost(host string) string {
	if host == "" {
		return GitHubHost
	} else if strings.EqualFold(host, GitHubHost) {
		return "api.github.com"
	} else if strings.EqualFold(host, "github.localhost") {
		return "api.github.localhost"
	} else {
		return strings.ToLower(host)
	}
}

func (client *Client) simpleApi() (c *simpleClient, err error) {
	err = client.ensureAccessToken()
	if err != nil {
		return
	}

	if client.cachedClient != nil {
		c = client.cachedClient
		return
	}

	c = client.apiClient()
	c.PrepareRequest = func(req *http.Request) {
		clientDomain := normalizeHost(client.Host.Host)
		if strings.HasPrefix(clientDomain, "api.github.") {
			clientDomain = strings.TrimPrefix(clientDomain, "api.")
		}
		requestHost := strings.ToLower(req.URL.Host)
		if requestHost == clientDomain || strings.HasSuffix(requestHost, "."+clientDomain) {
			req.Header.Set("Authorization", "token "+client.Host.AccessToken)
		}
	}

	client.cachedClient = c
	return
}

func (client *Client) FetchPullRequests(project *Project, filterParams map[string]interface{}, limit int,
filter func(pr *PullRequest) bool) (prs []PullRequest, err error) {

	api, err := client.simpleApi()
	if err != nil {
		return
	}
	path := fmt.Sprintf("repos/%s/%s/pulls?per_page=%d", project.Owner, project.Name, perPage(limit, 100))
	if filterParams != nil {
		path = addQuery(path, filterParams)
	}

	prs = []PullRequest{}
	var res *simpleResponse

	for path != "" {
		res, err :=	api.GetFile(path, draftsType)
		if err = checkStatus(200, "fetching pull requests", res, err); err != nil {
			return
		}
		path = res.Link("next")

		pullsPage := []PullRequest{}
		if err = res.Unmarshal(&pullsPage); err != nil {
			return
		}

		for _, pr := range pullsPage {
			if filter == nil || filter(&pr) {
				prs = append(prs, pr)
				if limit > 0 && len(prs) == limit {
					path = ""
					break
				}
			}
		}
	}
	return
}

func addQuery(path string, params map[string]interface{}) string {
	if len(params) == 0 {
		return path
	}

	query := url.Values{}
	for key, val := range params {
		switch v := val.(type) {
		case string:
			query.Add(key, v)
		case nil:
			query.Add(key, "")
		case int:
			query.Add(key, fmt.Sprintf("%d", v))
		case bool:
			query.Add(key, fmt.Sprintf("%v", v))
		}
	}
	sep := "?"
	if strings.Contains(path, sep) {
		sep = "&"
	}
	return path + sep + query.Encode()
}

func perPage(limit, max int) int {
	if limit > 0 {
		limit = limit + (limit / 2)
		if limit < max {
			return limit
		}
	}
	return max
}

func checkStatus(expectedStatus int, action string, response *simpleResponse, err error) error {
	if err != nil {
		return fmt.Errorf("error %s: %s", action, err.Error())
	} else if response.StatusCode != expectedStatus {
		errInfo, err := response.ErrorInfo()
		if err == nil {
			return FormatError(action, errInfo)
		} else {
			return fmt.Errorf("error %s: %s (HTTP %d)", action, err.Error(), response.StatusCode)
		}
	} else {
		return nil
	}
}

func FormatError(action string, err error) (ee error) {
	switch e := err.(type) {
	default:
		ee = err
	case *errorInfo:
		statusCode := e.Response.StatusCode
		var reason string
		if s := strings.SplitN(e.Response.Status, " ", 2); len(s) >= 2 {
			reason = strings.TrimSpace(s[1])
		}

		errStr := fmt.Sprintf("Error %s: %s (HTTP %d)", action, reason, statusCode)

		var errorSentences []string
		for _, err := range e.Errors {
			switch err.Code {
			case "custom":
				errorSentences = append(errorSentences, err.Message)
			case "missing_field":
				errorSentences = append(errorSentences, fmt.Sprintf("Missing field: \"%s\"", err.Field))
			case "already_exists":
				errorSentences = append(errorSentences, fmt.Sprintf("Duplicate value for \"%s\"", err.Field))
			case "invalid":
				errorSentences = append(errorSentences, fmt.Sprintf("Invalid value for \"%s\"", err.Field))
			case "unauthorized":
				errorSentences = append(errorSentences, fmt.Sprintf("Not allowed to change field \"%s\"", err.Field))
			}
		}

		var errorMessage string
		if len(errorSentences) > 0 {
			errorMessage = strings.Join(errorSentences, "\n")
		} else {
			errorMessage = e.Message
			if action == "getting current user" && e.Message == "Resource not accessible by integration" {
				errorMessage = errorMessage + "\nYou must specify GITHUB_USER via environment variable."
			}
		}

		if errorMessage != "" {
			errStr = fmt.Sprintf("%s\n%s", errStr, errorMessage)
		}

		ee = fmt.Errorf(errStr)
	}

	return
}

func NewClientWithHost(host *Host) *Client {
	return &Client{Host: host}
}

func (client *Client) FindOrCreateToken(user, password, twoFactorCode string) (token string, err error) {
	api := client.apiClient()

	if len(password) >= 40 && isToken(api, password) {
		return password, nil
	}

	params := map[string]interface{}{
		"scopes":   []string{"repo"},
		"note_url": OAuthAppURL,
	}

	api.PrepareRequest = func(req *http.Request) {
		req.SetBasicAuth(user, password)
		if twoFactorCode != "" {
			req.Header.Set("X-GitHub-OTP", twoFactorCode)
		}
	}

	count := 1
	maxTries := 9
	for {
		params["note"], err = authTokenNote(count)
		if err != nil {
			return
		}

		res, postErr := api.PostJSON("authorizations", params)
		if postErr != nil {
			err = postErr
			break
		}

		if res.StatusCode == 201 {
			auth := &AuthorizationEntry{}
			if err = res.Unmarshal(auth); err != nil {
				return
			}
			token = auth.Token
			break
		} else if res.StatusCode == 422 && count < maxTries {
			count++
		} else {
			errInfo, e := res.ErrorInfo()
			if e == nil {
				err = errInfo
			} else {
				err = e
			}
			return
		}
	}

	return
}

func isToken(api *simpleClient, password string) bool {
	api.PrepareRequest = func(req *http.Request) {
		req.Header.Set("Authorization", "token "+password)
	}

	res, _ := api.Get("user")
	if res != nil && res.StatusCode == 200 {
		return true
	}
	return false
}

func (client *simpleClient) jsonRequest(method, path string, body interface{}, configure func(*http.Request)) (*simpleResponse, error) {
	json, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(json)

	return client.performRequest(method, path, buf, func(req *http.Request) {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		if configure != nil {
			configure(req)
		}
	})
}

func authTokenNote(num int) (string, error) {
	n := os.Getenv("USER")

	if n == "" {
		n = os.Getenv("USERNAME")
	}

	if n == "" {
		whoami := exec.Command("whoami")
		whoamiOut, err := whoami.Output()
		if err != nil {
			return "", err
		}
		n = strings.TrimSpace(string(whoamiOut))
	}

	h, err := os.Hostname()
	if err != nil {
		return "", err
	}

	if num > 1 {
		return fmt.Sprintf("hub for %s@%s %d", n, h, num), nil
	}

	return fmt.Sprintf("hub for %s@%s", n, h), nil
}

type AuthorizationEntry struct {
	Token string `json:"token"`
}

func (client *Client) CurrentUser() (user *User, err error) {
	api, err := client.simpleApi()
	if err != nil {
		return
	}

	res, err := api.Get("user")
	if err = checkStatus(200, "getting current user", res, err); err != nil {
		return
	}

	user = &User{}
	err = res.Unmarshal(user)
	return
}