package git

import (
	"bufio"
	"github.com/mitchellh/go-homedir"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	hostRegexStr = "(?i)^[ \t]*(host|hostname)[ \t]+(.+)$"
)

type SSHConfig map[string]string

func newSSHConfigReader() *SSHConfigReader {
	configFiles := []string {
		"/etc/ssh_config",
		"/etc/ssh/ssh_config",
	}
	if home, err := homedir.Dir(); err == nil {
		userConfig := filepath.Join(home, ".ssh", "config")
		configFiles = append([]string{userConfig}, configFiles...)
	}

	return &SSHConfigReader{
		Files: configFiles,
	}
}

type SSHConfigReader struct {
	Files []string
}

func (reader *SSHConfigReader) Read() SSHConfig {
	config := make(SSHConfig)
	hostRegex := regexp.MustCompile(hostRegexStr)

	for _, filename := range reader.Files {
		reader.ReadFile(config, hostRegex, filename)
	}

	return config
}

func (reader *SSHConfigReader) ReadFile(config SSHConfig, hostRegex *regexp.Regexp, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	hosts := []string{"*"}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		match := hostRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		names := strings.Fields(match[2])
		if strings.EqualFold(match[1], "host") {
			hosts = names
		} else {
			for _, host := range hosts {
				for _, name := range names {
					config[host] = expandTokens(name, host)
				}
			}
		}
	}

	return scanner.Err()
}

func expandTokens(text, host string) string {
	re := regexp.MustCompile(`%[%h]`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		switch match {
		case "%h":
			return host
		case "%%":
			return "%"
		}
		return ""
	})
}