# BriefOnce 文档中心

> 最近维护：2026-10-15 · 事实基线：当前 `main` 分支

## 阅读顺序

1. [根目录 README](../README.md)：产品现状、启动方式和安全边界。
2. [产品地址清单](product-addresses.md)：前台、开发者模式、API、Langfuse 与仓库地址。
3. [需求文档](requirements.md)：当前产品范围、用户流程和验收约束。
4. [技术设计](technical-design.md)：架构、数据、模型和接口设计。
5. [Agent Runtime 与 Harness](agent-runtime-harness.md)：Workflow、Agent Loop、可恢复性和观测。

## 专题文档

| 文档 | 内容 | 当前状态 |
| --- | --- | --- |
| [产品使用手册](product-user-manual.md) | 资料管理、两个工具、三个工作流、作品与常见问题 | 当前用户操作指南 |
| [GitHub 项目展示维护](github-discoverability.md) | 简介、Topics、README 与演示素材维护 | 持续维护 |
| [版本更新日志](CHANGELOG.md) | 新功能、安全变化、升级步骤与已知限制 | 随版本维护 |
| [模型能力](model-capabilities.md) | 能力路由、托管/BYOK、默认模型 | 已按当前设置页更新 |
| [电商工具箱](ecommerce-toolbox.md) | 两个工具、三个工作流与作品复用 | 当前可用 |
| [电商姿势库与详情页](ecommerce-pose-and-details.md) | 模特姿势、电商图、多平台详情图导出与规则来源 | 需配置图片编辑模型 |
| [邀请测试部署](invite-deployment.md) | 唯一管理员、邀请码、安全配置与备份 | 当前 Beta 基线 |
| [自托管部署](managed-deployment.md) | 环境配置、访问控制、持久化与备份 | 已实现，需部署配置 |
| [Token 优化](token-optimization.md) | 摘要、上下文、工具调用与缓存策略 | 已实施策略 |
| [Langfuse](langfuse-observability.md) | OTLP 接入、隐私与排障 | 可选能力 |
| [UGC 视频生成](ugc-video-generation.md) | 产品参考图、异步生成、预览与下载 | 当前可用 |
| [品牌规范](../brand-spec.md) | 色彩、层级与交互语言 | 当前 UI 基线 |

## 状态标签

- **当前可用**：已存在于 `main`，且有对应前后端实现。
- **需配置**：代码已存在，但需要密钥、环境变量或外部服务。
- **未合并**：其他分支存在实现，当前产品不可使用。
- **规划中**：只有设计或需求，不应出现在用户可用能力清单中。

## 维护规则

- 页面、API、模型、数据结构或环境变量变化时，同一提交更新相应文档。
- `internal/web/routes.go` 是 API 地址事实来源；`cmd/server/main.go` 是运行配置事实来源。
- `web/app/src/App.jsx` 是当前页面和文案事实来源。
- 已下线功能不作为当前能力保留在公开说明中；内部历史方案单独保存在私密资料库。
