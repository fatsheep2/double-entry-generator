---
title: 贡献指南
description: 如何为 Double Entry Generator 项目做出贡献
---


# 贡献指南

感谢您对 Double Entry Generator 项目的关注！我们欢迎各种形式的贡献，包括代码、文档、问题报告等。

## 贡献方式

### 🐛 报告问题

如果您发现了 bug 或有功能请求，请：

1. 查看 [现有 Issues](https://github.com/deb-sig/double-entry-generator/issues) 确认问题未被报告
2. 创建新的 Issue，包含：
   - 清晰的问题描述
   - 复现步骤
   - 期望的行为
   - 实际的行为
   - 环境信息（操作系统、Go 版本等）

### 💻 贡献代码

#### 开发环境设置

```bash
# 1. Fork 并克隆仓库
git clone https://github.com/YOUR_USERNAME/double-entry-generator.git
cd double-entry-generator

# 2. 添加上游仓库
git remote add upstream https://github.com/deb-sig/double-entry-generator.git

# 3. 安装依赖
go mod download

# 4. 运行测试
make test
```

#### 开发流程

1. **创建功能分支**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **编写代码**
   - 遵循 Go 代码规范
   - 添加必要的测试
   - 更新相关文档

3. **运行测试**
   ```bash
   make test
   make lint
   ```

4. **提交代码**
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

5. **推送并创建 PR**
   ```bash
   git push origin feature/your-feature-name
   ```

#### 代码规范

- 使用 `gofmt` 格式化代码
- 遵循 Go 官方代码规范
- 添加适当的注释
- 编写单元测试

### 📝 贡献文档

文档贡献包括：

- 修复文档中的错误
- 添加新的使用示例
- 改进文档结构
- 翻译文档

文档文件位于 `docs/` 目录下，使用 Markdown 格式。

### 🔧 添加新的账单格式：模板优先

普通账单格式应优先提交到 [`deb-sig/deg-provider-template`](https://github.com/deb-sig/deg-provider-template)。模板仓库提供 v2 schema 规范、最小示例、注册表格式和自动校验；贡献必须包含 `template.yaml`、`rules.yaml`、脱敏账单样例和精确的 `expected.beancount`。

先用以下标准判断：

- CSV、XLS、XLSX、表头跳过、编码/分隔符差异、退款、手续费、转账、投资分录和辅助行，都先尝试通用模板。
- 能用条件、字段方法、算术、`vars`、`from` / `to` 或显式 `postings` 表达的逻辑，留在模板规则中。
- 只有确实需要跨行状态、多文件或外部数据合并、通用 reader 不支持的容器格式，或无法由现有 actions 表达的输出语义时，才提出 Go provider。
- 规则很多、性能顾虑或格式陌生本身不构成新增 Go provider 的理由。

模板贡献流程：

1. 按模板仓库的 `CONTRIBUTING.md` 复制最接近的示例。
2. 将账单格式语义放入 `templateRules`，把用户账户选择放入可编辑的 `personalRules`。
3. 新增 `latest/` 与不可变的 `YYYY-MM-DD/` pin，并登记到 `registry.yaml`。
4. 使用当前 DEG `main` 运行 `scripts/verify.sh`，验证 schema、注册表、`config init`、`import` 和期望输出。

如模板确实无法表达，请在 DEG Issue 中提供最小化、已脱敏的样例，指出具体能力缺口，并说明为何现有规则/actions 不足。获准使用 Go provider 后，再实现接口、单元测试、端到端样例和 provider 文档。

## 提交规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

- `feat:` 新功能
- `fix:` 修复 bug
- `docs:` 文档更新
- `style:` 代码格式调整
- `refactor:` 代码重构
- `test:` 测试相关
- `chore:` 构建过程或辅助工具的变动

示例：
```bash
git commit -m "feat(alipay): add support for new transaction types"
git commit -m "fix(ccb): handle empty description field"
git commit -m "docs: update installation guide"
```

## 代码审查

所有提交的代码都会经过审查：

1. **自动化检查**
   - 代码格式检查
   - 单元测试
   - 集成测试

2. **人工审查**
   - 代码质量
   - 功能正确性
   - 文档完整性

## 发布流程

1. 代码合并到主分支
2. 自动触发 CI/CD 流程
3. 生成发布版本
4. 更新文档网站

## 社区准则

### 行为准则

- 保持友善和尊重
- 欢迎不同背景的贡献者
- 专注于对社区最有利的事情
- 对其他社区成员保持同理心

### 沟通渠道

- **GitHub Issues**: 问题报告和功能请求
- **GitHub Discussions**: 一般讨论和问答
- **Pull Requests**: 代码审查和讨论

## 许可证

通过贡献代码，您同意您的贡献将在 Apache 2.0 许可证下发布。

## 致谢

感谢所有为项目做出贡献的开发者！您的贡献让这个项目变得更好。

## 需要帮助？

如果您有任何问题，请：

1. 查看 [FAQ](/faq/)
2. 在 [GitHub Discussions](https://github.com/deb-sig/double-entry-generator/discussions) 提问
3. 创建 [Issue](https://github.com/deb-sig/double-entry-generator/issues)
