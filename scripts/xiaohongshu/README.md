# 小红书连接助手本地验证构建

上游 https://github.com/xpzouying/xiaohongshu-mcp ，Apache-2.0。2026-10-15 下载的 main 源码压缩包 SHA256：`a7140999c95ce15e098a821866ce38fb9e697452c87997c3b8981ab3a3df2caa`。

`ordinary-chrome.patch` 仅用于此次 macOS 本地验证：使用已安装的普通 Chrome，跳过上游指纹浏览器下载与安装，不启用 stealth 或代理。主产品只通过受认证的读取与登录接口访问它。发布前需要选择固定上游版本并跨平台验证，当前适配不等同于已完成 SaaS 部署。

源码解压后执行：

```sh
patch -p1 < /path/to/scriptagent/scripts/xiaohongshu/ordinary-chrome.patch
go build -o collector .
```

每位用户需要独立 cookie 路径、端口和 AUTH_TOKEN；显式设置 COOKIES_PATH，禁止使用上游默认共享 /tmp/cookies.json。umask 077，目录权限 0700。绑定配置和产品交互见 docs/ecommerce-toolbox.md。
