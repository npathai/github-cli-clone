package github

import "time"

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