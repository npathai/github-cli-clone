package github

type Host struct {
	Host        string `toml:"host"`
	User        string `toml:"user"`
	AccessToken string `toml:"access_token"`
	Protocol    string `toml:"protocol"`
	UnixSocket  string `toml:"unix_socket,omitempty"`
}
