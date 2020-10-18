package github

import (
	"bufio"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/npathai/github-cli-clone/ui"
	"github.com/npathai/github-cli-clone/utils"
	"golang.org/x/crypto/ssh/terminal"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

func (config *Config) PromptForHost(host string) (h *Host, err error) {
	token := config.DetectToken()
	tokenFromEnv := token != ""

	if host != GitHubHost {
		if _, e := url.Parse("https://" + host); e != nil {
			err = fmt.Errorf("invalid hostname: %q", host)
			return
		}
	}

	h = config.Find(host)
	if h != nil {
		if h.User == "" {
			utils.Check(CheckWriteable(configsFile()))
			// User is missing from the config: this is a broken config probably
			// because it was created with an old (broken) version of hub. Let's fix
			// it now. See issue #1007 for details.
			user := config.PromptForUser(host)
			if user == "" {
				utils.Check(fmt.Errorf("missing user"))
			}
			h.User = user
			err := newConfigService().Save(configsFile(), config)
			utils.Check(err)
		}
		if tokenFromEnv {
			h.AccessToken = token
		} else {
			return
		}
	} else {
		h = &Host{
			Host:        host,
			AccessToken: token,
			Protocol:    "https",
		}
		config.Hosts = append(config.Hosts, h)
	}

	client := NewClientWithHost(h)

	if !tokenFromEnv {
		utils.Check(CheckWriteable(configsFile()))
		err = config.authorizeClient(client, host)
		if err != nil {
			return
		}
	}

	userFromEnv := os.Getenv("GITHUB_USER")
	repoFromEnv := os.Getenv("GITHUB_REPOSITORY")
	if userFromEnv == "" && repoFromEnv != "" {
		repoParts := strings.SplitN(repoFromEnv, "/", 2)
		if len(repoParts) > 0 {
			userFromEnv = repoParts[0]
		}
	}
	if tokenFromEnv && userFromEnv != "" {
		h.User = userFromEnv
	} else {
		var currentUser *User
		currentUser, err = client.CurrentUser()
		if err != nil {
			return
		}
		h.User = currentUser.Login
	}

	if !tokenFromEnv {
		err = newConfigService().Save(configsFile(), config)
	}

	return
}

func (c *Config) DetectToken() string {
	return os.Getenv("GITHUB_TOKEN")
}

func (c *Config) Find(host string) *Host {
	for _, h := range c.Hosts {
		if h.Host == host {
			return h
		}
	}

	return nil
}

func CheckWriteable(filename string) error {
	// Check if file exists already. if it doesn't, we will delete it after
	// checking for writeabilty
	fileExistsAlready := false

	if _, err := os.Stat(filename); err == nil {
		fileExistsAlready = true
	}

	err := os.MkdirAll(filepath.Dir(filename), 0771)
	if err != nil {
		return err
	}

	w, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	w.Close()

	if !fileExistsAlready {
		err := os.Remove(filename)
		if err != nil {
			return err
		}
	}
	return nil
}

func (config *Config) PromptForUser(host string) (user string) {
	user = os.Getenv("GITHUB_USER")
	if user != "" {
		return
	}

	ui.Printf("%s username: ", host)
	user = config.scanLine()

	return
}

func (c *Config) scanLine() string {
	var line string
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		line = scanner.Text()
	}
	utils.Check(scanner.Err())

	return line
}

func (config *Config) authorizeClient(client *Client, host string) (err error) {
	user := config.PromptForUser(host)
	pass := config.PromptForPassword(host, user)

	var code, token string
	for {
		token, err = client.FindOrCreateToken(user, pass, code)
		if err == nil {
			break
		}

		if ae, ok := err.(*errorInfo); ok && strings.HasPrefix(ae.Response.Header.Get("X-GitHub-OTP"), "required;") {
			if code != "" {
				ui.Errorln("warning: invalid two-factor code")
			}
			code = config.PromptForOTP()
		} else {
			break
		}
	}

	if err == nil {
		client.Host.AccessToken = token
	}

	return
}

func (c *Config) PromptForPassword(host, user string) (pass string) {
	pass = os.Getenv("GITHUB_PASSWORD")
	if pass != "" {
		return
	}

	ui.Printf("%s password for %s (never stored): ", host, user)
	if ui.IsTerminal(os.Stdin) {
		if password, err := getPassword(); err == nil {
			pass = password
		}
	} else {
		pass = c.scanLine()
	}

	return
}

func (c *Config) PromptForOTP() string {
	fmt.Print("two-factor authentication code: ")
	return c.scanLine()
}

func getPassword() (string, error) {
	stdin := int(syscall.Stdin)
	initialTermState, err := terminal.GetState(stdin)
	if err != nil {
		return "", err
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		terminal.Restore(stdin, initialTermState)
		switch sig := s.(type) {
		case syscall.Signal:
			if int(sig) == 2 {
				fmt.Println("^C")
			}
		}
		os.Exit(1)
	}()

	passBytes, err := terminal.ReadPassword(stdin)
	if err != nil {
		return "", err
	}

	signal.Stop(c)
	fmt.Print("\n")
	return string(passBytes), nil
}