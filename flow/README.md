# flow

一个简洁、可扩展的文本流水线：约 400 行无第三方依赖的 Go 代码。

`flow` 从 stdin 按行读取，按顺序经过一串已注册的 stage，再把结果写到
stdout。stage 都放在 registry 里，新增一个 stage 只需注册一个函数，引擎
本身无需改动。

```console
$ printf 'banana\nApple\nbanana\n' | go run ./flow -steps 'sort,uniq,upper'
APPLE
BANANA
```

## 用法

```console
$ printf '  alice \n  bob\n' | go run ./flow -steps 'trim,upper,prefix:- '
- ALICE
- BOB
```

```console
$ printf 'alice 30\nbob 25\n' | go run ./flow -steps 'field:2: ,sort,number:> '
1> 25
2> 30
```

```console
$ printf 'the quick brown fox jumps over the lazy dog\n' | go run ./flow -steps 'wrap:20'
the quick brown fox
jumps over the lazy
dog
```

列出所有已注册的 stage：

```console
$ go run ./flow -list
```

stage 之间用逗号分隔。第一个冒号把 stage 名称和参数分开，参数会原样
传递，所以给 `prefix:> ` 加上引号时，末尾空格会保留。由于逗号用于分隔
stage，参数中不能包含逗号。

## 内置 stage

| Stage | 参数 | 作用 |
| --- | --- | --- |
| `upper` / `lower` | — | 转换大小写。 |
| `trim` | — | 去掉行首和行尾空白。 |
| `reverse` | — | 反转每行的 rune 顺序。 |
| `prefix` / `suffix` | `TEXT` | 在每行前后追加文本。 |
| `replace` | `OLD:NEW` | 将所有 `OLD` 替换为 `NEW`；`NEW` 可以包含冒号。 |
| `number` | `[SEP]` | 添加行号，默认分隔符为 `": "`。 |
| `grep` / `drop` | `TEXT` | 只保留包含 `TEXT` 的行，或删除这些行。 |
| `field` | `N:SEP` | 按 `SEP` 分割后保留第 `N` 列（从 1 开始）。 |
| `sort` / `rsort` | — | 按升序或降序排列行。 |
| `uniq` | — | 去重并保留首次出现的顺序。 |
| `head` / `tail` | `N` | 保留前 `N` 行或后 `N` 行。 |
| `wrap` | `N` | 按空格在 `N` 个 rune 处换行，不拆单词。 |
| `join` | `SEP` | 用 `SEP` 把所有行合并为一行。 |
| `rot13` | — | `custom.go` 中的自定义 stage 示例。 |

## 扩展方式

`custom.go` 是扩展示例。`rot13` stage 在 `init` 中注册，因此无需改动引擎
就能用于所有流水线：

```go
func init() {
	Must(Default.Register("rot13", Simple(rot13Encode)))
}
```

带参数的 stage 则从原始参数字符串构造 `Stage`：

```go
func init() {
	Must(Default.Register("repeat", repeatStep))
}

func repeatStep(arg string) (Stage, error) {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("want repeat:N")
	}
	return PerLine(func(line string) (string, error) {
		return strings.Repeat(line, n), nil
	}), nil
}
```

`Simple` 用于适配无参的 `func(string) string`，`PerLine` 用于适配单行
变换。stage 也可以直接处理整个行流（`sort`、`uniq`、`head`、`wrap`
就是这样），因此同一个接口既能完成逐行过滤，也能完成重排。

把新 stage 加到 `steps.go` 或新文件中，再运行 `go run ./flow -list`
确认注册成功。

## 设计

- `Registry` 把 stage 名称映射到 `Definition` 函数。
- `Compile` 一次性解析 spec 列表；`Pipeline.Run` 依次应用各个 stage。
- 重复、非法或为 nil 的注册会立即报错，不会静默覆盖已有 stage。
- 错误信息包含 stage 序号；逐行 stage 还会包含出错的行号。

## 测试

```console
$ go test ./...
```
