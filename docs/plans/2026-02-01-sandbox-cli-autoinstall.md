# Plan: Sandbox auto-install Codex + Claude Code (2026-02-01)

## Goal

确保本地 sandbox 容器启动后自动安装 Codex 与 Claude Code CLI，并提供可选关闭开关与文档说明。

## Scope

- dev/deploy 脚本的 sandbox 启动流程
- 共享 sandbox 工具函数
- Sandbox 运维文档

## Plan

- [x] 盘点 sandbox 启动流程与插入点
- [x] 新增 sandbox CLI 安装 helper（npm 方式、可选关闭、失败仅告警）
- [x] 接入 dev.sh / deploy.sh 启动流程
- [x] 更新 Sandbox 文档与脚本注释
- [x] 运行 lint + tests，重启服务并记录结果

## Notes

- CLI 安装依赖 sandbox 镜像内置 `npm`/Node。
- 默认仅在本地容器启动路径执行，远端 sandbox 仅做健康检查。
- 修复 `env_flags` 数组在 `set -u` 下未初始化导致的 dev.sh 重启失败。
- `./dev.sh lint`/`./dev.sh test` 已通过；`./dev.sh down && ./dev.sh` 已完成。
