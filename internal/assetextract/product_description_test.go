package assetextract

import (
	"github.com/tian1363/scriptagent/internal/jobs"
	"strings"
	"testing"
)

func TestProductAssetDescriptionExcludesReviewReasoning(t *testing.T) {
	item := jobs.AssetExtraction{Kind: "product", Analysis: "关键帧选择：依据清晰程度。\n### 原帧可见事实\n- 原帧中的夹克。\n### 生成图可见事实\n- **产品展示**：白色背景下的灰绿色连帽夹克正面图。\n- **细节**：带拉链门襟和侧边口袋。\n### 两图差异与不确定处\n- 证据显示两图一致。"}
	result := ProductAssetDescription(item)
	if !strings.Contains(result, "灰绿色连帽夹克") || strings.Contains(result, "证据") || strings.Contains(result, "关键帧") || strings.Contains(result, "一致") {
		t.Fatal(result)
	}
	item.Analysis = "### 结论\n审核通过，判定一致，证据充分。"
	if got := ProductAssetDescription(item); got != "商品外观参考图。" {
		t.Fatal(got)
	}
}
