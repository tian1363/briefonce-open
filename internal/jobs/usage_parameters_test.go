package jobs

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestUsageParametersPreservesControlsAndOmitsPrivatePayload(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "parameters.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.ConfigureSecretEncryption(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.encryptSecret(`{"model":"gpt-image-2-4K","size":"4K","aspect_ratio":"3:4","seed":0,"prompt":"商品图片","images":["data:image/png;base64,private-image"],"api_key":"private-key","unknown":"private-field"}`)
	if err != nil {
		t.Fatal(err)
	}
	p := s.usageParameters(snapshot)
	if p["size"] != "4K" || p["aspect_ratio"] != "3:4" || p["model"] != "gpt-image-2-4K" || p["seed"] != float64(0) || p["reference_count"] != 1 {
		t.Fatalf("missing submitted controls: %+v", p)
	}
	raw, _ := json.Marshal(p)
	if strings.Contains(string(raw), "private-") || strings.Contains(string(raw), "base64") {
		t.Fatalf("private payload leaked: %s", raw)
	}
	if s.usageParameters("invalid encrypted snapshot") != nil {
		t.Fatal("invalid snapshot should not be exposed")
	}
}
