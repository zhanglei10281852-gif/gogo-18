# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA，再应用仓库内的可信测试补丁；不要在当前修复结果源码上期待重新出现修复前失败。

## 问题现象

搜索会静默接受明显损坏的查询，请帮我修复。

未闭合引号/括号、尾随 AND/OR/NOT、单独 NOT、空括号、多余右括号和查询后的垃圾都不报 Parse error，而是返回不完整 AST、错误结果甚至后续异常。

期望 Parser.Parse 对所有不完整或未完全消费的输入返回带位置上下文的稳定错误，不 panic；合法嵌套、隐式 AND、短语和前缀保持兼容。修复后保证 go test ./... 全绿。

## 含 Bug 版本

- 仓库：zhanglei10281852-gif/gogo-18
- 仓库地址：https://github.com/zhanglei10281852-gif/gogo-18.git
- parent SHA：9769e661a0856fcd785d130c9546f2e7e02b4386

## 复现步骤

```bash
git clone -- https://github.com/zhanglei10281852-gif/gogo-18.git bug-repro
cd bug-repro
git checkout --detach 9769e661a0856fcd785d130c9546f2e7e02b4386
git apply ../BENZHI_VALIDATION/trusted-test.patch
go test ./internal/query -run "^TestParserParse" -count=1 -v
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/query -run "^TestParserParse" -count=1 -v
=== RUN   TestParserParseRejectsMalformedQueries
=== RUN   TestParserParseRejectsMalformedQueries/unterminated_phrase
    parser_parse_test.go:37: Parser.Parse("\"alpha beta") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/unterminated_group
    parser_parse_test.go:37: Parser.Parse("alpha AND (beta OR gamma") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/trailing_AND
    parser_parse_test.go:37: Parser.Parse("alpha AND") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/trailing_OR
    parser_parse_test.go:37: Parser.Parse("alpha OR") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/trailing_NOT
    parser_parse_test.go:37: Parser.Parse("alpha NOT") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/lone_NOT
    parser_parse_test.go:37: Parser.Parse("NOT") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/empty_group
    parser_parse_test.go:37: Parser.Parse("()") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/whitespace-only_group
    parser_parse_test.go:37: Parser.Parse("(   )") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/extra_closing_parenthesis
    parser_parse_test.go:37: Parser.Parse("alpha)") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/garbage_after_query
    parser_parse_test.go:37: Parser.Parse("(alpha)) garbage") returned no error
--- FAIL: TestParserParseRejectsMalformedQueries (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/unterminated_phrase (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/unterminated_group (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/trailing_AND (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/trailing_OR (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/trailing_NOT (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/lone_NOT (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/empty_group (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/whitespace-only_group (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/extra_closing_parenthesis (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/garbage_after_query (0.00s)
=== RUN   TestParserParseValidSyntaxRegression
=== RUN   TestParserParseValidSyntaxRegression/nested_boolean_expression
=== RUN   TestParserParseValidSyntaxRegression/implicit_AND_with_phrase_and_prefix
--- PASS: TestParserParseValidSyntaxRegression (0.00s)
    --- PASS: TestParserParseValidSyntaxRegression/nested_boolean_expression (0.00s)
    --- PASS: TestParserParseValidSyntaxRegression/implicit_AND_with_phrase_and_prefix (0.00s)
FAIL
FAIL	github.com/localsearch/cli/internal/query	0.002s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/query -run "^TestParserParse" -count=1 -v
=== RUN   TestParserParseRejectsMalformedQueries
=== RUN   TestParserParseRejectsMalformedQueries/unterminated_phrase
    parser_parse_test.go:37: Parser.Parse("\"alpha beta") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/unterminated_group
    parser_parse_test.go:37: Parser.Parse("alpha AND (beta OR gamma") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/trailing_AND
    parser_parse_test.go:37: Parser.Parse("alpha AND") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/trailing_OR
    parser_parse_test.go:37: Parser.Parse("alpha OR") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/trailing_NOT
    parser_parse_test.go:37: Parser.Parse("alpha NOT") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/lone_NOT
    parser_parse_test.go:37: Parser.Parse("NOT") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/empty_group
    parser_parse_test.go:37: Parser.Parse("()") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/whitespace-only_group
    parser_parse_test.go:37: Parser.Parse("(   )") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/extra_closing_parenthesis
    parser_parse_test.go:37: Parser.Parse("alpha)") returned no error
=== RUN   TestParserParseRejectsMalformedQueries/garbage_after_query
    parser_parse_test.go:37: Parser.Parse("(alpha)) garbage") returned no error
--- FAIL: TestParserParseRejectsMalformedQueries (0.02s)
    --- FAIL: TestParserParseRejectsMalformedQueries/unterminated_phrase (0.01s)
    --- FAIL: TestParserParseRejectsMalformedQueries/unterminated_group (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/trailing_AND (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/trailing_OR (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/trailing_NOT (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/lone_NOT (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/empty_group (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/whitespace-only_group (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/extra_closing_parenthesis (0.00s)
    --- FAIL: TestParserParseRejectsMalformedQueries/garbage_after_query (0.00s)
=== RUN   TestParserParseValidSyntaxRegression
=== RUN   TestParserParseValidSyntaxRegression/nested_boolean_expression
=== RUN   TestParserParseValidSyntaxRegression/implicit_AND_with_phrase_and_prefix
--- PASS: TestParserParseValidSyntaxRegression (0.00s)
    --- PASS: TestParserParseValidSyntaxRegression/nested_boolean_expression (0.00s)
    --- PASS: TestParserParseValidSyntaxRegression/implicit_AND_with_phrase_and_prefix (0.00s)
FAIL
FAIL	github.com/localsearch/cli/internal/query	0.140s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

全部坏输入返回含 position 的 error 且不 panic；合法语法 AST 回归通过；双架构定向/全量/build/vet 通过。
