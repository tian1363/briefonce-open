package storage

import (
	"bytes"
	"errors"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
)

func (s *LocalStore) SaveProductImage(data []byte) (string, error) {
	if len(data) == 0 || len(data) > 32<<20 {
		return "", errors.New("商品图需为 32MB 以内的 PNG 或 JPG")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg" && format != "webp") {
		return "", errors.New("图片服务未返回有效的 PNG 或 JPG 商品图")
	}
	if cfg.Width < 384 || cfg.Height < 384 || cfg.Width > 4096 || cfg.Height > 4096 {
		return "", errors.New("生成商品图宽高需在 384–4096 像素之间")
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return "", errors.New("生成商品图数据不完整")
	}
	dir := filepath.Join(s.root, "product-images")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "product-*."+format)
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		return "", errors.New("商品图保存失败")
	}
	return path, nil
}
