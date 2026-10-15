# 一起改进 BriefOnce

欢迎报告问题、提交创作案例和改进代码。先在 Issue 或 Discussion 中描述使用场景，方便确认问题与复现路径。

## 报告问题

请说明使用的工具或工作流、操作步骤、预期结果和实际表现。附上经过脱敏的截图，以及浏览器和部署方式。不要提交账号密码、模型密钥、运行数据库或客户素材。

安全问题请按 [SECURITY.md](SECURITY.md) 私密报告。

## 分享案例

用你有权公开的素材说明“输入 → 操作 → 结果”。说明哪些图片是真实生成结果，哪些只是示意；提供必要的商品事实与模型参数即可。

## 提交代码

```bash
go test ./...
node --test web/app/src/*.test.js
npm --prefix web/app run build
python3 scripts/check-public-content.py
```

Pull Request 请写清问题、改变后的行为和验证方式。界面变化附截图；涉及外部模型时，说明哪些部分使用了模拟服务验证。
