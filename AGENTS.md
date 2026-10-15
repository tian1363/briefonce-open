# BriefOnce 工作约定

- 以包含本文件的 Git 仓库作为实际源码目录，不修改外部同步镜像。
- 修改前读取 README 与相关文档、检查工作区状态，保留用户和其他任务的变更。
- 前端遵循 `docs/frontend-design-guidelines.md`；工具箱参考 `docs/ecommerce-toolbox.md`。
- 公开文档只描述当前可见功能。内部运营、商业资料、真实凭证及管理截图保存在仓库外的私密目录。
- 发布前运行 `python3 scripts/check-public-content.py`，不得将运行数据库、环境密钥和用户素材加入 Git。
