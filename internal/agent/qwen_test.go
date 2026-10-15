package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/model"
)

func TestScriptWorkflowResumesAfterFissionFailure(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "reference.mp4")
	if err := os.WriteFile(videoPath, []byte("sample-video"), 0600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := jobs.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	job, err := store.CreateJob(jobs.CreateJobInput{Title: "Test", VideoPath: videoPath, Industry: "beauty", FissionCount: 1, FissionDirections: "视听层-换BGM", ContextSnapshot: "product"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 3 {
			http.Error(w, "temporary error", http.StatusServiceUnavailable)
			return
		}
		value := "analysis"
		if calls == 2 {
			value = `{"storyboards":[{"scene_index":1,"visual":"product"}]}`
		}
		if calls == 4 {
			value = `[{"storyboards":[{"scene_index":1,"visual":"product"}],"metadata":{"fission_dimension":"视听层-换BGM"}}]`
		}
		fmt.Fprintf(w, `{"output":{"choices":[{"message":{"content":[{"text":%q}]}}]}}`, value)
	}))
	defer server.Close()
	agent := NewQwenScriptAgent(QwenConfig{Client: model.NewDashScopeClient(model.DashScopeConfig{APIKey: "test", Endpoint: server.URL}), Checkpoints: store})
	if _, err := agent.Run(context.Background(), jobs.RunContext{}, *job, func(string, string) {}); err == nil {
		t.Fatal("expected fission call to fail")
	}
	analysis, replica, _, fingerprint, err := store.ScriptCheckpoint(job.ID)
	if err != nil || analysis == "" || replica == "" || fingerprint == "" {
		t.Fatalf("checkpoint missing: %q %q %q %v", analysis, replica, fingerprint, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = jobs.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	agent = NewQwenScriptAgent(QwenConfig{Client: model.NewDashScopeClient(model.DashScopeConfig{APIKey: "test", Endpoint: server.URL}), Checkpoints: store})
	result, err := agent.Run(context.Background(), jobs.RunContext{}, *job, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || result.AnalysisMarkdown != "analysis" || result.ReplicaScriptJSON == "" || result.FissionScriptsJSON == "" {
		t.Fatalf("resume repeated calls or lost result: calls=%d result=%+v", calls, result)
	}
	if _, err := agent.Run(context.Background(), jobs.RunContext{}, *job, func(string, string) {}); err != nil || calls != 4 {
		t.Fatalf("completed stages should resume without another model call: calls=%d err=%v", calls, err)
	}
}

func TestFissionPromptOmitsUpstreamBulkContextAndUnusedRules(t *testing.T) {
	job := jobs.Job{
		FissionCount:      1,
		FissionDirections: "结构层-换CTA",
		Requirement:       "保留品牌语气",
	}
	prompt := fissionScriptPrompt(job, `{"replica_script":{"title":"base"}}`)
	if strings.Contains(prompt, "产品 Markdown") || strings.Contains(prompt, "视频理解结果") {
		t.Fatal("fission prompt should not repeat full product or analysis context")
	}
	if !strings.Contains(prompt, "结构层-换CTA") || !strings.Contains(prompt, "重点改写最后 1-2 个分镜") {
		t.Fatal("expected selected dimension and its rule")
	}
	if strings.Contains(prompt, "视听层-换BGM：") {
		t.Fatal("unselected dimension rules should not be injected")
	}
}

func TestValidateFissionDimensionsAllowsSingleSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM"); err != nil {
		t.Fatalf("expected selected single element to pass: %v", err)
	}
}

func TestValidateFissionDimensionsRejectsMixedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM+结构层-换CTA"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n结构层-换CTA"); err == nil {
		t.Fatal("expected mixed fission element to fail")
	}
}

func TestValidateFissionDimensionsRejectsOutOfOrderSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "结构层-换CTA"}},
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n结构层-换CTA"); err == nil {
		t.Fatal("expected out-of-order fission element to fail")
	}
}

func TestValidateFissionDimensionsAllowsRepeatedSelectedElement(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
		{Metadata: metadata{FissionDimension: "视听层-换BGM"}},
	}
	if err := validateFissionDimensions(scripts, "视听层-换BGM\n视听层-换BGM"); err != nil {
		t.Fatalf("expected repeated selected element to pass: %v", err)
	}
}

func TestValidateFissionDimensionsUsesAllDirectionsWhenUnselected(t *testing.T) {
	scripts := []scriptPayload{
		{Metadata: metadata{FissionDimension: "元素层-字幕语言本地化"}},
	}
	if err := validateFissionDimensions(scripts, ""); err != nil {
		t.Fatalf("expected default all directions to pass: %v", err)
	}
}
