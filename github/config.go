package github

import (
	"github.com/mitchellh/go-homedir"
	"os"
	"path/filepath"
	"strings"
)

type Host struct {
	Host        string `toml:"host"`
	User        string `toml:"user"`
	AccessToken string `toml:"access_token"`
	Protocol    string `toml:"protocol"`
	UnixSocket  string `toml:"unix_socket,omitempty"`
}

type yamlHost struct {
	User       string `yaml:"user"`
	OAuthToken string `yaml:"oauth_token"`
	Protocol   string `yaml:"protocol"`
	UnixSocket string `yaml:"unix_socket,omitempty"`
}

type Config struct {
	Hosts []*Host `toml:"hosts"`
}

var currentConfig *Config
var configLoadedFrom = ""

func CurrentConfig() *Config {
	filename := configsFile()
	if configLoadedFrom != filename {
		currentConfig =	&Config{}
		newConfigService().Load(filename, currentConfig)
		configLoadedFrom = filename
	}

	return currentConfig
}

var defaultConfigsFile string

func configsFile() string {
	if configFromEnv := os.Getenv("HUB_CONFIG"); configFromEnv != "" {
		return configFromEnv
	}
	if defaultConfigsFile == "" {
		var err error
		defaultConfigsFile, err = determineConfigLocation()
		utils.Check(err)
	}
	return defaultConfigsFile
}

func homeConfig() (string, error) {
	if home, err := homedir.Dir(); err != nil {
		return "", err
	} else {
		return filepath.Join(home, ".config"), nil
	}
}

func determineConfigLocation() (string, error) {
	var err error

	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	configDir := xdgHome
	if configDir == "" {
		if configDir, err = homeConfig(); err != nil {
			return "", err
		}
	}

	xdgDirs := os.Getenv("XDG_CONFIG_DIRS")
	if xdgDirs == "" {
		xdgDirs = "/etc/xdg"
	}
	searchDirs := append([]string{configDir}, strings.Split(xdgDirs, ":")...)

	for _, dir := range searchDirs {
		filename := filepath.Join(dir, "hub")
		if _, err := os.Stat(filename); err == nil {
			return filename, nil
		}
	}

	configFile := filepath.Join(configDir, "hub")

	if configDir == xdgHome {
		if homeDir, _ := homeConfig(); homeDir != "" {
			legacyConfig := filepath.Join(homeDir, "hub")
			if _, err = os.Stat(legacyConfig); err == nil {
				ui.Errorf("Notice: config file found but not respected at: %s\n", legacyConfig)
				ui.Errorf("You might want to move it to `%s' to avoid re-authenticating.\n", configFile)
			}
		}
	}

	return configFile, nil
}