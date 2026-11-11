// Admin commands, run from a trusted machine with database access:
//
//	go run ./cmd/admin migrate
//	go run ./cmd/admin user create <username> [--name "Display Name"] [--days 30]
//	go run ./cmd/admin user renew <username> [--days 30]
//	go run ./cmd/admin user expire <username>
//	go run ./cmd/admin user reset-password <username>
//	go run ./cmd/admin user list
//
// Generated passwords are printed once and only their Argon2id hash is stored.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"github.com/Joshtri/rackup-api/internal/auth"
	"github.com/Joshtri/rackup-api/internal/db"
)

const usage = `usage:
  migrate
  user create <username> [--name "Display Name"] [--days 30]
  user renew <username> [--days 30]
  user expire <username>
  user reset-password <username>
  user list`

func main() {
	_ = godotenv.Load()
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	ctx := context.Background()
	switch args[0] {
	case "migrate":
		conn, err := connect(ctx)
		if err != nil {
			return err
		}
		defer conn.Close(ctx)
		applied, err := db.Migrate(ctx, conn)
		for _, v := range applied {
			fmt.Println("applied", v)
		}
		if err != nil {
			return err
		}
		fmt.Println("migrations applied")
		return nil
	case "user":
		return userCommand(ctx, args[1:])
	default:
		return errors.New(usage)
	}
}

func userCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	cmd, rest := args[0], args[1:]
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	switch cmd {
	case "create":
		username, err := positional(rest)
		if err != nil {
			return err
		}
		name := flag(rest, "--name")
		if name == "" {
			name = username
		}
		days, err := daysFlag(rest)
		if err != nil {
			return err
		}
		user, password, err := auth.CreateAccount(ctx, conn, username, name, days)
		if err != nil {
			return err
		}
		fmt.Println("created", describe(user))
		printPassword(password)
	case "renew":
		username, err := positional(rest)
		if err != nil {
			return err
		}
		days, err := daysFlag(rest)
		if err != nil {
			return err
		}
		user, err := auth.RenewAccount(ctx, conn, username, days)
		if err != nil {
			return err
		}
		fmt.Println("renewed", describe(user))
	case "expire":
		username, err := positional(rest)
		if err != nil {
			return err
		}
		user, err := auth.ExpireAccount(ctx, conn, username)
		if err != nil {
			return err
		}
		fmt.Println("expired", describe(user))
	case "reset-password":
		username, err := positional(rest)
		if err != nil {
			return err
		}
		user, password, err := auth.ResetPassword(ctx, conn, username)
		if err != nil {
			return err
		}
		fmt.Println("new password for", describe(user))
		printPassword(password)
	case "list":
		users, err := auth.ListAccounts(ctx, conn)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			fmt.Println("no users")
		}
		for i := range users {
			fmt.Println(describe(&users[i]))
		}
	default:
		return errors.New(usage)
	}
	return nil
}

// connect prefers the direct connection: migrations take a session-level lock
// that Neon's pooler doesn't support.
func connect(ctx context.Context) (*pgx.Conn, error) {
	url := os.Getenv("DATABASE_URL_UNPOOLED")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return nil, errors.New("set DATABASE_URL_UNPOOLED (or DATABASE_URL)")
	}
	return pgx.Connect(ctx, url)
}

func positional(rest []string) (string, error) {
	if len(rest) == 0 || strings.HasPrefix(rest[0], "--") {
		return "", errors.New(usage)
	}
	return rest[0], nil
}

func flag(rest []string, name string) string {
	for i, a := range rest {
		if a == name && i+1 < len(rest) {
			return rest[i+1]
		}
	}
	return ""
}

func daysFlag(rest []string) (int64, error) {
	v := flag(rest, "--days")
	if v == "" {
		return auth.DefaultAccountDays, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("--days: not a number: %q", v)
	}
	return n, nil
}

func describe(u *auth.User) string {
	status := "active"
	if u.IsExpired() {
		status = "EXPIRED"
	}
	return fmt.Sprintf("%s (%s) - %s, expires %s",
		u.Username, u.DisplayName, status, u.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC"))
}

func printPassword(password string) {
	fmt.Println("password:", password)
	fmt.Println("(shown once, only its hash is stored; hand it over privately)")
}
