package repository

import (
	"fmt"
	"os"
	"strings"
	"testing"
    "errors"
	"github/marveldo/eda-monolith/internal/repository/db"
	"github/marveldo/eda-monolith/shared"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func pgPass() string {
	b, err := os.ReadFile("../../.env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "DB_PASSWORD=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "DB_PASSWORD="))
		}
	}
	return ""
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "host=localhost user=postgres password=" + pgPass() + " dbname=digiwallet port=5432 sslmode=disable"
	conn, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Skip("no db:", err)
	}
	if err := conn.AutoMigrate(&db.User{}); err != nil {
		t.Skip("migrate:", err)
	}
	return conn
}

func TestUpdateUserAppliesChanges(t *testing.T) {
	conn := testDB(t)
	repo := UserRepository{}
	ctx := &RepoCtx{Context: t.Context(), DB: conn}

	email := fmt.Sprintf("bugcheck+%d@example.com", os.Getpid())
	conn.Unscoped().Where("email = ?", email).Delete(&db.User{})
	t.Cleanup(func() { conn.Unscoped().Where("email = ?", email).Delete(&db.User{}) })

	fn, ln, pw := "Before", "User", "x"
	created, err := repo.CreateUser(ctx, &UserInputParam{
		FirstName: &fn, LastName: &ln, Email: &email, Passwordhash: &pw,
	})
	if err != nil {
		t.Fatal(err)
	}

	newName := "After"
	updated, err := repo.UpdateUser(ctx, &UserInputParam{ID: &created.ID, FirstName: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if updated.FirstName != "After" {
		t.Errorf("BUG 1: returned FirstName still %q", updated.FirstName)
	}
	var row db.User
	conn.Where("email = ?", email).First(&row)
	if row.FirstName != "After" {
		t.Errorf("BUG 1: db row FirstName still %q", row.FirstName)
	}
}

func TestIsVerifiedIsMapped(t *testing.T) {
	repo := UserRepository{}
	u := repo.MapUserModelToUser(&db.User{IsVerified: true})
	if !u.IsVerified {
		t.Error("BUG 2: IsVerified not mapped, always false")
	}
}

func TestDeletedUserIsStillReturned(t *testing.T) {
	conn := testDB(t)
	repo := UserRepository{}
	ctx := &RepoCtx{Context: t.Context(), DB: conn}

	email := fmt.Sprintf("delcheck+%d@example.com", os.Getpid())
	conn.Unscoped().Where("email = ?", email).Delete(&db.User{})
	t.Cleanup(func() { conn.Unscoped().Where("email = ?", email).Delete(&db.User{}) })

	fn, ln, pw := "Gone", "User", "x"
	created, err := repo.CreateUser(ctx, &UserInputParam{
		FirstName: &fn, LastName: &ln, Email: &email, Passwordhash: &pw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteUser(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetUserByEmail(ctx, email); err == nil {
		t.Error("BUG 4: deleted user still returned by GetUserByEmail")
	}
	exists, _ := repo.UserEmailExists(ctx, email)
	if exists {
		t.Error("BUG 4: deleted user still counts in UserEmailExists")
	}
}

func TestUpdateByEmailAndEdgeCases(t *testing.T) {
	conn := testDB(t)
	repo := UserRepository{}
	ctx := &RepoCtx{Context: t.Context(), DB: conn}

	email := fmt.Sprintf("byemail+%d@example.com", os.Getpid())
	conn.Unscoped().Where("email = ?", email).Delete(&db.User{})
	t.Cleanup(func() { conn.Unscoped().Where("email = ?", email).Delete(&db.User{}) })

	fn, ln, pw := "Before", "User", "x"
	if _, err := repo.CreateUser(ctx, &UserInputParam{
		FirstName: &fn, LastName: &ln, Email: &email, Passwordhash: &pw,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.UpdateUser(ctx, &UserInputParam{
		Email:      &email,
		IsVerified: shared.Ptr(true),
	})
	if err != nil {
		t.Fatalf("update by email: %v", err)
	}
	if !got.IsVerified {
		t.Error("IsVerified not set")
	}

	if _, err := repo.UpdateUser(ctx, &UserInputParam{Email: &email}); !errors.Is(err, ErrNothingToUpdate) {
		t.Errorf("empty update: want ErrNothingToUpdate, got %v", err)
	}
	if _, err := repo.UpdateUser(ctx, &UserInputParam{IsVerified: shared.Ptr(true)}); !errors.Is(err, ErrNoIdentifier) {
		t.Errorf("no identifier: want ErrNoIdentifier, got %v", err)
	}
	missing := "nobody+zzz@example.com"
	if _, err := repo.UpdateUser(ctx, &UserInputParam{Email: &missing, IsVerified: shared.Ptr(true)}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing row: want ErrNotFound, got %v", err)
	}
}
