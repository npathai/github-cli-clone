package github

import (
	"gopkg.in/yaml.v2"
	"io"
)

type configEncoder interface {
	Encode(w io.Writer, c *Config)	error
}

type yamlConfigEncoder struct {
}

func (enc *yamlConfigEncoder) Encode(w io.Writer, c *Config) error {
	yc := yaml.MapSlice{}
	for _, h := range c.Hosts {
		yc = append(yc, yaml.MapItem{
			Key: h.Host,
			Value: []yamlHost{
				{
					User:       h.User,
					OAuthToken: h.AccessToken,
					Protocol:   h.Protocol,
					UnixSocket: h.UnixSocket,
				},
			},
		})
	}

	d, err := yaml.Marshal(yc)
	if err != nil {
		return err
	}

	n, err := w.Write(d)
	if err == nil && n < len(d) {
		err = io.ErrShortWrite
	}

	return err
}
