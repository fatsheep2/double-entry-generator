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

### 🔧 添加新的账单格式支持

添加一种新的账单格式，**优先使用[通用模板 Provider](providers/template.md)**，而不是在 DEG 本体里新增 Go 包。传统 Go provider 方式仍然可用，但只建议在模板引擎确实无法表达账单格式时使用（详见下文）。

#### 方式一：通用模板（推荐）

大多数账单格式（CSV/XLSX/XLS，字段固定，规则可用 `when`/`actions` 表达）都可以只写 YAML 就完成接入：

1. 参考[模板文件](providers/template.md#模板文件)和[规则文件](providers/template.md#规则文件)语法，编写模板文件（描述账单如何被读取）和模板规则（描述账单业务含义，如收支方向、退款、手续费等）。
2. 用你自己的账单验证：`double-entry-generator import ./your-template.yaml bill.csv -o output.bean`。
3. 将模板提交到 [deb-sig/deg-provider-template](https://github.com/deb-sig/deg-provider-template) 仓库（该仓库独立于本仓库维护），而不是本仓库。
4. 模板合入后，用户即可通过 `double-entry-generator import your-provider bill.csv` 直接使用，无需 DEG 发布新版本。

这种方式不需要理解 DEG 内部结构（IR、analyser、compiler），review 也只需要检查模板和规则本身。

#### 方式二：传统 Go Provider（仅在模板引擎无法表达时使用）

当账单格式无法用模板引擎表达时（例如二进制/加密文件格式、需要调用外部 API、需要有状态的多行合并逻辑等），才在本仓库新增 Go provider：

1. **创建 Provider 目录**
   ```bash
   mkdir pkg/provider/your-provider
   ```

2. **实现接口**
   - `pkg/provider/interface.go` 中定义的接口
   - 参考现有 provider 的实现

3. **添加测试**
   - 在 `test/` 目录下添加测试脚本
   - 提供示例数据文件

4. **更新文档**
   - 在 `docs/providers/` 下添加文档
   - 更新 README 和导航

提交这类 PR 时，请在描述中简单说明为什么模板引擎无法满足需求，方便维护者 review。

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
