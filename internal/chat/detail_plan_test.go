package chat

import (
	"context"
	"encoding/json"
	"github.com/tian1363/scriptagent/internal/detailpage"
	"github.com/tian1363/scriptagent/internal/model"
	"github.com/tian1363/scriptagent/internal/userctx"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetailPlanParsingKeepsAuthoritativeInput(t *testing.T) {
	in := detailpage.Input{Facts: detailpage.Facts{Title: "真实产品"}, Assets: []detailpage.Asset{{ID: "owned"}}, PlatformIDs: []string{"taobao"}}
	p := detailpage.Plan{Input: detailpage.Input{Facts: detailpage.Facts{Title: "模型伪造资料"}}, Driver: "visual", StyleLock: "暖白柔光"}
	for _, id := range []string{"hero", "benefits", "details", "specs", "usage"} {
		p.Pages = append(p.Pages, detailpage.Page{ID: id, SceneID: "hero", Purpose: "核对商品", SourceAssetID: "owned", Headline: "真实产品", Visual: "完整商品"})
	}
	raw, _ := json.Marshal(p)
	got, err := parseDetailPlan("```json\n"+string(raw)+"\n```", in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Input.Facts.Title != "真实产品" {
		t.Fatal("model overwrote facts")
	}
	if got.Pages[0].Missing == nil || got.Pages[0].Evidence == nil {
		t.Fatal("frontend arrays not normalized")
	}
	if _, err = parseDetailPlan(strings.Replace(string(raw), "owned", "foreign", 1), in); err == nil {
		t.Fatal("unowned source accepted")
	}
	if _, err = parseDetailPlan("{bad json}", in); err == nil {
		t.Fatal("bad response accepted")
	}
}

func TestDetailPlanUsesTextProviderAndGroundedBrief(t *testing.T) {
	in := detailpage.Input{Facts: detailpage.Facts{Title: "已确认棉衬衫", Specs: "S / M / L"}, Assets: []detailpage.Asset{{ID: "front", Role: "商品正面"}}, PlatformIDs: []string{"taobao", "amazon"}}
	expected := detailpage.Plan{Driver: "lifestyle", StyleLock: "暖白背景，柔光，中性文字"}
	for _, id := range []string{"hero", "benefits", "details", "specs", "usage"} {
		expected.Pages = append(expected.Pages, detailpage.Page{ID: id, SceneID: "editorial", Purpose: "核对商品", SourceAssetID: "front", Headline: "棉衬衫", Visual: "完整商品与短标题", Missing: []string{"核对素材可见细节"}})
	}
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		prompt = string(body)
		raw, _ := json.Marshal(expected)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"output": map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": []any{map[string]string{"text": string(raw)}}}}}}})
	}))
	defer server.Close()
	provider := &toolboxConfig{endpoint: server.URL}
	service := NewService(nil, model.NewDashScopeClient(model.DashScopeConfig{Provider: provider}))
	plan, err := service.GenerateDetailPlan(userctx.WithUser(context.Background(), userctx.User{ID: "detail-user"}), in)
	if err != nil {
		t.Fatal(err)
	}
	if provider.user != "detail-user" || provider.capability != "text" {
		t.Fatalf("wrong user/model scope: %+v", provider)
	}
	for _, required := range []string{"已确认棉衬衫", "商品正面", "没有看过图片", "style_lock", "不能随机轮换素材"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing planning input/constraint: %s", required)
		}
	}
	if len(plan.Pages) != 5 || len(plan.Pages[0].Missing) != 1 || len(plan.Input.PlatformIDs) != 2 {
		t.Fatalf("lost plan information: %+v", plan)
	}
}
