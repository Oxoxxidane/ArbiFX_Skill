package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const version = "1.3.0"
const maxBody = 1024 * 1024

type object = map[string]any
type command struct {
	Target      string            `json:"target"`
	Cmd         string            `json:"cmd"`
	Fields      map[string]string `json:"fields"`
	Mutating    bool              `json:"mutating"`
	Description string            `json:"description"`
}

var tags = []string{"use_comp_camera", "is_filter", "is_alpha", "use_comp_light", "only_sdf"}
var objTypes = []string{"obj", "mtl", "texture1", "texture2", "texture3"}
var commands = []command{
	{"ae", "status", nil, false, "Read AE HTTP listener status"},
	{"ae", "instances", nil, false, "List current ArbiFX instances (each query refreshes instance IDs)"},
	{"ae", "project", nil, false, "Read composition, layer, and effect names and enabled flags"},
	{"ae", "preview_frame", map[string]string{"path": "png"}, true, "Export a real composition PNG to a new path; raw requests support comp_id and time"},
	{"ae", "open_start", map[string]string{"id": "id"}, true, "Open Start for an instance without rebinding an existing window"},
	{"ae", "script", map[string]string{"code": "text"}, true, "Execute an ExtendScript function body with emit and return"},
	{"start", "get_status", nil, false, "Read current busy/status text and pending task snapshot; not task history"},
	{"start", "get_api_status", nil, false, "Read configured/verified/verifying flags without validating the key or contacting the backend"},
	{"start", "set_reference", map[string]string{"path": "asset"}, true, "Set a reference file; empty or nonexistent paths clear it"},
	{"start", "get_reference", nil, false, "Read the reference filename and loaded state"},
	{"start", "set_obj", map[string]string{"slot": "slot", "type": "obj_type", "path": "asset"}, true, "Set OBJ/MTL/texture; empty or nonexistent paths clear the selected type"},
	{"start", "get_obj", nil, false, "Read all eight OBJ slots"},
	{"start", "set_svg", map[string]string{"slot": "slot", "path": "asset"}, true, "Set SVG; empty or nonexistent paths clear the slot"},
	{"start", "get_svg", nil, false, "Read all eight SVG slots"},
	{"start", "set_text", map[string]string{"text": "text"}, true, "Set TXT text"},
	{"start", "get_text", nil, false, "Read TXT text"},
	{"start", "set_font", map[string]string{"font": "id"}, true, "Set a font ID (including default)"},
	{"start", "get_font", nil, false, "Read the current font object"},
	{"start", "set_tag", map[string]string{"tag": "tag", "value": "bool"}, true, "Set one of the five boolean tags"},
	{"start", "get_tags", nil, false, "Read all tags"},
	{"start", "set_prompt", map[string]string{"prompt": "text", "parameters": "text"}, true, "Set both prompt and parameter-control text"},
	{"start", "get_prompt", nil, false, "Read prompt and parameter-control text"},
	{"start", "send", nil, true, "Submit the current Start generation task; may use a paid backend"},
	{"start", "save_afx", map[string]string{"path": "save"}, true, "Save the existing scene to an .afx file"},
	{"start", "load_afx", map[string]string{"path": "load"}, true, "Load an .afx file and apply it to the bound instance"},
	{"start", "close", nil, true, "Close the Start window and listener normally"},
}

type cliError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Details object `json:"details,omitempty"`
	Code    int    `json:"-"`
}

func (e *cliError) Error() string { return e.Message }
func failure(kind string, code int, message string, details object) *cliError {
	return &cliError{kind, message, details, code}
}
func input(message string) *cliError { return failure("input", 2, message, nil) }
func success(data any) object        { return object{"ok": true, "data": data} }
func findCommand(target, cmd string) *command {
	for i := range commands {
		if commands[i].Target == target && commands[i].Cmd == cmd {
			return &commands[i]
		}
	}
	return nil
}
func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
func keys(fields map[string]string) []string {
	result := make([]string, 0, len(fields))
	for key := range fields {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
func validate(target string, body object, allowUnknown bool) (*command, *cliError) {
	name, ok := body["cmd"].(string)
	if !ok || name == "" {
		return nil, input("JSON must contain a nonempty string cmd.")
	}
	spec := findCommand(target, name)
	if spec == nil {
		if allowUnknown {
			return nil, nil
		}
		return nil, input("Unknown command; inspect commands first. Future commands require --allow-unknown.")
	}
	if target == "ae" && (name == "status" || name == "project" || name == "preview_frame") {
		return spec, validateInspection(body)
	}
	if len(body) != len(spec.Fields)+1 {
		return nil, input("Request fields must be exactly cmd and: " + strings.Join(keys(spec.Fields), ", "))
	}
	for key, kind := range spec.Fields {
		value, exists := body[key]
		if !exists {
			return nil, input("Missing field: " + key)
		}
		str, isString := value.(string)
		valid := isString
		switch kind {
		case "slot":
			n, err := jsonNumber(value)
			valid = err == nil && n >= 0 && n <= 7
		case "bool":
			_, valid = value.(bool)
		case "id", "save", "load":
			valid = isString && strings.TrimSpace(str) != ""
		case "obj_type":
			valid = isString && contains(objTypes, str)
		case "tag":
			valid = isString && contains(tags, str)
		}
		if !valid {
			return nil, input(fmt.Sprintf("Field %s does not match type/range %s.", key, kind))
		}
	}
	return spec, nil
}

// New optional query fields use raw request JSON; legacy named commands stay unchanged.
func validateInspection(body object) *cliError {
	name := body["cmd"].(string)
	allowed := []string{"cmd"}
	view := ""
	if name == "status" {
		allowed = append(allowed, "diagnose", "timeout_ms")
	} else if name == "preview_frame" {
		allowed = append(allowed, "comp_id", "time", "path")
		if p, ok := body["path"].(string); !ok || strings.TrimSpace(p) == "" {
			return input("preview_frame requires path.")
		} else if strings.ContainsRune(p, '\x00') || !strings.HasSuffix(strings.ToLower(p), ".png") {
			return input("preview_frame requires a PNG path without null characters.")
		}
	} else if _, exists := body["target"]; exists {
		view, _ = body["target"].(string)
		if !contains([]string{"comp", "layers", "properties", "keyframes"}, view) {
			return input("Invalid project target.")
		}
		allowed = append(allowed, "target", "comp_id")
		if view != "comp" {
			allowed = append(allowed, "limit")
		}
		if view == "layers" || view == "keyframes" {
			allowed = append(allowed, "offset")
		}
		if view == "properties" || view == "keyframes" {
			allowed = append(allowed, "layer_id", "property_path")
			if _, ok := body["layer_id"]; !ok {
				return input("layer_id is required.")
			}
		}
		if view == "properties" {
			allowed = append(allowed, "depth", "sample_time")
		}
	}
	for key := range body {
		if !contains(allowed, key) {
			return input("Unexpected field: " + key)
		}
	}
	for key, bounds := range map[string][2]int64{"comp_id": {1, 2147483647}, "layer_id": {1, 2147483647}, "limit": {1, 1000}, "offset": {0, 2147483647}, "depth": {1, 8}, "timeout_ms": {100, 10000}} {
		if v, ok := body[key]; ok {
			n, e := jsonNumber(v)
			if e != nil || n < bounds[0] || n > bounds[1] {
				return input("Invalid " + key)
			}
		}
	}
	if v, ok := body["diagnose"]; ok {
		if _, valid := v.(bool); !valid {
			return input("diagnose must be boolean.")
		}
	}
	for _, key := range []string{"time", "sample_time"} {
		if v, exists := body[key]; exists {
			n, ok := v.(json.Number)
			if !ok {
				return input("Invalid " + key)
			}
			f, e := n.Float64()
			if e != nil || f < 0 {
				return input("Invalid " + key)
			}
		}
	}
	if v, exists := body["property_path"]; exists {
		p, ok := v.([]any)
		if !ok || len(p) > 32 {
			return input("Invalid property_path.")
		}
		for _, v := range p {
			s, ok := v.(map[string]any)
			if !ok || len(s) != 2 {
				return input("Invalid property path segment.")
			}
			i, e := jsonNumber(s["index"])
			_, str := s["match_name"].(string)
			if e != nil || i < 0 || i >= 2147483647 || !str {
				return input("Invalid property path segment.")
			}
		}
		if view == "keyframes" && len(p) == 0 {
			return input("keyframes requires property_path.")
		}
	} else if view == "keyframes" {
		return input("keyframes requires property_path.")
	}
	return nil
}
func jsonNumber(value any) (int64, error) {
	switch n := value.(type) {
	case json.Number:
		return n.Int64()
	case int:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("not integer")
	}
}
