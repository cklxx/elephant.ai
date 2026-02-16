---
name: drive-file
description: 飞书云盘文件和文件夹管理，支持浏览、创建、复制和删除。
triggers:
  intent_patterns:
    - "文件|drive|云盘|上传|下载|文件夹|folder"
  context_signals:
    keywords: ["文件", "drive", "云盘", "文件夹", "folder"]
  confidence_threshold: 0.6
priority: 7
exclusive_group: lark-drive
requires_tools: [channel]
max_tokens: 200
cooldown: 30
---

# drive-file

飞书云盘管理：浏览文件、创建文件夹、复制和删除文件。

## 调用

通过 channel tool 的 action 参数调用：

| Action | 说明 |
|--------|------|
| `list_drive_files` | 列出文件夹中的文件 |
| `create_drive_folder` | 创建新文件夹 |
| `copy_drive_file` | 复制文件到指定文件夹 |
| `delete_drive_file` | 删除文件 |

## 参数

### list_drive_files
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| folder_token | string | 否 | 文件夹 token（默认根目录） |
| page_size | integer | 否 | 每页数量 |
| page_token | string | 否 | 分页 token |

### create_drive_folder
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| folder_token | string | 否 | 父文件夹 token（默认根目录） |
| name | string | 是 | 文件夹名称 |

### copy_drive_file
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_token | string | 是 | 源文件 token |
| folder_token | string | 是 | 目标文件夹 token |
| name | string | 是 | 新文件名 |
| file_type | string | 否 | 文件类型（默认 file） |

### delete_drive_file
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file_token | string | 是 | 文件 token |
| file_type | string | 否 | 文件类型（默认 file） |

## 示例

```
浏览根目录文件
→ channel(action="list_drive_files")

创建新文件夹
→ channel(action="create_drive_folder", name="项目资料")

复制文件到另一个文件夹
→ channel(action="copy_drive_file", file_token="fileXXX", folder_token="folderYYY", name="副本.pdf")
```

## 安全等级

- `list_drive_files`: L1 只读
- `create_drive_folder` / `copy_drive_file`: L3 高影响
- `delete_drive_file`: L4 不可逆
