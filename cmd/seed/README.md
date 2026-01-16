# Seed 测试数据工具

用于向数据库导入测试数据的命令行工具。

## 编译

```bash
go build -o seed.exe ./cmd/seed
```

## 使用

```bash
seed [选项]
```

### 命令行参数

| 参数 | 短参数 | 默认值 | 说明 |
|------|--------|--------|------|
| `--db-type` | `-d` | sqlite | 数据库类型: sqlite, mysql, postgres |
| `--db-dsn` | `-c` | genimage.db | 数据库连接字符串 |

### 连接字符串示例

**SQLite:**
```bash
seed -d sqlite -c "genimage.db"
```

**MySQL:**
```bash
seed -d mysql -c "user:password@tcp(localhost:3306)/genimage?charset=utf8mb4&parseTime=True&loc=Local"
```

**PostgreSQL:**
```bash
seed -d postgres -c "host=localhost user=postgres password=secret dbname=genimage port=5432 sslmode=disable"
```

## 测试数据

导入完成后将创建以下数据：

### 用户 (23个)

| 邮箱 | 密码 | 角色 |
|------|------|------|
| admin@example.com | Test123456 | 超级管理员 |
| org1_admin@example.com | Test123456 | 组织1管理员 |
| org2_admin@example.com | Test123456 | 组织2管理员 |
| multi_org_admin@example.com | Test123456 | 组织3,4管理员 |
| user1@example.com | Test123456 | 普通用户 |

### 组织 (4个)

| 名称 | 积分 |
|------|------|
| 技术研发部 | 5000 |
| 市场营销部 | 3000 |
| 设计创意部 | 4000 |
| 测试团队 | 2000 |

### 成员关系

- 技术研发部: org1_admin (管理员), user1, user2, test_a1, test_a2
- 市场营销部: org2_admin (管理员), user3, user4, test_b1, test_b2
- 设计创意部: multi_org_admin (管理员), user5, test_c1, test_c2, demo1
- 测试团队: multi_org_admin (管理员), demo2, demo3, demo4

## 注意事项

- 运行此工具会**清空**现有的用户、组织和成员关系数据
- 仅用于开发和测试环境，切勿在生产环境使用
