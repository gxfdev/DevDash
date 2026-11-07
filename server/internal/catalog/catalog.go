package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gxfdev/aidash/server/internal/model"
)

//go:embed catalog.json
var rawCatalog []byte

type Catalog struct {
	Models []model.CatalogItem `json:"models"`
	Apps   []model.CatalogItem `json:"apps"`
}

func Load() (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(rawCatalog, &c); err != nil {
		return nil, fmt.Errorf("catalog.json: %w", err)
	}
	seen := map[string]bool{}
	for i := range c.Models {
		if err := normalize(&c.Models[i], "model"); err != nil {
			return nil, err
		}
		if seen[c.Models[i].ID] {
			return nil, fmt.Errorf("catalog: duplicate id %q", c.Models[i].ID)
		}
		seen[c.Models[i].ID] = true
	}
	for i := range c.Apps {
		if err := normalize(&c.Apps[i], "app"); err != nil {
			return nil, err
		}
		if seen[c.Apps[i].ID] {
			return nil, fmt.Errorf("catalog: duplicate id %q", c.Apps[i].ID)
		}
		seen[c.Apps[i].ID] = true
	}
	sort.SliceStable(c.Models, func(i, j int) bool { return c.Models[i].Order < c.Models[j].Order })
	sort.SliceStable(c.Apps, func(i, j int) bool { return c.Apps[i].Order < c.Apps[j].Order })
	return &c, nil
}

func normalize(it *model.CatalogItem, kind string) error {
	if strings.TrimSpace(it.ID) == "" {
		return fmt.Errorf("catalog: %s entry without id", kind)
	}
	if it.Kind == "" {
		it.Kind = kind
	}
	if it.Kind != kind {
		return fmt.Errorf("catalog: %q kind mismatch (%s vs %s)", it.ID, it.Kind, kind)
	}
	if it.Image == "" {
		return fmt.Errorf("catalog: %q missing image", it.ID)
	}
	if it.ImageTag == "" {
		it.ImageTag = "latest"
	}
	if it.Env == nil {
		it.Env = map[string]string{}
	}
	if it.Summary == "" {
		it.Summary = it.Description
	}
	if it.Name == "" {
		it.Name = it.ID
	}
	if it.Category == "" {
		it.Category = "其他"
	}
	for i, p := range it.Ports {
		if p.ContPort <= 0 || p.ContPort > 65535 {
			return fmt.Errorf("catalog: %q has invalid container port %d", it.ID, p.ContPort)
		}
		if p.Protocol == "" {
			it.Ports[i].Protocol = "tcp"
		}
		if p.HostPort < 0 || p.HostPort > 65535 {
			return fmt.Errorf("catalog: %q has invalid host port %d", it.ID, p.HostPort)
		}
	}
	if it.ModelName == "" && kind == "model" {
		it.ModelName = it.ID
	}
	return nil
}

func (c *Catalog) All() []model.CatalogItem {
	out := make([]model.CatalogItem, 0, len(c.Models)+len(c.Apps))
	out = append(out, c.Models...)
	out = append(out, c.Apps...)
	return out
}

func (c *Catalog) ModelsOnly() []model.CatalogItem {
	return c.Models
}

func (c *Catalog) AppsOnly() []model.CatalogItem {
	return c.Apps
}

func (c *Catalog) Get(id string) (*model.CatalogItem, bool) {
	for i := range c.Models {
		if c.Models[i].ID == id {
			item := c.Models[i]
			return &item, true
		}
	}
	for i := range c.Apps {
		if c.Apps[i].ID == id {
			item := c.Apps[i]
			return &item, true
		}
	}
	return nil, false
}

func (c *Catalog) Categories(kind string) []string {
	seen := map[string]bool{}
	out := []string{}
	items := c.All()
	for _, it := range items {
		if kind != "" && it.Kind != kind {
			continue
		}
		if !seen[it.Category] {
			seen[it.Category] = true
			out = append(out, it.Category)
		}
	}
	return out
}
