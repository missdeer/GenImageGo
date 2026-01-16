# adduser

命令行工具，用于向数据库添加超级管理员用户。

## 编译

```bash
go build -o adduser.exe ./cmd/adduser/
```

## 使用

```bash
adduser --email <邮箱> (--password <密码> | --password-stdin) [--db-type <类型>] [--db-dsn <连接字符串>]
```

## 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--email` | (必填) | 用户邮箱 |
| `--password` | - | 用户密码 |
| `--password-stdin` | `false` | 从标准输入读取密码 (更安全) |
| `--db-type` | `sqlite` | 数据库类型: `sqlite`, `mysql`, `postgres` |
| `--db-dsn` | `genimage.db` | 数据库连接字符串 |

## 示例

### SQLite

```bash
# 直接传入密码
./adduser.exe --email admin@example.com --password MySecurePass123

# 从标准输入读取密码 (推荐，密码不会出现在命令历史中)
echo "MySecurePass123" | ./adduser.exe --email admin@example.com --password-stdin
```

### MySQL

```bash
./adduser.exe \
  --db-type mysql \
  --db-dsn "user:password@tcp(localhost:3306)/genimage?parseTime=true" \
  --email admin@example.com \
  --password MySecurePass123
```

### PostgreSQL

```bash
./adduser.exe \
  --db-type postgres \
  --db-dsn "host=localhost user=postgres password=secret dbname=genimage sslmode=disable" \
  --email admin@example.com \
  --password MySecurePass123
```

## 用户属性

创建的用户自动设置为：

- 邮箱已验证 (`email_verified = true`)
- 超级管理员权限 (`type = 1`)
