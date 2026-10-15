package jobs

import (
	"errors"
	"github.com/tian1363/scriptagent/internal/storage"
	"os"
	"strings"
)

// AppendProductReference keeps existing facts intact and makes retries idempotent.
func (s *Store) AppendProductReference(owner, productID, content string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	p, err := s.GetUserProduct(owner, productID)
	if err != nil {
		return err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return errors.New("参考内容不能为空")
	}
	original, err := os.ReadFile(p.MDPath)
	if err != nil {
		return err
	}
	if strings.Contains(string(original), content) {
		return nil
	}
	next := strings.TrimRight(string(original), "\n") + "\n\n" + content + "\n"
	if len(next) > storage.MaxMarkdownBytes {
		return errors.New("产品文档空间不足")
	}
	return writeProductMarkdown(p.MDPath, next)
}
