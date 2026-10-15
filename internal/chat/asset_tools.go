package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tian1363/scriptagent/internal/assetextract"
	"github.com/tian1363/scriptagent/internal/reactagent"
)

func (s *Service) assetExtractionTools(userID, conversationID, productID string) []reactagent.Tool {
	return []reactagent.Tool{
		{
			Name:        "list_reference_videos",
			Description: "列出当前对话上传的视频、所选产品的视频和已生成视频，返回可用于资产提取的来源 ID。",
			InputSchema: `{"type":"object","properties":{},"additionalProperties":false}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				rows := []map[string]string{}
				if attachments, err := s.store.ListChatAttachments(conversationID); err == nil {
					for _, group := range attachments {
						for _, a := range group {
							if a.Kind == "video" {
								rows = append(rows, map[string]string{"type": "chat_attachment", "attachment_id": a.ID, "name": a.OriginalName})
							}
						}
					}
				}
				if productID != "" {
					if assets, err := s.store.ListProductAssets(productID); err == nil {
						for _, a := range assets {
							if a.Kind == "video" {
								rows = append(rows, map[string]string{"type": "product_asset", "source_asset_id": a.ID, "product_id": a.ProductID, "name": a.OriginalName})
							}
						}
					}
				}
				if videos, err := s.store.ListUserVideoGenerations(userID); err == nil {
					for _, v := range videos {
						if v.Status == "completed" && v.LocalPath != "" {
							rows = append(rows, map[string]string{"type": "generated_video", "source_video_id": v.ID, "product_id": v.ProductID, "name": v.Prompt})
						}
					}
				}
				data, err := json.Marshal(rows)
				return string(data), err
			},
		},
		{
			Name:        "extract_reference_video_asset",
			Description: "从用户自己的参考视频提取人物、商品、品牌 Logo、视频水印、场景、角色姿势、动作或镜头资产。品牌 Logo 只裁切可确认的商品实体标识，视频水印只裁切画面固定的叠加标识；不可辨认时不编造。商品仅在原视频有对应画面时生成多视角与细节图并检查一致性。场景/姿势保留参考图，动作/镜头保留短片。先调用 list_reference_videos 确认来源；at_seconds 是优先搜索位置。结果先保存在提取任务中，用户核对后可导入产品。返回异步任务 ID。",
			InputSchema: `{"type":"object","properties":{"kind":{"type":"string","enum":["person","product","brand_logo","video_watermark","scene","pose","action","camera"]},"label":{"type":"string"},"source_asset_id":{"type":"string"},"source_video_id":{"type":"string"},"source_batch_id":{"type":"string"},"source_variant_id":{"type":"string"},"attachment_id":{"type":"string"},"at_seconds":{"type":"number","minimum":0}},"required":["kind"],"additionalProperties":false}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var in assetextract.Input
				if err := json.Unmarshal(raw, &in); err != nil {
					return "", err
				}
				in.ConversationID = conversationID
				if in.SourceAssetID == "" && in.SourceVideoID == "" && in.SourceBatchID == "" && in.AttachmentID == "" {
					attachments, err := s.store.ListChatAttachments(conversationID)
					if err != nil {
						return "", err
					}
					for _, group := range attachments {
						for _, a := range group {
							if a.Kind == "video" {
								if in.AttachmentID != "" {
									return "", errors.New("有多条视频附件，请指定 attachment_id")
								}
								in.AttachmentID = a.ID
							}
						}
					}
					if in.AttachmentID == "" {
						return "", errors.New("请先上传参考视频或指定来源 ID")
					}
				}
				item, err := s.extractor.Start(ctx, userID, in)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("提取任务已开始：id=%s，状态=%s。原帧：/api/asset-extractions/%s/frame。完成后调用 get_reference_video_asset 查看分视角资产和解析；如质检失败，任务不会提供可导入图片。", item.ID, item.Status, item.ID), nil
			},
		},
		{
			Name:        "get_reference_video_asset",
			Description: "查看资产提取任务状态、视觉解析、各视角图片或动态片段地址、角色 ID 和产品导入结果。ready 表示质检通过，product_id 非空表示已导入产品素材库和文档。",
			InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var in struct {
					ID string `json:"id"`
				}
				if err := json.Unmarshal(raw, &in); err != nil {
					return "", err
				}
				item, err := s.store.GetAssetExtraction(userID, strings.TrimSpace(in.ID))
				if err != nil {
					return "", errors.New("提取任务不存在")
				}
				variantURLs := []map[string]any{}
				for index, variant := range item.Variants {
					base := fmt.Sprintf("/api/asset-extractions/%s/variants/%d", item.ID, index)
					row := map[string]any{"view": variant.View, "at_seconds": variant.AtSeconds, "frame_url": base + "/frame", "image_url": base + "/image", "product_asset_id": variant.ProductAssetID}
					if variant.VideoPath != "" {
						row["clip_url"] = base + "/clip"
					}
					variantURLs = append(variantURLs, row)
				}
				data, err := json.Marshal(map[string]any{"id": item.ID, "kind": item.Kind, "status": item.Status, "target_id": item.TargetID, "product_id": item.ProductID, "product_asset_id": item.ProductAssetID, "analysis": item.Analysis, "error": item.Error, "variants": variantURLs, "frame_url": "/api/asset-extractions/" + item.ID + "/frame", "image_url": "/api/asset-extractions/" + item.ID + "/image"})
				return string(data), err
			},
		},
		{
			Name:        "import_reference_video_asset",
			Description: "用户明确要求将已完成的视频提取结果导入某个已有产品时使用。把合格图片或短片保存到该产品素材库，并把视觉解析保存到独立的资产观察文档，不改写产品事实。需要提取任务 ID 和当前账号拥有的产品 ID；重复导入同一产品不会重复添加。",
			InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"product_id":{"type":"string"}},"required":["id","product_id"],"additionalProperties":false}`,
			Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
				var in struct {
					ID        string `json:"id"`
					ProductID string `json:"product_id"`
				}
				if err := json.Unmarshal(raw, &in); err != nil {
					return "", err
				}
				item, err := s.extractor.Import(ctx, userID, strings.TrimSpace(in.ID), strings.TrimSpace(in.ProductID))
				if err != nil {
					return "", err
				}
				data, err := json.Marshal(map[string]any{"id": item.ID, "product_id": item.ProductID, "product_asset_id": item.ProductAssetID, "status": item.Status})
				return string(data), err
			},
		},
	}
}
