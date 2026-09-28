package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
)

func TestInspectionRawRequests(t *testing.T) {
	for _, body := range []string{
		`{"cmd":"status","diagnose":true,"timeout_ms":2000}`,
		`{"cmd":"project","target":"comp","comp_id":3}`,
		`{"cmd":"project","target":"layers","comp_id":3,"offset":2,"limit":1}`,
		`{"cmd":"project","target":"properties","comp_id":3,"layer_id":4,"sample_time":1.5,"depth":2}`,
		`{"cmd":"project","target":"keyframes","layer_id":4,"property_path":[{"index":0,"match_name":"ADBE Position"}]}`,
		`{"cmd":"preview_frame","comp_id":3,"time":1.5,"path":"E:/output/test.png"}`,
	} {
		r, code := invoke(t, body, "request", "ae", "--body-file", "-", "--dry-run", "--json")
		if code != 0 || r["ok"] != true {
			t.Fatalf("%s: %v", body, r)
		}
	}
	for _, body := range []string{
		`{"cmd":"status","diagnose":"true"}`,
		`{"cmd":"status","timeout_ms":-1}`,
		`{"cmd":"project","target":"invalid"}`,
		`{"cmd":"project","target":"properties","layer_id":3,"limit":1001}`,
		`{"cmd":"project","target":"keyframes","layer_id":3,"property_path":[]}`,
		`{"cmd":"preview_frame","time":-1,"path":"E:/output/test.png"}`,
	} {
		_, code := invoke(t, body, "request", "ae", "--body-file", "-", "--dry-run", "--json")
		if code != 2 {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestInspectionWireRequests(t *testing.T) {
	cases := []struct{ body, reply string }{
		{`{"cmd":"status","diagnose":true,"timeout_ms":2000}`, `{"ok":true,"data":{"online":true,"ae_version":null,"diagnostic":{"main_thread_responsive":false,"stage":"probe","error":"timeout"}}}`},
		{`{"cmd":"project","target":"comp","comp_id":3}`, `{"ok":true,"data":{"id":3,"name":"Comp","width":1920,"height":1080}}`},
		{`{"cmd":"project","target":"layers","comp_id":3,"offset":2,"limit":1}`, `{"ok":true,"data":{"layers":[{"id":4,"index":2}],"total":4,"offset":2,"truncated":true}}`},
		{`{"cmd":"project","target":"properties","comp_id":3,"layer_id":4,"depth":1,"limit":200,"sample_time":1.5}`, `{"ok":true,"data":{"properties":[{"property_path":[{"index":0,"match_name":"group"}],"children_omitted":true},{"sample":{"supported":false,"reason":"unsupported"}}],"truncated":false}}`},
		{`{"cmd":"project","target":"keyframes","layer_id":4,"property_path":[{"index":0,"match_name":"ADBE Position"}],"offset":0,"limit":1}`, `{"ok":true,"data":{"keyframes":[{"index":0,"time":1.5,"value":[1,2,3]}],"total":2,"offset":0,"truncated":true}}`},
		{`{"cmd":"preview_frame","comp_id":3,"time":1.5,"path":"E:/output/new.png"}`, `{"ok":true,"data":{"path":"E:/output/new.png","comp_id":3,"time":1.5,"width":960,"height":540,"comp_width":1920,"comp_height":1080,"resolution_factor":[2,2]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			count := atomic.Int32{}
			port, stop := mock(t, func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				content, _ := io.ReadAll(r.Body)
				got, err := decode(content)
				want, _ := decode([]byte(tc.body))
				if err != nil || !reflect.DeepEqual(got, want) || r.Method != "POST" || r.URL.Path != "/command" || r.ContentLength != int64(len(content)) {
					t.Errorf("incorrect request: %s", content)
				}
				_, _ = io.WriteString(w, tc.reply)
			})
			defer stop()
			res, code := invoke(t, tc.body, "request", "ae", "--body-file", "-", "--ae-port", port, "--json")
			want, _ := decode([]byte(tc.reply))
			if code != 0 || !reflect.DeepEqual(res, want) || count.Load() != 1 {
				t.Fatalf("code=%d response=%v requests=%d", code, res, count.Load())
			}
		})
	}
}

func TestInspectionValidationBeforeConnection(t *testing.T) {
	count := atomic.Int32{}
	port, stop := mock(t, func(w http.ResponseWriter, r *http.Request) { count.Add(1) })
	defer stop()
	for _, body := range []string{
		`{"cmd":"project","target":"comp","comp_id":0}`,
		`{"cmd":"project","target":"layers","offset":-1}`,
		`{"cmd":"project","target":"properties","layer_id":4,"depth":9}`,
		`{"cmd":"project","target":"properties","layer_id":4,"sample_time":1e999}`,
		`{"cmd":"project","target":"keyframes","layer_id":4}`,
		`{"cmd":"project","target":"keyframes","layer_id":4,"property_path":[{"index":-1,"match_name":"x"}]}`,
		`{"cmd":"project","target":"keyframes","layer_id":4,"property_path":[{"index":0}]}`,
		`{"cmd":"preview_frame","path":"E:/out.png\u0000"}`,
		`{"cmd":"preview_frame","path":"E:/out.jpg"}`,
		`{"cmd":"status","timeout_ms":10001}`,
	} {
		_, code := invoke(t, body, "request", "ae", "--body-file", "-", "--ae-port", port)
		if code != 2 {
			t.Fatalf("accepted: %s", body)
		}
	}
	if count.Load() != 0 {
		t.Fatal("invalid request reached server")
	}
}

func TestPreviewNamedPathPreflight(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.png")
	_ = os.WriteFile(existing, []byte("preserve"), 0600)
	count := atomic.Int32{}
	port, stop := mock(t, func(w http.ResponseWriter, r *http.Request) { count.Add(1) })
	defer stop()
	for _, path := range []string{existing, filepath.Join(dir, "missing", "frame.png"), filepath.Join(dir, "frame.jpg"), ""} {
		_, code := invoke(t, "", "ae", "preview-frame", "--path", path, "--ae-port", port)
		if code != 2 {
			t.Fatalf("accepted path: %s", path)
		}
	}
	// Use a new local output path; dry-run must not create it or contact the service.
	output := filepath.Join(dir, "new.PNG")
	res, code := invoke(t, "", "ae", "preview-frame", "--path", output, "--ae-port", port, "--dry-run")
	if code != 0 || data(res)["mutating"] != true || data(res)["body"].(map[string]any)["path"] != output {
		t.Fatal(res)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("dry-run created output")
	}
	original, _ := os.ReadFile(existing)
	if string(original) != "preserve" || count.Load() != 0 {
		t.Fatal("preflight changed state")
	}
}

func TestNewHostCommandErrorsRemainErrors(t *testing.T) {
	for _, body := range []string{`{"cmd":"preview_frame","path":"E:/output/new.png"}`, `{"cmd":"project","target":"comp","comp_id":3}`, `{"cmd":"status","diagnose":true}`} {
		count := atomic.Int32{}
		port, stop := mock(t, func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			jsonReply(w, object{"ok": false, "error": "Unknown command or unavailable capability."})
		})
		_, code := invoke(t, body, "request", "ae", "--body-file", "-", "--ae-port", port)
		stop()
		if code != 4 || count.Load() != 1 {
			t.Fatalf("code=%d requests=%d", code, count.Load())
		}
	}
}

func TestAPIStatusSnapshots(t *testing.T) {
	for _, snapshot := range []object{
		{"configured": false, "verified": false, "verifying": false},
		{"configured": true, "verified": false, "verifying": false},
		{"configured": true, "verified": false, "verifying": true},
		{"configured": true, "verified": true, "verifying": false},
		{"configured": true, "verified": true, "verifying": true},
	} {
		count := atomic.Int32{}
		port, stop := mock(t, func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			b, _ := io.ReadAll(r.Body)
			if string(b) != `{"cmd":"get_api_status"}` {
				t.Errorf("unexpected body: %s", b)
			}
			jsonReply(w, success(snapshot))
		})
		for _, args := range [][]string{{"start", "get-api-status"}, {"start", "get_api_status"}, {"request", "start", "--body", `{"cmd":"get_api_status"}`}} {
			res, code := invoke(t, "", append(args, "--start-port", port, "--json")...)
			if code != 0 || !reflect.DeepEqual(data(res), snapshot) {
				t.Fatalf("%d %v", code, res)
			}
		}
		if count.Load() != 3 {
			t.Fatal(count.Load())
		}
		stop()
	}
}

func TestAPIStatusDryRunAndOldHost(t *testing.T) {
	count := atomic.Int32{}
	port, stop := mock(t, func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		jsonReply(w, object{"ok": false, "error": "Unknown command."})
	})
	defer stop()
	res, code := invoke(t, "", "start", "get-api-status", "--dry-run", "--start-port", port)
	if code != 0 || data(res)["mutating"] != false || count.Load() != 0 {
		t.Fatal(res)
	}
	_, code = invoke(t, `{"cmd":"get_api_status","verify":true}`, "request", "start", "--body-file", "-", "--start-port", port)
	if code != 2 || count.Load() != 0 {
		t.Fatal("unexpected fields reached host")
	}
	_, code = invoke(t, "", "start", "get-api-status", "--start-port", port)
	if code != 4 || count.Load() != 1 {
		t.Fatal("old host response was hidden or retried")
	}
}
