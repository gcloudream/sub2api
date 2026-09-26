# 幻想家界面维护与升级

## 维护方式

官方项目：<https://github.com/Wei-Shaw/Sub2API>。保留官方发布标签；幻想家改动放在独立 Git 分支，不直接修改线上编译产物。当前基础版本为 `v0.2.8`，UI 提交为 `4a1921b`；后端与该官方标签完全一致。历史定制版保存在服务器的 `fantasy-custom-v0.2.5` 标签中。

主要定制入口：`frontend/src/styles/luxury.css`、`BrandPortal.vue`、`ThemeToggle.vue`、`HomeView.vue`、`AuthLayout.vue`、Tailwind 颜色配置、品牌图标和默认语言。业务页面只改样式；后续优先在这些入口维护品牌外观。首页定制仍可能与上游首页改动发生冲突，必须检查，不能保证所有升级都自动无冲突。

本机维护仓库：`fantasy-maintenance/source`；服务器副本：`/home/ubuntu/sub2api-maintenance/upstream`。当前并未创建 GitHub Fork。服务器仓库与本机副本都须保存提交；另外保留源码归档。数据库、密码、运行配置与备份不得进入 Git。

## 更新官方代码

1. 添加只读来源：`git remote add upstream https://github.com/Wei-Shaw/Sub2API.git`（已经存在时不要重复）。
2. 检查官方发布说明，获取目标标签，创建 `fantasy-ui-v新版本` 分支。当前仓库是浅克隆；若缺少共同祖先，先补足历史，直到 `git merge-base HEAD v新版本` 成功。不要使用 `--allow-unrelated-histories`，不要整文件选择旧版来解决后端冲突。
3. 在定制分支合并目标官方标签，处理界面冲突；运行 `git diff v新版本 -- backend/`，应无后端差异。审查依赖、数据库迁移、运行 Dockerfile 和入口脚本变化。
4. 提交完整源码后在有足够内存的工作站运行 `tools/fantasy/build-release.sh 新版本 /绝对路径/产物目录`。脚本依次执行 UI 测试、前端构建和 Linux amd64 交叉编译，并输出源码包、工具版本及校验和。

## 镜像与预览

- 禁止在 4GB 正式服务器上运行 Node/Go 编译或同时启动多个构建。服务器只做文件校验和镜像打包。
- 使用 `tools/fantasy/Dockerfile.artifact` 打包产物。`RUNTIME_IMAGE` 必须指定已验证的不可变镜像 ID/摘要；每次升级先比较官方运行依赖。v0.2.5 到 v0.2.8 的官方 Dockerfile 和入口脚本相同，因此本次可以复用已构建的官方 v0.2.5 运行环境，仅替换新版完整应用程序及官方资源。
- 采用带版本及提交号的镜像名，不用 `latest`。构建命令示例：`docker build --network=none --build-arg RUNTIME_IMAGE=已验证镜像ID --build-arg VERSION=版本 --build-arg COMMIT=提交号 -t fantasy-sub2api:ui-版本-提交号 产物目录`。
- 先备份 `/home/ubuntu/fantasy-preview`，只替换预览应用容器。预览使用独立数据库、缓存及 8081 端口。
- 验证健康检查、日志中的迁移结果、登录/注册页面、品牌样式与主题切换。登录后检查用户及管理员主要页面；无法实际验证的路径在回执中注明。

## 正式切换与回退

1. 在 `/home/ubuntu/sub2api-maintenance/backups/时间戳` 保存正式和预览数据库的 `pg_dump -Fc`、配置与应用数据、旧镜像归档、SHA256 校验和，目录权限 700、文件 600。使用 `pg_restore --list` 检查备份能被解析；这不等于完整恢复演练。
2. 记录旧镜像 ID、Compose 配置和版本。不要操作数据库/缓存容器来更新前端。
3. 预览验收后，仅将正式 Compose 的应用镜像改为经过验证的候选镜像，重建应用服务。
4. 回读实际镜像 ID、公开版本、HTTP 健康检查、容器健康状态、应用日志。留存不含凭据的发布回执。
5. 如需回退，先核对迁移兼容性：仅有兼容迁移时可以直接恢复旧镜像；不兼容迁移需要停写、导出升级后数据，再恢复升级前数据库及配置。恢复数据库会丢失备份后写入，不能自动执行。

## 2026-09-26 构建故障

在 4GB 云主机上并行编译曾耗尽 2GB 交换空间，负载超过 40，正式站健康检查超时。已停止构建并正常重启；恢复后应用、PostgreSQL、Redis 健康，公网健康检查返回 200。Docker Engine 为 29.1.3，安装 Buildx 0.30.1 即可解决原先缺少构建组件的问题，不需要更换 Engine 软件源。后续构建全部移到工作站。
