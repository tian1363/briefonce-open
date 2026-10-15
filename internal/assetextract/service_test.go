package assetextract

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tian1363/scriptagent/internal/jobs"
	"github.com/tian1363/scriptagent/internal/storage"
)

func TestExtractFrameAtChosenTime(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe unavailable")
	}
	path := filepath.Join(t.TempDir(), "reference.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=red:s=320x240:d=2", "-pix_fmt", "yuv420p", path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make video: %v: %s", err, output)
	}
	duration, err := probeDuration(context.Background(), path)
	if err != nil || duration < 1.5 {
		t.Fatalf("duration = %v, %v", duration, err)
	}
	frame, err := extractFrame(context.Background(), path, 1)
	if err != nil {
		t.Fatal(err)
	}
	image, err := jpeg.DecodeConfig(bytes.NewReader(frame))
	if err != nil || image.Width != 1024 || image.Height != 1024 {
		t.Fatalf("frame dimensions = %+v, %v", image, err)
	}
	store, err := jobs.OpenStore(filepath.Join(t.TempDir(), "standalone.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser(jobs.CreateUserInput{Email: "standalone@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	service := New(store, storage.NewLocalStore(t.TempDir()), false)
	_, err = service.Start(context.Background(), user.ID, Input{Kind: "product", SourcePath: path, AtSeconds: 1})
	if err == nil || !strings.Contains(err.Error(), "图片编辑模型") {
		t.Fatalf("expected model configuration error after productless source validation, got %v", err)
	}
}

func TestImportCompletedExtractionToOwnedProduct(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "extract-owner@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(jobs.CreateUserInput{Email: "extract-other@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	productPath := filepath.Join(dir, "product.md")
	if err := os.WriteFile(productPath, []byte("# Product\n"), 0600); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "Product", MDPath: productPath, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(owner.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	files := storage.NewLocalStore(filepath.Join(dir, "uploads"))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 512, 512)), nil); err != nil {
		t.Fatal(err)
	}
	imagePath, err := files.SaveCharacterImage(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateAssetExtraction(jobs.AssetExtraction{UserID: owner.ID, Kind: "product", Label: "Bag", SourceType: "upload", SourceID: "video", AtSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteAssetExtraction(owner.ID, item.ID, "", imagePath, "", "", "Visible blue bag"); err != nil {
		t.Fatal(err)
	}
	service := New(store, files, false)
	if _, err := service.Import(context.Background(), other.ID, item.ID, product.ID); err == nil {
		t.Fatal("other user imported extraction")
	}
	if _, err := service.Import(context.Background(), owner.ID, item.ID, "missing"); err == nil {
		t.Fatal("missing product accepted")
	}
	imported, err := service.Import(context.Background(), owner.ID, item.ID, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ProductID != product.ID || imported.ProductAssetID == "" {
		t.Fatalf("imported = %+v", imported)
	}
	assets, err := store.ListProductAssets(product.ID)
	if err != nil || len(assets) != 1 || assets[0].Path == imagePath {
		t.Fatalf("assets = %+v, %v", assets, err)
	}
	content, err := os.ReadFile(productPath)
	if err != nil || strings.Contains(string(content), "Visible blue bag") {
		t.Fatalf("product facts were changed = %s, %v", content, err)
	}
	observation, err := store.ReadProductObservation(owner.ID, product.ID, item.ID)
	if err != nil || !strings.Contains(observation, "Visible blue bag") {
		t.Fatalf("observation = %s, %v", observation, err)
	}
	if _, err := service.Import(context.Background(), owner.ID, item.ID, product.ID); err != nil {
		t.Fatal(err)
	}
	assets, _ = store.ListProductAssets(product.ID)
	if len(assets) != 1 {
		t.Fatalf("duplicate assets = %d", len(assets))
	}
	otherProductPath := filepath.Join(dir, "other-product.md")
	if err := os.WriteFile(otherProductPath, []byte("# Other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	otherProduct, err := store.CreateProduct(jobs.CreateProductInput{Title: "Other", MDPath: otherProductPath, MDName: "other-product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(owner.ID, "product", otherProduct.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Import(context.Background(), owner.ID, item.ID, otherProduct.ID); err == nil {
		t.Fatal("second destination accepted")
	}
}

func TestImportAllObservedViews(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "views@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "product.md")
	if err := os.WriteFile(doc, []byte("# Product\n"), 0600); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "Jacket", MDPath: doc, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(owner.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	files := storage.NewLocalStore(filepath.Join(dir, "uploads"))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 512, 512)), nil); err != nil {
		t.Fatal(err)
	}
	frontPath, err := files.SaveCharacterImage(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	sidePath, err := files.SaveCharacterImage(encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateAssetExtraction(jobs.AssetExtraction{UserID: owner.ID, Kind: "product", Label: "Jacket", SourceType: "upload", SourceID: "video"})
	if err != nil {
		t.Fatal(err)
	}
	variants := []jobs.AssetExtractionVariant{{View: "main", AtSeconds: 1, FramePath: frontPath, ImagePath: frontPath}, {View: "side", AtSeconds: 3, FramePath: sidePath, ImagePath: sidePath}}
	if err := store.CompleteAssetExtractionWithVariants(owner.ID, item.ID, frontPath, frontPath, "", "", "Two observed views", variants); err != nil {
		t.Fatal(err)
	}
	service := New(store, files, false)
	imported, err := service.Import(context.Background(), owner.ID, item.ID, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Variants) != 2 || imported.Variants[0].ProductAssetID == "" || imported.Variants[1].ProductAssetID == "" {
		t.Fatalf("variants not imported: %+v", imported.Variants)
	}
	assets, err := store.ListProductAssets(product.ID)
	if err != nil || len(assets) != 2 {
		t.Fatalf("assets = %+v, %v", assets, err)
	}
	content, err := os.ReadFile(doc)
	if err != nil || strings.Contains(string(content), "侧面图") {
		t.Fatalf("product facts were changed = %s, %v", content, err)
	}
	observation, err := store.ReadProductObservation(owner.ID, product.ID, item.ID)
	if err != nil || strings.TrimSpace(observation) != "Two observed views" || strings.Contains(observation, "产品素材 ID") {
		t.Fatalf("observation = %s, %v", observation, err)
	}
}

func TestImportActionAsVideoAsset(t *testing.T) {
	dir := t.TempDir()
	store, err := jobs.OpenStore(filepath.Join(dir, "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	owner, err := store.CreateUser(jobs.CreateUserInput{Email: "action@example.com", Role: "member", Status: "active", PasswordHash: "unused"})
	if err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "product.md")
	if err := os.WriteFile(doc, []byte("# Product\n"), 0600); err != nil {
		t.Fatal(err)
	}
	product, err := store.CreateProduct(jobs.CreateProductInput{Title: "Product", MDPath: doc, MDName: "product.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimResource(owner.ID, "product", product.ID); err != nil {
		t.Fatal(err)
	}
	files := storage.NewLocalStore(filepath.Join(dir, "uploads"))
	var imageData bytes.Buffer
	if err := jpeg.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 512, 512)), nil); err != nil {
		t.Fatal(err)
	}
	framePath, err := files.SaveCharacterImage(imageData.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	clipPath, err := files.SaveGeneratedVideo("test-action", bytes.NewReader([]byte("\x00\x00\x00\x18ftypisom00000000")))
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateAssetExtraction(jobs.AssetExtraction{UserID: owner.ID, Kind: "action", Label: "Turn", SourceType: "upload", SourceID: "video"})
	if err != nil {
		t.Fatal(err)
	}
	variant := jobs.AssetExtractionVariant{View: "main", AtSeconds: 2, FramePath: framePath, ImagePath: framePath, VideoPath: clipPath}
	if err := store.CompleteAssetExtractionWithVariants(owner.ID, item.ID, framePath, framePath, "", "", "Observed turn", []jobs.AssetExtractionVariant{variant}); err != nil {
		t.Fatal(err)
	}
	imported, err := New(store, files, false).Import(context.Background(), owner.ID, item.ID, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ProductAssetID == "" {
		t.Fatal("missing imported asset")
	}
	assets, err := store.ListProductAssets(product.ID)
	if err != nil || len(assets) != 1 || assets[0].Kind != "video" || !strings.HasSuffix(assets[0].OriginalName, ".mp4") {
		t.Fatalf("assets = %+v, %v", assets, err)
	}
}
