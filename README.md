# appinnCBot



## 配置文件

主程序会从 `config.json` 读取配置，并在文件变更后自动热重载：

```json
{
	"username_patterns": ["..."],
	"meaningless_patterns": ["..."],
	"whitelist_domains": ["appinn.com", "appinn.net", "github.com"]
}
```

说明：
- `username_patterns` 用于用户名命中检查。
- `meaningless_patterns` 用于首条消息内容命中检查。
- `whitelist_domains` 用于链接白名单检查。
- 不再提供内置默认值；请在 `config.json` 中显式配置所需字段。

