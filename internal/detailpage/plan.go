// Package detailpage plans product content independently of platform rendering.
package detailpage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Platform struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Sections []string `json:"sections"`
	Guidance string   `json:"guidance"`
}

// Dimensions are BriefOnce presets, not a claim of universal marketplace rules.
var Platforms = []Platform{
	{"taobao", "淘宝 / 天猫", 750, 1000, []string{"hero", "benefits", "details", "specs", "usage"}, "移动纵向阅读；短标题、大图、分屏叙事。"},
	{"jd", "京东", 790, 1053, []string{"hero", "details", "specs", "usage"}, "强调可核对的商品、工艺、规格与服务，避免密集促销贴纸。"},
	{"douyin", "抖店", 750, 1000, []string{"hero", "benefits", "details", "specs"}, "移动端快速阅读；一屏一个主张，精简短标签。"},
	{"miniprogram", "微信小程序", 750, 1200, []string{"hero", "benefits", "specs"}, "纵向卡片，大留白，清晰商品与必要参数。"},
	{"tiktok", "TikTok Shop", 1000, 1000, []string{"hero", "benefits", "details", "specs", "usage"}, "方形商品图卡，信息集中，适合手机阅读。保留用户确认的文案语言，不自动翻译事实。"},
	{"ozon-mobile", "Ozon 移动", 640, 640, []string{"hero", "benefits", "specs", "usage"}, "移动富内容方图，少量短文案；保留确认语言，按站点核对。"},
	{"ozon-desktop", "Ozon 桌面", 1416, 708, []string{"hero", "benefits", "specs", "usage"}, "桌面横幅富内容，横向图文构图；不要把竖版整屏压缩到横幅。"},
	{"amazon", "Amazon 基础 A+", 970, 300, []string{"hero", "benefits", "details", "specs", "usage"}, "宽幅横向模块，仅保留一个核心信息。不要添加价格、优惠、紧迫促销、购买按钮或未经证实的徽章。按实际站点模块核对。"},
	{"pdd", "拼多多", 750, 1000, []string{"hero", "benefits", "specs", "usage"}, "紧凑清楚的商品、卖点、参数和服务分图，避免堆砌信息。"},
}

type Scene struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
}

// Authored for BriefOnce: scene purposes, not copied third-party prompt text.
var Scenes = []Scene{
	{"hero", "白底 / 纯色商品", "清楚呈现完整商品轮廓，干净背景，不增加道具。"},
	{"lifestyle", "生活场景", "让商品在与用途相符的环境中自然出现，人物和动作服务于本屏目的。"},
	{"flat-lay", "俯拍平铺", "俯拍真实商品及已提供配件，有序摆放，不虚构套装内容。"},
	{"macro", "细节微距", "只放大参考图中清晰可见的细节，不臆造纹理和隐藏结构。"},
	{"banner", "活动海报", "主视觉与简短标题形成层级，仅使用已确认的活动信息。"},
	{"social", "社交内容", "自然生活视角，简洁构图，不伪造发布界面、点赞或评论。"},
	{"ugc", "自然使用", "自然人物状态和真实质感；属于创意示意，不伪造真实买家证言。"},
	{"model", "模特展示", "穿着或持用关系准确，姿势避免遮挡关键商品结构。"},
	{"comparison", "对比展示", "仅对比有事实依据的可观察差异，不虚构功效前后变化。"},
	{"packaging", "包装展示", "仅展示原图或事实中已有的包装与配件。"},
	{"infographic", "卖点信息图", "围绕一个问题，用图文分区、少量标签和对应细节作答。"},
	{"concept", "创意广告", "创意环境可设计，但保持商品准确，不把艺术效果当成功能证据。"},
	{"size-spec", "尺寸与步骤", "测量箭头或步骤区域清楚，数值严格使用已确认事实。"},
	{"bundle", "商品组合", "只组合真实提供的商品规格与配件，避免暗示未包含赠品。"},
	{"livestream", "演示场景", "人物自然演示商品，不伪造观看人数、成交或实时评论。"},
	{"try-on", "试穿 / 场景融合", "商品与人物、空间透视合理，保持商品版型与结构。"},
	{"exploded", "结构拆解", "仅在有结构参考和零件资料时展示拆解，否则提示缺少素材。"},
	{"ghost", "立体服装", "保持衣物原版型和垂坠，不虚构未提供的内衬或背面。"},
	{"grid", "多角度网格", "使用已提供视角或颜色，统一光线，不创造不存在的款式。"},
	{"editorial", "品牌杂志", "克制的编辑式摄影与留白，自然人物情绪，减少装饰堆砌。"},
	{"seasonal", "季节主题", "环境可随季节变化，商品事实和整套品牌风格保持一致。"},
	{"atmospheric", "氛围主视觉", "氛围用于衬托商品，不用烟雾等效果遮挡重要细节。"},
	{"device", "设备界面", "只有提供真实界面时才展示内容，不虚构软件能力或数据。"},
	{"space", "空间展示", "环境为创意场景，不暗示不存在的实体店或工厂资质。"},
	{"sports", "运动场景", "动作符合商品已确认的适用范围，不暗示未经证实的专业性能。"},
}

type Facts struct {
	Title   string `json:"title"`
	Claims  string `json:"claims"`
	Details string `json:"details"`
	Specs   string `json:"specs"`
	Usage   string `json:"usage"`
}
type Asset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}
type Input struct {
	Facts       Facts    `json:"facts"`
	Audience    string   `json:"audience"`
	Goal        string   `json:"goal"`
	Brand       string   `json:"brand"`
	Assets      []Asset  `json:"assets"`
	PlatformIDs []string `json:"platform_ids"`
	ReferenceID string   `json:"reference_id"`
}
type Page struct {
	ID            string   `json:"id"`
	SceneID       string   `json:"scene_id"`
	Purpose       string   `json:"purpose"`
	SourceAssetID string   `json:"source_asset_id"`
	Headline      string   `json:"headline"`
	Copy          string   `json:"copy"`
	Visual        string   `json:"visual"`
	Evidence      []string `json:"evidence"`
	Missing       []string `json:"missing"`
}
type Plan struct {
	RenderMode string `json:"render_mode,omitempty"`
	Input      Input  `json:"input"`
	Driver     string `json:"driver"`
	Rationale  string `json:"rationale"`
	StyleLock  string `json:"style_lock"`
	Pages      []Page `json:"pages"`
}
type Task struct {
	SourceAssetIDs []string `json:"source_asset_ids,omitempty"`
	SectionIDs     []string `json:"section_ids,omitempty"`
	PageID         string   `json:"page_id"`
	PlatformID     string   `json:"platform_id"`
	PlatformName   string   `json:"platform_name"`
	Headline       string   `json:"headline"`
	SourceAssetID  string   `json:"source_asset_id"`
	ReferenceID    string   `json:"reference_id"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Prompt         string   `json:"prompt"`
	StyleVersion   string   `json:"style_version"`
}

func bounded(s string, max int) bool { return utf8.RuneCountInString(s) <= max }
func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
func PlatformsFor(ids []string) ([]Platform, error) {
	if len(ids) == 0 || len(ids) > 8 {
		return nil, errors.New("请选择原有平台中的 1–8 个")
	}
	seen := map[string]bool{}
	result := []Platform{}
	for _, id := range ids {
		if seen[id] {
			return nil, errors.New("平台不能重复")
		}
		seen[id] = true
		found := false
		for _, p := range Platforms {
			if p.ID == id || id == "ozon" && strings.HasPrefix(p.ID, "ozon-") {
				result = append(result, p)
				found = true
			}
		}
		if !found || id == "ozon-mobile" || id == "ozon-desktop" {
			return nil, errors.New("平台无效，请使用原平台选项")
		}
	}
	return result, nil
}
func (in Input) Validate() error {
	if strings.TrimSpace(in.Facts.Title) == "" || len(in.Assets) < 1 || len(in.Assets) > 5 {
		return errors.New("请提供商品名称和 1–5 张商品素材")
	}
	for _, s := range []string{in.Facts.Title, in.Facts.Claims, in.Facts.Details, in.Facts.Specs, in.Facts.Usage, in.Audience, in.Goal, in.Brand} {
		if !bounded(s, 3000) {
			return errors.New("单项内容不能超过 3000 字")
		}
	}
	seen := map[string]bool{}
	for _, a := range in.Assets {
		if a.ID == "" || !bounded(a.ID, 100) || seen[a.ID] || !bounded(a.Name, 200) || !bounded(a.Role, 200) {
			return errors.New("素材信息无效或重复")
		}
		seen[a.ID] = true
	}
	_, err := PlatformsFor(in.PlatformIDs)
	return err
}
func SceneByID(id string) (Scene, bool) {
	for _, s := range Scenes {
		if s.ID == id {
			return s, true
		}
	}
	return Scene{}, false
}
func (p Plan) Validate() error {
	if p.RenderMode != "" && p.RenderMode != "whole" && p.RenderMode != "modules" {
		return errors.New("出图方式无效")
	}
	if err := p.Input.Validate(); err != nil {
		return err
	}
	if !contains([]string{"visual", "problem", "lifestyle"}, p.Driver) || strings.TrimSpace(p.StyleLock) == "" || !bounded(p.StyleLock, 2000) || !bounded(p.Rationale, 600) {
		return errors.New("购买动机或整套品牌风格不完整")
	}
	if len(p.Pages) != 5 {
		return errors.New("方案须包含首屏、卖点、细节、规格、使用五类内容，平台按原有模块选用")
	}
	seen := map[string]bool{}
	ids := map[string]bool{}
	for _, a := range p.Input.Assets {
		ids[a.ID] = true
	}
	facts := map[string]bool{"title": true, "claims": true, "details": true, "specs": true, "usage": true}
	for _, page := range p.Pages {
		if !contains([]string{"hero", "benefits", "details", "specs", "usage"}, page.ID) || seen[page.ID] {
			return errors.New("内容模块缺失或重复")
		}
		seen[page.ID] = true
		if _, ok := SceneByID(page.SceneID); !ok {
			return errors.New("场景模板无效")
		}
		if !ids[page.SourceAssetID] {
			return errors.New("每一屏都需要明确选择本产品素材")
		}
		if strings.TrimSpace(page.Purpose) == "" || strings.TrimSpace(page.Visual) == "" || strings.TrimSpace(page.Headline) == "" || !bounded(page.Purpose, 300) || !bounded(page.Visual, 1500) || !bounded(page.Headline, 60) || !bounded(page.Copy, 600) {
			return errors.New("每屏目的、标题、构图或文案不完整或过长")
		}
		if len(page.Evidence) > 5 || len(page.Missing) > 8 {
			return errors.New("事实来源或缺项过多")
		}
		for _, s := range page.Evidence {
			if !facts[s] {
				return errors.New("事实来源必须指向已有商品字段")
			}
		}
		for _, s := range page.Missing {
			if !bounded(s, 200) {
				return errors.New("缺项说明过长")
			}
		}
	}
	return nil
}

// Compile keeps the same style block byte-for-byte and adds platform-specific composition.
// Freeform model output cannot override the trusted constraints appended at the end.
func Compile(p Plan) ([]Task, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	platforms, _ := PlatformsFor(p.Input.PlatformIDs)
	sum := sha256.Sum256([]byte(p.StyleLock))
	version := hex.EncodeToString(sum[:])[:12]
	pages := map[string]Page{}
	for _, pg := range p.Pages {
		pages[pg.ID] = pg
	}
	tasks := []Task{}
	for _, platform := range platforms {
		if p.RenderMode == "whole" {
			tasks = append(tasks, compileWhole(p, platform, pages, version))
			continue
		}
		for _, id := range platform.Sections {
			pg := pages[id]
			role := "图片1是当前商品素材，唯一商品外观依据。"
			if p.Input.ReferenceID != "" {
				role += "图片2仅为视觉风格参考，不复制其中商品、人物身份、品牌与文字。"
			}
			copyLines := []string{fmt.Sprintf("%q", strings.TrimSpace(pg.Headline))}
			if copy := strings.TrimSpace(pg.Copy); copy != "" {
				copyLines = append(copyLines, fmt.Sprintf("%q", copy))
			}
			prompt := fmt.Sprintf(`生成一张 %d×%d 的商品图片。

%s

视觉风格：
%s

构图与画面：
%s

保留商品原图的颜色、轮廓、结构、部件数量与位置、缝线和标识。不新增部件，不臆造原图不可见的背面、内里或微观纹理。

仅绘制以下文案，各出现一次，不绘制引号或其他文字：
%s`, platform.Width, platform.Height, role, p.StyleLock, pg.Visual, strings.Join(copyLines, "\n"))
			tasks = append(tasks, Task{PageID: pg.ID, PlatformID: platform.ID, PlatformName: platform.Name, Headline: pg.Headline, SourceAssetID: pg.SourceAssetID, ReferenceID: p.Input.ReferenceID, Width: platform.Width, Height: platform.Height, Prompt: prompt, StyleVersion: version})
		}
	}
	return tasks, nil
}

func RenderSize(t Task) string {
	if t.Width > t.Height*2 {
		return "1536*512"
	}
	if t.Width > t.Height {
		return "1440*720"
	}
	if t.Height > t.Width*3/2 {
		return "960*1536"
	}
	if t.Height > t.Width {
		return "960*1280"
	}
	return "1024*1024"
}

// Whole-page mode makes a single image request per platform, using all selected
// product references in their declared order. It never stitches module images.
func compileWhole(p Plan, platform Platform, pages map[string]Page, version string) Task {
	ids := []string{}
	roles := []string{}
	for i, asset := range p.Input.Assets {
		ids = append(ids, asset.ID)
		roles = append(roles, fmt.Sprintf("图片%d是商品参考，名称：%q，用途：%q。只依据实际可见内容，不从名称推断商品事实。", i+1, asset.Name, asset.Role))
	}
	if p.Input.ReferenceID != "" {
		roles = append(roles, fmt.Sprintf("图片%d仅参考视觉风格，不复制其中商品、人物身份、品牌或文字。", len(ids)+1))
	}
	blocks := []string{}
	for i, id := range platform.Sections {
		pg := pages[id]
		number := 1
		for j, assetID := range ids {
			if assetID == pg.SourceAssetID {
				number = j + 1
			}
		}
		blocks = append(blocks, fmt.Sprintf("内容区%d（商品依据图片%d）：\n画面：%s\n仅绘制标题：%q\n仅绘制正文：%q", i+1, number, pg.Visual, strings.TrimSpace(pg.Headline), strings.TrimSpace(pg.Copy)))
	}
	width, height := 1440, 2560
	if platform.Width > platform.Height {
		width, height = 2560, 1440
	}
	prompt := fmt.Sprintf(`一次生成一张完整的商品详情设计图，画幅 %d×%d。整页统一设计，按以下内容区顺序阅读；不要分别输出图片，不要生成拼图边框，不添加文案以外的文字。根据画幅合理分配信息，不保证所有原始资料逐字上图。

%s

统一视觉风格：
%s

画面与确认文案：
%s

保留商品参考的真实颜色、轮廓、结构、部件、缝线与标识。不臆造不可见的背面、内里和材质。标题与正文各出现一次，不绘制引号或其他文字。`, width, height, strings.Join(roles, "\n"), p.StyleLock, strings.Join(blocks, "\n\n"))
	return Task{SectionIDs: platform.Sections, SourceAssetIDs: ids, PageID: "whole", PlatformID: platform.ID, PlatformName: platform.Name, Headline: p.Input.Facts.Title + " · 完整详情图", SourceAssetID: ids[0], ReferenceID: p.Input.ReferenceID, Width: width, Height: height, Prompt: prompt, StyleVersion: version}
}

func (t Task) ProductAssetIDs() []string {
	if len(t.SourceAssetIDs) > 0 {
		return t.SourceAssetIDs
	}
	return []string{t.SourceAssetID}
}
