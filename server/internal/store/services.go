package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gxfdev/aidash/server/internal/model"
)

const serviceCols = `id,name,kind,catalog_id,image,model_name,description,spec_json,status,health,container_id,endpoints,error,owner,created_at,updated_at,started_at`

func (s *Store) CreateService(svc *model.Service) error {
	spec, err := json.Marshal(svc.Spec)
	if err != nil {
		return err
	}
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = parseTime(nowStr())
	}
	svc.UpdatedAt = svc.CreatedAt
	_, err = s.db.Exec(`INSERT INTO services(id,name,kind,catalog_id,image,model_name,description,spec_json,status,health,container_id,endpoints,error,owner,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		svc.ID, svc.Name, svc.Kind, svc.CatalogID, svc.Image, svc.ModelName, svc.Description, string(spec),
		svc.Status, svc.Health, svc.ContainerID, strings.Join(svc.Endpoints, ","), svc.Error, svc.Owner,
		svc.CreatedAt.UTC().Format(timeLayout), svc.UpdatedAt.UTC().Format(timeLayout))
	return err
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func scanService(sc interface{ Scan(...any) error }) (*model.Service, error) {
	var svc model.Service
	var specJSON, endpoints, created, updated string
	var started sql.NullString
	err := sc.Scan(&svc.ID, &svc.Name, &svc.Kind, &svc.CatalogID, &svc.Image, &svc.ModelName, &svc.Description,
		&specJSON, &svc.Status, &svc.Health, &svc.ContainerID, &endpoints, &svc.Error, &svc.Owner, &created, &updated, &started)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(specJSON), &svc.Spec)
	if endpoints != "" {
		svc.Endpoints = strings.Split(endpoints, ",")
	} else {
		svc.Endpoints = []string{}
	}
	svc.CreatedAt = parseTime(created)
	svc.UpdatedAt = parseTime(updated)
	svc.StartedAt = parseNullTime(started)
	svc.HostAccess = svc.Spec.HostAccess.Enabled
	return &svc, nil
}

func (s *Store) GetService(id string) (*model.Service, error) {
	row := s.db.QueryRow(`SELECT `+serviceCols+` FROM services WHERE id=?`, id)
	return scanService(row)
}

func (s *Store) GetServiceByName(name string) (*model.Service, error) {
	row := s.db.QueryRow(`SELECT `+serviceCols+` FROM services WHERE name=?`, name)
	return scanService(row)
}

func (s *Store) ListServices() ([]model.Service, error) {
	rows, err := s.db.Query(`SELECT ` + serviceCols + ` FROM services ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Service{}
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *svc)
	}
	return out, rows.Err()
}

func (s *Store) UpdateServiceSpec(svc *model.Service) error {
	spec, err := json.Marshal(svc.Spec)
	if err != nil {
		return err
	}
	svc.UpdatedAt = parseTime(nowStr())
	_, err = s.db.Exec(`UPDATE services SET image=?, model_name=?, description=?, spec_json=?, endpoints=?, updated_at=? WHERE id=?`,
		svc.Image, svc.ModelName, svc.Description, string(spec), strings.Join(svc.Endpoints, ","), svc.UpdatedAt.UTC().Format(timeLayout), svc.ID)
	return err
}

type ServiceStatusPatch struct {
	Status      string
	Health      string
	ContainerID string
	Endpoints   []string
	Error       string
	MarkStarted bool
}

func (s *Store) PatchService(id string, p ServiceStatusPatch) error {
	sets := []string{"status=?", "health=?", "container_id=?", "endpoints=?", "error=?", "updated_at=?"}
	args := []any{p.Status, p.Health, p.ContainerID, strings.Join(p.Endpoints, ","), p.Error, nowStr()}
	if p.MarkStarted {
		sets = append(sets, "started_at=?")
		args = append(args, nowStr())
	}
	args = append(args, id)
	_, err := s.db.Exec(`UPDATE services SET `+strings.Join(sets, ", ")+` WHERE id=?`, args...)
	return err
}

func (s *Store) DeleteService(id string) error {
	_, err := s.db.Exec(`DELETE FROM services WHERE id=?`, id)
	if err != nil {
		return err
	}
	_, _ = s.db.Exec(`DELETE FROM service_events WHERE service_id=?`, id)
	return nil
}

func (s *Store) AddServiceEvent(serviceID, level, message string) {
	if len(message) > 4000 {
		message = message[:4000]
	}
	_, _ = s.db.Exec(`INSERT INTO service_events(service_id,level,message,created_at) VALUES(?,?,?,?)`, serviceID, level, message, nowStr())
}

func (s *Store) ListServiceEvents(serviceID string, limit int) ([]model.ServiceEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT id,service_id,level,message,created_at FROM service_events WHERE service_id=? ORDER BY id DESC LIMIT ?`, serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ServiceEvent{}
	for rows.Next() {
		var e model.ServiceEvent
		var created string
		if err := rows.Scan(&e.ID, &e.ServiceID, &e.Level, &e.Message, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		out = append(out, e)
	}
	// reverse -> chronological
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}
