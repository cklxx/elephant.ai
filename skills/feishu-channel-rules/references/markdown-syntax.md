# 飞书 Markdown 语法参考

> 飞书消息卡片支持的 Markdown 语法参考。

## 1. 标题

```
#### 四级标题
##### 五级标题
```

- **不支持**一二三级标题（`#`、`##`、`###`），会导致卡片显示异常
- 可用加粗替代标题效果

## 2. 换行

```
第一行\n第二行
```

## 3. 文本样式

| 语法 | 效果 |
|------|------|
| `**加粗**` | **加粗** |
| `*斜体*` | *斜体* |
| `~~删除线~~` | ~~删除线~~ |

> **注意**：加粗中间的内容只能是中文或英文，不能有中文符号或表情符号

## 4. 链接

```
[链接文本](https://www.example.com)
```

## 5. @指定人

```
<at id=id_01></at>
<at ids=id_01,id_02,xxx></at>
```

- 用户 id 必须是真实 id，不能编造
- 可能是：`ou_` 开头的字符串、不超过 10 位的字符串、邮箱

## 6. 彩色文本

```
<font color='green'>绿色文本</font>
```

颜色枚举：`neutral`, `blue`, `turquoise`, `lime`, `orange`, `violet`, `wathet`, `green`, `yellow`, `red`, `purple`, `carmine`

## 7. 图片

```
![hover_text](image_key)
```

> image_key 不支持 http 链接

## 8. 分割线

```
---
```

## 9. 标签

```
<text_tag color='red'>标签文本</text_tag>
```

## 10. 有序列表

```
1. 一级列表①
    1.1 二级列表
    1.2 二级列表
2. 一级列表②
```

- 4 个空格代表一层缩进

## 11. 无序列表

```
- 一级列表①
    - 二级列表
- 一级列表②
```

## 12. 代码块

````
```JSON
{"This is": "JSON demo"}
```
````

- 支持指定编程语言

## 13. 人员组件

```
<person id='user_id' show_name=true show_avatar=true style='normal'></person>
```

- `style`：`normal` 普通样式，`capsule` 胶囊样式
- **注意**：person 标签不能嵌套在 font 中
