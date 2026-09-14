package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var boolOptions = []string{"json", "dry-run", "offline", "allow-unknown", "clear", "help", "version"}
var valueOptions = []string{"ae-port", "start-port", "config", "timeout", "target", "body", "body-file", "id", "slot", "type", "path", "code", "code-file", "text", "text-file", "font", "tag", "value", "prompt", "prompt-file", "parameters", "parameters-file", "comp", "layer", "comp-id", "layer-id", "layer-index", "effect-index"}
var globals = []string{"json", "dry-run", "ae-port", "start-port", "config", "timeout", "help", "version"}

// Parse with the standard flag package; reorder flags to allow global options before or after commands.
func parse(argv []string) ([]string, map[string]string, *cliError) {
	fs := flag.NewFlagSet("arbifx", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, key := range boolOptions {
		fs.Bool(key, false, "")
	}
	for _, key := range valueOptions {
		fs.String(key, "", "")
	}
	var flags, words []string
	seen := map[string]bool{}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "-h" {
			arg = "--help"
		}
		if !strings.HasPrefix(arg, "--") {
			words = append(words, arg)
			continue
		}
		name := strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--")
		if fs.Lookup(name) == nil {
			return nil, nil, input("Unknown option: --" + name)
		}
		if seen[name] {
			return nil, nil, input("Duplicate option: --" + name)
		}
		seen[name] = true
		flags = append(flags, arg)
		if contains(valueOptions, name) && !strings.Contains(arg, "=") {
			i++
			if i >= len(argv) {
				return nil, nil, input("Missing value for option: --" + name)
			}
			flags = append(flags, argv[i])
		}
	}
	if err := fs.Parse(flags); err != nil {
		return nil, nil, input("Invalid command arguments: " + err.Error())
	}
	opts := map[string]string{}
	fs.Visit(func(f *flag.Flag) { opts[f.Name] = f.Value.String() })
	return words, opts, nil
}
func enabled(opts map[string]string, key string) bool { return opts[key] == "true" }
func checkOptions(opts map[string]string, extra ...string) *cliError {
	for key := range opts {
		if !contains(globals, key) && !contains(extra, key) {
			return input("This command does not accept --" + key)
		}
	}
	return nil
}
func help(words []string) string {
	var b strings.Builder
	b.WriteString("ArbiFX CLI " + version + " — All 23 AE/Start local HTTP commands\n")
	b.WriteString("Usage: arbifx [--json] <command> [options]\n\n")
	b.WriteString("doctor [--target ae|start|both] [--offline]  Read-only diagnostics\ncommands [--target ae|start|both]  Command catalog\nae resolve --comp NAME [--layer NAME] [--comp-id N] [--layer-id N] [--layer-index N] [--effect-index N]\nrequest ae|start --body-file JSON  Raw POST /command (file - reads stdin)\n\n")
	for _, c := range commands {
		if len(words) > 0 && (words[0] == "ae" || words[0] == "start") && c.Target != words[0] {
			continue
		}
		if len(words) > 1 && strings.ReplaceAll(words[1], "-", "_") != c.Cmd {
			continue
		}
		fmt.Fprintf(&b, "%s %s", c.Target, strings.ReplaceAll(c.Cmd, "_", "-"))
		for _, key := range keys(c.Fields) {
			fmt.Fprintf(&b, " --%s <%s>", key, c.Fields[key])
		}
		fmt.Fprintf(&b, "\n    %s\n", c.Description)
	}
	b.WriteString("\nText fields support --FIELD-file UTF8_PATH (- for stdin). Asset paths support --clear.\nOBJ type: obj, mtl, texture1, texture2, texture3; slot: 0..7\nTag: " + strings.Join(tags, ", ") + "; value: true|false\n")
	b.WriteString("\nGlobal options: --json --dry-run --ae-port N --start-port N --config PATH --timeout SECONDS\n--dry-run never connects; default timeout is 30 seconds, with no automatic retries. Underscore command aliases are accepted.\nExit: 0 success; 2 input/config; 3 network/protocol; 4 server/script/instance-resolution failure\n")
	return b.String()
}
func readText(path string, stdin io.Reader) (string, *cliError) {
	var b []byte
	var err error
	if path == "-" {
		b, err = io.ReadAll(io.LimitReader(stdin, maxBody+1))
	} else {
		f, e := os.Open(path)
		if e != nil {
			return "", input("Cannot open the UTF-8 input file.")
		}
		defer f.Close()
		b, err = io.ReadAll(io.LimitReader(f, maxBody+1))
	}
	if err != nil || !utf8.Valid(b) {
		return "", input("Cannot read the UTF-8 input file.")
	}
	if len(b) > maxBody {
		return "", input("Input file exceeds 1 MiB.")
	}
	return strings.TrimPrefix(string(b), "\ufeff"), nil
}
func buildBody(spec *command, opts map[string]string, stdin io.Reader) (object, *cliError) {
	allowed := keys(spec.Fields)
	stdinCount := 0
	for field, kind := range spec.Fields {
		if kind == "text" {
			allowed = append(allowed, field+"-file")
			if opts[field+"-file"] == "-" {
				stdinCount++
			}
		}
		if kind == "asset" {
			allowed = append(allowed, "clear")
		}
	}
	if e := checkOptions(opts, allowed...); e != nil {
		return nil, e
	}
	if stdinCount > 1 {
		return nil, input("Only one field per request may read stdin.")
	}
	body := object{"cmd": spec.Cmd}
	for field, kind := range spec.Fields {
		value, has := opts[field]
		if filename, fileHas := opts[field+"-file"]; fileHas {
			if has {
				return nil, input("--" + field + " and its -file option are mutually exclusive.")
			}
			var e *cliError
			value, e = readText(filename, stdin)
			if e != nil {
				return nil, e
			}
			has = true
		}
		if kind == "asset" && enabled(opts, "clear") {
			if has {
				return nil, input("--path and --clear are mutually exclusive.")
			}
			value = ""
			has = true
		}
		if !has {
			return nil, input("Missing --" + field + ".")
		}
		switch kind {
		case "slot":
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, input("slot must be an integer in 0..7.")
			}
			body[field] = n
		case "bool":
			if value != "true" && value != "false" {
				return nil, input("value must be true or false.")
			}
			body[field] = value == "true"
		case "asset", "save", "load":
			if value != "" {
				var err error
				value, err = filepath.Abs(value)
				if err != nil {
					return nil, input("Cannot resolve an absolute path.")
				}
			}
			body[field] = value
			if kind == "save" {
				info, err := os.Stat(filepath.Dir(value))
				if err != nil || !info.IsDir() {
					return nil, input("The parent directory for save_afx must exist.")
				}
			}
			if kind == "load" {
				info, err := os.Stat(value)
				if err != nil || !info.Mode().IsRegular() {
					return nil, input("load_afx must point to an existing regular file.")
				}
			}
		default:
			body[field] = value
		}
	}
	if _, e := validate(spec.Target, body, false); e != nil {
		return nil, e
	}
	return body, nil
}
func instanceField(item object, key string) any {
	key = strings.ReplaceAll(key, "-", "_")
	if key == "comp" || key == "layer" {
		if nested, ok := item[key].(map[string]any); ok {
			return nested["name"]
		}
		return item[key]
	}
	if key == "comp_id" || key == "layer_id" || key == "layer_index" {
		pair := strings.Split(key, "_")
		if nested, ok := item[pair[0]].(map[string]any); ok {
			return nested[pair[1]]
		}
	}
	return item[key]
}
func targetSelection(opts map[string]string) (string, *cliError) {
	target := opts["target"]
	if target == "" {
		target = "both"
	}
	if !contains([]string{"ae", "start", "both"}, target) {
		return "", input("target must be ae, start, or both.")
	}
	return target, nil
}

func run(words []string, opts map[string]string, stdin io.Reader) (object, *cliError) {
	family := words[0]
	if family == "commands" {
		if len(words) != 1 {
			return nil, input("commands does not accept positional arguments.")
		}
		if e := checkOptions(opts, "target"); e != nil {
			return nil, e
		}
		target, e := targetSelection(opts)
		if e != nil {
			return nil, e
		}
		data := []command{}
		for _, c := range commands {
			if target == "both" || c.Target == target {
				data = append(data, c)
			}
		}
		return success(data), nil
	}
	seconds := 30.0
	if value, ok := opts["timeout"]; ok {
		var err error
		seconds, err = strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > 86400 {
			return nil, input("timeout must be a positive finite number of seconds no greater than 86400.")
		}
	}
	timeout := time.Duration(seconds * float64(time.Second))
	if timeout <= 0 {
		return nil, input("timeout is too small.")
	}
	cfg, e := loadConfig(opts)
	if e != nil {
		return nil, e
	}
	if family == "doctor" {
		if len(words) != 1 {
			return nil, input("doctor does not accept positional arguments.")
		}
		if e = checkOptions(opts, "target", "offline"); e != nil {
			return nil, e
		}
		target, e := targetSelection(opts)
		if e != nil {
			return nil, e
		}
		offline := enabled(opts, "offline") || enabled(opts, "dry-run")
		checks := object{}
		healthy := true
		for _, t := range []string{"ae", "start"} {
			if target != "both" && t != target {
				continue
			}
			if offline {
				checks[t] = object{"checked": false}
				continue
			}
			name := "status"
			if t == "start" {
				name = "get_tags"
			}
			res, err := request(t, cfg.Ports[t], object{"cmd": name}, timeout)
			if err == nil {
				data, ok := res["data"].(map[string]any)
				if !ok {
					err = failure("protocol", 3, "Diagnostic response data is not an object.", nil)
				} else if t == "ae" {
					state, found := data["online"]
					if !found {
						state = data["running"]
					}
					if state != true {
						err = failure("protocol", 3, "AE did not report being online.", nil)
					}
				} else {
					for _, tag := range tags {
						if _, ok := data[tag].(bool); !ok {
							err = failure("protocol", 3, "Start did not return all boolean tags.", nil)
							break
						}
					}
				}
			}
			if err != nil {
				healthy = false
				checks[t] = object{"checked": true, "reachable": false, "error": err}
			} else {
				checks[t] = object{"checked": true, "reachable": true, "response": res}
			}
		}
		return object{"ok": healthy, "data": object{"cli_version": version, "platform": runtime.GOOS + "-" + runtime.GOARCH, "auth": "not_required", "config": cfg, "offline": offline, "targets": checks, "hint": "AE must have initialized ArbiFX, and Start must be open. The protocol provides no server-version or task-completion query."}}, nil
	}
	if family == "ae" && len(words) == 2 && words[1] == "resolve" {
		fields := []string{"comp", "layer", "comp-id", "layer-id", "layer-index", "effect-index"}
		if e = checkOptions(opts, fields...); e != nil {
			return nil, e
		}
		filters := map[string]string{}
		for _, key := range fields {
			if value, ok := opts[key]; ok {
				if strings.Contains(key, "-") {
					n, err := strconv.Atoi(value)
					if err != nil || n < 0 {
						return nil, input("Instance indices/IDs must be nonnegative integers.")
					}
				}
				filters[key] = value
			}
		}
		if len(filters) == 0 {
			return nil, input("resolve requires at least one exact-match filter.")
		}
		if enabled(opts, "dry-run") {
			return success(object{"dry_run": true, "body": object{"cmd": "instances"}, "filters": filters}), nil
		}
		res, err := request("ae", cfg.Ports["ae"], object{"cmd": "instances"}, timeout)
		if err != nil {
			return nil, err
		}
		list, ok := res["data"].([]any)
		if !ok {
			return nil, failure("protocol", 3, "instances is not an array.", nil)
		}
		matches := []any{}
		for _, row := range list {
			item, ok := row.(map[string]any)
			if !ok {
				return nil, failure("protocol", 3, "An instance is not an object.", nil)
			}
			match := true
			for key, value := range filters {
				if fmt.Sprint(instanceField(item, key)) != value {
					match = false
					break
				}
			}
			if match {
				matches = append(matches, item)
			}
		}
		if len(matches) != 1 {
			return nil, failure("resolve", 4, "Instance match is not unique; refine the filters.", object{"count": len(matches), "matches": matches})
		}
		return success(matches[0]), nil
	}
	if len(words) != 2 {
		return nil, input("Expected an ae/start command or request ae|start.")
	}
	var body object
	var spec *command
	target := family
	if family == "request" {
		target = words[1]
		if !contains([]string{"ae", "start"}, target) {
			return nil, input("request target must be ae or start.")
		}
		if e = checkOptions(opts, "body", "body-file", "allow-unknown"); e != nil {
			return nil, e
		}
		text, hasBody := opts["body"]
		filename, hasFile := opts["body-file"]
		if hasBody == hasFile {
			return nil, input("Specify exactly one of --body or --body-file.")
		}
		if hasFile {
			text, e = readText(filename, stdin)
			if e != nil {
				return nil, e
			}
		}
		var err error
		body, err = decode([]byte(text))
		if err != nil {
			return nil, input("Input must be a strict UTF-8 JSON object.")
		}
		spec, e = validate(target, body, enabled(opts, "allow-unknown"))
		if e != nil {
			return nil, e
		}
	} else {
		spec = findCommand(target, strings.ReplaceAll(words[1], "-", "_"))
		if spec == nil {
			return nil, input("Unknown command; run arbifx --help.")
		}
		body, e = buildBody(spec, opts, stdin)
		if e != nil {
			return nil, e
		}
	}
	if _, e = encode(body); e != nil {
		return nil, e
	}
	if enabled(opts, "dry-run") {
		mutating := true
		if spec != nil {
			mutating = spec.Mutating
		}
		effects := []string{}
		if name, _ := body["cmd"].(string); contains([]string{"set_reference", "set_obj", "set_svg"}, name) {
			path, _ := body["path"].(string)
			_, err := os.Stat(path)
			effect := "load"
			if path == "" || os.IsNotExist(err) {
				effect = "clear"
			}
			effects = append(effects, effect)
		} else if name == "save_afx" {
			path, _ := body["path"].(string)
			if _, err := os.Stat(path); err == nil {
				effects = append(effects, "overwrite_existing_file")
			}
		}
		return success(object{"dry_run": true, "method": "POST", "url": fmt.Sprintf("http://127.0.0.1:%d/command", cfg.Ports[target]), "body": body, "mutating": mutating, "effects": effects}), nil
	}
	return request(target, cfg.Ports[target], body, timeout)
}
func execute(argv []string, stdin io.Reader, stdout io.Writer) int {
	words, opts, e := parse(argv)
	compact := enabled(opts, "json")
	if e == nil && enabled(opts, "version") {
		fmt.Fprintln(stdout, "arbifx", version, runtime.GOOS+"-"+runtime.GOARCH)
		return 0
	}
	if e == nil && (len(words) == 0 || enabled(opts, "help")) {
		fmt.Fprint(stdout, help(words))
		return 0
	}
	var result object
	code := 0
	if e == nil {
		result, e = run(words, opts, stdin)
	}
	if e != nil {
		result = object{"ok": false, "error": e}
		code = e.Code
	} else if result["ok"] == false {
		code = 3
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if !compact {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(result); err != nil {
		return 3
	}
	return code
}
func main() { os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout)) }
