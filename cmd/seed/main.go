package main

import (
	"fmt"
	"os"
	"time"

	"genimage/model"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbType := pflag.StringP("db-type", "d", "sqlite", "数据库类型: sqlite, mysql, postgres")
	dbDSN := pflag.StringP("db-dsn", "c", "genimage.db", "数据库连接字符串")
	pflag.Parse()

	db, err := model.InitDB(*dbType, *dbDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据库连接失败: %v\n", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取数据库连接失败: %v\n", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	fmt.Printf("连接数据库: %s (%s)\n", *dbType, *dbDSN)

	// 清空现有数据
	db.Exec("DELETE FROM memberships")
	db.Exec("DELETE FROM organizations")
	db.Exec("DELETE FROM sessions")
	db.Exec("DELETE FROM password_reset_tokens")
	db.Exec("DELETE FROM email_verification_tokens")
	db.Exec("DELETE FROM users")

	// 生成密码哈希 (Test123456)
	hash, _ := bcrypt.GenerateFromPassword([]byte("Test123456"), 10)
	passwordHash := string(hash)

	now := time.Now()

	// 创建用户
	users := []model.User{
		{ID: 1, Email: "admin@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeSuperAdmin, Points: 1000, CreatedAt: now.AddDate(0, 0, -30)},
		{ID: 2, Email: "org1_admin@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 500, CreatedAt: now.AddDate(0, 0, -25)},
		{ID: 3, Email: "org2_admin@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 450, CreatedAt: now.AddDate(0, 0, -24)},
		{ID: 4, Email: "multi_org_admin@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 600, CreatedAt: now.AddDate(0, 0, -23)},
		{ID: 5, Email: "user1@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 100, CreatedAt: now.AddDate(0, 0, -20)},
		{ID: 6, Email: "user2@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 150, CreatedAt: now.AddDate(0, 0, -19)},
		{ID: 7, Email: "user3@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 200, CreatedAt: now.AddDate(0, 0, -18)},
		{ID: 8, Email: "user4@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 80, CreatedAt: now.AddDate(0, 0, -17)},
		{ID: 9, Email: "user5@example.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 120, CreatedAt: now.AddDate(0, 0, -16)},
		{ID: 10, Email: "user6@example.com", PasswordHash: passwordHash, EmailVerified: false, Type: model.UserTypeNormal, Points: 0, CreatedAt: now.AddDate(0, 0, -15)},
		{ID: 11, Email: "test_a1@company.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 50, CreatedAt: now.AddDate(0, 0, -14)},
		{ID: 12, Email: "test_a2@company.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 60, CreatedAt: now.AddDate(0, 0, -13)},
		{ID: 13, Email: "test_b1@company.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 70, CreatedAt: now.AddDate(0, 0, -12)},
		{ID: 14, Email: "test_b2@company.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 80, CreatedAt: now.AddDate(0, 0, -11)},
		{ID: 15, Email: "test_c1@startup.io", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 90, CreatedAt: now.AddDate(0, 0, -10)},
		{ID: 16, Email: "test_c2@startup.io", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 100, CreatedAt: now.AddDate(0, 0, -9)},
		{ID: 17, Email: "demo1@demo.org", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 110, CreatedAt: now.AddDate(0, 0, -8)},
		{ID: 18, Email: "demo2@demo.org", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 120, CreatedAt: now.AddDate(0, 0, -7)},
		{ID: 19, Email: "demo3@demo.org", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 130, CreatedAt: now.AddDate(0, 0, -6)},
		{ID: 20, Email: "demo4@demo.org", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 140, CreatedAt: now.AddDate(0, 0, -5)},
		{ID: 21, Email: "solo_user1@mail.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 200, CreatedAt: now.AddDate(0, 0, -4)},
		{ID: 22, Email: "solo_user2@mail.com", PasswordHash: passwordHash, EmailVerified: true, Type: model.UserTypeNormal, Points: 180, CreatedAt: now.AddDate(0, 0, -3)},
		{ID: 23, Email: "newbie@test.com", PasswordHash: passwordHash, EmailVerified: false, Type: model.UserTypeNormal, Points: 0, CreatedAt: now.AddDate(0, 0, -2)},
	}

	for _, u := range users {
		u.UpdatedAt = now
		if err := db.Create(&u).Error; err != nil {
			fmt.Fprintf(os.Stderr, "创建用户 %s 失败: %v\n", u.Email, err)
		}
	}
	fmt.Printf("创建用户: %d 个\n", len(users))

	// 创建组织
	orgs := []model.Organization{
		{ID: 1, Name: "技术研发部", Points: 5000, CreatedAt: now.AddDate(0, 0, -28)},
		{ID: 2, Name: "市场营销部", Points: 3000, CreatedAt: now.AddDate(0, 0, -27)},
		{ID: 3, Name: "设计创意部", Points: 4000, CreatedAt: now.AddDate(0, 0, -26)},
		{ID: 4, Name: "测试团队", Points: 2000, CreatedAt: now.AddDate(0, 0, -20)},
	}

	for _, o := range orgs {
		o.UpdatedAt = now
		if err := db.Create(&o).Error; err != nil {
			fmt.Fprintf(os.Stderr, "创建组织 %s 失败: %v\n", o.Name, err)
		}
	}
	fmt.Printf("创建组织: %d 个\n", len(orgs))

	// 创建成员关系
	memberships := []model.Membership{
		// 组织1: 技术研发部
		{UserID: 2, OrganizationID: 1, Role: model.MemberRoleAdmin},
		{UserID: 5, OrganizationID: 1, Role: model.MemberRoleMember},
		{UserID: 6, OrganizationID: 1, Role: model.MemberRoleMember},
		{UserID: 11, OrganizationID: 1, Role: model.MemberRoleMember},
		{UserID: 12, OrganizationID: 1, Role: model.MemberRoleMember},
		// 组织2: 市场营销部
		{UserID: 3, OrganizationID: 2, Role: model.MemberRoleAdmin},
		{UserID: 7, OrganizationID: 2, Role: model.MemberRoleMember},
		{UserID: 8, OrganizationID: 2, Role: model.MemberRoleMember},
		{UserID: 13, OrganizationID: 2, Role: model.MemberRoleMember},
		{UserID: 14, OrganizationID: 2, Role: model.MemberRoleMember},
		// 组织3: 设计创意部
		{UserID: 4, OrganizationID: 3, Role: model.MemberRoleAdmin},
		{UserID: 9, OrganizationID: 3, Role: model.MemberRoleMember},
		{UserID: 15, OrganizationID: 3, Role: model.MemberRoleMember},
		{UserID: 16, OrganizationID: 3, Role: model.MemberRoleMember},
		{UserID: 17, OrganizationID: 3, Role: model.MemberRoleMember},
		// 组织4: 测试团队
		{UserID: 4, OrganizationID: 4, Role: model.MemberRoleAdmin},
		{UserID: 18, OrganizationID: 4, Role: model.MemberRoleMember},
		{UserID: 19, OrganizationID: 4, Role: model.MemberRoleMember},
		{UserID: 20, OrganizationID: 4, Role: model.MemberRoleMember},
		// 跨组织成员
		{UserID: 5, OrganizationID: 2, Role: model.MemberRoleMember},
		{UserID: 7, OrganizationID: 3, Role: model.MemberRoleMember},
	}

	for _, m := range memberships {
		m.CreatedAt = now
		m.UpdatedAt = now
		if err := db.Create(&m).Error; err != nil {
			fmt.Fprintf(os.Stderr, "创建成员关系失败 (user=%d, org=%d): %v\n", m.UserID, m.OrganizationID, err)
		}
	}
	fmt.Printf("创建成员关系: %d 个\n", len(memberships))

	fmt.Println("\n测试数据导入完成!")
	fmt.Println("\n测试账号:")
	fmt.Println("| 邮箱                        | 密码        | 角色          |")
	fmt.Println("|-----------------------------|-------------|---------------|")
	fmt.Println("| admin@example.com           | Test123456  | 超级管理员    |")
	fmt.Println("| org1_admin@example.com      | Test123456  | 组织1管理员   |")
	fmt.Println("| org2_admin@example.com      | Test123456  | 组织2管理员   |")
	fmt.Println("| multi_org_admin@example.com | Test123456  | 组织3,4管理员 |")
	fmt.Println("| user1@example.com           | Test123456  | 普通用户      |")
}
