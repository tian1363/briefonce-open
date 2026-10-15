package videogen

import (
	"errors"
	"strings"
)

const Endpoint = "https://zexapi.com/v1/videos"

type Model struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	FirstLast     bool     `json:"first_last"`
	Ratios        []string `json:"ratios"`
	Resolutions   []string `json:"resolutions"`
	MinSeconds    int      `json:"min_seconds"`
	MaxSeconds    int      `json:"max_seconds"`
	MaxReferences int      `json:"max_references"`
}

var Models = []Model{
	{ID: "seedance-2.0", Name: "Seedance 2.0", FirstLast: true},
	{ID: "seedance-2.0-mini", Name: "Seedance 2.0 Mini", FirstLast: true},
	{ID: "minimax-h3", Name: "MiniMax H3"},
	{ID: "minimax-h3-max", Name: "MiniMax H3 Max"},
}

func FindModel(id string) (Model, bool) {
	for _, m := range Models {
		if m.ID == id {
			m.Ratios = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}
			m.Resolutions = []string{"default", "720p", "1080p"}
			m.MinSeconds = 4
			m.MaxSeconds = 15
			m.MaxReferences = 9
			return m, true
		}
	}
	return Model{}, false
}
func Validate(model, resolution, ratio string, duration int, mode string) error {
	m, ok := FindModel(model)
	if !ok {
		return errors.New("请选择已接入的章鱼哥视频模型")
	}
	if duration < 4 || duration > 15 {
		return errors.New("视频时长需为 4 到 15 秒")
	}
	if !contains(m.Ratios, ratio) {
		return errors.New("不支持该视频画幅")
	}
	if !contains(m.Resolutions, strings.ToLower(resolution)) {
		return errors.New("请选择模型默认分辨率、720p 或 1080p")
	}
	if (mode == "first_frame" || mode == "first_last") && !m.FirstLast {
		return errors.New("MiniMax 请使用参考内容模式，不支持指定首尾帧")
	}
	return nil
}
func contains(values []string, v string) bool {
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}
