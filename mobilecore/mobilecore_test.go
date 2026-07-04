package mobilecore

import (
	"encoding/json"
	"os"
	"testing"
)

func TestResponseEnvelopeSuccess(t *testing.T) {
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(ok(map[string]string{"hello": "world"})), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != true {
		t.Fatalf("ok = %v, want true", got["ok"])
	}
	if got["error"] != "" {
		t.Fatalf("error = %q, want empty", got["error"])
	}
	if got["code"] != "" {
		t.Fatalf("code = %q, want empty", got["code"])
	}
}

func TestResponseEnvelopeFailure(t *testing.T) {
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(fail(CodeUnsupported, "not ready")), &got); err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %v, want false", got["ok"])
	}
	if got["data"] != nil {
		t.Fatalf("data = %v, want nil", got["data"])
	}
	if got["error"] != "not ready" {
		t.Fatalf("error = %q", got["error"])
	}
	if got["code"] != CodeUnsupported {
		t.Fatalf("code = %q", got["code"])
	}
}

func TestInitAndConfigRoundTrip(t *testing.T) {
	dir, err := os.MkdirTemp("", "yatori-mobilecore-*")
	if err != nil {
		t.Fatal(err)
	}
	if !mustOK(Init(dir)) {
		t.Fatal("Init failed")
	}
	if !mustOK(SaveConfigJSON(`{"setting":{"maxWorkers":3}}`)) {
		t.Fatal("SaveConfigJSON failed")
	}
	var resp struct {
		OK   bool                   `json:"ok"`
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal([]byte(GetConfigJSON()), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatal("GetConfigJSON returned failure")
	}
	if _, ok := resp.Data["setting"]; !ok {
		t.Fatalf("setting missing: %#v", resp.Data)
	}
}

func mustOK(raw string) bool {
	var resp struct {
		OK bool `json:"ok"`
	}
	_ = json.Unmarshal([]byte(raw), &resp)
	return resp.OK
}
