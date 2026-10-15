# BriefOnce 官网

BriefOnce 的开源 Beta 官网，位于主仓库的 `web/website`。首页按“产品资料 → 创意对话 → 工具与工作流”介绍工作方式。工具为视频素材提取、创意解读；工作流为模特姿势、挂拍图精修、商品详情长图。详情图由 AI 结合实际参考图策划并出图；姿势按选中数量生成，不对商品图做排列组合。实际案例与创意示意分别标注，参考图复刻案例为开发预览。

## 本地运行

需要 Node.js 20.19+ 或 22.12+。

```bash
npm ci
npm run dev
```

运行 `npm run test:capabilities` 核对官网是否覆盖当前工具与工作流，并防止重新宣传已下掉入口的功能。产品入口、场景清单及公开 Skill 源码变化也会触发官网发布检查。运行 `npm run build` 生成站点产物，运行 `npm run test:sites` 检查静态资源与路由回退。静态站点配置位于 `.openai/hosting.json`。

本目录只包含官网，BriefOnce 产品代码和运行说明见[主仓库 README](../../README.md)。

## 公网官网

GitHub Pages 从 `main` 自动发布到 <https://tian1363.github.io/briefonce-open/>。发布流程见 [pages.yml](../../.github/workflows/pages.yml)。官网的“申请内测”按钮打开公开 GitHub Issue 表单，不收集私人联系方式。

## 公开 Skill 页面

首页的“公开 Skill”区块链接到 `/briefonce-open/skills/`。构建脚本 `scripts/generate-public-skills.mjs` 从 `internal/chat/service.go` 的内置定义生成四个可直接分享的静态页面，包括「开头钩子复刻」，附完整工作流、输入示例和源码入口；只发布仓库里已有的内置 Skill，不读取用户产品资料或自定义 Skill。生成结果位于 `public/skills/`，同时生成 `sitemap.xml` 和 `robots.txt`。运行 `npm run test:public-skills` 检查页面与来源同步。

页面可供 Google 等搜索引擎发现，但是否收录由搜索引擎决定。发布后可在 Google Search Console 提交站点地图：<https://tian1363.github.io/briefonce-open/sitemap.xml>。

首页顶部和公开 Skill 内容后设有 GitHub Star 入口；Skill 列表与详情页也可返回项目仓库。入口只打开公开仓库，收藏动作由访客在 GitHub 完成，不展示未经核实的 Star 数。

## Google 搜索

中文首页和英文首页 `/en/` 均在构建时生成完整 HTML，并设置独立 canonical、双向 hreflang、分享信息与面包屑。构建产物的 `sitemap.xml` 从实际静态页面生成，包含真实案例图片及来源修改时间。运行 `npm run test:seo` 校验。

Google Search Console 使用网址前缀属性 `https://tian1363.github.io/briefonce-open/`。选择 HTML 标记验证后，将标记的 `content` 值设置为仓库 Actions 变量 `GOOGLE_SITE_VERIFICATION`，再重新运行 Publish website。构建不会生成虚假的验证标签；配置后，验证标记保留在首页源码中。

所有权验证完成后，提交 `https://tian1363.github.io/briefonce-open/sitemap.xml`，并对首页、英文首页与模特图案例申请编入索引。是否完成验证与收录，应以 Search Console 的实际状态为准。GitHub 项目站点的 `/briefonce-open/robots.txt` 不属于域名根目录，不能代替在 Search Console 提交站点地图。

参考：[Google 所有权验证](https://support.google.com/webmasters/answer/9008080)、[多语言页面](https://developers.google.com/search/docs/specialty/international/localized-versions)、[站点地图](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap)。
