package creative

import (
	"context"
	"strings"
	"testing"
)

func TestParsePageMetadataPrefersOpenGraph(t *testing.T) {
	document := `<html><head>
		<title>fallback title</title>
		<meta name="description" content="public summary">
		<meta property="og:title" content="note title">
		<meta property="og:image" content="https://sns-img.example/cover.jpg">
		<meta property="og:image" content="https://sns-img.example/cover.jpg">
	</head></html>`
	title, description, images := parsePageMetadata(document)
	if title != "note title" || description != "public summary" {
		t.Fatalf("unexpected metadata: %q %q", title, description)
	}
	if len(images) != 1 || images[0] != "https://sns-img.example/cover.jpg" {
		t.Fatalf("unexpected images: %#v", images)
	}
}

func TestParsePageContentsKeepsOnlyVisibleArticleText(t *testing.T) {
	document := `<html><head><meta property="og:video" content="https://example.com/video.mp4"></head><body><nav>navigation</nav><article><h1>开箱体验</h1><p>三步展示产品</p><script>ignore me</script></article></body></html>`
	_, _, _, video, pageText := parsePageContents(document)
	if video != "https://example.com/video.mp4" || !strings.Contains(pageText, "开箱体验") || !strings.Contains(pageText, "三步展示产品") || strings.Contains(pageText, "navigation") || strings.Contains(pageText, "ignore me") {
		t.Fatalf("unexpected content: video=%q text=%q", video, pageText)
	}
}

func TestValidateXiaohongshuURLRejectsOtherHostsBeforeFetch(t *testing.T) {
	err := validateXiaohongshuURL(context.Background(), "https://example.com/note/123")
	if err == nil || !strings.Contains(err.Error(), "仅支持") {
		t.Fatalf("expected unsupported host error, got %v", err)
	}
}

func TestValidateXiaohongshuURLRequiresHTTPS(t *testing.T) {
	err := validateXiaohongshuURL(context.Background(), "http://www.xiaohongshu.com/explore/123")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS error, got %v", err)
	}
}

func TestValidateXiaohongshuURLRejectsSearchResults(t *testing.T) {
	url := "https://www.xiaohongshu.com/search_result_ai?keyword=codex%25E7%25BC%25BA%25E7%2582%25B9&source=web_explore_feed"
	err := validateXiaohongshuURL(context.Background(), url)
	if err == nil || !strings.Contains(err.Error(), "搜索结果页") {
		t.Fatalf("expected single-note guidance, got %v", err)
	}
}

func TestHasNoteMetadataRejectsGenericAppShell(t *testing.T) {
	snapshot := XiaohongshuSnapshot{
		Title:  "小红书 - 你的生活兴趣社区",
		Images: []string{"https://sns-img.example/generic.png"},
	}
	if hasNoteMetadata(snapshot) {
		t.Fatal("generic app shell must not be treated as parsed note content")
	}
}

func TestHasNoteMetadataRejectsSearchPageTitle(t *testing.T) {
	if hasNoteMetadata(XiaohongshuSnapshot{Title: "codex缺点 - 小红书搜索"}) {
		t.Fatal("search title is not note content")
	}
}

func TestParseXiaohongshuInputFromShareMessage(t *testing.T) {
	input := "47 【lululemon 之 想再买一个 gym 包 - Jerry 大魔王 | 小红书 - 你的生活兴趣社区】 😆 fiSPnwBcQwXpCRW 😆 https://www.xiaohongshu.com/discovery/item/69901f19000000000c036f72?xsec_token=abc&xsec_source=pc_user，复制链接打开"
	link, heading := parseXiaohongshuInput(input)
	if link != "https://www.xiaohongshu.com/discovery/item/69901f19000000000c036f72?xsec_token=abc&xsec_source=pc_user" {
		t.Fatalf("unexpected link: %q", link)
	}
	if heading != "lululemon 之 想再买一个 gym 包 - Jerry 大魔王 | 小红书 - 你的生活兴趣社区" {
		t.Fatalf("unexpected heading: %q", heading)
	}
}

func TestParseXiaohongshuInputPlainURL(t *testing.T) {
	link, heading := parseXiaohongshuInput("https://www.xiaohongshu.com/explore/abc?xsec_token=one，")
	if link != "https://www.xiaohongshu.com/explore/abc?xsec_token=one" || heading != "" {
		t.Fatalf("unexpected parsed input: %q %q", link, heading)
	}
}
