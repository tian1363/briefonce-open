package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tian1363/scriptagent/internal/storage"
)

// Product facts and generated visual observations have separate documents.
func (s *Store) WriteProductDocument(userID, productID, content string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	product, err := s.GetUserProduct(userID, productID)
	if err != nil {
		return err
	}
	content = strings.TrimSpace(content)
	if len(content)+1 > storage.MaxMarkdownBytes {
		return errors.New("产品文档空间不足")
	}
	return writeProductMarkdown(product.MDPath, content+"\n")
}

var observationBlock = regexp.MustCompile(`(?s)<!-- asset-extraction:([^\n]+) -->\n?(.*?)\n?<!-- /asset-extraction -->`)
var extractionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func productObservationPath(productPath, productID, extractionID string) (string, error) {
	if !extractionIDPattern.MatchString(productID) || !extractionIDPattern.MatchString(extractionID) {
		return "", errors.New("资产解析 ID 无效")
	}
	return filepath.Join(filepath.Dir(productPath), "observations", productID, extractionID+".md"), nil
}

func (s *Store) WriteProductObservation(userID, productID, extractionID, section string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	product, err := s.GetUserProduct(userID, productID)
	if err != nil {
		return err
	}
	path, err := productObservationPath(product.MDPath, productID, extractionID)
	if err != nil {
		return err
	}
	section = strings.TrimSpace(section)
	if len(section)+1 > storage.MaxMarkdownBytes {
		return errors.New("资产解析文档过大")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return writeProductMarkdown(path, section+"\n")
}

func (s *Store) ReadProductObservation(userID, productID, extractionID string) (string, error) {
	product, err := s.GetUserProduct(userID, productID)
	if err != nil {
		return "", err
	}
	path, err := productObservationPath(product.MDPath, productID, extractionID)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	return string(data), err
}

// MoveLegacyProductObservations extracts only machine-delimited legacy blocks.
func (s *Store) MoveLegacyProductObservations(userID, productID string) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	product, err := s.GetUserProduct(userID, productID)
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(product.MDPath)
	if err != nil {
		return 0, err
	}
	matches := observationBlock.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return 0, nil
	}
	for _, match := range matches {
		path, err := productObservationPath(product.MDPath, productID, match[1])
		if err != nil {
			return 0, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return 0, err
		}
		if err := writeProductMarkdown(path, strings.TrimSpace(match[2])+"\n"); err != nil {
			return 0, err
		}
	}
	clean := strings.TrimSpace(observationBlock.ReplaceAllString(string(data), "")) + "\n"
	if err := writeProductMarkdown(product.MDPath, clean); err != nil {
		return 0, err
	}
	return len(matches), nil
}

func writeProductMarkdown(path, content string) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".product-update-*.md")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = temp.WriteString(content); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
