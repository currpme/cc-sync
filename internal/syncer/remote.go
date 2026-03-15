package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"ccsync/internal/model"
	"ccsync/internal/webdav"
)

type RemoteStore struct {
	client *webdav.Client
	root   string
}

func NewRemoteStore(client *webdav.Client, root string) *RemoteStore {
	return &RemoteStore{client: client, root: strings.Trim(root, "/")}
}

func (s *RemoteStore) manifestPath(tool string) string {
	return path.Join(s.root, tool, "manifest.json")
}

func (s *RemoteStore) itemPath(tool, relPath string) string {
	return path.Join(s.root, tool, relPath)
}

func (s *RemoteStore) Load(ctx context.Context, tool string) (model.Snapshot, error) {
	snapshot := model.Snapshot{Tool: tool}
	data, err := s.client.ReadFile(ctx, s.manifestPath(tool))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return snapshot, nil
		}
		return snapshot, err
	}
	var manifest model.Snapshot
	if err := json.Unmarshal(data, &manifest); err != nil {
		return snapshot, fmt.Errorf("decode remote manifest: %w", err)
	}
	snapshot.Tool = tool
	for _, item := range manifest.Items {
		content, err := s.client.ReadFile(ctx, s.itemPath(tool, item.RelPath))
		if err != nil {
			return snapshot, err
		}
		item.Content = content
		snapshot.Items = append(snapshot.Items, item)
	}
	return snapshot, nil
}

func (s *RemoteStore) Save(ctx context.Context, snapshot model.Snapshot) error {
	current, err := s.Load(ctx, snapshot.Tool)
	if err != nil {
		return err
	}
	for _, item := range snapshot.Items {
		if err := s.client.WriteFile(ctx, s.itemPath(snapshot.Tool, item.RelPath), item.Content); err != nil {
			return err
		}
	}
	keep := make(map[string]bool, len(snapshot.Items))
	for _, item := range snapshot.Items {
		keep[item.RelPath] = true
	}
	for _, item := range current.Items {
		if keep[item.RelPath] {
			continue
		}
		if err := s.client.DeleteFile(ctx, s.itemPath(snapshot.Tool, item.RelPath)); err != nil {
			return err
		}
	}
	manifest := snapshot
	for i := range manifest.Items {
		manifest.Items[i].Content = nil
	}
	data, marshalErr := json.MarshalIndent(manifest, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	if err := s.client.WriteFile(ctx, s.manifestPath(snapshot.Tool), data); err != nil {
		return err
	}
	for _, item := range current.Items {
		if keep[item.RelPath] {
			continue
		}
		item.Content = nil
	}
	return nil
}

func (s *RemoteStore) WriteItem(ctx context.Context, tool string, item model.ManagedItem) error {
	return s.client.WriteFile(ctx, s.itemPath(tool, item.RelPath), item.Content)
}

func (s *RemoteStore) DeleteItem(ctx context.Context, tool string, item model.ManagedItem) error {
	return s.client.DeleteFile(ctx, s.itemPath(tool, item.RelPath))
}
