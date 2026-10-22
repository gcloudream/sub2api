# 幻想家界面维护与升级

## 维护方式

官方项目：<https://github.com/Wei-Shaw/Sub2API>。保留官方发布标签；幻想家改动放在独立 Git 分支，不直接修改线上编译产物。当前仓库基础版本升级为正式发布 `v0.2.17`（`060d0160417782cd962a0c7f460f6cd4565d4916`），保留幻想家的品牌界面、移动端布局、精确金额充值赠送、付费盲盒和 Codex 使用文档。后端含经过测试的充值定制，不再要求与官方标签完全相同。历史定制版保存在服务器的 `fantasy-custom-v0.2.5` 标签中。

界面定制入口：`frontend/src/styles/luxury.css`、`BrandPortal.vue`、`ThemeToggle.vue`、`HomeView.vue`、`AuthLayout.vue`、Tailwind 颜色配置、品牌图标和默认语言。充值定制主要位于 `backend/internal/service/payment_amounts.go`、`payment_blind_box.go`、订单及履约链路，以及前端 payment 组件和页面。升级时必须同时检查上游和幻想家的订单金额、优惠快照、退款与盲盒逻辑，避免重复赠送或用新配置重算旧订单。

GitHub Fork：<https://github.com/gcloudream/sub2api/tree/fantasy-ui>，默认定制分支为 `fantasy-ui`，`upstream` 指向官方仓库。在独立本地工作区进行合并，并按实际发布回执核对线上镜像、二进制哈希及对应源码；不能把本机或服务器任一旧 checkout 的 HEAD 直接当作线上版本。服务器 `/home/ubuntu/sub2api-maintenance/upstream` 可能落后于已发布产物。保留原始提交和源码归档，数据库、密码、运行配置与备份不得进入 Git。

## 更新官方代码

1. 添加只读来源：`git remote add upstream https://github.com/Wei-Shaw/Sub2API.git`（已经存在时不要重复）。
2. 检查官方发布说明，获取目标标签，创建 `fantasy-ui-v新版本` 分支。若维护仓库为浅克隆或缺少共同祖先，先补足历史，直到 `git merge-base HEAD v新版本` 成功。不要使用 `--allow-unrelated-histories`，不要整文件选择旧版来解决后端冲突。
3. 在定制分支合并目标官方标签，按语义处理冲突。检查 `git diff v新版本 -- backend/ frontend/`，逐项核对保留的定制行为及测试，不能把后端差异一律删除。审查依赖、数据库迁移、运行 Dockerfile 和入口脚本变化。普通分支 push 与正式部署分开，避免通过发布标签触发额外发布流程。
4. 提交完整源码后在有足够内存的工作站运行 `tools/fantasy/build-release.sh 新版本 /绝对路径/产物目录`。脚本依次执行 UI 测试、前端构建和 Linux amd64 交叉编译，并输出源码包、工具版本及校验和。

## 镜像与预览

- 禁止在 4GB 正式服务器上运行 Node/Go 编译或同时启动多个构建。服务器只做文件校验和镜像打包。
- 使用 `tools/fantasy/Dockerfile.artifact` 打包产物。`RUNTIME_IMAGE` 必须指定已验证的运行镜像；使用本地标签时，打包前必须核对其实际镜像 ID 与发布回执相同；每次升级都先比较官方运行依赖。历史上 v0.2.5 到 v0.2.8 的运行文件相同，不代表 v0.2.17 仍可复用同一运行环境。
- 采用带版本及提交号的镜像名，不用 `latest`。构建命令示例：`docker build --network=none --build-arg RUNTIME_IMAGE=已验证镜像ID --build-arg VERSION=版本 --build-arg COMMIT=提交号 -t fantasy-sub2api:ui-版本-提交号 产物目录`。
- 先备份 `/home/ubuntu/fantasy-preview`，只替换预览应用容器。预览使用独立数据库、缓存及 8081 端口。
- 验证健康检查、日志中的迁移结果、登录/注册页面、品牌样式与主题切换。登录后检查用户及管理员主要页面；无法实际验证的路径在回执中注明。

## 正式切换与回退

1. 在 `/home/ubuntu/sub2api-maintenance/backups/时间戳` 保存正式和预览数据库的 `pg_dump -Fc`、配置与应用数据、旧镜像归档、SHA256 校验和，目录权限 700、文件 600。使用 `pg_restore --list` 检查备份能被解析；这不等于完整恢复演练。
2. 记录旧镜像 ID、Compose 配置和版本。不要操作数据库/缓存容器来更新前端。
3. 预览验收后，仅将正式 Compose 的应用镜像改为经过验证的候选镜像，重建应用服务。
4. 回读实际镜像 ID、公开版本、HTTP 健康检查、容器健康状态、应用日志。留存不含凭据的发布回执。
5. 如需回退，先核对迁移、持久化业务状态和审计记录的兼容性；三者都兼容时才可恢复旧镜像。不兼容时需要停写、导出升级后数据，制定恢复数据库及配置的方案。恢复数据库会丢失备份后写入，不能自动执行。

## 2026-09-26 构建故障

在 4GB 云主机上并行编译曾耗尽 2GB 交换空间，负载超过 40，正式站健康检查超时。已停止构建并正常重启；恢复后应用、PostgreSQL、Redis 健康，公网健康检查返回 200。Docker Engine 为 29.1.3，安装 Buildx 0.30.1 即可解决原先缺少构建组件的问题，不需要更换 Engine 软件源。后续构建全部移到工作站。

## 2026-10-22 仓库合并

本次仅合并官方 `v0.2.17` 并更新 GitHub 源码，不构建或部署正式服务器，也不执行数据库迁移。线上应用仍为 `e4146bba6c7eb077b2e3b3ec9d8cdb07f5d2a8f5` 对应的 v0.2.8 定制发布；将来上线候选版本时另行完成预览、迁移兼容性、备份及功能验收。

官方发布标签中的 `backend/cmd/server/VERSION` 仍为上一个版本，官方工作流在发布后才同步默认分支版本文件。本次将该文件同步为 `0.2.17`，让没有官方标签引用的 Fork 源码构建也能正确标识基础版本；不向 Fork 推送官方 `v*` 标签。

官方 v0.2.17 停止通过 ChatGPT OAuth / SetupToken 账号调度 `gpt-5.5`，API Key 账号不受影响。将来部署前应按官方发布说明检查模型映射。充值兼容需保留幻想家精确金额档位与付费盲盒，并覆盖官方可配置优惠、订单快照和退款回归。

优惠规则：官方 `recharge_bonus_tiers` 非空时优先，包含 0% 档位以及未命中阈值时都不回退或叠加幻想家赠送；未配置官方阶梯时，`recharge_fixed_bonus_tiers` 提供 20/50/100/200/500 的精确金额档位，附近金额不享受该档位。盲盒和订阅绕过两套优惠。新订单冻结 `amount`、`pay_amount` 和 `bonus_amount`，履约及退款使用原始快照；旧订单的 `bonus_amount` 默认为 0，不按现行配置回填历史赠送。

前端充值报价使用十进制乘法、加法及折扣计算，按与 Go `decimal.Round` 一致的半入远零规则舍入，避免浮点乘法或固定 epsilon 让半分边界差 0.01。相关 TypeScript 和 Go golden 回归纳入构建检查。

v0.2.17 的待定退款会保留余额或订阅扣减，并写入 `DeductionHeld` 状态；旧 v0.2.8 的恢复逻辑不理解该状态。部署或降级前必须检查并收敛 `REFUND_PENDING` / `REFUNDING` 订单及审计记录。新版产生此类状态后，不能仅凭 SQL 迁移兼容就换回旧二进制，避免重复扣减或漏归还。

## 历史 v0.2.8 发布

0.2.8 应用镜像：`fantasy-sub2api:ui-v0.2.8-f6fb3be`，源码提交 `f6fb3bef635b0ff6fca3c0887b87eeb942a6df8a`。运行环境基于本地官方镜像 `sub2api:official-v0.2.5`，已核对 ID 为 `sha256:12b22980426b197703815d05c526382cf7593890270a647302a913949ca3d79f`，两个官方版本的 Dockerfile 和入口脚本无差异。

本机完成 83 项 UI 测试、3 项 i18n 构建检查、TypeScript 检查、前端构建、Go 1.27 Linux amd64 编译。预览站完成页面及管理员会话验收；未新建真实用户、未消耗上游模型调用额度。具体正式站结果保存在本机 `fantasy-maintenance/receipts` 中。
