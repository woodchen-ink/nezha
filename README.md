<div align="center">
  <br>
  <img width="360" style="max-width:80%" src=".github/brand.svg" title="哪吒监控 Nezha Monitoring">
  <br>
  <small><i>LOGO designed by <a href="https://xio.ng" target="_blank">熊大</a> .</i></small>
  <br><br>
<img alt="GitHub release (with filter)" src="https://img.shields.io/github/v/release/nezhahq/nezha?color=brightgreen&style=for-the-badge&logo=github&label=Dashboard">&nbsp;<img src="https://img.shields.io/github/v/release/nezhahq/agent?color=brightgreen&label=Agent&style=for-the-badge&logo=github">&nbsp;<img src="https://img.shields.io/github/actions/workflow/status/nezhahq/agent/agent.yml?label=Agent%20CI&logo=github&style=for-the-badge">&nbsp;<a href="https://hosted.weblate.org/engage/nezha/"><img src="https://img.shields.io/weblate/progress/nezha?color=brightgreen&label=Translated&style=for-the-badge&logo=weblate" alt="Translation status" /></a>
  <br>
  <br>
  <p>:trollface: <b>Nezha Monitoring: Self-hostable, lightweight, servers and websites monitoring and O&M tool.</b></p>
  <p>Supports <b>monitoring</b> system status, HTTP (SSL certificate change, upcoming expiration, expired), TCP, Ping and supports <b>push alerts</b>, run scheduled tasks and <b>web terminal</b>.</p>
</div>

\>> Telegram Channel: [哪吒监控（中文通知频道）](https://t.me/nezhanews)<br>
\>> Telegram Group: [Nezha Monitoring Global (English Only)](https://t.me/nezhamonitoring_global), [哪吒监控（中文群组）](https://t.me/nezhamonitoring)

\>> [Use Cases | 我们的用户](https://www.google.com/search?q=%22%E5%93%AA%E5%90%92%E7%9B%91%E6%8E%A7+Nezha+Monitoring%22) (Google) <br>


## User Guide

- [English](https://nezhahq.github.io/en_US/index.html)
- [中文文档](https://nezhahq.github.io/index.html)


## 使用示例

> 完整兼容官方配置, 所有新增功能通过环境变量开启, 可选. 

```yml
services:
  dashboard:
    image: ghcr.io/woodchen-ink/nezha:dev-15.1-c2ebdf7
    container_name: nezha-dashboard
    restart: always
    volumes:
      - nezha-data:/dashboard/data
    ports:
      - 18009:18009
    environment:
      - NZ_EXTRA_USER_THEME_REPOSITORY=woodchen-ink/nezha-liquidglass
      - NZ_EXTRA_USER_THEME_VERSION=v2.0.8
      - NZ_EXTRA_USER_THEME_PATH=nezha-liquidglass-dist
      - NZ_EXTRA_USER_THEME_DEFAULT=true
      - NZ_AUTOGROUPBYCOUNTRY=true
    networks:
      - dokploy-network
      - default
volumes:
  nezha-data: null
networks:
  dokploy-network:
    external: true
```

## 中文说明（本仓库改动）

### 1. Docker 发布策略调整

- 现在仅发布 Docker 镜像到 `ghcr.io`。
- 不再推送到阿里云镜像仓库。
- 不构建 `s390x` 平台镜像。

### 2. 新增运行时主题加载（环境变量）

容器启动时可额外拉取一个 GitHub 主题仓库的 `dist.zip`，并注册为可选前端主题。

- `NZ_EXTRA_USER_THEME_REPOSITORY`：主题仓库，支持 `owner/repo` 或 `https://github.com/owner/repo`
- `NZ_EXTRA_USER_THEME_VERSION`：Release 版本，如 `v2.0.4`
- `NZ_EXTRA_USER_THEME_PATH`：可选，主题目录名，默认 `<repo>-dist`
- `NZ_EXTRA_USER_THEME_NAME`：可选，显示名称，默认仓库名
- `NZ_EXTRA_USER_THEME_DEFAULT`：可选，`true` 时自动设为默认用户主题（等价设置 `NZ_USER_TEMPLATE`）

对应的 `frontend-templates.yaml` 配置示例：

```yaml
- path: "nezha-liquidglass-dist"
  name: "Nezha-LiquidGlass"
  repository: "https://github.com/woodchen-ink/nezha-liquidglass"
  author: "woodchen-ink"
  version: "v2.0.4"
```

示例：

```bash
docker run -d --name nezha \
  -p 8008:8008 \
  -v $(pwd)/data:/dashboard/data \
  -e NZ_EXTRA_USER_THEME_REPOSITORY=woodchen-ink/nezha-liquidglass \
  -e NZ_EXTRA_USER_THEME_VERSION=v2.0.4 \
  -e NZ_EXTRA_USER_THEME_PATH=nezha-liquidglass-dist \
  -e NZ_EXTRA_USER_THEME_DEFAULT=true \
  ghcr.io/woodchen-ink/nezha:latest
```

### 3. 自动根据服务器国家分组

开启后，Dashboard 会在收到服务器 GeoIP 国家码时，自动将服务器加入对应国家分组。

- 环境变量：`NZ_AUTOGROUPBYCOUNTRY=true`
- 分组命名：`[AUTO] Country: <国家码>`（例如 `US`、`SG`）
- 若分组不存在，会自动创建
- 若服务器国家发生变化，会自动迁移到新国家分组
- 服务器在自动国家分组中只保留一份关联

示例（docker）：

```bash
docker run -d --name nezha \
  -p 8008:8008 \
  -v $(pwd)/data:/dashboard/data \
  -e NZ_AUTO_GROUP_BY_COUNTRY=true \
  ghcr.io/woodchen-ink/nezha:latest
```

### 4. 从官方镜像切换到本仓库镜像

本仓库镜像地址：

- `ghcr.io/woodchen-ink/nezha`

如果你当前使用官方 GHCR 镜像 `ghcr.io/nezhahq/nezha`，直接替换镜像名即可。

`docker run` 示例：

```bash
# 官方
docker run ... ghcr.io/nezhahq/nezha:latest

# 切换后
docker run ... ghcr.io/woodchen-ink/nezha:latest(请看实际版本号, latest是占位符)
```

`docker-compose.yml` 示例：

```yaml
services:
  nezha:
    # image: ghcr.io/nezhahq/nezha:latest
    image: ghcr.io/woodchen-ink/nezha:latest(请看实际版本号, latest是占位符)
```
