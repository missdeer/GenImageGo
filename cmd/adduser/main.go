package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"genimage/model"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbType := pflag.String("db-type", "sqlite", "数据库类型: sqlite, mysql, postgres")
	dbDSN := pflag.String("db-dsn", "genimage.db", "数据库连接字符串")
	email := pflag.String("email", "", "用户邮箱 (必填)")
	password := pflag.String("password", "", "用户密码 (必填)")
	passwordStdin := pflag.Bool("password-stdin", false, "从标准输入读取用户密码 (更安全)")
	pflag.Parse()

	if *passwordStdin && *password != "" {
		fmt.Fprintln(os.Stderr, "错误: 不能同时使用 --password 和 --password-stdin")
		pflag.Usage()
		os.Exit(1)
	}

	emailValue := strings.TrimSpace(*email)
	passwordValue := *password
	if *passwordStdin {
		input, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取密码失败: %v\n", err)
			os.Exit(1)
		}
		passwordValue = strings.TrimRight(string(input), "\r\n")
	}

	if emailValue == "" || passwordValue == "" {
		fmt.Fprintln(os.Stderr, "错误: 必须提供 --email 和 (--password 或 --password-stdin)")
		pflag.Usage()
		os.Exit(1)
	}

	db, err := model.InitDB(*dbType, *dbDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据库连接失败: %v\n", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "数据库连接失败: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "关闭数据库失败: %v\n", err)
		}
	}()

	hash, err := bcrypt.GenerateFromPassword([]byte(passwordValue), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "密码加密失败: %v\n", err)
		os.Exit(1)
	}

	user := model.User{
		Email:         emailValue,
		PasswordHash:  string(hash),
		EmailVerified: true,
		Type:          model.UserTypeSuperAdmin,
	}

	if err := db.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate") {
			fmt.Fprintf(os.Stderr, "创建用户失败: 邮箱 %s 已存在\n", emailValue)
		} else {
			fmt.Fprintf(os.Stderr, "创建用户失败: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("超级管理员用户创建成功: %s (ID: %d)\n", user.Email, user.ID)
}
