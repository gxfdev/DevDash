package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gxfdev/aidash/server/internal/model"
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, password, role string, mustChange bool) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	mu := 0
	if mustChange {
		mu = 1
	}
	now := nowStr()
	res, err := s.db.Exec(`INSERT INTO users(username,password_hash,role,must_change_pwd,created_at) VALUES(?,?,?,?,?)`,
		username, string(hash), role, mu, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &model.User{ID: id, Username: username, Role: role, MustChangePwd: mustChange, CreatedAt: parseTime(now)}, nil
}

func (s *Store) GetUserByName(username string) (*model.User, error) {
	row := s.db.QueryRow(`SELECT id,username,password_hash,role,must_change_pwd,created_at,last_login_at FROM users WHERE username=?`, username)
	return scanUser(row)
}

func (s *Store) GetUser(id int64) (*model.User, error) {
	row := s.db.QueryRow(`SELECT id,username,password_hash,role,must_change_pwd,created_at,last_login_at FROM users WHERE id=?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var created string
	var last sql.NullString
	var mu int
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &mu, &created, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.MustChangePwd = mu == 1
	u.CreatedAt = parseTime(created)
	u.LastLoginAt = parseNullTime(last)
	return &u, nil
}

func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(`SELECT id,username,password_hash,role,must_change_pwd,created_at,last_login_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.User, 0, 8)
	for rows.Next() {
		var u model.User
		var created string
		var last sql.NullString
		var mu int
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &mu, &created, &last); err != nil {
			return nil, err
		}
		u.MustChangePwd = mu == 1
		u.CreatedAt = parseTime(created)
		u.LastLoginAt = parseNullTime(last)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUserPassword(id int64, password string, mustChange bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash=?, must_change_pwd=? WHERE id=?`, string(hash), boolInt(mustChange), id)
	return err
}

func (s *Store) UpdateUserRole(id int64, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role=? WHERE id=?`, role, id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) TouchLogin(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at=? WHERE id=?`, nowStr(), id)
	return err
}

func (s *Store) CheckPassword(u *model.User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// EnsureAdmin creates the default administrator on first boot.
func (s *Store) EnsureAdmin(username, password string) error {
	n, err := s.CountUsers()
	if err != nil || n > 0 {
		return err
	}
	_, err = s.CreateUser(username, password, "admin", false)
	return err
}

func (s *Store) CleanupMetrics(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM host_metrics WHERE ts < ?`, before.Unix())
	return err
}
