package git

import "net/url"

func Remotes() ([]string, error) {
	return []string{}, nil
}

func ParseUrl(rawUrl string) (u *url.URL, err error) {
	return &url.URL{}, nil
}