# appinnCBot

## 迁移说明（SQLite）

### 启动参数
- `BOT_TOKEN`：Telegram Bot Token（必填）。
- `SQLITE_PATH`：SQLite 数据库文件路径（可选，默认 `appinn.db`）。

示例：

```bash
export BOT_TOKEN="<your-bot-token>"
export SQLITE_PATH="./appinn.db"
./appinnCbot
```

### 迁移命令（seens.json -> SQLite）
先备份：

```bash
cp seens.json seens.json.bak
```

执行迁移：

```bash
go run ./cmd/migrate-seens -json seens.json -db appinn.db
```

说明：
- 旧 `seens.json` 中“存在记录”的用户会迁移为 `count=1`。
- 迁移命令是幂等的，重复执行不会把已迁移用户计数继续累加。

### 回滚说明
若迁移后需要回滚：
1. 停止服务进程。
2. 删除或替换当前 SQLite 文件（默认 `appinn.db`）。
3. 使用备份恢复旧数据文件：

```bash
cp seens.json.bak seens.json
```

4. 启动旧版本程序（使用 JSON 存储逻辑的版本）。

> 当前代码已使用 SQLite 作为运行时状态存储，`seens.json` 不再被主程序实时读写。
