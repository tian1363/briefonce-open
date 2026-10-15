package detailpage

import (
	"os"
	"strings"
	"testing"
)

func validPlan() Plan {
	p := Plan{Input: Input{Facts: Facts{Title: "棉衬衫"}, Assets: []Asset{{ID: "front", Role: "正面"}, {ID: "detail", Role: "领口"}}, PlatformIDs: []string{"taobao", "jd", "douyin", "miniprogram", "tiktok", "ozon", "amazon", "pdd"}, ReferenceID: "style"}, Driver: "visual", StyleLock: "暖白背景，中性光线，统一黑色无衬线字体"}
	for _, id := range []string{"hero", "benefits", "details", "specs", "usage"} {
		source := "front"
		if id == "details" {
			source = "detail"
		}
		p.Pages = append(p.Pages, Page{ID: id, SceneID: "hero", Purpose: "回答商品问题", SourceAssetID: source, Headline: "棉衬衫", Visual: "完整商品，左图右文", Evidence: []string{"title"}})
	}
	return p
}
func TestCompilePreservesPlatformsAndSharedStyle(t *testing.T) {
	p := validPlan()
	tasks, err := Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 38 {
		t.Fatalf("lost original modules: %d", len(tasks))
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		seen[task.PlatformID] = true
		if !strings.Contains(task.Prompt, p.StyleLock) || !strings.Contains(task.Prompt, "图片2仅为视觉风格参考") || task.StyleVersion != tasks[0].StyleVersion {
			t.Fatal("style/reference contract changed")
		}
		if task.PageID == "details" && task.SourceAssetID != "detail" {
			t.Fatal("incorrect source mapping")
		}
	}
	if len(seen) != 9 {
		t.Fatal(seen)
	}
	p.StyleLock += "柔和光"
	updated, _ := Compile(p)
	if updated[0].StyleVersion == tasks[0].StyleVersion {
		t.Fatal("style revision missing")
	}
}
func TestValidationRejectsForeignSourceAndDuplicateModules(t *testing.T) {
	p := validPlan()
	p.Pages[0].SourceAssetID = "foreign"
	if p.Validate() == nil {
		t.Fatal("foreign source accepted")
	}
	p = validPlan()
	p.Pages[0].ID = "details"
	if p.Validate() == nil {
		t.Fatal("duplicate page accepted")
	}
}

// Keep backend dimensions/sections synchronized with the pre-existing UI presets.
func TestOriginalPlatformPresets(t *testing.T) {
	data, e := os.ReadFile("../../web/app/src/detailPageBuilder.js")
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range Platforms {
		if strings.HasPrefix(p.ID, "ozon-") {
			continue
		}
		needle := "id: \"" + p.ID + "\""
		start := strings.Index(string(data), needle)
		if start < 0 {
			t.Fatal(p.ID)
		}
		line := strings.SplitN(string(data)[start:], "\n", 2)[0]
		for _, id := range p.Sections {
			if !strings.Contains(line, "\""+id+"\"") {
				t.Fatalf("%s missing %s", p.ID, id)
			}
		}
	}
}

func TestCompileDoesNotPrintPlanningFacts(t *testing.T) {
	p := validPlan()
	p.Input.Facts.Specs = "内部资料：尺码未知，不补造数值"
	p.Input.Facts.Usage = "洗涤方式未知，改为搭配示意"
	tasks, err := Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if strings.Contains(task.Prompt, p.Input.Facts.Specs) || strings.Contains(task.Prompt, p.Input.Facts.Usage) || strings.Contains(task.Prompt, "模块：hero") {
			t.Fatal("raw planning facts leaked into render prompt")
		}
		if !strings.Contains(task.Prompt, "不绘制引号或其他文字") {
			t.Fatal("missing printable-copy boundary")
		}
	}
}

func TestCompileExcludesPlanningMetadata(t *testing.T) {
	p := validPlan()
	p.Rationale = "内部策划原因"
	p.Input.Audience = "内部人群资料"
	p.Input.Goal = "内部推广目标"
	for i := range p.Pages {
		p.Pages[i].Purpose = "内部购买问题"
		p.Pages[i].Missing = []string{"内部缺项说明"}
	}
	tasks, err := Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		for _, excluded := range []string{p.Rationale, p.Input.Audience, p.Input.Goal, p.Pages[0].Purpose, p.Pages[0].Missing[0], task.PlatformName, "幕后", "两行", `""`} {
			if strings.Contains(task.Prompt, excluded) {
				t.Fatalf("planning text leaked: %s", excluded)
			}
		}
		if !strings.Contains(task.Prompt, p.Pages[0].Visual) || !strings.Contains(task.Prompt, p.Pages[0].Headline) {
			t.Fatal("render content missing")
		}
	}
}

func TestWholeDetailMakesOneTaskPerPlatformWithOrderedReferences(t *testing.T) {
	p := validPlan()
	p.RenderMode = "whole"
	tasks, err := Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 9 {
		t.Fatalf("want one task per platform variant, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.PageID != "whole" || len(task.ProductAssetIDs()) != 2 || task.ProductAssetIDs()[0] != "front" || task.ProductAssetIDs()[1] != "detail" || len(task.SectionIDs) < 3 {
			t.Fatalf("invalid whole task: %+v", task)
		}
		if !strings.Contains(task.Prompt, "一次生成一张完整") || !strings.Contains(task.Prompt, "图片3仅参考视觉风格") || !strings.Contains(task.Prompt, p.StyleLock) {
			t.Fatal("whole render contract missing")
		}
		if task.PlatformID == "taobao" && (task.Width != 1440 || task.Height != 2560) {
			t.Fatal("whole portrait dimensions")
		}
		if task.PlatformID == "amazon" && (task.Width != 2560 || task.Height != 1440) {
			t.Fatal("whole landscape dimensions")
		}
	}
	p.RenderMode = "unknown"
	if _, err := Compile(p); err == nil {
		t.Fatal("unknown render mode accepted")
	}
}
