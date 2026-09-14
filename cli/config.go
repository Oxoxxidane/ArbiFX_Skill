package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct {
	Ports    map[string]int    `json:"ports"`
	Sources  map[string]string `json:"sources"`
	Warnings []string          `json:"warnings"`
}

func portNumber(value string) (int, error) {
	n, e := strconv.Atoi(value)
	if e != nil || n < 1 || n > 65535 {
		return 0, input("Port must be an integer in 1..65535.")
	}
	return n, nil
}

// Read only ArbiFX-generated [local_http] integer keys; never output other configuration or secrets.
// This reader matches the host configuration writer; it is not a general TOML editor.
func readPorts(path string, explicit bool) (map[string]string, *cliError) {
	file, err := os.Open(path)
	if os.IsNotExist(err) && !explicit {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, input("Cannot read the configuration file.")
	}
	defer file.Close()
	result := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), maxBody)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))
		if strings.HasPrefix(line, "[") {
			section = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			continue
		}
		if section != "[local_http]" {
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		if key != "ae_global_port" && key != "start_port" {
			continue
		}
		if _, exists := result[key]; exists {
			return nil, input("Configuration contains duplicate local_http port keys.")
		}
		result[key] = strings.TrimSpace(strings.SplitN(pair[1], "#", 2)[0])
	}
	if scanner.Err() != nil {
		return nil, input("Failed to read local_http configuration.")
	}
	return result, nil
}
func loadConfig(opts map[string]string) (config, *cliError) {
	result := config{map[string]int{}, map[string]string{}, []string{}}
	path, explicit := opts["config"]
	if !explicit {
		home, err := os.UserHomeDir()
		if err != nil {
			return result, input("Cannot locate the home directory; specify --config.")
		}
		path = filepath.Join(home, ".ArbiFX", "config.toml")
	}
	values, e := readPorts(path, explicit)
	if e != nil {
		return result, e
	}
	for _, target := range []string{"ae", "start"} {
		key, defaultValue := "ae_global_port", "28154"
		if target == "start" {
			key, defaultValue = "start_port", "28153"
		}
		value, source := defaultValue, "default"
		if c, ok := values[key]; ok {
			value, source = c, "config"
		}
		if env, ok := os.LookupEnv("ARBIFX_" + strings.ToUpper(target) + "_PORT"); ok {
			value, source = env, "env"
		}
		if flag, ok := opts[target+"-port"]; ok {
			value, source = flag, "flag"
		}
		port, err := portNumber(value)
		if err != nil {
			if source != "config" {
				return result, input("Invalid port option/environment variable: " + target)
			}
			port, _ = portNumber(defaultValue)
			source = "default"
			result.Warnings = append(result.Warnings, "Invalid local_http."+key+"; using the default port.")
		}
		result.Ports[target], result.Sources[target] = port, source
	}
	return result, nil
}
