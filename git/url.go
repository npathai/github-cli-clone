package git

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	cachedSSHConfig SSHConfig
	protocolRegex   = regexp.MustCompile("^[A-Za-z_+-]+://")
)

type URLParser struct {
	SSHConfig SSHConfig
}

func (p *URLParser) Parse(rawUrl string) (u *url.URL, err error) {
	if !protocolRegex.MatchString(rawUrl) &&
		strings.Contains(rawUrl, ":") &&
		// Not a windows path
		!strings.Contains(rawUrl, "\\") {

		rawUrl = "ssh://" + strings.Replace(rawUrl, ":", "/", 1)
	}

	u, err = url.Parse(rawUrl)
	if err != nil {
		return
	}

	if u.Scheme == "git+ssh" {
		u.Scheme = "ssh"
	}

	if u.Scheme != "ssh" {
		return
	}

	if strings.HasPrefix(u.Path, "//") {
		u.Path = strings.TrimPrefix(u.Path, "/")
	}

	if idx := strings.Index(u.Host, ":"); idx >= 0 {
		u.Host = u.Host[0:idx]
	}

	sshHost := p.SSHConfig[u.Host]
	// ignore replacing host that fixes for limited network
	// https://help.github.com/articles/using-ssh-over-the-https-port
	ignoredHost := u.Host == "github.com" && sshHost == "ssh.github.com"
	if !ignoredHost && sshHost != "" {
		u.Host = sshHost
	}

	return
}

func ParseUrl(rawUrl string) (u *url.URL, err error) {
	if cachedSSHConfig == nil {
		cachedSSHConfig = newSSHConfigReader().Read()
	}

	parser := &URLParser{cachedSSHConfig}

	return parser.Parse(rawUrl)
}
