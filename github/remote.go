package github

import (
	"fmt"
	"github.com/npathai/github-cli-clone/git"
	"net/url"
	"regexp"
	"strings"
)

var (
	OriginNamesInPriorityOrder = []string{"upstream", "github", "origin"}
)

type Remote struct {
	Name    string
	URL     *url.URL
	PushURL *url.URL
}


func Remotes() (remotes []Remote, err error) {
	re := regexp.MustCompile(`(.+)\s+(.+)\s+\((push|fetch)\)`)

	rs, err := git.Remotes()
	if err != nil {
		err = fmt.Errorf("can't load git remote")
		return
	}

	// build the remotes map
	remotesMap := make(map[string]map[string]string)
	for _, r := range rs {
		if re.MatchString(r) {
			match := re.FindStringSubmatch(r)
			name :=	strings.TrimSpace(match[1])
			url := strings.TrimSpace(match[2])
			urlType := strings.TrimSpace(match[3])
			urlTypeMap, ok := remotesMap[name]
			if !ok {
				urlTypeMap = make(map[string]string)
				remotesMap[name] = urlTypeMap
			}
			urlTypeMap[urlType] = url
		}
	}

	// construct remotes in priority order
	names := OriginNamesInPriorityOrder
	for _, name := range names {
		if urlTypeMap, ok := remotesMap[name]; ok {
			remote, err := newRemote(name, urlTypeMap)
			if err == nil {
				remotes = append(remotes, remote)
				delete(remotesMap, name)
			}
		}
	}

	// For remaining remotes
	for name, urlTypeMap := range remotesMap {
		remote, err := newRemote(name, urlTypeMap)
		if err == nil {
			remotes = append(remotes, remote)
		}
	}

	return
}

func newRemote(name string, urlTypeMap map[string]string) (Remote, error) {
	remote := Remote{}
	fetchUrl, fErr := git.ParseUrl(urlTypeMap["fetch"])
	pushUrl, pErr := git.ParseUrl(urlTypeMap["push"])
	if fErr != nil && pErr != nil {
		return remote, fmt.Errorf("no valid remote URLs")
	}

	remote.Name = name
	if fErr == nil {
		remote.URL = fetchUrl
	}
	if pErr == nil {
		remote.PushURL = pushUrl
	}
	return remote, nil
}

func (remote *Remote) Project() (*Project, error) {
	return &Project{}, nil
}
