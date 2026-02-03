---
name: auto-skill-creation
description: 自动创建技能并将重复流程沉淀为技能，支持使用Codex/Claude等外部代理执行任务并生成技能文件。
---

# 自动技能创建流程

## When to use this skill
- 需要将重复执行的任务流程沉淀为可复用的技能
- 需要使用外部代理（如Codex、Claude）自动生成代码或文档
- 需要快速创建符合规范的技能文件结构

## 必备输入
- 任务描述：需要自动化的任务内容
- 代理类型：使用的外部代理（如"codex"、"claude"）
- 技能名称：新技能的唯一标识符
- 技能描述：新技能的功能说明

## 工作流
1. **任务调度**：使用bg_dispatch工具将任务发送给指定的外部代理
   - 设置agent_type为所需的代理类型
   - 提供清晰的任务prompt
   - 记录生成的task_id用于后续操作

2. **状态监控**：使用bg_status工具定期检查任务执行状态
   - 等待任务完成或出现需要人工干预的情况
   - 处理可能的错误或重试请求

3. **结果收集**：使用bg_collect工具获取任务执行结果
   - 提取生成的代码或文档内容
   - 验证结果是否符合预期

4. **技能生成**：将任务流程和结果沉淀为技能文件
   - 创建技能目录结构
   - 编写SKILL.md文件，包含技能元数据、使用场景、输入要求和工作流
   - 确保符合现有技能的格式规范

5. **代码提交**：将生成的技能文件提交到代码仓库
   - 使用git命令添加新文件
   - 提交并推送代码变更

## 输出结构
- 技能目录：`skills/<skill-name>/`
- 技能文件：`skills/<skill-name>/SKILL.md`
- 示例输出：符合规范的技能文件，包含完整的使用说明和工作流

## 示例用法
```
# 创建测试文件的技能示例
1. 调度任务：bg_dispatch(agent_type="codex", prompt="写个测试文件")
2. 监控状态：bg_status(task_id="xxx")
3. 收集结果：bg_collect(task_id="xxx")
4. 生成技能：创建skills/test-file-creation/SKILL.md
5. 提交代码：git add skills/test-file-creation/SKILL.md && git commit -m "Add test file creation skill"
```