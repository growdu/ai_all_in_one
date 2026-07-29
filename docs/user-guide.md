# 用户手册 — AI All-in-One 1.0

> 面向最终用户的简明使用指南
> 详见 docs/frontend/03-web.md（前端设计）

## 一、3 分钟上手

### 1. 打开网站

浏览器访问你部署的地址（如 `http://localhost:8080` 或 `https://ai.example.com`）。

### 2. 选 AI 服务（首次）

页面显示 4-5 个 AI 服务卡片：

- **豆包**（字节跳动，国内，速度快）
- **DeepSeek**（深度求索，国内，性价比高）
- **Kimi**（月之暗面，国内，长文档）
- **OpenAI**（海外，2.0 阶段支持）
- **Claude**（海外，2.0 阶段支持）

点你常用的那个。1.0 阶段：选 1 个就够用；选 2+ 个可以解锁"对比模式"。

### 3. 配 API Key

跳转 Settings 页面：

1. 找到你要配的 provider（如"豆包"）
2. 在输入框里粘贴你的 API Key（去对应厂商官网申请）
3. 点"添加 Key"
4. 状态变成"已配"——完成！

> 💡 **API Key 从哪来？**
> - 豆包：https://www.volcengine.com/product/ark
> - DeepSeek：https://platform.deepseek.com/
> - Kimi：https://platform.moonshot.cn/
>
> 申请过程通常要实名 + 充值几块钱

### 4. 开始聊天

回 Home 页面：

- 顶部选模型（默认选第一个）
- 选模式：单个 / 自动选 / 对比
- 底部输入消息
- **Ctrl+Enter** 发送

## 二、3 种聊天模式

### 单个（默认）

显式选一个模型 + 模式，所有请求发到这个模型。

**适合**：明确知道用哪家

### 自动选（auto）

后端根据"近 5 分钟成功率 + 延迟 + 你的偏好"自动选最佳模型。

**适合**：
- 懒得每次选
- 一家用挂了自动切到另一家

### 对比（compare）

同一个问题发给多个模型，并排展示结果。

**适合**：
- 写代码：对比哪家方案更优
- 写文案：对比哪家风格更对
- 做决策：需要"second opinion"

## 三、Settings 页面

### 我的 API Key

每行一个 provider：
- 已配 Key 的 provider 显示"已配"
- 点输入框可更新 Key
- 点"删除"清除（会弹确认框）

### AI 角色 (System Prompt)

你给 AI 的"人设"。每次新对话自动注入。

**示例**：
```
你是一个 X 领域的资深工程师。
回答用简体中文，简洁直接。
涉及代码时用对应语言的标准 markdown code block。
```

**高级**：
- 勾选"锁定"——截断时不会丢这段
- 编辑单条 system 消息（长按消息）

### 高级参数

- **温度**（0-2）：越高越有创造性，越低越确定
  - 0.0-0.3：事实/代码
  - 0.5-0.7：通用
  - 0.8-1.2：创意/写作
- **最大长度**：单次回复最大 token 数
- **JSON 模式**（部分模型）：强制输出 JSON
- **推理模式**（o1 类）：深度思考，慢但准

## 四、快捷键

| 快捷键 | 动作 |
|--------|------|
| `Ctrl/Cmd + Enter` | 发送 |
| `Ctrl/Cmd + L` | 清空对话 |
| `Ctrl/Cmd + N` | 新对话 |
| `Esc` | 停止生成 |
| `Ctrl/Cmd + /` | 切换模式（single/auto/compare） |

## 五、安全注意

1. **API Key 是你的钱**——泄露后别人会用你的额度
2. 我们的服务**只在本地加密**你的 Key（`/data/keyring.json`），永不上传
3. 建议：
   - 不要截图带 Key 的 Settings 页面
   - 定期去厂商后台 rotate Key
   - 不用时点"删除"清掉本地的

## 六、常见问题

### Q: 配了 Key 但一直报错

1. 检查 Key 是不是复制完整（没多余空格/换行）
2. 去厂商后台看 Key 是否有效 + 有余额
3. 部分厂商 Key 有时效/限流，1-2 分钟后重试

### Q: 对比模式一直报"need >= 2 providers"

至少要配 2 个 provider 的 Key。

### Q: 怎么清空对话

Home 页面，**Ctrl+L**。或刷新页面（localStorage 还会保留）。

### Q: 能不能用手机访问

可以。1.0 已经做了移动端适配（iOS Safari / Android Chrome）。

### Q: 能不能导出对话

1.0 不支持。2.0 计划加 Markdown / JSON 导出。

### Q: 能不能分享对话

1.0 不支持。2.0 计划加 `?share=token` URL。

## 七、隐私

- 所有数据存在你的本地服务器（`/data/`）
- AI 请求只发到**你配置**的 provider
- 我们**不收集任何**遥测/分析数据
- 卸载时 `docker volume rm aiio-data` 即可清除所有本地数据

## 八、参考

- 部署：[deploy.md](deploy.md)
- 架构：[architecture/00-overview.md](architecture/00-overview.md)
- 输入处理：[architecture/02-input-processing.md](architecture/02-input-processing.md)（附件预处理 + Prompt 增强）
