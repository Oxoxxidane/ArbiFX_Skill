package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func invoke(t *testing.T, stdin string, args ...string) (object, int) {
	t.Helper()
	var output bytes.Buffer
	var code int
	if binary := os.Getenv("ARBIFX_TEST_EXE"); binary != "" {
		cmd := exec.Command(binary, args...)
		cmd.Stdin = strings.NewReader(stdin)
		cmd.Stdout = &output
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				code = exit.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		if stderr.Len() != 0 {
			t.Fatalf("unexpected stderr: %s", stderr.String())
		}
	} else {
		code = execute(args, strings.NewReader(stdin), &output)
	}
	result, err := decode(output.Bytes())
	if err != nil {
		t.Fatalf("not JSON: %s", output.String())
	}
	return result, code
}
func mock(t *testing.T, handler http.HandlerFunc) (string, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	u, _ := url.Parse(server.URL)
	return u.Port(), server.Close
}
func jsonReply(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
func data(result object) object { return result["data"].(map[string]any) }

func TestEveryProtocolCommandOverHTTP(t *testing.T) {
	// Independent fixtures assert method/path/body/Content-Length; all writes target random-port mocks.
	dir := t.TempDir()
	asset := filepath.Join(dir, "中文 model.afx")
	if err := os.WriteFile(asset, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		target, cmd string
		args        []string
		want        object
	}{
		{"ae", "status", nil, object{}}, {"ae", "instances", nil, object{}}, {"ae", "project", nil, object{}},
		{"ae", "open_start", []string{"--id", "opaque"}, object{"id": "opaque"}},
		{"ae", "script", []string{"--code", "emit('你好'); return 42;"}, object{"code": "emit('你好'); return 42;"}},
		{"start", "set_reference", []string{"--path", asset}, object{"path": asset}}, {"start", "get_reference", nil, object{}},
		{"start", "set_obj", []string{"--slot", "7", "--type", "texture3", "--path", asset}, object{"slot": json.Number("7"), "type": "texture3", "path": asset}},
		{"start", "get_obj", nil, object{}},
		{"start", "set_svg", []string{"--slot", "0", "--path", asset}, object{"slot": json.Number("0"), "path": asset}},
		{"start", "get_svg", nil, object{}},
		{"start", "set_text", []string{"--text", "中文\n第二行"}, object{"text": "中文\n第二行"}}, {"start", "get_text", nil, object{}},
		{"start", "set_font", []string{"--font", "default"}, object{"font": "default"}}, {"start", "get_font", nil, object{}},
		{"start", "set_tag", []string{"--tag", "is_alpha", "--value", "false"}, object{"tag": "is_alpha", "value": false}}, {"start", "get_tags", nil, object{}},
		{"start", "set_prompt", []string{"--prompt", "提示", "--parameters", ""}, object{"prompt": "提示", "parameters": ""}}, {"start", "get_prompt", nil, object{}},
		{"start", "send", nil, object{}}, {"start", "save_afx", []string{"--path", asset}, object{"path": asset}},
		{"start", "load_afx", []string{"--path", asset}, object{"path": asset}}, {"start", "close", nil, object{}},
	}
	for _, tc := range cases {
		t.Run(tc.target+"/"+tc.cmd, func(t *testing.T) {
			count := atomic.Int32{}
			port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				b, _ := io.ReadAll(r.Body)
				got, err := decode(b)
				want := object{"cmd": tc.cmd}
				for k, v := range tc.want {
					want[k] = v
				}
				if err != nil || !reflect.DeepEqual(got, want) || r.Method != "POST" || r.URL.Path != "/command" || r.ContentLength != int64(len(b)) {
					t.Errorf("invalid wire request: %s %s %s", r.Method, r.URL.Path, b)
				}
				jsonReply(w, success(object{"echo": got}))
			})
			defer closeServer()
			args := append([]string{"--json", tc.target, strings.ReplaceAll(tc.cmd, "_", "-"), "--" + tc.target + "-port", port}, tc.args...)
			got, code := invoke(t, "", args...)
			if code != 0 || got["ok"] != true || count.Load() != 1 {
				t.Fatalf("%d %v count=%d", code, got, count.Load())
			}
		})
	}
}
func TestDryRunNeverConnectsAndPredictsClear(t *testing.T) {
	count := atomic.Int32{}
	port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) { count.Add(1) })
	defer closeServer()
	for _, args := range [][]string{{"--clear"}, {"--path", filepath.Join(t.TempDir(), "missing.svg")}} {
		base := []string{"start", "set-svg", "--slot", "0", "--start-port", port, "--dry-run", "--json"}
		res, code := invoke(t, "", append(base, args...)...)
		if code != 0 || data(res)["effects"].([]any)[0] != "clear" {
			t.Fatalf("%d %v", code, res)
		}
	}
	if count.Load() != 0 {
		t.Fatal("dry-run connected")
	}
}
func TestBadInputsFailBeforeHTTP(t *testing.T) {
	for _, args := range [][]string{
		{"start", "set-obj", "--slot", "8", "--type", "obj", "--clear"},
		{"start", "set-tag", "--tag", "is_alpha", "--value", "1"},
		{"start", "set-prompt", "--prompt", "missing parameters"},
		{"start", "set-text", "--text", "a", "--text-file", "-"},
		{"start", "set-reference", "--path", "a", "--clear"},
		{"ae", "status", "--path", "unexpected"},
		{"request", "ae", "--body", `{"cmd":"status","typo":1}`},
		{"request", "start", "--body", `{"cmd":"set_obj","slot":true,"type":"obj","path":""}`},
		{"request", "ae", "--body", `{"cmd":"status"} {}`},
		{"doctor", "--offline", "--ae-port", "65536"},
		{"doctor", "--offline", "--timeout", "NaN"},
		{"start", "set-prompt", "--prompt-file", "-", "--parameters-file", "-"},
		{"ae", "resolve"},
	} {
		res, code := invoke(t, "", append(args, "--dry-run", "--json")...)
		if code != 2 || res["ok"] != false {
			t.Fatalf("args=%v code=%d %v", args, code, res)
		}
	}
}
func TestUTF8FileAndStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "提示词.txt")
	_ = os.WriteFile(path, []byte("\ufeff你好\n第二行"), 0600)
	res, code := invoke(t, "param\nvalue", "start", "set-prompt", "--prompt-file", path, "--parameters-file", "-", "--dry-run", "--json")
	body := data(res)["body"].(map[string]any)
	if code != 0 || body["prompt"] != "你好\n第二行" || body["parameters"] != "param\nvalue" {
		t.Fatal(res)
	}
	res, code = invoke(t, `{"cmd":"status"}`, "request", "ae", "--body-file", "-", "--dry-run")
	if code != 0 || data(res)["body"].(map[string]any)["cmd"] != "status" {
		t.Fatal(res)
	}
}
func TestRawUnknownAndSize(t *testing.T) {
	res, code := invoke(t, "", "request", "ae", "--body", `{"cmd":"future","x":1}`, "--allow-unknown", "--dry-run")
	if code != 0 || data(res)["mutating"] != true {
		t.Fatal(res)
	}
	path := filepath.Join(t.TempDir(), "huge.txt")
	_ = os.WriteFile(path, bytes.Repeat([]byte("x"), maxBody+1), 0600)
	res, code = invoke(t, "", "ae", "script", "--code-file", path, "--dry-run")
	if code != 2 {
		t.Fatal(res)
	}
}
func TestErrorsAndNoRedirectOrRetry(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reply  string
		code   int
	}{
		{"api", 200, `{"ok":false,"error":"Start is busy."}`, 4},
		{"script", 200, `{"ok":true,"data":{"error":"syntax error"}}`, 4},
		{"malformed", 200, `not JSON`, 3}, {"badok", 200, `{"ok":"true","data":{}}`, 3},
		{"missingdata", 200, `{"ok":true}`, 3}, {"redirect", 302, `{}`, 3}, {"server", 500, `{}`, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := atomic.Int32{}
			port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				w.Header().Set("Location", "/other")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.reply)
			})
			defer closeServer()
			res, code := invoke(t, "", "ae", "script", "--code", "return 1;", "--ae-port", port, "--json")
			if code != tc.code || count.Load() != 1 || res["ok"] != false {
				t.Fatalf("%d %v count %d", code, res, count.Load())
			}
		})
	}
}
func TestTimeoutNoReplay(t *testing.T) {
	count := atomic.Int32{}
	port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		time.Sleep(100 * time.Millisecond)
		jsonReply(w, success(object{}))
	})
	defer closeServer()
	res, code := invoke(t, "", "start", "send", "--start-port", port, "--timeout", "0.02", "--json")
	if code != 3 || res["error"].(map[string]any)["kind"] != "timeout" || count.Load() != 1 {
		t.Fatal(res, count.Load())
	}
}
func TestResolveCurrentAndDocumentedShapes(t *testing.T) {
	for _, rows := range []string{
		`[{"id":"27","comp":{"id":3,"name":"合成"},"layer":{"id":4,"name":"图层","index":0},"effect_index":0}]`,
		`[{"id":"27","comp":"合成","layer":"图层","layer_index":0,"effect_index":0}]`,
	} {
		port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, `{"ok":true,"data":%s}`, rows) })
		defer closeServer()
		res, code := invoke(t, "", "ae", "resolve", "--comp", "合成", "--layer", "图层", "--effect-index", "0", "--ae-port", port)
		if code != 0 || data(res)["id"] != "27" {
			t.Fatal(res)
		}
		res, code = invoke(t, "", "ae", "resolve", "--comp", "absent", "--ae-port", port)
		if code != 4 {
			t.Fatal(res)
		}
	}
	port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true,"data":[{"id":"1","comp":"same"},{"id":"2","comp":"same"}]}`)
	})
	defer closeServer()
	_, code := invoke(t, "", "ae", "resolve", "--comp", "same", "--ae-port", port)
	if code != 4 {
		t.Fatal(code)
	}
}
func TestConfigPrecedenceAndNoSecret(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(file, []byte("[network]\napi_key = \"SECRET_NEVER_PRINT\"\n[local_http]\nae_global_port = 30001 # comment\nstart_port = 30002\n"), 0600)
	t.Setenv("ARBIFX_AE_PORT", "30003")
	res, code := invoke(t, "", "doctor", "--offline", "--config", file, "--ae-port", "30004", "--json")
	if code != 0 {
		t.Fatal(res)
	}
	b, _ := json.Marshal(res)
	if bytes.Contains(b, []byte("SECRET")) {
		t.Fatal("secret leaked")
	}
	ports := data(res)["config"].(map[string]any)["ports"].(map[string]any)
	if ports["ae"] != json.Number("30004") || ports["start"] != json.Number("30002") {
		t.Fatal(res)
	}
	res, code = invoke(t, "", "doctor", "--offline", "--config", file)
	if code != 0 || data(res)["config"].(map[string]any)["ports"].(map[string]any)["ae"] != json.Number("30003") {
		t.Fatal(res)
	}
}
func TestDoctorBothAndProxyBypass(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	ae, closeAe := mock(t, func(w http.ResponseWriter, r *http.Request) { jsonReply(w, success(object{"online": true, "port": 1})) })
	defer closeAe()
	start, closeStart := mock(t, func(w http.ResponseWriter, r *http.Request) {
		states := object{}
		for _, tag := range tags {
			states[tag] = false
		}
		jsonReply(w, success(states))
	})
	defer closeStart()
	res, code := invoke(t, "", "doctor", "--ae-port", ae, "--start-port", start, "--json")
	if code != 0 || res["ok"] != true {
		t.Fatal(res)
	}
}
func TestConnectionFailure(t *testing.T) {
	port, closeServer := mock(t, func(w http.ResponseWriter, r *http.Request) {})
	closeServer()
	_, code := invoke(t, "", "ae", "status", "--ae-port", port, "--timeout", "0.5")
	if code != 3 {
		t.Fatal(code)
	}
}
func TestCatalogAndHelp(t *testing.T) {
	res, code := invoke(t, "", "commands", "--json")
	if code != 0 || len(res["data"].([]any)) != 23 {
		t.Fatal(res)
	}
	if !strings.Contains(help(nil), "set-prompt") || !strings.Contains(help([]string{"start", "set-prompt"}), "--parameters") {
		t.Fatal("help missing command fields")
	}
}
func TestStandaloneNoRuntimeOnPath(t *testing.T) {
	binary := os.Getenv("ARBIFX_TEST_EXE")
	if binary == "" {
		t.Skip("set ARBIFX_TEST_EXE to verify compiled distribution")
	}
	cmd := exec.Command(binary, "doctor", "--offline", "--json")
	cmd.Dir = t.TempDir()
	env := []string{}
	for _, value := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(value, "=", 2)[0])
		if key != "PATH" && key != "PYTHONHOME" && key != "PYTHONPATH" && key != "GOROOT" && key != "GOPATH" {
			env = append(env, value)
		}
	}
	cmd.Env = append(env, "PATH="+cmd.Dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	result, err := decode(out)
	if err != nil || result["ok"] != true {
		t.Fatal(string(out), err)
	}
}
func TestInvalidConfiguredPortFallback(t *testing.T) {
	f := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(f, []byte("[local_http]\nae_global_port = -1\nstart_port = 999999\n"), 0600)
	cfg, e := loadConfig(map[string]string{"config": f})
	if e != nil || cfg.Ports["ae"] != 28154 || cfg.Ports["start"] != 28153 || len(cfg.Warnings) != 2 {
		t.Fatal(cfg, e)
	}
}
func TestRequestIntegerPrecision(t *testing.T) {
	b, e := decode([]byte(`{"cmd":"future","id":9007199254740993}`))
	if e != nil {
		t.Fatal(e)
	}
	out, err := encode(b)
	if err != nil || !bytes.Contains(out, []byte(strconv.FormatInt(9007199254740993, 10))) {
		t.Fatal(string(out), err)
	}
}
