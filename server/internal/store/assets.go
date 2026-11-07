package store

import (
	"database/sql"
	"errors"

	"github.com/gxfdev/aidash/server/internal/model"
)

const assetCols = `id,name,source,repo,revision,file,mirror,local_path,host_path,volume,size_bytes,file_count,status,error,log,container_id,runtime,gguf,created_at,updated_at`

func (s *Store) CreateAsset(a *model.ModelAsset) error {
	now := nowStr()
	a.CreatedAt = parseTime(now)
	a.UpdatedAt = a.CreatedAt
	_, err := s.db.Exec(`INSERT INTO model_assets(`+assetCols+`) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.ID, a.Name, a.Source, a.Repo, a.Revision, a.File, a.Mirror, a.LocalPath, a.HostPath, a.Volume,
		a.SizeBytes, a.FileCount, a.Status, a.Error, a.Log, a.ContainerID, a.Runtime, a.GGUF,
		a.CreatedAt.UTC().Format(timeLayout), a.UpdatedAt.UTC().Format(timeLayout))
	return err
}

func scanAsset(sc interface{ Scan(...any) error }) (*model.ModelAsset, error) {
	var a model.ModelAsset
	var created, updated string
	err := sc.Scan(&a.ID, &a.Name, &a.Source, &a.Repo, &a.Revision, &a.File, &a.Mirror, &a.LocalPath, &a.HostPath,
		&a.Volume, &a.SizeBytes, &a.FileCount, &a.Status, &a.Error, &a.Log, &a.ContainerID, &a.Runtime, &a.GGUF,
		&created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt = parseTime(created)
	a.UpdatedAt = parseTime(updated)
	return &a, nil
}

func (s *Store) GetAsset(id string) (*model.ModelAsset, error) {
	return scanAsset(s.db.QueryRow(`SELECT `+assetCols+` FROM model_assets WHERE id=?`, id))
}

func (s *Store) ListAssets() ([]model.ModelAsset, error) {
	rows, err := s.db.Query(`SELECT ` + assetCols + ` FROM model_assets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ModelAsset{}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

type AssetPatch struct {
	Status      string
	Error       string
	Log         string
	ContainerID string
	SizeBytes   int64
	FileCount   int
	GGUF        string
	Runtime     string
	LocalPath   string
	HostPath    string
}

func (s *Store) PatchAsset(id string, p AssetPatch) error {
	sets := []string{"status=?", "error=?", "log=?", "container_id=?", "size_bytes=?", "file_count=?", "gguf=?", "runtime=?", "local_path=?", "host_path=?", "updated_at=?"}
	args := []any{p.Status, p.Error, p.Log, p.ContainerID, p.SizeBytes, p.FileCount, p.GGUF, p.Runtime, p.LocalPath, p.HostPath, nowStr()}
	args = append(args, id)
	_, err := s.db.Exec(`UPDATE model_assets SET `+joinSets(sets)+` WHERE id=?`, args...)
	return err
}

func joinSets(sets []string) string {
	out := ""
	for i, s := range sets {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func (s *Store) DeleteAsset(id string) error {
	_, err := s.db.Exec(`DELETE FROM model_assets WHERE id=?`, id)
	return err
}
