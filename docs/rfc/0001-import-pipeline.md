# RFC：重新设计 DEG 的账单导入 —— 云端格式模板 + 本地个人规则

> 状态：草案，征求意见
> 范围：`translate`（Go provider）与 `import`（runtime 模板）两条导入路径的统一设计
> 调研基线：double-entry-generator `97a4f65`（已含 #223、#231），deg-provider-template `dev-v3`（`a00f7c1`）
> 附录（逐账单格式特征、能力矩阵、已知问题清单、外部参考）见 [0001-appendix.md](0001-appendix.md)
> 文中示例语法均为草案，字段名和写法以讨论结果为准

## 0. 摘要

**问题**

- 同一件事（读账单 → 判断方向 → 分类 → 记账）在 23 个 Go provider 里各实现了一遍。每家的规则字段、默认账户、出错方式都不一样，已知 bug 散落各处。
- runtime 模板（v3）把格式解析、个人分类、分录生成放进同一套规则语言；金额先取绝对值，再由 `from`/`to` 决定方向。结果是：个人规则要按收入、支出各写一遍；格式知识被复制到用户本地，用户的账户却写进了云端模板；还有若干情况会静默产出错账。
- 跨行能力（退款配对、换汇配对、去重、余额断言）两条路径都基本没有。

**提议**

1. 一条固定的导入管线：识别 → 读取 → 规范化 → 关联 → 口径 → 分类 → 记账 → 校验。
2. 中间统一成一种"规范交易"：金额带符号（正数 = 钱进本账户），本账户用 `account_key` 标识，每一行映射到同一套 `kind`/`status`/`via` 词汇。
3. **事实与口径分开。** 云端模板只判定事实（这一行是什么、和哪一行有关），**不出现任何个人账户，也不替用户做口径决定**；退款抵不抵消、不计收支记不记等口径由引擎统一定义，用户在本地全局设一次。
4. 本地只有四样东西：账户绑定表、全局口径、分类规则、单笔覆盖。一条分类规则同时适用于收入和支出。
5. 分录形态（普通、手续费、转账、换汇、证券交易）由引擎内置；出错一律报错或标记待复核，不静默；输出只由"账单 + 模板版本 + 本地配置"决定，可以随时重建。
6. PDF、邮件账单和需要脚本获取的来源，由独立的转换器仓库转成统一的交换格式 CSV，引擎只认表格类输入。

**路线**：先修正确性（Go 主干止血、v3 引擎 fail-closed），再引入规范交易、口径和本地配置，然后补读取层、关联层、证券分录和工具。见第 16 节。

## 1. 背景与调研方法

DEG 目前有两条导入路径：

- `translate --provider X`：23 个 Go provider 包，按变体拆开约 29 种账单——支付平台 4 种、国内借记卡 9 种、国内信用卡 6 种、海外银行 5 种、证券/加密 5 种。
- `import <模板>`：runtime 模板引擎（#231），模板放在 deg-provider-template，`dev-v3` 分支目前有 26 个模板。

这次调研做了：

- 读完全部 provider、analyser、示例输入输出、示例配置和文档；读完全部 dev-v3 模板。
- 用 master 二进制和 dev-v3 所用引擎分别跑全部样例；对可疑点构造边界输入复现；投资类输出用 beancount 3 的 `bean-check` 校验。
- 调研 OFX、QIF、camt.053、MT940 等格式，以及 beangulp、beancount-import、hledger CSV rules、Firefly III、Actual Budget、GnuCash、BeanHub、china_bean_importers 等工具的设计（链接见附录 D）。

社区里反复出现的问题也指向同一组根因：#162（账单重复）、#124（微信退款怎么配规则）、#103（targetAccount 与 defaultCashAccount 的增减关系）、#134（规则能覆写哪些字段）、#200（规则文档冲突）、#194（本地怎么新增解析器）。

## 2. 现状：问题和证据

### 2.1 Go provider

逐条清单和代码链接见附录 C，这里只按类型归纳：

| 类型 | 例子 |
|---|---|
| 规则字段声明了但不参与匹配，等于恒为真 | citic、bocom_credit 的 `peer`，icbc 的 `item`。一条只写 `peer` 的 ignore 规则会丢掉整个文件（已复现） |
| 加一条无关规则会改变默认账户 | abc、bmo、bocom_credit、ccb、citic、cmb、hsbchk、icbc、td：没有规则时用 defaultMinus/Plus，有任意一条规则后改用 defaultCashAccount |
| 金额精度 | alipay、mt 用 float32 解析（1234567.89 → 1234567.88）；jd 删掉小数点后按整数解析 |
| 格式写死 | alipay 写死跳过 22 行，较新的导出多一行说明，master 已解析失败；bmo 靠行号区分信用卡/借记卡 |
| 关键字截断 | cib、spdb 只要整行包含"说明"/"合计"就停止读取，后面的交易静默丢失 |
| 静默兜底 | cib 不认识的币种当 CNY；boc 非人民币一律 USD；icbc、citic 的币种没进 IR；ccb 冲正行金额变 0 |
| 崩溃 | hsbchk 5 列文件 panic；huobi 手续费为空 panic |
| 分录 | 证券成本按单价 `%.3f`、数量 `%.2f` 截断，碎股和高精度价格不平；港股数字代码、合约地址被直接当作 commodity |
| 命名和语义不一致 | `tag`/`tags`、`status`/`orderStatus` 并存；`time` 在 oklink 表示日期区间；hxsec 首条命中即停，其余是后者覆盖前者 |

结论：每家银行一套 Go 代码，等于同一条管线写了 23 遍，也有 23 套各不相同的 bug 和配置写法。

### 2.2 runtime 模板（v3）

dev-v3 模板依赖若干尚未合入 master 的引擎能力（如表头定位、`postingsMode`），下文称"开发中引擎"。

**verify 结果。** 用开发中引擎跑 26 个模板：16 个与 expected 一致，9 个不一致，1 个报错。10 个异常都已定位，没有一个是读表本身的问题：

| 原因 | 模板 |
|---|---|
| 空 metadata 被输出成 `key: ""`，Go 版跳过 | abc_debit、alipay、hsbchk-credit、hxsec |
| 编译器改为总是输出 payee 和 narration 两段，expected 由旧编译器生成 | icbc-credit、icbc-debit-v1、icbc-debit-v2 |
| 模板把描述放进了 payee，应放 narration | hsbchk-debit |
| `format("%06.0f")` 不支持补零、静默返回原值（excelize 版本差异已随合并 master 消除，问题仍在） | htsec |
| 缺 `options.title`；`.number` 无法解析 `131.77(已退款89.84)` | alipay、jd |

一致也不等于正确：spdb 的 expected 是用模板重新生成的，与 Go 输出相比已经丢了 tag；citic、boc 的模板把 Go 的币种 bug 一起照抄了。

**静默产出错账的路径**（均已复现；最后一列是 master `97a4f65` 上的复现结果）：

| 现象 | 复现 | master |
|---|---|---|
| 冲正行的负号被取绝对值，退回的钱被记成又支出一次 | ccb `支出=-67`；cib `隔日冲账 -5.50`；最小样例 `-5.00` 输出 `Expenses 5.00` | 同样存在 |
| 没有规则判定方向时，输出没有账户的分录，不报错 | bocom 借贷列出现模板没覆盖的值；mt 的"不计收支"；hxsec 未知业务类型 | 同样存在 |
| 表头与模板不一致时不报错 | 用 icbc-debit-v1 模板导入 v2 账单：退出码 0、输出 0 笔交易、没有任何提示（行级错误被 `skipInvalidRows` 全部吞掉） | 同样存在 |
| 未声明日期格式时逐行猜 | td 模板没写 `dateFormat`：同一文件里 `13/05/2023` 按日/月、`05/01/2023` 按月/日解析。日期解析失败时只报 "rule did not set date"，不带行号和原值 | 同样存在 |
| 条件里日期按算式比较 | `date > "2025-01-01"` 得到 false | 不存在，仅开发中引擎 |
| 表达式出错静默 | master 不支持 `.replace`，开发中引擎在逗号后有空格时失效；两者都把参数原样拼进结果，不报错 | 同样存在 |
| 不做平衡校验 | 引用不存在的列，生成一条省略金额的自动平衡腿 | 同样存在 |
| ledger 目标 | runtime 导入用 `-t ledger` 时，分录里的账户全部丢失（ledger 编译器不处理 Postings） | 同样存在 |

**规则繁琐。** 金额先取绝对值，再由 `from`/`to` 决定方向，所以同一个分类要按收入、支出各写一条：

| 模板 | dev-v3 personalRules 条数 | 说明 |
|---|---|---|
| abc_debit | 10 | Go 示例配置是 5 条 |
| bocom_debit | 11 | 按"贷/借"成对 |
| hsbchk-debit / citic-credit | 13 / 13 | 约 7 个分类 |
| boc-debit | 27 | 每个卡号尾号收、支各一条 |
| alipay / jd | 28 / 36 | 每种付款方式 2～3 条 |

**云端和本地放反了。**

- 云端模板里写死了个人数据：BMO、TD、HSBC 的本人账户；ccb 的样例账号、icbc 的卡别名、cmb-credit 的年份 `2024/<记账日>`（2025 年的账单会被记成 2024 年，已复现）；mt 的 starter 规则里还有带个人 ID 的账户名。
- 用户本地的 personalRules 里反而是格式知识：微信提现服务费的拆分（按银行复制了三份）、支付宝用 `订单号 ~ "_"` 丢弃退款（部分退款和跨文件退款会静默消失）、"忽略新增证券"。`config init` 把它们复制到本地之后，就不再随云端更新。

**分录是手写的 beancount 文本。** 证券、币币交易的分录形态几乎一样，每个模板却手写 2～4 遍 `{…} @@` 语法，精度和 commodity 的 bug 跟着一起被复制；这些文本也没办法输出成 ledger。

### 2.3 根因

1. **一套规则语言同时承担规范化、分类和分录生成。** 模板作者和用户写的是同一种东西，边界无从划分。
2. **金额取绝对值，方向由 from/to 决定。** 符号信息丢了，规则必须收支各写一遍，冲正必然出错。
3. **规则条件读的是前面规则改过的值。** 结果依赖规则顺序，难以预测。
4. **没有文件级上下文，也没有跨行阶段。** 表头上方的账号、币种、账单年月读不到；退款、换汇、去重无处表达。
5. **没有存放"本人账户"的地方。** 只能写死在模板或规则里。
6. **默认是"出错也继续"。** 静默跳过、静默兜底、静默输出原值。

### 2.4 值得保留的部分

精确小数运算；registry 按日期固定模板版本（pin）；能力声明遇到不认识的就拒绝；表头定位要求唯一匹配；模板和个人规则分文件存放的方向；现有的契约测试写法。

## 3. 目标与非目标

**目标**

- **完整**：覆盖现有 29 种账单，并能扩展到 OFX、XML、多段表格；PDF、邮件账单和需要脚本获取的来源，通过转换器接入（第 9 节）。
- **简单**：用户只回答三类问题——"我的账户是哪个""这个商户记到哪""这几种情况我想怎么记"。一个商户一条规则；口径全局设一次。
- **可靠**：认不出就报错；拿不准就标记待复核；每笔分录保证平衡；能用账单自带的余额和合计核对。
- **可重建**：输出只由"账单 + 模板版本 + 本地配置"决定，随时可以重新生成。
- **分工清楚**：格式知识在云端，可以随银行改版更新；个性化在用户自己的账本仓库里。
- **两种输出等价**：beancount 和 ledger 用同一套分录。

**非目标**（本 RFC 不做）

- 获取数据（抓取平台接口、登录网银）。由转换器仓库或用户自己的脚本完成，DEG 只处理文件。
- 引擎内置 PDF、JSON、邮件解析。由转换器转成交换格式（第 9 节）。
- 机器学习自动分类。规范交易和复核标记为以后接入留了口子。
- 全屏 TUI。复核用逐条提问的命令和 JSON 报告完成（第 11 节）。
- 图灵完备的规则语言。

## 4. 总体架构

### 4.1 四部分职责

| 部分 | 在哪 | 谁维护 | 内容 |
|---|---|---|---|
| 引擎 | DEG 仓库 | DEG 维护者 | 读取器、规范化、关联、全局口径、分录形态、校验、报告、交换格式规范 |
| 格式模板 | deg-provider-template（云端，按版本发布） | 模板贡献者 | 每种账单怎么读、每一行在统一词汇里是什么，以及样例和期望输出 |
| 转换器 | 独立仓库（暂称 `deg-converters`） | 转换器贡献者 | 把 PDF、邮件账单、脚本获取的数据转成交换格式 CSV；语言不限 |
| 本地配置 | 用户自己的账本仓库 | 用户 | 账户绑定、全局口径、分类规则、单笔覆盖 |

判断一样东西放哪，问两个问题：

- **换一个用户还成立吗？** 成立就放云端（例如"招行信用卡账单里正数是消费"）；不成立就放本地（例如"尾号 1234 是 `Liabilities:CMB`"）。
- **银行改版会影响它吗？** 会，就必须放云端，用户才不用手改。

### 4.2 设计原则

1. **事实与口径分开。** "这一行是退款、对应哪笔原单"是事实，由模板判定；"退款要抵消还是两笔都记"是口径，由用户在本地全局选择。模板不替用户做口径决定。
2. **统一词汇。** 所有账单的每一行都映射到同一套 `kind`、`status`、`via`（第 5.2 节）。用户的规则和口径只认这套词汇，写一次对所有账单生效。
3. **识别不删除。** 关联阶段只标记行之间的关系（退款对、冲正对、换汇对、重复），是否合并、是否丢弃由口径决定；任何没进账本的行都出现在报告里。
4. **输出可重建。** 人的判断沉淀为规则或单笔覆盖，写进本地配置，不去改生成的账本文件。
5. **出错就报错。** 不静默跳过、不静默兜底、不静默输出原值（第 12 节）。

### 4.3 管线

```
识别 detect      按文件指纹选模板和变体；认不出就报错，不猜
  ↓
读取 read        表格行 + 表头前元信息（context）+ 尾部停止；多文件、多段、XML、OFX、交换格式
  ↓
规范化 normalize  每行 → 一条规范交易；方向、币种、日期、kind、status 在这一步确定，且只确定一次
  ↓
关联 link        识别退款/冲正对、多行聚合、换汇/互转配对、重复（模板声明，引擎实现）；只标记，不删除
  ↓
口径 policy      按用户的全局口径决定：中性交易记不记、退款抵消还是两笔、换汇合不合并……
  ↓
分类 classify    本地规则：对方账户、payee、narration、tags、keep/ignore；再应用单笔覆盖
  ↓
记账 post        引擎按 kind 生成分录，保证平衡、commodity 合法
  ↓
校验 check       余额断言、账单合计核对；输出复核报告
```

| 阶段 | 输入 | 输出 | 谁来配置 | 失败时 |
|---|---|---|---|---|
| 识别 | 文件 | 模板 id + 变体 | 云端 `detect` | 报错，列出最接近的模板 |
| 读取 | 文件 | 原始行 + context | 云端 `read` / `context` | 报错，带行号 |
| 规范化 | 原始行 | 规范交易 | 云端 `fields` / `rows` | 报错，带行号和原值 |
| 关联 | 规范交易流 | 带关系标记的交易流 | 云端 `link` | 有歧义就不配对，标记待复核 |
| 口径 | 带标记的交易流 | 每笔：记 / 默认忽略 / 合并 | 本地 `policy` | — |
| 分类 | 规范交易 | 对方账户等 | 本地 `rules` / `overrides` | 没命中就记到默认账户并标记 |
| 记账 | 分类后的交易 | 分录 | 引擎 | 不平衡、空账户、非法 commodity 直接报错 |
| 校验 | 分录 + 余额/合计 | 断言、报告 | 云端 `checks` | 不符时报错，或写进报告（由 `--strict` 决定） |

**为什么拆成这么多阶段：** 每个阶段只看上一阶段的输出，写在哪一层就只影响那一层。分类规则拿到的永远是已经确定了方向、币种和 kind 的交易，不需要知道这份账单原来是"收/支两列"还是"借贷方向列"。

## 5. 规范交易（Canonical Transaction）

规范交易是规范化阶段的输出，也是口径和分类规则唯一能看到的东西（另外可以只读访问 `raw`）。

### 5.1 字段

| 字段 | 类型 | 必填 | 含义 | 为什么需要 |
|---|---|---|---|---|
| `source` | `{template, variant, pin, file, file_sha256, row}` | 引擎填 | 来源 | 复现、报告、去重；输出为 `deg-src` 元数据 |
| `owner` | 字符串 | 否 | 账单所属人的标识，取自表头前元信息（支付宝账号、卡号等） | 一个账本记多人账户时（家庭账本），同一模板的账单按人区分 |
| `account_key` | `类别/值` | 是 | 本账户标识，如 `card/1234`、`method/花呗`、`sub/002`、`addr/0x…`、`acct/U123456` | 不同用户的账户不同，交给本地绑定表决定账户名 |
| `date` | 日期 | 是 | 交易日 | |
| `time` | 时间 | 否 | 交易时间 | 按时段分类；同日排序 |
| `tz` | 时区 | 模板声明 | 默认取模板 | 不再依赖运行机器的时区 |
| `booked_date` | 日期 | 否 | 记账日、入账日、交收日 | 信用卡交易日与记账日不同；证券交收 |
| `amount` | 精确小数，**带符号** | 是 | **正数 = 钱进本账户，负数 = 钱出本账户** | 唯一的方向约定。冲正自然正确；一条分类规则同时管收支 |
| `currency` | ISO 代码或 commodity | 是 | 记账币种 | 多币种账户 |
| `original` | `{amount, currency}` | 否 | 原币金额 | 外币消费以人民币入账时保留原币 |
| `fees` | `[{amount, currency, kind}]` | 否 | 附带的费用或优惠；`kind`：`commission` `tax` `service` `discount` `cashback` | 微信提现服务费；证券佣金、印花税；IBKR 佣金币种与成交币种不同；美团订单金额与实付金额之差 |
| `counterparty` | `{name, account, bank}` | 否 | 对手方，三项分开 | 不再把户名和卡号拼在一起（boc），按卡号尾号匹配不会误中 |
| `description` | 字符串 | 否 | 摘要、商品说明 | |
| `channel` | 字符串 | 否 | 资金渠道原文，如"招商银行信用卡(1234)" | 排查、规则匹配 |
| `via` | 枚举 | 否 | 这笔银行/卡流水经由哪个支付平台：`alipay` `wechat` `unionpay` `jd` `meituan` … | 同一笔同时出现在支付平台账单和银行流水里（#162），见口径 `mirrors` |
| `kind` | 枚举（5.2） | 是 | 交易种类 | 决定口径、默认对方账户和分录形态 |
| `status` | 枚举（5.2），另有 `status_raw` | 是，默认 `booked` | 交易状态 | 统一的状态口径 |
| `ids` | `{order, merchant_order, txn, trade, hash, serial}` | 否 | 账单自带的 ID | 去重、退款配对、幂等导入 |
| `balance` | `{amount, currency}` | 否 | 该行之后的余额 | 余额断言；期初余额 |
| `trade` | `{security, side, qty, price, total, settle_date}` | `kind=trade` 时 | 证券或币币交易 | 由引擎生成标准证券分录 |
| `related` | `{refund_of, reversal_of, group, pair, duplicate_of}` | 模板或关联阶段填 | 行间关系 | 口径据此合并或保留 |
| `raw` | 只读字典 | 引擎填 | 原始单元格 + context | 规则能读不能改；排查问题 |
| `flags` | 列表 | 引擎填 | `unclassified` `ambiguous_pair` `possible_duplicate` `unknown_type` `unpaired` … | 复核报告 |

给规则用的派生字段（只读）：`direction`（`in` / `out`）、`abs_amount`、`weekday`。

**两条关键约定：**

- **金额只在规范化阶段定一次符号。** 账单的三种方向表示（带符号金额、收入/支出两列、单独的方向列）都在模板里转成带符号金额，之后谁也不再取绝对值。
- **本账户和对方账户分开处理。** 本账户由 `account_key`（加上 `owner`）经本地绑定表得到；对方账户由分类规则、单笔覆盖或按 `kind` 的默认值得到。引擎根据 `amount` 的符号决定谁在借方、谁在贷方。

### 5.2 统一词汇

**kind**

| 值 | 含义 | 各账单里的例子 |
|---|---|---|
| `normal` | 普通收支 | 消费、工资、转账给他人 |
| `transfer` | 本人账户之间的转移 | 信用卡还款、提现、充值、转存、银证转账 |
| `neutral` | 账单自己标为"不计收支/中性"、经济含义不确定的行 | 支付宝不计收支、微信 `/`、京东冻结/解冻、先享后付 0 元行 |
| `refund` | 退款 | 支付宝退款、微信退款、信用卡退货 |
| `reversal` | 冲正 | ccb 消费冲正、cib 隔日冲账 |
| `fee` | 费用本身是一笔交易 | 年费、账户管理费 |
| `interest` | 利息 | 结息、逆回购利息 |
| `dividend` | 股息、红利 | 红利入账 |
| `tax` | 税 | 预扣税、差异化扣税 |
| `fx` | 换汇的一条腿 | 购汇、结汇 |
| `trade` | 证券或币币交易 | 证券买卖、币币成交 |
| `installment` | 分期相关（待讨论） | 信用卡分期本金、分期手续费 |
| `adjust` | 其他调整 | 公司行为、ETF 份额合并 |

**status**：`booked`（已入账，默认）、`pending`（未入账、预授权）、`closed`（关闭、未付款）、`failed`（失败）。

模板作者必须把每一行映射到这套词汇；遇到映射不了的类型，按第 12 节处理（报错，或模板声明 `unknown: flag`），不能留空。词汇的增减属于本地协议的版本变更。

## 6. 云端格式模板

### 6.1 目录与发布

```
registry.yaml                    id、名称、分类、latest、pins、sha256、最低引擎版本
<id>/<pin>/
  template.yaml
  README.md                      这种账单怎么导出
  fixtures/<case>/
    bill.*                       脱敏样例（覆盖退款、冲正、未知类型等边界）
    local.yaml                   样例用的本地配置
    expected.beancount
    expected.ledger
```

### 6.2 template.yaml 的结构

| 段 | 作用 |
|---|---|
| `schema` `id` `name` `pin` `requires` | 元信息。`requires` 列出用到的引擎能力，引擎不支持就拒绝加载 |
| `detect` / `variants` | 指纹：表头前文本的正则、必需表头。一个 id 下可以有多个变体（icbc 借记 v1/v2、wechat csv/xlsx），按指纹自动选择 |
| `read` | 格式、编码、分隔符、sheet、表头定位、尾部停止、单元格清洗、是否多文件、行序 |
| `context` | 从表头前的元信息里取值：账号、子账户、币种、账单年月、所属人、合计数 |
| `maps` | 值映射表：中文币种名 → ISO、类型 → kind。可以引用引擎内置表 |
| `fields` | 规范交易各字段的取值表达式 |
| `rows` | 逐行的事实判定：设定 `kind`、`status`、`via`、`related`；拆出费用；`drop` 掉不是交易的行 |
| `link` | 关联声明：退款、冲正、聚合、配对、去重键 |
| `slots` | 除本账户外还需要用户绑定的账户槽位，如证券的 `position` `fee` `pnl` |
| `commodity` | commodity 命名规则：交易所前缀、补零、映射表 |
| `checks` | 余额断言、账单合计核对 |
| `params` | **格式参数**，只用于格式本身的调校（如换汇配对的时间窗口），由模板作者设定默认值；**不得用于口径** |

**`drop` 与口径的区别：** `drop` 只用于根本不是交易的行——表头重复、尾部合计、说明文字、零金额占位行，必须写原因，计入报告。"交易关闭""不计收支"这类是交易，模板只能把它们标成 `status: closed`、`kind: neutral`，记不记由用户的口径决定。

**本地模板：** 用户可以在本地放自己的模板文件，用于云端没有的小众账单，或者在云端修复前临时使用。本地模板与云端模板同 id 时，本地优先，并在报告里提示。

### 6.3 示例：招行信用卡

只描述格式，不出现任何个人账户：

```yaml
schema: deg/template/2
id: cmb-credit
name: 招商银行信用卡
pin: 2026-10-01

detect:
  preamble: '^招商银行信用卡对账单'
  headers: [交易日, 记账日, 交易摘要, 人民币金额, 卡号末四位]

read:
  format: csv
  header: { locate: [交易日, 记账日, 交易摘要, 人民币金额, 卡号末四位] }

context:
  period: { preamble: '(\d{4})年(\d{2})月' }     # 账单年月，用来补全 MM/DD 的年份

fields:
  date:        { value: coalesce(<交易日>, <记账日>), format: MM/DD, yearFrom: ctx.period }
  booked_date: { value: <记账日>, format: MM/DD, yearFrom: ctx.period }
  amount:      -num(<人民币金额>)       # 账单里正数 = 消费；取反后正数 = 钱进本账户
  currency:    CNY
  description: <交易摘要>
  account_key: card/<卡号末四位>

rows:
  - when: description ^= "还款"
    kind: transfer
  - when: description ~ "年费"
    kind: fee
  - when: description ^= "退货" || description ^= "退款"
    kind: refund
```

`yearFrom` 是引擎内置语义：账单月份是 1 月、交易是 12 月时，年份自动减 1。现在的模板只能写死 `2024/<记账日>`。

### 6.4 示例：支付宝（只标事实，不做口径）

```yaml
context:
  owner: { preamble: '支付宝账户[:：]\s*(\S+)' }

fields:
  amount:      signed(num(<金额>), map(dir, <收/支>))    # 收入 +，支出 −；不计收支见下
  account_key: method/<收/付款方式>
  owner:       ctx.owner
  ids:         { order: <交易订单号>, merchant_order: <商家订单号> }

maps:
  dir: { 收入: in, 支出: out, 不计收支: unknown, '': unknown }

rows:
  - when: raw.交易状态 == "交易关闭"
    status: closed
  - when: raw.收/支 == "不计收支"
    kind: neutral          # 方向不确定时，由 neutral 的口径决定是否记；记的话需要模板给出方向
  - when: raw.交易分类 == "退款"
    kind: refund

link:
  - refund:
      match: { by: ids.order, prefixOf: true, amount: equal }   # 待核实：支付宝退款单号是否一定是"原单号_后缀"
      ambiguous: flag
```

模板里没有任何"退款要不要抵消""不计收支要不要记"的开关——那是用户的口径（第 7 节）。

### 6.5 示例：兴业借记卡（多文件 + 换汇配对）

```yaml
read:
  format: xlsx
  multiFile: true                      # 同一账户各币种的文件合并成一条流
  header: { locate: [交易时间, 记账日, 支出, 收入, 账户余额, 摘要] }
  stop:   { firstCell: 说明 }          # 只看首列，摘要里出现"说明"不会误停

context:
  account:    { label: 账户账号 }      # 取"账户账号"右边一格
  subAccount: { label: 卡内账户 }      # 如"002 活期储蓄存款 港币 现汇"

maps:
  ccy: builtin:zh-currency             # 人民币→CNY、港币→HKD、美元→USD…；未知名称报错

fields:
  date:     { value: <交易时间>, format: 'YYYY-MM-DD HH:mm:ss', fallback: <记账日> }
  amount:   num(<收入>, 0) - num(<支出>, 0)   # 支出列的 -5.50（冲正）自然变成 +5.50
  currency: map(ccy, extract(ctx.subAccount, '(人民币|港币|美元|欧元|日元|英镑)'))
  account_key: sub/extract(ctx.subAccount, '^(\d+)')
  owner:    ctx.account
  balance:  num(<账户余额>)
  counterparty: { name: <对方户名>, bank: <对方银行>, account: <对方账号> }
  description:  join(' ', <摘要>, <用途>, <备注>)   # 跳过空值

rows:
  - when: amount == 0
    drop: 零金额占位行
  - when: raw.摘要 in ["购汇", "结汇"]
    kind: fx
  - when: raw.摘要 == "隔日冲账"
    kind: reversal

link:
  - pair: fx
    when: kind == fx
    same: [owner]
    differ: [currency]
    window: param.fxWindow
    ambiguous: flag                    # 一笔能配上多笔时不配对，标记待复核

params:
  fxWindow: 3m                         # 格式参数

checks:
  balance: { order: ascending }
```

### 6.6 示例：证券交割单（槽位 + 内置分录）

```yaml
slots: [cash, position, fee, pnl]      # 具体账户由用户在本地绑定

commodity:
  security:
    value: pad(<证券代码>, 6)           # 不依赖 xlsx 的显示格式来补零
    prefix: { when: <股东账号> ^= "A", then: SH, else: SZ }

maps:
  side: { 买: buy, 卖: sell }           # 其他值报错

fields:
  kind: trade
  account_key: acct/<资金账号>
  trade:
    security: commodity.security
    side:     map(side, <操作>)
    qty:      num(<成交数量>)
    price:    num(<成交价格>)
    total:    num(<成交金额>)
  fees:
    - { amount: num(<手续费>, 0), kind: commission }
    - { amount: num(<印花税>, 0), kind: tax }
```

分录由引擎生成：现金腿、持仓腿 `{成本}`、费用腿，卖出时再加盈亏腿。成本按 `total` 写总额，数量和价格保留账单的原始精度。模板不再手写 `{…} @@`。

### 6.7 模板里的表达式

表达式只在云端模板里使用，用户的本地规则不需要算术和字符串处理。

- **引用**：`<列名>`、`ctx.名字`、`param.名字`、`raw.列名`、已经算出的规范字段。
- **函数**：`num(x[, 空值默认])`、`signed(金额, in|out)`、`coalesce(…)`、`join(分隔符, …)`（跳过空值）、`extract(x, 正则)`、`replace(x, a, b)`、`trim`、`pad(x, 宽度)`、`map(表, 值[, 默认])`、`lower`、`upper`。
- **运算**：精确小数的 `+ - * /`。结果的小数位按输入中最多的那个，不会出现 `10.0000`。
- **出错就报错**：`num` 遇到认不出的文本、`extract` 不匹配、`map` 找不到键（且没给默认值）、参数语法不对，都带行号报错。

**为什么改用函数调用，而不是现在的 `<列>.method()` 链：** 参数里的空格、引号、逗号都按普通语法解析，不会再有"逗号后加空格就失效"这类问题；类型也更明确。

## 7. 全局口径

口径是"同一个事实，不同用户想怎么记"。把 29 种账单里需要用户选择的地方放在一起看，它们是同一小组问题在不同账单里反复出现，所以由**引擎统一定义**，用户在本地**全局设一次**，对所有账单含义相同。

| 口径 | 取值 | 默认 | 含义 |
|---|---|---|---|
| `neutral` | `skip` / `book` | `skip` | `kind: neutral` 的行记不记。记的话对方账户取 `defaults.neutral` |
| `closed` | `skip` / `book` | `skip` | 关闭、失败的交易 |
| `pending` | `skip` / `book` / `flag` | `skip` | 未入账、预授权 |
| `refunds` | `keep` / `merge` | `keep` | `keep`：原单和退款各记一笔，退款自动冲减原单的同一个对方账户，两笔加同一个 link。`merge`：已配对的退款对互相抵消，都不进账本（报告里列出）。没配上对的退款两种取值下都照记 |
| `reversals` | `keep` / `merge` | `keep` | 冲正，同上 |
| `fxPairs` | `merge` / `keep` | `merge` | `merge`：换汇两条腿合成一笔 `@@`；`keep`：各记一笔，经 `defaults.fx` 中转 |
| `mirrors` | `keep-both` / `prefer-app` / `prefer-bank` | `keep-both` | 银行或信用卡流水里 `via` 为某支付平台的行，与该平台账单里的同一笔重复时保留哪一边。只在两边都导入且配上对时生效；配不上的照记并标记 |
| `date` | `trade` / `booked` | `trade` | 用交易日还是记账日 |
| `discounts` | `ignore` / `record` | `ignore` | `record`：费用按原价记，优惠单独记一条腿到 `defaults.discount` |
| `opening` | `none` / `pad` | `none` | 首次导入时，用第一行的余额倒推期初，生成 `pad` + `balance` |

```yaml
# 本地，只写一次
policy:
  refunds: keep
  neutral: skip
```

**为什么默认 `refunds: keep`：** 金额带符号以后，退款以负数冲减原单同一个账户，净额正确，同时保留了"买过又退了"的记录，可审计。现在 Go 版支付宝默认抵消（#244 加了开关），迁移工具会把用户原来的开关值转成对应的口径（第 14 节）。

**确实需要某个账单与众不同时**，用同样的口径名写在 `by` 下，不引入新名字：

```yaml
policy:
  refunds: keep
  by:
    jd: { refunds: merge }
```

**单行例外**写分类规则的 `keep` / `ignore`（第 8.3 节），用的也是同一套词汇，例如"不计收支里只有余额宝收益要记"：

```yaml
rules:
  - when: kind == neutral && description ~ "余额宝"
    keep: true
    account: Income:Investment:Yuebao
```

## 8. 本地配置

### 8.1 结构

本地配置是一个文件，放在用户自己的账本仓库里，可以用 `include` 拆分：

```yaml
version: deg-rules/2
include: [deg/food.yaml]

output:
  title: 我的账本
  operatingCurrency: CNY
  bookingMethod: FIFO

accounts:                              # 本人账户绑定：<模板>[(所属人)]:<account_key 或 slot>
  cmb-credit:card/1234:      Liabilities:CreditCard:CMB:1234
  cmb-credit:card/*:         Liabilities:CreditCard:CMB       # 通配兜底
  alipay:method/余额:         Assets:Alipay:Balance
  alipay:method/花呗:         Liabilities:Alipay:Huabei
  alipay:method/招商银行信用卡(1234): Liabilities:CreditCard:CMB:1234
  alipay(138****0000):method/余额:   Assets:Family:Mom:Alipay   # 家庭账本里另一个人的账单
  cib_debit:sub/*:           Assets:Bank:CIB
  htsec:slot/cash:           Assets:Broker:HTSEC:Cash
  htsec:slot/position:       Assets:Broker:HTSEC:Stocks
  htsec:slot/fee:            Expenses:Broker:Fees
  htsec:slot/pnl:            Income:Broker:PnL

defaults:                              # 规则没命中时，按 kind / 方向决定对方账户
  out:      Expenses:FIXME
  in:       Income:FIXME
  transfer: Equity:Transfers
  neutral:  Equity:Transfers
  fx:       Equity:FX
  fee:      Expenses:Fees
  interest: Income:Interest
  discount: Income:Discounts

policy:                                # 第 7 节
  refunds: keep

rules:
  - when: counterparty ~ "国网" || counterparty ~ "电费"
    account: Expenses:Utilities:Electricity

  - when: counterparty ~ "滴滴" && time >= 22:00
    account: Expenses:Transport:Taxi
    tags: [late-night]

  - when: source == "wechat" && direction == in && abs_amount < 1
    account: Income:RedPacket

overrides:                             # 单笔覆盖，按 deg-id
  "alipay:2026091922001438…":
    account: Expenses:Gifts
    narration: 给妈妈的生日礼物
```

对比现状：dev-v3 的 cmb-debit 里，"电费"要写成"电费支出"和"电费收入"两条。上面第一条规则对缴费和退费都生效，退费时引擎自动把同一个账户放到另一侧。

### 8.2 账户绑定表

- key 的格式是 `<模板 id>[(所属人)]:<类别>/<值>`，值可以用 `*` 通配。精确匹配优先于通配，带所属人的优先于不带的。
- **本账户找不到绑定时报错**，并列出所有缺失的 key。原因：本账户是唯一不能猜的东西，猜错会让余额整体错位。
- `deg init <账单>` 扫一遍账单，把发现的卡号、付款方式、子账户、槽位、所属人全部写进本地文件的骨架，用户只需要填账户名。
- 多张卡、多种付款方式、多币种子账户、多个钱包地址、多个券商账户、家庭成员的账单，都是同一种写法。不再需要每个 provider 各自发明 `methodAccount`、`cashAccount`、`positionAccount`。

### 8.3 分类规则

**条件语法**

```
expr := expr '||' expr | expr '&&' expr | '!' expr | '(' expr ')' | cmp
cmp  := field op value
op   := ==  !=  ~(包含)  !~  ^=(开头)  $=(结尾)  =~(正则，RE2)  <  <=  >  >=  in [..]
```

- 可用字段：第 5 节的规范字段和派生字段、`source`（模板 id）、`key`（account_key）、`owner`、`raw.列名`、`ctx.名字`。
- **字段有类型**：日期按日期比较，时间按 `HH:MM` 比较，金额按精确小数比较，其余按字符串比较。不会再把日期当成算式。

**动作**

| 动作 | 多条规则命中时 |
|---|---|
| `account` | 对方账户，后命中的覆盖先命中的 |
| `payee` `narration` `flag` | 后者覆盖 |
| `tags` `links` | 累加并去重 |
| `metadata` | 按 key 覆盖 |
| `slots` | 覆盖本笔交易的槽位账户（例如某只 ETF 记到另一个持仓账户） |
| `keep: true` | 把口径默认忽略的这一行捞回来记账 |
| `ignore: <原因>` | 不记这一行，原因写进报告 |
| `split` | 拆到多个对方账户（8.5） |

`keep` 和 `ignore` 是同一个开关的两面，后命中的覆盖先命中的；都没有时取口径的结果。

**语义**

1. 所有规则读的是**同一份不可变的规范交易**，规则之间看不到彼此的输出。规则顺序只决定"同一个字段谁覆盖谁"。
2. 动作的值只能是字面量或简单插值（如 `narration: "{description} - {counterparty}"`），没有算术。
3. 配置里出现不认识的键、字段、运算符、口径名、模板参数，在读账单之前就报错。

**退款的对方账户：** `refunds: keep` 时，退款的对方账户优先取它所配对原单的分类结果（同一批导入里的原单，或 `--existing` 账本里带同一 deg-id 的原单）；找不到原单时按规则走——由于金额带符号，原单命中的那条规则通常也会命中退款。

**为什么这样定：**

- 不可变输入消除了"前一条规则改了 payee，后一条规则因此命中或不命中"的问题，这是现在 v3 行为最难预测的地方。
- 不给本地规则算术和自由文本分录：本地文件越简单，模板升级时越不容易坏；需要算术的地方几乎都是格式知识，应该放在模板里。
- Go 主干那批"字段写了不生效"的 bug，根本原因是 YAML 里不认识的键被静默忽略。加载时校验能一次性消除这一类问题。

### 8.4 单笔覆盖

有些判断只针对某一笔，不构成规律（"这笔淘宝是给妈妈买的礼物"）。为它写一条带金额和日期的规则，既啰嗦又容易误中别的交易。`overrides` 按 deg-id 精确指定某一笔的动作，动作集合与规则相同，优先级高于所有规则。

- `deg review` 处理复核项时，"只改这一笔"写入 `overrides`，"以后都这样"写入 `rules`。
- 重新生成账本时覆盖依然生效，这是"输出可重建"的前提。
- 某个 deg-id 在本次导入的账单里找不到时不报错（覆盖可能属于别的月份），`deg lint` 会列出从未命中的覆盖。

### 8.5 拆分分录

少数场景需要把一笔拆到多个对方账户（例如合租的房租）。用结构化写法，而不是 beancount 文本：

```yaml
  - when: counterparty ~ "房东"
    split:
      - { account: Expenses:Housing:Rent, ratio: 0.5 }
      - { account: Assets:Receivable:Roommate, ratio: 0.5 }
```

比例拆分产生的尾差（分）记到第一个账户，保证平衡。

### 8.6 用户自定义的四个层级

从轻到重，用户应该优先用前面的：

1. **全局口径**（第 7 节）：几个开关，所有账单通用。
2. **分类规则**：按条件批量指定账户、标签、`keep`/`ignore`、拆分。
3. **单笔覆盖**：按 deg-id 改某一笔。
4. **本地模板**：云端模板事实判定错了，或者云端没有这种账单时，在本地放一份模板（第 6.2 节）。同时建议给云端提 issue 或 PR。

模板认错了某一行的 `kind`，属于模板 bug，应该修模板；在本地用规则绕过只是临时办法。

## 9. 交换格式与转换器

### 9.1 交换格式

交换格式是规范交易的 CSV 表示，表头固定，UTF-8。引擎内置一个读取它的模板（id `exchange`），按表头指纹自动识别。任何外部程序只要输出这种 CSV，就能跳过模板，直接进入关联、口径、分类、记账。

| 列 | 必填 | 说明 |
|---|---|---|
| `source` | 是 | 来源名，同一文件内相同，规则里用 `source == "pdd"` 引用 |
| `date` | 是 | `YYYY-MM-DD` |
| `time` `tz` `booked_date` | 否 | |
| `account_key` | 是 | 同第 5 节 |
| `owner` | 否 | |
| `amount` | 是 | 带符号，正数 = 钱进本账户 |
| `currency` | 是 | |
| `original_amount` `original_currency` | 否 | |
| `fee` `fee_currency` `fee_kind` | 否 | 多项费用用 `fee2`、`fee2_currency`、`fee2_kind`…… |
| `counterparty` `counterparty_account` `counterparty_bank` | 否 | |
| `description` `channel` `via` | 否 | |
| `kind` `status` | 否 | 取第 5.2 节的词汇，缺省为 `normal` / `booked`；其他值报错 |
| `id` `order_id` `merchant_order_id` | 否 | `id` 用作去重键 |
| `balance` `balance_currency` | 否 | |
| `trade_security` `trade_side` `trade_qty` `trade_price` `trade_total` | `kind=trade` 时 | |
| `refund_of` `reversal_of` | 否 | 引用同来源另一行的 `id` |

其他列原样进入 `raw`，规则里可以用 `raw.列名` 读取。

**什么时候用交换格式，什么时候写模板：** 账单本身是表格、多人会用、格式会随银行改版的，写云端模板；需要代码才能解析的（PDF、邮件、接口数据）、或者一次性的，由转换器输出交换格式。

### 9.2 转换器仓库

- 每个转换器：代码（语言不限）、README（原始文件怎么获取）、脱敏的原始样例和期望的交换格式 CSV、CI。
- 密码和凭据只通过环境变量或交互输入，不写进任何文件。
- 主要收录"解析用户自己导出的文件"的转换器，例如招行/交行/汇丰的 PDF 账单、银行邮件账单。现有的 bill-file-converter 可以作为起点。
- 抓取平台接口的脚本（例如拼多多订单）可能违反平台条款、接口也随时会变，是否收录、以什么方式收录由维护者决定（第 19 节）。
- 转换器的输出用 `deg check-exchange <file>` 校验，引擎随版本发布交换格式的规范。

## 10. 引擎内置能力

| 类别 | 内容 |
|---|---|
| 读取器 | csv/tsv、xls、xlsx（解析库版本锁定并纳入回归测试）、xml（按元素名或路径选行，属性映射成列）、ofx 1.x/2.x、多段表格、多文件合并、交换格式 |
| 规范化 | 精确小数解析；严格日期（只按模板声明的格式）；时区；内置中文币种名表；`yearFrom` 补年份 |
| 关联（只识别） | 退款（订单号或 ID 前缀 + 金额）；冲正（同账户、金额相反、时间窗口）；聚合（按 key 合并多行，如同一 txHash、新股中签拆成的两行）；配对（跨行、跨文件，按 key、时间窗口、金额或汇率容差）；重复 |
| 口径 | 第 7 节的全部口径 |
| 分录 | `normal` 两腿；有 `fees` 时加费用腿（可以是另一种币种）；`transfer`；`fx`（`@@` 总价）；`trade` 买、卖（成本写总额、保留原始精度、费用腿、卖出盈亏腿）；`dividend` + `tax`；`refund`/`reversal` 按口径冲减 |
| commodity | 校验是否合法（如不能以数字开头）；按模板规则加前缀、补零、映射；不合法就报错 |
| 校验 | 每笔分录平衡；账户不为空；余额断言（按日取最后一行，在次日生成 `balance`）；账单合计核对；`opening: pad` 时生成期初 |
| 输出 | 稳定排序（先按日期，再按文件内顺序）；元数据 `deg-id`、`deg-src`；空元数据不输出；beancount 与 ledger 用同一套分录 |
| open 指令 | 只为 `--existing` 账本里还没有的账户生成，日期取该账户第一次出现的日期，不再一律 1970-01-01 |
| 报告 | 读取、记账、跳过、合并的行数及原因；待复核清单；校验结果；`--strict` 下有未决项就失败；`--report` 输出 JSON |

报告示例：

```
alipay@2026-05-23  bill.csv
  读取 412 行：记账 380，口径忽略 30（交易关闭 20 / 不计收支 10），丢弃 2（零金额）
  关联：退款对 10（keep，已加 link）/ 配对歧义 2
  待复核 17：未分类 14（Expenses:FIXME）/ 配对歧义 2 / 疑似重复 1
  校验：账单合计 收入 1,234.00 / 支出 5,678.90，与表头一致
```

### 10.1 去重与幂等

1. 有账单 ID 就用 ID：`deg-id: "<模板>:<account_key>:<id>"`。
2. 没有 ID 时，用**原始行**的哈希：模板 id、account_key、原始单元格（去首尾空白），再加上"同样内容在本文件里是第几次出现"。
3. **deg-id 不能依赖模板计算出的字段。** 否则模板修了一个解析 bug，同一笔交易的 deg-id 就变了，重新导入会重复记账。
4. 重复导入时用 `--existing main.bean` 读取已有账本里的 `deg-id`，跳过已经导入的交易。
5. 模糊匹配（日期接近、金额相同）**只标记，不自动删除**。

**为什么分级：** 只有账单 ID 能确定是同一笔；哈希能防止同一份文件导两次；模糊匹配误删的代价远高于漏删，所以只提示。

### 10.2 跨来源重复

同一笔消费会同时出现在微信/支付宝账单和银行卡、信用卡流水里（#162）。设计上分两步：

- 模板把银行流水里经由支付平台的行标上 `via`（这是事实，例如摘要以"支付宝-"开头）。
- 口径 `mirrors` 决定保留哪一边（第 7 节）。配对按 `via` + 金额 + 时间窗口进行，只在两边都导入时才生效；配不上的照记并标记，绝不单方面丢弃。

在配对实现之前（路线图 E），用户可以用 `via` 写一条 `ignore` 规则作为过渡。

## 11. 复核与工具

DEG 的默认形态是**不交互的命令行**：脚本、Makefile、CI 都依赖确定的输出和退出码。需要人参与的只有复核，用逐条提问的命令和 JSON 报告解决，不做全屏 TUI；网页前端（如 BeanBridge，基于 WASM）读同一份报告、写同一种决定。

| 命令 | 作用 |
|---|---|
| `deg init <账单>` | 识别模板，列出发现的 account_key、槽位、所属人，生成本地配置骨架 |
| `deg import <账单…>` | 导入；`--existing` 去重；`--strict` 有未决项就失败；`--report` 输出 JSON |
| `deg review` | 按对手方聚合待复核项，逐组提问；回答写入本地配置："以后都这样"写 `rules`，"只这一笔"写 `overrides`，"不记"写 `ignore`。只在交互终端里提问 |
| `deg explain <deg-id 或 文件:行号>` | 打印这一行的完整经过：原始单元格 → 模板判定的 kind/status/via → 关联结果 → 口径 → 命中的规则（按顺序）→ 最终分录 |
| `deg lint` | 检查本地配置：不认识的键、从未命中的规则和覆盖、被后面规则完全遮住的规则、用到 `raw.列名` 而模板已改版的规则 |
| `deg template new <样例账单>` | 按样例生成模板骨架和 fixture，供模板贡献者使用 |
| `deg template test` | 跑模板的全部 fixture：输出与 expected 一致、`bean-check` 通过 |
| `deg upgrade --preview` | 用新的模板 pin 重跑已归档的账单，与当前输出做 diff，确认后再切换 |

**归档原始账单。** 建议用户把原始账单放进账本仓库（可加密）。`deg-src` 记录文件的 sha256，配合模板 pin 和本地配置，任何一笔都能追溯和重建。

复核报告和决定的 JSON 格式随本地协议版本发布，CLI 与网页前端共用。

## 12. 出错策略（fail-closed）

| 情况 | 行为 |
|---|---|
| 认不出文件格式；表头与模板不符 | 报错，列出最接近的模板和缺少的表头。只有模板显式声明 `headerless` 时才按列位置读 |
| 日期、金额解析失败；`map` 找不到值；表达式出错 | 报错，带文件名、行号、原值 |
| 某行映射不出 kind 或方向 | 报错（属于模板 bug）；或者按模板的 `unknown: flag` 记到 FIXME 并标记 |
| 本账户没有绑定 | 报错，列出缺失的 key |
| 分类规则都没命中 | 记到 `defaults` 对应的账户，打 `!` 标记，计入报告 |
| 配对有歧义 | 不配对，标记 |
| 分录不平衡、账户为空、commodity 不合法 | 报错 |
| 尾部行、说明行、零金额占位行 | 按模板的 `drop` 丢弃，按原因计数写进报告 |
| 口径忽略的行 | 不记账，按原因计数写进报告 |
| 配置里有不认识的键、口径名、模板参数 | 加载时报错 |

现在的 `skipInvalidRows`（跳过一切出错的行）拆成两件事：一是模板显式声明的 `drop`，二是真正的错误。后者不能静默。

## 13. 版本与分发

**几个概念要分开：**

| 概念 | 含义 | 例子 |
|---|---|---|
| id | 机构 + 产品 | `icbc-debit` |
| variant | 同一 id 下的格式变体，按指纹**自动**识别 | `icbc-debit` 的 13 列版和 15 列版 |
| pin | 模板包的冻结版本 | `icbc-debit@2026-10-01` |
| 模板 schema | 模板文件格式的版本 | `deg/template/2` |
| 本地协议 | 本地配置格式、统一词汇、口径集合的版本 | `deg-rules/2` |
| 交换格式 | 转换器输出的 CSV 规范的版本 | `deg-exchange/1` |
| capability | 模板或本地文件用到的引擎能力 | `read.xml`、`link.pair` |

现在 icbc 用 `-v1`/`-v2` 两个 id 区分格式，选错也不报错。改成同一个 id 下的变体后，用户不需要知道银行改过几次版。

**registry 与缓存**

- 每个文件带 `sha256`，下载后校验；声明最低引擎版本。
- 本地缓存，可以离线使用；网络请求有超时；`--offline` 只用缓存。
- 输出文件头记录用到的模板和 pin，结果可以复现；`deg upgrade --preview` 在切换 pin 之前展示输出差异。

**模板仓库的 CI**

- 每个 fixture：引擎输出与 expected 一致、`bean-check` 通过、ledger 输出也通过检查。
- lint：模板里不能出现具体账户名（只能用槽位），不能出现个人信息；`rows` 不得用 `drop` 处理交易行；每一行都必须映射到统一词汇。
- 改动导致输出变化时，必须同时更新 expected，并在 PR 里说明原因。
- 银行改版时，新增变体或发布新 pin，`latest` 指向新版本，旧 pin 保留。

## 14. 兼容与迁移

- `translate --provider X` 继续可用。Go provider 冻结，只修 bug；对应模板通过与 Go 输出的对照测试后，改为调用新引擎；最终删除各银行的 Go 代码，只保留通用读取器。
- 提供 `deg migrate`，把旧配置转换成新的本地配置：

| 旧字段（Go config） | 新写法 |
|---|---|
| `defaultCashAccount`，或作本账户用的 `defaultMinusAccount` | `accounts` 绑定 |
| `methodAccount` + `method` | `accounts` 绑定 `method/…` |
| `cashAccount` `positionAccount` `commissionAccount` `pnlAccount` | `accounts` 绑定 `slot/…` |
| `alipay.keepRefundRecords` | `policy.refunds`（`true` → `keep`，`false` → `merge`，写在 `by.alipay` 下以保持原行为） |
| wechat `--ignore-invalid-tx-types` | 模板 `unknown: flag` |
| `peer` | `counterparty ~ …` |
| `item` | `description ~ …` |
| `type` / `txType` / `category` | `raw.列名 == …`，或 `kind == …` |
| `status` / `orderStatus` | `status == …` |
| `time` / `timestamp_range` | `time >= … && time <= …` / `date >= … && date <= …` |
| `minPrice` / `maxPrice` | `abs_amount >= …` / `abs_amount <= …` |
| `sep` + 多个值 | `in [...]` 或 `\|\|` |
| `fullMatch` | `==`（不写则是 `~`） |
| `targetAccount` | `account` |
| `tag` / `tags` | `tags` |
| `ignore` | `ignore: <原因>` |

- dev-v3 的 rules.yaml：收支成对的规则合并成一条；格式知识（服务费拆分、退款判断、"忽略新增证券"）移回模板并改成统一词汇；口径类的规则（忽略交易关闭、丢弃退款）改成 `policy`；模板里写死的账户移到 `accounts`。转换器拿不准的地方输出注释，让用户确认。

## 15. 其他账单格式

| 优先级 | 格式 | 接入方式 |
|---|---|---|
| 1 | OFX / QFX / QBO | 引擎内置读取器 + 一个通用模板，覆盖 BMO、TD 和绝大多数北美银行；自带唯一 ID（FITID）、余额、币种 |
| 2 | XML、多段 CSV | 引擎内置读取器；IBKR Flex 及以后的券商 |
| 3 | PDF / 邮件账单 | 转换器 → 交换格式。中行、交行、招行、民生的借记卡和信用卡，汇丰的电子月结单 |
| 4 | 加密交易所 CSV（Binance、OKX、Coinbase） | 模板 + 聚合（一笔交易拆成多行） |
| 5 | camt.053 / MT940、PayPal、Wise、云闪付 | 视需求；云闪付与银行流水重复，依赖 `via` 和 `mirrors` |
| — | 无官方导出的平台（拼多多等） | 用户自己的脚本或转换器 → 交换格式。这类数据多数是电商订单，付款已在微信/支付宝账单里，见第 18 节"明细来源" |

抖音支付、拼多多、携程，以及光大、平安、邮储等银行，没有找到可靠的导出格式资料，需要用户提供脱敏样例。

## 16. 路线图

| 步骤 | 内容 | 验收 | 完成后可以迁移的模板 |
|---|---|---|---|
| A. Go 主干止血 | alipay 按表头定位；alipay/mt/jd 金额改精确小数；ccb 冲正；cib/spdb 的尾部判断改为只看首列；币种不再静默兜底；hsbchk、huobi 的 panic；配置里不认识的键先告警 | 每个修复带回归样例 | 无（修的是用户正在用的路径） |
| B. v3 引擎 fail-closed | 第 2.2 节的全部静默问题；ledger 目标；空元数据；稳定排序 | verify 改为对照 Go 输出，并跑 `bean-check` | 无 |
| C. 规范交易 + 本地配置 | 带符号金额；统一词汇；全局口径中的 `neutral` `closed` `pending` `date`；`account_key`、所属人与绑定表；不分方向的分类规则；`keep`/`ignore`；单笔覆盖；交换格式；`deg init`、`deg explain`；内置 normal/transfer/fee 分录；`deg-rules/2`；`deg migrate` | 现有 fixture 在新格式下的输出与 B 一致 | bmo×2、td、hsbchk×2、abc_debit、bocom_debit、cmb-debit、mt、wechat |
| D. 读取层 | context、尾部停止、变体识别、值映射、多文件、多段、XML、OFX | icbc v1/v2 合并为一个 id；cmb-credit 不再写死年份 | cmb-credit、icbc×3、ccb、spdb、citic、cgb、boc×2、bocom_credit、jd、alipay |
| E. 关联层 | 退款/冲正识别与 `refunds`/`reversals` 口径、聚合、换汇配对与 `fxPairs`、重复识别、`mirrors`、余额断言、`opening`、合计核对 | 支付宝的部分退款、跨文件退款不再丢失；cib 换汇合并成一笔 | alipay/jd/wechat 的退款处理、cib |
| F. 证券/外汇分录 + commodity | 内置 trade/fx/dividend；成本写总额、保留原始精度；commodity 校验与映射；`discounts` | 投资类 fixture 全部通过 `bean-check` | htsec、hxsec、huobi、ibkr、oklink |
| G. 工具 | `deg review`、`deg lint`、`deg template new/test`、`deg upgrade --preview`；复核报告与决定的 JSON 格式 | BeanBridge 能读同一份报告 | — |
| 并行 | 转换器仓库：交换格式校验器、现有 bill-file-converter 迁过去、第一批 PDF 转换器 | 每个转换器带 fixture 和 CI | 依赖 PDF 的 abc_debit、bocom、cmb、cgb |

A 和 B 可以并行，也可以在本 RFC 定稿之前开始。C 是规则格式的分水岭：之后的步骤只增加能力和口径取值，不再改变用户的写法。**dev-v3 在 C 完成之前不应以当前的规则格式发布**，以免用户依赖一个即将被替换的格式。

## 17. 考虑过的其他方案

| 方案 | 不选的原因 |
|---|---|
| 在现有 v3 规则语言上继续加能力 | 第 2.3 节的根因 1～3 不解决，能力加得越多越难预测 |
| 每家银行继续写 Go provider | 就是现状：同一条管线写 23 遍，bug 和配置写法各不相同 |
| 口径做成每个模板自己的参数 | 用户要逐个账单学参数名，模板之间难以一致；口径本质上是同一小组问题，应由引擎统一定义 |
| 口径做成模板里写死的行为（如"忽略交易关闭"） | 把用户的记账习惯固化进云端，无法满足不同口径的用户 |
| 像 beangulp 那样用 Python 写导入器 | DEG 是单个 Go 二进制并支持 WASM；模板需要作为数据安全地在线分发，代码插件还需要沙箱。需要代码的场景交给转换器 |
| 引擎直接读 JSON、PDF | 每种 JSON/PDF 都要专门的解析逻辑，Go 的 PDF 库也偏弱；转成交换格式后引擎只需维护一种入口 |
| 照搬 hledger CSV rules | 声明式、适合单表 CSV，本 RFC 借鉴了它处理收支两列和条件规则的方式；但它没有跨行处理和文件级上下文 |
| 本地规则保留 postings 文本 | 用户文件会依赖 beancount 语法，没法输出 ledger，也没法做平衡校验；改用结构化的 `split` |
| 复核时直接编辑生成的账本 | 重新导入就丢失，结果不可重建；改为把决定写进 `rules`/`overrides` |
| 全屏 TUI | 需要人参与的只有复核，逐条提问足够；交互界面已有基于 WASM 的网页前端，再做 TUI 等于同一套逻辑写两遍 |
| 机器学习自动分类（如 smart_importer） | 不在本 RFC 范围。规范交易和 `flags` 让它以后可以作为一个可选的分类步骤接入 |

## 18. 以后再做

- **明细来源（enrich）。** 电商订单（拼多多、淘宝、京东订单、美团订单）多数不是资金来源，付款已在微信/支付宝账单里；它们的价值是补充"买了什么"。以后在关联阶段加一种 enrich 关联：按订单号或"金额 + 时间"匹配到已有付款，补 narration、tags，或按商品拆分。规范交易里的 `ids.order`、`channel`、`via` 已经为此留好位置。
- **自动跨来源配对的扩展**：信用卡还款与借记卡扣款、本人不同银行之间的转账，按金额和时间窗口配成一笔 `transfer`。
- **分类建议**：根据已有账本中同一对手方的记账结果给出建议（`deg review` 里已经用到最简单的频次统计），以后可以接入更复杂的模型。

## 19. 待讨论

1. `account_key` 与绑定 key 的写法（`card/1234`、`alipay(owner):…`），以及通配语法。
2. 本账户没有绑定时，是报错，还是记到 `Assets:FIXME:<key>` 并标记？本 RFC 倾向报错。
3. 本地规则能不能读 `raw.列名`？允许的话，本地规则会依赖账单列名，模板改版时可能失效。倾向于允许，由 `deg lint` 在模板改版时提示。
4. 各口径的默认值，尤其是 `refunds`（`keep` 还是与 Go 版支付宝一致的 `merge`）和 `neutral`。
5. 分期（`installment`）怎么建模：原始消费一次记全额负债，还是按每期入账。
6. `--existing` 去重是解析完整账本，还是只扫描 `deg-id` 元数据行。
7. 转换器仓库的名字、归属，以及是否收录抓取平台接口的脚本。
8. `translate` 和 `import` 两个命令最终是否合并。
9. 是否需要英文版。
