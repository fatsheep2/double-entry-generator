# RFC 0001 附录

配合 [0001-import-pipeline.md](0001-import-pipeline.md) 阅读。

- 代码链接固定到 double-entry-generator [`97a4f65`](https://github.com/deb-sig/double-entry-generator/tree/97a4f653347dcacfc383580ab00d78d7e8eeda8f) 和 deg-provider-template `dev-v3` [`a00f7c1`](https://github.com/deb-sig/deg-provider-template/tree/a00f7c1ce5bfc7b3b008d9329761d1159df4f505)。
- 标"已复现"的条目是用构造的输入实际跑过的；标"未验证"的是读代码得出的结论或常识，还需要确认。

## A. 逐账单格式特征

| # | 账单 | 文件 | 方向怎么表示 | 币种来源 | 表头前元信息 | 余额 | 唯一 ID | 需要特别处理 |
|---|---|---|---|---|---|---|---|---|
| 1 | alipay | csv，gb18030 | 收/支列（收入/支出/不计收支），金额恒正 | 固定 CNY | 姓名、账户、起止时间、笔数和收支合计 | 无 | 交易订单号、商家订单号 | 退款配对（订单号前缀 + 金额）；交易关闭；不计收支要细分方向；付款方式决定资金账户 |
| 2 | wechat | csv / xlsx | 收/支列（含 `/` 中性交易） | 固定 CNY | 昵称、起止时间、笔数和收支合计；xlsx 多一行时区说明 | 无 | 交易单号、商户单号 | 备注里的服务费要拆出来；退款不配对；`¥`；单元格内有 Tab |
| 3 | jd | csv，BOM | 收/支列（支出/收入/不计收支） | 固定 CNY | 账号、日期区间、合计 | 无 | 交易订单号 | 金额后缀 `(已退款x)`；冻结/解冻；先享后付的 0 元行；付款方式嵌套（"微信-招商银行信用卡"） |
| 4 | mt | csv，BOM | 收/支列 | 固定 CNY | 账号、合计 | 无 | 无 | 订单金额与实付金额；全角 `￥`；退款不配对；与微信账单重复 |
| 5 | abc_debit | csv（由 PDF 转换） | 带符号，必须有 `+`/`-` | 固定 CNY | 无（转换时丢失） | 有 | 日志号（结息行不唯一） | 交易时间可为空；`--` 表示空 |
| 6 | bocom_debit | csv（由 PDF 转换） | 借贷列（`借 Dr` / `贷 Cr`），金额无符号 | 固定 CNY | 无 | 有 | 序号 | Go 把户名和账号拼成 payee |
| 7 | spdb_debit | xls | 存入/取出两列 | 固定 CNY | 账号、账户类型、日期范围（目前被丢弃） | 有 | 顺序号 | 日期和时间在同一格、中间两个空格；尾部"合计" |
| 8 | cib_debit | xlsx/xls，每个币种一个文件 | 收入/支出两列，可以为负（冲正） | 表头前的"卡内账户" | 户名、账号、卡内账户、起止日期 | 有 | 无 | 多文件；购汇两条腿配对；尾部"说明" |
| 9 | ccb | xls（另有 csv 路径） | 收入/支出两列，用 `0` 表示没有 | 表头前"币种"，每行也有币种列（中文） | 开户机构、币种、账号 | 有 | 无 | 对手方打码，真实商户只出现在"交易地点"；冲正 |
| 10 | boc-debit | csv | 带符号 | 币别列（中文） | 标题 | 有 | 无 | 倒序；`----` 占位；Go 把户名和卡号直接拼接 |
| 11 | cmb-debit | csv（由 PDF 转换） | 带符号 | 货币列 | 无 | 有（联机余额） | 无 | 导出时列可选，要按表头名对齐 |
| 12 | icbc-debit v1 | csv | 收入/支出两列 | 币种列（中文） | 卡号、卡别名、子账户序号和类别 | 有 | 无 | 尾部合计；单元格尾随空格和 Tab；行尾逗号 |
| 13 | icbc-debit v2 | csv | 同 v1 | 同 v1 | 同 v1 | 有 | 无 | 比 v1 多"交易详情""对方账户" |
| 14 | cmb-credit | csv（由 PDF 转换） | 带符号，正数 = 消费 | CNY；交易地金额带地区后缀 | 标题里有账单年月 | 无 | 无 | 日期只有 MM/DD，要用账单年月补年份；还款行没有交易日；卡号末四位 |
| 15 | citic | xls | 带符号，正数 = 消费 | 交易币种 / 结算币种（中文） | 标题 | 无 | 无 | 倒序；卡末四位；应按结算金额记账 |
| 16 | bocom_credit | csv（由邮件转换） | 金额无符号，方向看摘要的第一个词 | 金额格里写"币种 金额" | 无 | 无 | 无 | 原币和入账币两列；未知类型 |
| 17 | cgb_credit | csv（由 PDF 转换） | 入账金额带符号，正数 = 消费 | 交易货币 / 入账货币（中文） | 无 | 无 | 无 | 摘要开头括号里写类型 |
| 18 | icbc-credit | csv | 收入/支出两列 | 币种列 | 卡号、卡别名 | 有（负数表示欠款） | 无 | 交易日与记账日 |
| 19 | boc-credit | csv | 存入/支出两列 | 币种列 | 标题里有账单年月 | 无 | 无 | 一份账单多张卡；Go 用记账日 |
| 20 | bmo-credit | csv | 带符号，正数 = 消费 | 无（CAD） | 1 行说明 + 空行 | 无 | 无（`Item #` 只是序号） | 卡号用单引号包住 |
| 21 | bmo-debit | csv | 带符号，另有 CREDIT/DEBIT 列 | 无 | 1 行说明 + 空行 | 无 | 无 | 表头带前导空格；描述右侧补空格 |
| 22 | td | csv，无表头 | 取出/存入两列 | 无 | 无 | 有 | 无 | 日期 mm/dd/yyyy；倒序 |
| 23 | hsbchk-credit | csv | 带符号，另有 Credit/Debit 列 | 每行 Billing currency | 无 | 无 | 无 | 金额后面带 Tab、有千分位；日期 dd/mm；有状态列 |
| 24 | hsbchk-debit | csv | 带符号 | 每行一个币种，一个文件里混多个币种 | 无 | 有（含余额币种） | 无 | 日期 dd/mm；按币种分子账户；账户内换汇 |
| 25 | htsec | xlsx | 操作列（买/卖） | 无（CNY） | 无 | 有（资金、股份） | 合同号、成交号 | 新股中签拆成两行；代码要补零；四项费用；按股东账号判断 SH/SZ |
| 26 | hxsec | 扩展名 xls，实为 GBK 编码的 TSV | 业务类型列 | 有币种列（Go 未使用） | 无 | 有 | 成交编号 | `="…"`；逆回购；ETF 份额合并；红利 |
| 27 | huobi | csv，BOM | 买入/卖出 | 交易对 | 无 | 无 | 无 | 手续费是"数字+币种"连在一起；`1INCH` 这类以数字开头的币 |
| 28 | oklink | csv | 由用户自己的地址推出 | 代币符号（未认证代币是合约地址） | 无 | 无 | txHash | 同一 txHash 多条记录；ETH/TRON 两种表头；不含 gas |
| 29 | ibkr | xml（Flex Query） | `quantity` 符号 + `buySell` | `currency`；佣金有自己的币种 | 多个 FlexStatement（多账户） | 无 | tradeID、transactionID | 多种资产类别；外汇交易；股息与预扣税 |

## B. 能力矩阵

| 能力 | 需要它的账单（编号见 A） | runtime 引擎现状 |
|---|---|---|
| 读取表头前元信息（context） | 14（账单年月）、12/13/18（卡号、别名）、9（币种、账号）、8（币种）、7（账号）；1/2/3（合计数，用于核对） | 没有，模板只能写死 |
| 尾部停止标记 | 7、8、9、12、13 | 没有，靠 `skipInvalidRows` 把所有行级错误一起吞掉 |
| 按指纹自动识别格式变体 | 12/13、10/19、11/14、20/21、2（csv/xlsx） | 没有，拆成多个 id，选错不报错 |
| 保留金额符号 | 全部；冲正尤其依赖（8、9） | 取绝对值 |
| 值映射表 | 中文币种名（8、9、10、12、15、17）；类型 → 方向（16、17、26） | 没有 |
| 严格日期、时区、补年份 | 22、23、24（日月顺序）；14（补年份）；全部（时区） | 未声明格式时逐行猜；时区依赖本机 |
| 退款配对 | 1、2、3、4、23 | 没有 |
| 多行聚合 | 25（中签两行）、28（同一 txHash） | 没有 |
| 跨行/跨文件配对 | 8（购汇）、24（换汇）、本人账户互转 | 没有，只能单文件 |
| XML、多段表格 | 29 | 没有 |
| 证券/外汇分录、commodity 清洗 | 25～29 | 手写 beancount 文本 |
| 余额断言 | 5～13、18、22、24、25、26 | 没有 |
| 去重 ID | 1、2、3、25、26、28、29 | 没有 |
| 由本人地址推出方向 | 28 | 只能在个人规则里写死地址 |

## C. 已知问题清单

修复建议都只有一句，展开讨论建议各自开 issue。

### C.1 Go provider（master `97a4f65`）

链接前缀：`https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/`

| 级别 | 位置 | 问题 | 建议 |
|---|---|---|---|
| P0 | [cib_debit/cib_debit.go#L107](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/cib_debit/cib_debit.go#L107)、[#L184](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/cib_debit/cib_debit.go#L184) | 整行任意单元格含"说明"就停止读取，后面的交易静默丢失（已复现） | 只在首列等于"说明"时停止 |
| P0 | [ccb/parse.go#L116-L164](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/ccb/parse.go#L116-L164) | 支出列为负（冲正）时金额被置为 0（已复现） | 按 收入 − 支出 计算带符号金额 |
| P0 | [hsbchk/parse.go#L23](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/hsbchk/parse.go#L23)、[#L80](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/hsbchk/parse.go#L80) | 只检查 `len < 5`，却读取 `line[5]`，5 列文件直接 panic（已复现） | 检查改为 `< 6`，或让余额币种可选 |
| P0 | [huobi/parse.go#L60-L61](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/huobi/parse.go#L60-L61) | 手续费为空时数组越界 panic（已复现） | 先判空 |
| P1 | [spdb_debit/spdb_debit.go#L103](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/spdb_debit/spdb_debit.go#L103) | 整行含"合计"就停止读取 | 只看首列 |
| P1 | [alipay/alipay.go#L82](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/alipay/alipay.go#L82) | 写死跳过 22 行。deg-provider-template 里的样例账单多一行说明，master 解析失败（已复现） | 按表头内容定位 |
| P1 | [alipay/parse.go#L36](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/alipay/parse.go#L36)、[mt/parse.go#L29](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/mt/parse.go#L29) | 用 float32 解析金额，1234567.89 变成 1234567.88（已复现）；mt 按字节切掉 `¥`，遇到全角 `￥` 报错 | 改用精确小数，统一清洗货币符号 |
| P1 | [jd/jd.go#L197](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/jd/jd.go#L197) | 删掉小数点后按整数解析，`50`、`50.5` 这类金额会差 10～100 倍（京东是否导出这种金额未验证） | 改用精确小数 |
| P1 | [cib_debit/parse.go#L10-L20](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/cib_debit/parse.go#L10-L20) | 不认识的币种一律当 CNY | 用币种表映射，认不出就报错 |
| P1 | [boc/parse.go#L21-L23](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/boc/parse.go#L21-L23) | 非人民币一律当 USD（港币被记成 USD，已复现） | 同上 |
| P1 | [citic/parse.go#L31-L35](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/citic/parse.go#L31-L35) | 币种取结算币种，金额却取交易金额，外币交易金额错误；币种也没传进 IR | 用结算金额记账，原币写进 metadata |
| P1 | [icbc/convert.go#L13-L21](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/icbc/convert.go#L13-L21) | 币种没有进 IR，美元交易被记成 CNY（已复现） | 映射成 ISO 后写进 `Currency` |
| P1 | [analyser/citic#L70-L74](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/analyser/citic/citic.go#L70-L74)、[analyser/bocom_credit#L70-L74](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/analyser/bocom_credit/bocom_credit.go#L70-L74)、[analyser/bmo#L69-L74](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/analyser/bmo/bmo.go#L69-L74)、[analyser/td#L69-L74](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/analyser/td/td.go#L69-L74) | 只匹配 `item`/`type`，`peer` 不参与匹配，只写 `peer` 的规则会匹配全部交易；bocom_credit 一条 `peer` + `ignore` 规则丢掉整个文件（已复现）。这几家的文档示例用的都是 `peer` | 实现该字段，或者加载时对不支持的字段报错 |
| P1 | [analyser/icbc#L73-L88](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/analyser/icbc/icbc.go#L73-L88) | 只用了 `peer`/`type`/`txType`/`status`。`status` 依赖从未写入的 metadata，永远不命中；只写 `item` 的规则命中全部交易（已复现） | 同上 |
| P1 | [analyser/abc_debit#L43-L56](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/analyser/abc_debit/abc_debit.go#L43-L56) | 没有规则时返回 DefaultMinus/Plus，有任意规则后改用 defaultCashAccount。bmo、bocom_credit、ccb、citic、cmb、hsbchk、icbc、td 是同一写法（未逐一附行号） | 不论有没有规则，都先按方向套用本账户 |
| P1 | [hsbchk/config.go#L16](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/hsbchk/config.go#L16) | 用的是 `yaml:` tag，viper/mapstructure 读不到 `sep`（已复现）；oklink 同理 | 改用 `mapstructure:` tag |
| P1 | [hsbchk/parse.go#L118-L120](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/hsbchk/parse.go#L118-L120) | DEBIT 一律取负；金额符号与方向列不一致时输出 `--12.00`（已复现） | 用带符号金额，方向冲突时报错 |
| P1 | [bmo/bmo.go#L57-L68](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/bmo/bmo.go#L57-L68) | 按行号区分信用卡和借记卡；没有前导说明行时丢一行，并把入账日期当成金额（已复现） | 按表头内容判断 |
| P1 | [ibkr/ibkr.go#L86-L87](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/ibkr/ibkr.go#L86-L87) | 不是 STK/CASH 的资产类别（OPT、FUT、BOND…）被静默丢弃（已复现） | 告警或报错 |
| P1 | [ibkr/ibkr.go#L136-L137](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/ibkr/ibkr.go#L136-L137) | 佣金按成交币种记账，忽略 `ibCommissionCurrency`；外汇交易的佣金没有入账（已复现） | 佣金单独一条腿，用自己的币种 |
| P1 | [compiler/beancount/template.go#L143](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/compiler/beancount/template.go#L143) | 证券成本按单价 `%.3f`、数量 `%.2f` 输出，碎股和高精度价格不平（ibkr、huobi 已复现）；htsec、hxsec、ibkr 共用这个模板 | 成本写总额，保留原始精度 |
| P1 | [oklink/oklink.go#L283](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/oklink/oklink.go#L283) | 未认证代币的合约地址被直接当作 commodity，样例输出约 940 笔语法错误 | commodity 映射或清洗，或者默认忽略 |
| P1 | [oklink/oklink.go#L916](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/oklink/oklink.go#L916) | 规则里的 `currency` 会把资产腿的币种一起改掉（已复现） | 删掉该字段，或只作用于对方腿并加价格 |
| P1 | [htsec/htsec.go#L80-L109](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/htsec/htsec.go#L80-L109) | WASM 入口没有做中签两行合并，CLI 和 WASM 结果不同 | 抽成公共函数 |
| P1 | [compiler/ledger](https://github.com/deb-sig/double-entry-generator/tree/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/compiler/ledger) | 不处理 runtime 模板产生的 Postings，`import -t ledger` 输出没有账户的分录（已复现） | 与 beancount 编译器共用结构化分录 |
| P2 | [oklink/oklink.go#L149](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/oklink/oklink.go#L149) | 解析失败的行只打日志就跳过 | 报错，或在汇总里计数 |
| P2 | [wechat/parse.go#L15](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/wechat/parse.go#L15) | 服务费取备注里第一个两位小数，备注里有别的数字时会取错 | 锚定"服务费"之后的数字 |
| P2 | [bocom_credit/util.go#L89](https://github.com/deb-sig/double-entry-generator/blob/97a4f653347dcacfc383580ab00d78d7e8eeda8f/pkg/provider/bocom_credit/util.go#L89) | 一个未知类型（例如年费）就让整个文件失败 | 记到 FIXME 并标记，或者补全类型 |

另外还有：`tag` 与 `tags`、`status` 与 `orderStatus` 并存；`time` 在 oklink 里表示日期区间；hxsec 首条命中即停，其他 analyser 是后命中覆盖先命中。多家文档示例里的字段或取值实际不存在（htsec、hxsec、huobi、mt、ccb 等）。

### C.2 runtime 模板引擎

"master" 一列是在 `97a4f65` 上复现的结果。"未核对"表示只在开发中引擎上复现过。

| 问题 | master |
|---|---|
| `forceAmountDirection` 把金额取绝对值，冲正方向反了 | 同样存在 |
| 没有规则设置 from/to 时，输出没有账户的分录 | 同样存在 |
| posting 引用不存在的列，生成省略金额的腿；不做平衡校验 | 同样存在 |
| `import -t ledger` 分录里的账户丢失 | 同样存在 |
| 未声明 `dateFormat` 时逐行猜日期格式，`01/02` 优先于 `02/01` | 同样存在 |
| 日期解析失败只报 "runtime v2 rule did not set date"，不带行号和原值 | 同样存在 |
| `skipInvalidRows` 吞掉所有行级错误；表头不一致时把首行当数据、按位置映射。icbc v1 模板 + v2 账单：退出码 0、0 笔交易 | 同样存在 |
| 未知方法、参数解析失败时静默丢弃，参数原样拼进结果 | 同样存在 |
| 条件里 ISO 日期被当作算式比较 | 不存在，仅开发中引擎 |
| 空 metadata 输出成 `key: ""` | 不存在，仅开发中引擎 |
| 编译器总是输出 payee 和 narration 两段 | 仅开发中引擎（新行为更符合 beancount 语义，但 expected 需要重新生成） |
| `<` 运算符与 `<列名>` 写法冲突：`amount < 100 && <x> == "a"` 报错 | 未核对 |
| 加法让小数位膨胀（`0.85000000`）；`format` 不支持宽度和补零，失败时返回原值 | 未核对 |
| 金额为正时类型推断仍然返回"支出" | 未核对 |
| 日期解析依赖本机时区 | 未核对 |
| registry：不校验 sha256，没有缓存和超时，不检查 versions 列表 | 未核对 |
| 开发分支的 excelize 是 2.5.0（master 已是 2.11.0）。合并 master 后 htsec 仍不一致：根因是 `.format("%06.0f")` 不支持补零、静默返回原值 | `.format` 问题同样存在；excelize 已在 deb-sig `dev-v3` 合并 master 时升级 |

### C.3 dev-v3 模板

链接前缀：`https://github.com/deb-sig/deg-provider-template/blob/a00f7c1ce5bfc7b3b008d9329761d1159df4f505/`

| 模板 | 问题 |
|---|---|
| [cmb-credit rules.yaml#L8](https://github.com/deb-sig/deg-provider-template/blob/a00f7c1ce5bfc7b3b008d9329761d1159df4f505/cmb-credit/latest/rules.yaml#L8)、[#L17](https://github.com/deb-sig/deg-provider-template/blob/a00f7c1ce5bfc7b3b008d9329761d1159df4f505/cmb-credit/latest/rules.yaml#L17) | 年份写死 `2024/<记账日>`，2025 年的账单被记成 2024 年（已复现） |
| [ccb rules.yaml#L15](https://github.com/deb-sig/deg-provider-template/blob/a00f7c1ce5bfc7b3b008d9329761d1159df4f505/ccb/latest/rules.yaml#L15) | `accountNum` 写死样例值 |
| [icbc-debit-v2 rules.yaml#L12](https://github.com/deb-sig/deg-provider-template/blob/a00f7c1ce5bfc7b3b008d9329761d1159df4f505/icbc-debit-v2/latest/rules.yaml#L12) | `cardName` 写死样例值 |
| [mt rules.yaml#L72](https://github.com/deb-sig/deg-provider-template/blob/a00f7c1ce5bfc7b3b008d9329761d1159df4f505/mt/latest/rules.yaml#L72) | starter 规则里有带个人 ID 的账户名（同文件还有两处） |
| bmo-*、td、hsbchk-*、abc_debit、htsec、hxsec、huobi 等 | 本人账户写死在 templateRules 里 |
| td | 缺 `dateFormat`，同一文件混用日/月与月/日（已复现） |
| hsbchk-debit | 描述放进了 payee，应放 narration（verify DIFF 的原因） |
| icbc-* | 没有设置 narration；用 `skipInvalidRows` 当作尾部过滤器 |
| bocom_credit | 用子串判断方向，"退货 消费退款"同时命中收入、支出两条规则，最后被记成支出（已复现）；未知类型输出没有账户的分录 |
| citic-credit、boc-debit | 照抄了 Go 的币种 bug（交易金额 + 写死 CNY；非人民币当 USD） |
| alipay | 用 `交易订单号 ~ _` 丢弃退款，部分退款和跨文件退款静默消失（已复现）；缺 `options.title`；`skipLeadingRows: 22` 换成表头定位后新旧账单都能解析 |
| wechat | 服务费拆分写在 personalRules 里，按银行复制了三份；只支持 xlsx |
| jd | 用 `.number` 解析 `131.77(已退款89.84)` 失败（verify FAIL）；只为凑样例的 payee 规则放在了 templateRules |
| spdb_debit | expected 是用模板重新生成的，与 Go 输出相比丢了 tags |
| hxsec | `sourceHeaders` 用英文别名加位置映射，真实中文表头完全不校验 |
| huobi | 同币种手续费从现金账户扣，照抄了 Go 的写法；成本按单价写，bean-check 不平（已复现） |
| 大部分模板 | 个人规则收入、支出各写一条（条数见 RFC 2.2） |

## D. 外部格式与工具参考

### D.1 格式

| 格式 | 要点 | 参考 |
|---|---|---|
| OFX / QFX / QBO | 1.x 是 SGML，2.x 是 XML；`TRNAMT` 带符号；`FITID` 唯一；`LEDGERBAL` 余额；有银行把信用卡的符号导反 | [ofxtools](https://ofxtools.readthedocs.io/en/latest/parser.html)、[信用卡符号反转的讨论](https://www.reddit.com/r/plaintextaccounting/comments/st8bv3/credit_card_ofx_export_file_has_inverted/) |
| QIF | 没有币种、没有 ID；投资交易的数量永远为正，方向看动作 | [Wikipedia](https://en.wikipedia.org/wiki/Quicken_Interchange_Format) |
| camt.053 / MT940 | 金额无符号，方向由借贷标识决定；有期初、期末余额；一个文件可以有多个账户 | [camt.053](https://www.sepaforcorporates.com/swift-for-corporates/a-practical-guide-to-the-bank-statement-camt-053-format/)、[MT940](https://www.sepaforcorporates.com/swift-for-corporates/account-statement-mt940-file-format-overview/) |
| BAI2 | 按记录类型分层；方向由类型码决定；88 号是续行记录 | [Huntington](https://developer.huntington.com/enterprisepayments/docs/bai2) |
| IBKR Flex | 多个段、多个账户；佣金有自己的币种；IBKR 会新增字段，解析时宜宽松 | [ibflex](https://github.com/csingley/ibflex) |
| 开放银行 API | 各家符号约定相反（Plaid 正数 = 流出，SimpleFIN 正数 = 流入）；pending 与 booked 分开 | [Plaid](https://plaid.com/docs/api/products/transactions/)、[SimpleFIN](https://www.simplefin.org/protocol.html)、[GoCardless](https://developer.gocardless.com/bank-account-data/transactions) |
| 国内账单导出方式 | 多家银行只提供加密 PDF 或邮件账单；招行已出账单是 PDF | [china_bean_importers](https://github.com/jiegec/china_bean_importers)、[各平台账单导出方法](https://blog.triplez.cn/posts/bills-export-methods/)、[Beancount-Trans 文档](https://trans.dhr2333.cn/docs/%E6%93%8D%E4%BD%9C%E6%8C%87%E5%8D%97/bills-export-methods/) |

### D.2 工具

| 工具 | 本 RFC 借鉴的地方 | 参考 |
|---|---|---|
| beangulp | 导入器接口分成 identify / extract；按日期窗口和金额判断重复 | [importer.py](https://github.com/beancount/beangulp/blob/master/beangulp/importer.py)、[similar.py](https://github.com/beancount/beangulp/blob/master/beangulp/similar.py) |
| smart_importer | 分类可以作为一个独立的可选步骤 | [repo](https://github.com/beancount/smart_importer) |
| beancount-import | 未知的一腿记 FIXME；跨数据源合并同一笔交易；复核界面 | [repo](https://github.com/jbms/beancount-import) |
| hledger CSV rules | 收支两列、条件规则、余额断言的声明式写法；也说明了只用"最新日期"做去重的局限 | [manual](https://hledger.org/1.40/hledger.html#csv) |
| ledger-autosync | 每条分录带来源 ID，用于幂等导入 | [repo](https://github.com/egh/ledger-autosync) |
| Firefly III | 给列指定角色；按外部 ID 或内容哈希去重 | [roles](https://docs.firefly-iii.org/references/data-importer/roles/)、[duplicates](https://docs.firefly-iii.org/references/data-importer/duplicate-detection/) |
| Actual Budget | 导入的原始 payee 永远不变；规则分阶段执行 | [rules](https://actualbudget.org/docs/budgeting/rules/)、[importing](https://actualbudget.org/docs/transactions/importing/) |
| GnuCash | 记住"文件账户 → 账本账户"的映射；导入前逐条确认 | [manual](https://gnucash.org/docs/v5/C/gnucash-manual/trans-import.html) |
| BeanHub | 解析和分类分成两个库；交易带 import-id，可以原地更新 | [blog](https://beanhub.io/blog/2024/05/27/introduction-of-beanhub-import/)、[beanhub-extract](https://github.com/LaunchPlatform/beanhub-extract) |
| china_bean_importers | 用白名单/黑名单避免支付 App 与银行卡流水重复记账；按卡号末四位找账户；PDF 密码 | [repo](https://github.com/jiegec/china_bean_importers) |
