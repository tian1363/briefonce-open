package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tian1363/scriptagent/internal/charactergen"
	"github.com/tian1363/scriptagent/internal/imagepolicy"
	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/userctx"
)

type posePreset struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Use           string   `json:"use"`
	Instruction   string   `json:"instruction"`
	Action        string   `json:"action"`
	Style         string   `json:"style"`
	Gender        string   `json:"gender"`
	ImageURL      string   `json:"image_url"`
	Shot          string   `json:"shot"`
	ClothingTags  []string `json:"clothing_tags"`
	ActionTags    []string `json:"action_tags"`
	StyleTags     []string `json:"style_tags"`
	Scene         string   `json:"scene"`
	SourceAuthor  string   `json:"source_author"`
	SourceURL     string   `json:"source_url"`
	SourceURLType string   `json:"source_url_type"`
	PreviewStatus string   `json:"preview_status"`
}

// The library contains original pose directions, not third-party photos or a claimed ranking.
var posePresets = []posePreset{
	{ID: "front", Name: "正面自然站姿", Use: "首图 · 展示整体轮廓", Instruction: "正面面向镜头，双脚自然站立，双臂放松，不遮挡商品。", Action: "站姿", Style: "棚拍", Gender: "女装", ImageURL: "/pose-gallery/front.png"},
	{ID: "three-quarter", Name: "三分之二侧身", Use: "版型 · 展示立体结构", Instruction: "身体向镜头转约四十五度，头部自然回望，一只手轻搭腰侧。", Action: "站姿", Style: "棚拍", Gender: "女装", ImageURL: "/pose-gallery/three-quarter.png"},
	{ID: "side", Name: "侧面站姿", Use: "侧面 · 展示厚度与线条", Instruction: "身体侧对镜头站立，背部自然挺直，手臂避开商品主体。", Action: "站姿", Style: "棚拍", Gender: "女装", ImageURL: "/pose-gallery/side.png"},
	{ID: "back", Name: "背面回望", Use: "背面 · 展示后侧细节", Instruction: "身体背向镜头站立，头部轻微回望，保持后侧商品完整可见。", Action: "站姿", Style: "棚拍", Gender: "女装", ImageURL: "/pose-gallery/back.png"},
	{ID: "walk", Name: "自然走动", Use: "动态 · 展示穿着状态", Instruction: "向前自然迈一步，身体平衡，衣物与配饰保持清晰，不做夸张运动。", Action: "走动", Style: "动态", Gender: "女装", ImageURL: "/pose-gallery/walk.png"},
	{ID: "seated", Name: "坐姿展示", Use: "场景 · 展示实际使用", Instruction: "自然坐在简洁座椅上，双腿与手臂舒展，商品主体无遮挡。", Action: "坐姿", Style: "生活方式", Gender: "女装", ImageURL: "/pose-gallery/seated.png"},
	{ID: "lean", Name: "轻靠站姿", Use: "氛围 · 生活方式", Instruction: "肩部轻靠简洁墙面，重心自然落在一侧，保持商品形状真实。", Action: "站姿", Style: "生活方式", Gender: "女装", ImageURL: "/pose-gallery/lean.png"},
	{ID: "hold", Name: "手持展示", Use: "手持 · 突出商品比例", Instruction: "双手或单手自然持有商品，展示其实际大小与正面，手指不遮挡标识。", Action: "手持", Style: "棚拍", Gender: "女装", ImageURL: "/pose-gallery/hold.png"},
	{ID: "detail", Name: "近景细节展示", Use: "细节 · 材质和工艺", Instruction: "上半身或手部近景，动作自然，商品细节、材质和结构清楚可辨。", Action: "细节", Style: "棚拍", Gender: "女装", ImageURL: "/pose-gallery/detail.png"},
	{ID: "turn", Name: "转身回看", Use: "动态 · 展示多角度", Instruction: "身体轻微转身并回看镜头，动作自然，商品主体完整且清晰。", Action: "走动", Style: "动态", Gender: "女装", ImageURL: "/pose-gallery/turn.png"},
	{ID: "front-male", Name: "男模正面站姿", Use: "首图 · 展示整体轮廓", Instruction: "正面面向镜头，双脚自然站立，双臂放松，不遮挡商品。", Action: "站姿", Style: "棚拍", Gender: "男装", ImageURL: "/pose-gallery/front-male.png"},
	{ID: "walk-male", Name: "男模自然走动", Use: "动态 · 展示穿着状态", Instruction: "向前自然迈一步，身体平衡，衣物与配饰保持清晰。", Action: "走动", Style: "动态", Gender: "男装", ImageURL: "/pose-gallery/walk-male.png"},
	{ID: "seated-male", Name: "男模坐姿展示", Use: "场景 · 展示实际使用", Instruction: "自然坐在简洁座椅上，双腿与手臂舒展，商品主体无遮挡。", Action: "坐姿", Style: "生活方式", Gender: "男装", ImageURL: "/pose-gallery/seated-male.png"},
	{ID: "pocket", Name: "单手轻搭裤袋", Use: "裤装 · 展示腰线", Instruction: "正面站立，一手轻搭裤袋入口，另一手放松，手臂不要遮住腰头与裤型。", Action: "手臂动作", Style: "都市通勤", Gender: "通用", ImageURL: ""},
	{ID: "cross-step", Name: "前后错步站立", Use: "裤装 · 展示腿部线条", Instruction: "双脚前后轻微错开，膝盖自然伸展，身体保持直立，避免双腿重叠遮挡裤型。", Action: "交叉腿", Style: "自然松弛", Gender: "通用", ImageURL: ""},
	{ID: "strap", Name: "单手扶肩带", Use: "箱包 · 展示佩戴方式", Instruction: "自然站立，一手轻扶已提供商品图中的肩带，让包身正面朝向镜头且无遮挡。", Action: "手臂动作", Style: "都市通勤", Gender: "通用", ImageURL: ""},
	{ID: "arms-open", Name: "双臂微展", Use: "上衣 · 展示袖型", Instruction: "正面站立，双臂向身体两侧稍微展开，手肘自然弯曲，完整呈现衣身和袖型。", Action: "手臂动作", Style: "棚拍", Gender: "通用", ImageURL: ""},
	{ID: "seated-upright", Name: "端坐展示", Use: "上装 · 保持衣身平整", Instruction: "坐在无扶手简洁座椅上，背部自然挺直，双手放在腿侧，不遮挡衣身。", Action: "坐姿", Style: "棚拍", Gender: "通用", ImageURL: ""},
	{ID: "step-pause", Name: "迈步停顿", Use: "运动装 · 展示活动感", Instruction: "一脚向前小幅迈出后停顿，双臂自然摆动，保持商品轮廓清晰，避免运动模糊。", Action: "走动", Style: "运动休闲", Gender: "通用", ImageURL: ""},
	{ID: "close-front", Name: "上半身正面", Use: "上衣 · 展示领口与肩线", Instruction: "镜头取上半身正面，双肩放松，双手离开领口，只展示商品图确实可见的领口与肩线。", Action: "细节", Style: "棚拍", Gender: "通用", ImageURL: ""},
	{ID: "present-front", Name: "掌心托举", Use: "手持 · 展示商品正面", Instruction: "双手在胸前下方平稳托举商品，商品正面朝镜头，手指避开文字和标识，尺寸遵循参考图。", Action: "手持", Style: "棚拍", Gender: "通用", ImageURL: ""},
}

//go:embed pose_catalog.json
var poseCatalogJSON []byte

var resourcePosePresets = loadResourcePosePresets()

func loadResourcePosePresets() []posePreset {
	var items []posePreset
	if err := json.Unmarshal(poseCatalogJSON, &items); err != nil {
		panic(err)
	}
	// Legacy IDs remain accepted for saved batches and retries.
	posePresets = append(posePresets, items...)
	return items
}

func (h *Handler) listPosePresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, resourcePosePresets)
}

func (h *Handler) generatePoseImage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TargetColor        string               `json:"target_color"`
		SupportingAssetID  string               `json:"supporting_asset_id"`
		SameStyleConfirmed bool                 `json:"same_style_confirmed"`
		Scene              string               `json:"scene"`
		ProductID          string               `json:"product_id"`
		AssetID            string               `json:"asset_id"`
		AssetIDs           []string             `json:"asset_ids"`
		CharacterID        string               `json:"character_id"`
		PoseID             string               `json:"pose_id"`
		PoseIDs            []string             `json:"pose_ids"`
		Category           string               `json:"category"`
		Name               string               `json:"name"`
		ImageOptions       charactergen.Options `json:"image_options"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(&struct{}{}) != io.EOF {
		writeError(w, 400, errors.New("姿势生成参数无效"))
		return
	}
	userID := userIDFromRequest(r)
	if _, err := h.store.GetUserProduct(userID, in.ProductID); err != nil {
		writeError(w, 404, errors.New("产品资料不存在"))
		return
	}
	assetIDs := in.AssetIDs
	if len(assetIDs) == 0 {
		assetIDs = []string{in.AssetID}
	} else if in.AssetID != "" {
		writeError(w, 400, errors.New("请使用 asset_ids 选择多张商品图"))
		return
	}
	if len(assetIDs) == 0 || len(assetIDs) > 6 {
		writeError(w, 400, errors.New("每批请选择 1–6 张商品图"))
		return
	}
	assets := make([]*jobs.ProductAsset, 0, len(assetIDs))
	assetSeen := map[string]bool{}
	for _, id := range assetIDs {
		if id == "" || assetSeen[id] {
			writeError(w, 400, errors.New("同一批不能重复选择商品图"))
			return
		}
		assetSeen[id] = true
		asset, err := h.store.GetProductAsset(id)
		if err != nil || asset.ProductID != in.ProductID || asset.Kind != "image" {
			writeError(w, 404, errors.New("商品参考图不存在"))
			return
		}
		assets = append(assets, asset)
	}
	in.TargetColor = strings.TrimSpace(in.TargetColor)
	if utf8.RuneCountInString(in.TargetColor) > 40 {
		writeError(w, 400, errors.New("目标颜色最多 40 字"))
		return
	}
	if in.Scene == "" {
		in.Scene = "studio"
	}
	if _, ok := poseSceneDirections[in.Scene]; !ok {
		writeError(w, 400, errors.New("请选择支持的场景"))
		return
	}
	var supportingImage []byte
	if in.SupportingAssetID != "" {
		if in.Category != "apparel" || !in.SameStyleConfirmed {
			writeError(w, 400, errors.New("请确认结构参考与主商品是同款服装，仅颜色或视角不同"))
			return
		}
		if assetSeen[in.SupportingAssetID] {
			writeError(w, 400, errors.New("结构参考图不能同时作为主商品图"))
			return
		}
		supporting, e := h.store.GetProductAsset(in.SupportingAssetID)
		if e != nil || supporting.ProductID != in.ProductID || supporting.Kind != "image" {
			writeError(w, 404, errors.New("同款结构参考图不存在"))
			return
		}
		stat, e := os.Stat(supporting.Path)
		if e != nil || stat.Size() > 10<<20 {
			writeError(w, 400, errors.New("结构参考图不可用或超过 10MB"))
			return
		}
		supportingImage, e = os.ReadFile(supporting.Path)
		mime := http.DetectContentType(supportingImage)
		if e != nil || (mime != "image/jpeg" && mime != "image/png") {
			writeError(w, 400, errors.New("结构参考请使用 JPG 或 PNG"))
			return
		}
	}
	role, err := h.store.GetCharacter(userID, in.CharacterID)
	if err != nil || role.Status != "ready" {
		writeError(w, 404, errors.New("模特参考图不存在"))
		return
	}
	batch := len(in.PoseIDs) > 0 || len(in.AssetIDs) > 0
	poseIDs := in.PoseIDs
	if !batch {
		poseIDs = []string{in.PoseID}
	}
	if len(poseIDs) == 0 || len(poseIDs) > 6 {
		writeError(w, 400, errors.New("每批请选择 1–6 个姿势"))
		return
	}
	if len(poseIDs) > 6 {
		writeError(w, 400, errors.New("每批最多选择 6 个姿势"))
		return
	}
	poses := make([]posePreset, 0, len(poseIDs))
	seen := map[string]bool{}
	for _, id := range poseIDs {
		if seen[id] {
			writeError(w, 400, errors.New("同一批不能重复选择姿势"))
			return
		}
		seen[id] = true
		found := false
		for _, candidate := range posePresets {
			if candidate.ID == id {
				poses = append(poses, candidate)
				found = true
				break
			}
		}
		if !found {
			writeError(w, 400, errors.New("请选择姿势库中的动作"))
			return
		}
	}
	if in.Category != "apparel" && in.Category != "bag" && in.Category != "handheld" {
		writeError(w, 400, errors.New("请选择服装、箱包或手持商品"))
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" && !batch {
		in.Name = poses[0].Name + " · 电商图"
	}
	if utf8.RuneCountInString(in.Name) > 80 {
		writeError(w, 400, errors.New("图片名称最多 80 字"))
		return
	}
	personImage, err := os.ReadFile(role.Path)
	if err != nil {
		writeError(w, 400, errors.New("模特图读取失败"))
		return
	}
	productImages := make([][]byte, len(assets))
	for i, asset := range assets {
		if info, statErr := os.Stat(asset.Path); statErr != nil || info.Size() > 10<<20 || asset.SizeBytes > 10<<20 {
			writeError(w, 400, errors.New("商品参考图不可用或超过 10MB"))
			return
		}
		productImages[i], err = os.ReadFile(asset.Path)
		if err != nil {
			writeError(w, 400, errors.New("商品图读取失败"))
			return
		}
		if mime := http.DetectContentType(productImages[i]); mime != "image/jpeg" && mime != "image/png" {
			writeError(w, 400, errors.New("姿势生成请使用 JPG 或 PNG 商品图"))
			return
		}
	}
	poseImages := make(map[string][]byte, len(poses))
	for _, pose := range poses {
		data, loadErr := loadPoseReference(h.cfg.StaticDir, pose)
		if loadErr != nil {
			writeError(w, 400, loadErr)
			return
		}
		poseImages[pose.ID] = data
	}
	cfg, options, err := h.imageRuntime(r, "image_edit", in.ImageOptions)
	if err != nil {
		writeError(w, 400, err)
		return
	}
	if cfg.Model == "" {
		writeError(w, 400, errors.New("请先配置图片编辑模型"))
		return
	}
	if cfg.Source == "managed" {
		if !h.cfg.AllowManagedMode {
			writeError(w, 403, errors.New("当前账号不能使用平台图片额度"))
			return
		}
	}
	if cfg.APIKey == "" {
		writeError(w, 400, errors.New("请先连接图片编辑模型 API Key"))
		return
	}
	if !h.authLimit.allow("pose:"+userID, 3, 5*time.Minute) {
		writeError(w, 429, errors.New("生成请求较多，请稍后重试"))
		return
	}
	if cfg.Provider != "zexapi" && len(productImages)+2+map[bool]int{true: 1}[len(supportingImage) > 0] > 3 {
		writeError(w, 400, errors.New("当前图片模型最多支持 3 张参考图，请选择支持多图的图片模型"))
		return
	}
	if len(productImages)+2+map[bool]int{true: 1}[len(supportingImage) > 0] > 8 {
		writeError(w, 400, errors.New("含模特、姿势和结构参考最多 8 张图片；使用结构参考时最多选择 5 张商品图"))
		return
	}
	type poseTask struct {
		pose  posePreset
		asset *jobs.ProductAsset
	}
	tasks := make([]poseTask, 0, len(poses))
	for _, pose := range poses {
		tasks = append(tasks, poseTask{pose: pose, asset: assets[0]})
	}
	poseReferences := func(task poseTask) [][]byte {
		refs := append([][]byte{personImage}, productImages...)
		if len(supportingImage) > 0 {
			refs = append(refs, supportingImage)
		}
		return append(refs, poseImages[task.pose.ID])
	}
	posePrompt := func(task poseTask) string {
		return buildMultiPosePrompt(task.pose, in.Category, in.Scene, len(supportingImage) > 0, in.TargetColor, len(productImages))
	}
	if cfg.Provider == "zexapi" {
		options = normalizeImageOptions(options, "3:4")
		for _, task := range tasks {
			if err = charactergen.ValidateInput(options, posePrompt(task), poseReferences(task)); err != nil {
				writeError(w, 400, err)
				return
			}
		}
	}
	charges := make([]string, len(tasks))
	if cfg.Source == "managed" {
		for i := range tasks {
			charges[i], err = h.reserveImage(r, "image_edit", cfg, options)
			if err != nil {
				for _, id := range charges {
					_ = h.store.FinishUsage(id, false)
				}
				writeError(w, 402, err)
				return
			}
		}
	}
	batchID := "pose-" + time.Now().UTC().Format("20060102T150405.000000000")
	specs := make([]jobs.CharacterSpec, len(tasks))
	for i, task := range tasks {
		name := in.Name
		if batch && name != "" {
			name += " · " + task.pose.Name
		}
		if name == "" {
			name = task.pose.Name + " · 电商图"
		}
		if utf8.RuneCountInString(name) > 80 {
			writeError(w, 400, errors.New("批次名称和姿势名称合计最多 80 字"))
			for _, id := range charges {
				_ = h.store.FinishUsage(id, false)
			}
			return
		}
		allAssetIDs, _ := json.Marshal(assetIDs)
		metadata, _ := json.Marshal(map[string]string{"asset_ids": string(allAssetIDs), "pose_id": task.pose.ID, "product_id": in.ProductID, "asset_id": task.asset.ID, "category": in.Category, "batch_id": batchID, "supporting_asset_id": in.SupportingAssetID, "same_style_confirmed": fmt.Sprint(in.SameStyleConfirmed), "target_color": in.TargetColor, "scene": in.Scene, "view_status": map[bool]string{true: "AI 跨色视角补全，待核对", false: "基于主图生成，待核对"}[len(supportingImage) > 0]})
		specs[i] = jobs.CharacterSpec{Name: name, Controls: metadata}
	}
	results, err := h.store.CreateCharacterBatch(userID, "pose", role.ID, "queued", specs)
	if err != nil {
		for _, id := range charges {
			_ = h.store.FinishUsage(id, false)
		}
		writeError(w, 400, err)
		return
	}
	usageTasks := make([]jobs.UsageTask, len(results))
	for i, result := range results {
		usageTasks[i] = jobs.UsageTask{UserID: userID, TaskID: result.ID, ChargeID: charges[i], Title: result.Name, ProductID: in.ProductID, Model: cfg.Model, Source: cfg.Source, Capability: "image_edit"}
	}
	if err := h.store.RecordUsageTasks(usageTasks); err != nil {
		for i, result := range results {
			_ = h.store.FinishUsage(charges[i], false)
			_ = h.store.FinishCharacter(userID, result.ID, "", "计费记录保存失败")
		}
		writeError(w, 500, err)
		return
	}
	linkAnalyticsUsage(r, usageTasks)
	if cfg.Provider == "zexapi" {
		for i, result := range results {
			err = h.queueImage(jobs.ProviderImageJob{ID: result.ID, UserID: userID, Kind: "pose", ProductID: in.ProductID, ChargeID: charges[i], Capability: "image_edit"}, cfg, options, posePrompt(tasks[i]), poseReferences(tasks[i]))
			if err != nil {
				for k := i; k < len(results); k++ {
					_ = h.store.FinishUsage(charges[k], false)
					_ = h.store.FinishCharacter(userID, results[k].ID, "", err.Error())
				}
				writeError(w, 500, err)
				return
			}
		}
		if batch {
			writeJSON(w, http.StatusAccepted, results)
		} else {
			writeJSON(w, http.StatusAccepted, results[0])
		}
		return
	}
	user, _ := userctx.FromContext(r.Context())
	go func() {
		var batchStop error
		for i, result := range results {
			func() {
				success := false
				defer func() { _ = h.store.FinishUsage(charges[i], success) }()
				if err := h.store.StartCharacterGeneration(userID, result.ID); err != nil {
					_ = h.store.FinishCharacter(userID, result.ID, "", err.Error())
					return
				}
				if batchStop != nil {
					_ = h.store.FinishCharacter(userID, result.ID, "", "批次已停止："+batchStop.Error())
					return
				}
				ctx, cancel := context.WithTimeout(userctx.WithUser(context.Background(), user), 4*time.Minute)
				defer cancel()
				prompt := posePrompt(tasks[i])
				data, genErr := h.characterImages.EditReferences(ctx, cfg, prompt, "不同人物、错误商品、颜色或标识改变、多余手指、肢体畸形、假文字、水印", "960*1280", poseReferences(tasks[i]))
				if genErr != nil {
					if errors.Is(genErr, charactergen.ErrRateLimited) || errors.Is(genErr, charactergen.ErrModelUnavailable) || errors.Is(genErr, charactergen.ErrAccessDenied) {
						batchStop = genErr
					}
					_ = h.store.FinishCharacter(userID, result.ID, "", genErr.Error())
					return
				}
				path, saveErr := h.files.SaveCharacterImage(data)
				if saveErr != nil {
					_ = h.store.FinishCharacter(userID, result.ID, "", saveErr.Error())
					return
				}
				mime := http.DetectContentType(data)
				ext := filepath.Ext(path)
				_, saveErr = h.store.CreateProductAsset(jobs.ProductAsset{ProductID: in.ProductID, Kind: "image", Path: path, OriginalName: result.Name + ext, MimeType: mime, SizeBytes: int64(len(data))})
				if saveErr != nil {
					_ = os.Remove(path)
					_ = h.store.FinishCharacter(userID, result.ID, "", "电商图保存失败")
					return
				}
				if h.store.FinishCharacter(userID, result.ID, path, "") != nil {
					return
				}
				success = true
			}()
		}
	}()
	if batch {
		writeJSON(w, http.StatusAccepted, results)
	} else {
		writeJSON(w, http.StatusAccepted, results[0])
	}
}

// Pose images are local catalog assets and only define body position.
func loadPoseReference(staticDir string, pose posePreset) ([]byte, error) {
	if !(strings.HasPrefix(pose.ImageURL, "/pose-gallery/") || strings.HasPrefix(pose.ImageURL, "/pose-resource-gallery/")) || strings.Contains(pose.ImageURL, "..") {
		return nil, fmt.Errorf("姿势“%s”缺少可用参考图，请选择有预览图的姿势", pose.Name)
	}
	path := filepath.Join(staticDir, strings.TrimPrefix(pose.ImageURL, "/"))
	info, err := os.Stat(path)
	if err != nil || info.Size() > 10<<20 {
		return nil, fmt.Errorf("姿势“%s”参考图读取失败或超过 10MB，请重新选择", pose.Name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("姿势参考图读取失败")
	}
	mime := http.DetectContentType(data)
	if mime != "image/png" && mime != "image/jpeg" {
		return nil, errors.New("姿势参考图必须是 JPG 或 PNG")
	}
	return data, nil
}
func buildPosePrompt(pose posePreset, category string) string {
	product := map[string]string{
		"apparel":  "让模特穿着图二中的服装。保持服装的颜色、版型、轮廓、图案、纽扣、拉链、口袋及可见标识。",
		"bag":      "让模特自然携带图二中的箱包。保持形状、结构、配色、材质纹理、五金及可见标识。",
		"handheld": "让模特自然手持图二中的商品。保持包装、形状、颜色、图案、文字及可见标识。",
	}[category]
	return "图一只定义模特身份与外貌；图二是唯一商品视觉依据。图三是姿势参考图，仅参考身体朝向、肩胯关系、手臂、手指与腿部动作及重心；严禁沿用图三的人物身份、脸、发型、服装、配饰、商品、背景、文字或水印。动作描述用于辅助理解姿势图，不作为其他视觉来源。" +
		"不得使用模特图中的其他商品或服装替代图二商品，也不得修改图二商品已可见的颜色、轮廓、结构、图案、文字或标识。" +
		product + "姿势动作：" + pose.Instruction +
		"在不改变商品可见细节的前提下调整人物肢体和商品位置。对于商品图未展示的侧面、背面或隐藏细节，不要编造标识、文字或特殊结构；若动作冲突，优先保留商品真实外观。" +
		"生成真实摄影风格的电商模特图，柔和棚拍光线，简洁浅色背景，竖版构图。不得改变模特身份，不得虚构商品功能或额外配件。" + imagepolicy.NaturalSkin
}

var poseSceneDirections = map[string]string{
	"studio":    "柔和棚拍光线，简洁浅色背景。",
	"courtyard": "自然阴天柔光，极简石材庭院，背景虚化，商品不被道具遮挡。",
	"street":    "城市简洁街景，自然日光，不添加可识别品牌或文字，商品清晰。",
	"indoor":    "干净现代室内，自然窗光，简约背景，商品完整清晰。",
}

func buildSupportedPosePrompt(pose posePreset, category, scene string, supporting bool, targetColor ...string) string {
	prompt := buildPosePrompt(pose, category)
	if scene != "studio" {
		prompt = strings.ReplaceAll(prompt, "柔和棚拍光线，简洁浅色背景，", poseSceneDirections[scene])
	}
	if supporting {
		prompt = strings.ReplaceAll(prompt, "图三是姿势参考图", "图四是姿势参考图")
		prompt = strings.ReplaceAll(prompt, "严禁沿用图三的", "严禁沿用图四的")
		prompt = strings.ReplaceAll(prompt, "图二是唯一商品视觉依据。", "图二确定目标商品颜色与可见外观；图三是用户已确认的同款另一色或另一视角，只辅助补全其可见结构。")
		prompt += "图三只能提供版型、接缝、帽型、衣摆等实际可见结构，禁止采用图三颜色；目标颜色始终依照图二。仅允许对图二缺失而图三可见的同款视角做保守跨色补全，不能新造口袋、标识、文字、功能或隐藏内里。补全部分是 AI 推测，需人工核对，不代表已有该颜色实拍。"
	}
	if len(targetColor) > 0 && targetColor[0] != "" {
		prompt += fmt.Sprintf("最终目标颜色名称（用户确认数据）：%q。整件商品必须统一为此目标颜色；图三如有不同颜色，必须忽略并按目标色重着色，绝不输出结构参考的另一种颜色。", targetColor[0])
	}
	return prompt
}

// All selected product images support one output per pose.
func buildMultiPosePrompt(pose posePreset, category, scene string, supporting bool, color string, count int) string {
	prompt := buildSupportedPosePrompt(pose, category, scene, supporting, color)
	if count <= 1 {
		return prompt
	}
	if supporting {
		prompt = strings.ReplaceAll(prompt, "图四", fmt.Sprintf("第%d张图", count+3))
		prompt = strings.ReplaceAll(prompt, "图三", fmt.Sprintf("第%d张图", count+2))
	} else {
		prompt = strings.ReplaceAll(prompt, "图三", fmt.Sprintf("第%d张图", count+2))
	}
	prompt = strings.ReplaceAll(prompt, "图二是唯一商品视觉依据。", "图二是商品主参考，确定目标款式和颜色。")
	return prompt + fmt.Sprintf("图三至第%d张图是同一商品的补充参考，共同辅助核对实际可见的角度与细节，不代表多个输出任务。若颜色或款式冲突，以图二和用户确认的目标颜色为准，不混合多件商品。每个姿势只生成一张完整图片。", count+1)
}
