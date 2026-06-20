package lightrag

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type EntityData struct {
	Description string `json:"description"`
	EntityType  string `json:"entity_type"`
	SourceID    string `json:"source_id"`
	FilePath    string `json:"file_path"`
}

type CreateEntityPayload struct {
	EntityName string     `json:"entity_name"`
	EntityData EntityData `json:"entity_data"`
}

type UpdateEntityPayload struct {
	EntityName  string         `json:"entity_name"`
	UpdatedData map[string]any `json:"updated_data"`
	AllowRename bool           `json:"allow_rename"`
}

type DeleteEntityPayload struct {
	EntityName string `json:"entity_name"`
}

type GraphResponse struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

func (c *Client) CreateEntity(name, description, entityType, sourceID, filePath string) (map[string]any, error) {
	payload := CreateEntityPayload{
		EntityName: name,
		EntityData: EntityData{
			Description: description,
			EntityType:  entityType,
			SourceID:    sourceID,
			FilePath:    filePath,
		},
	}
	return c.do(http.MethodPost, "/graph/entity/create", payload)
}

func (c *Client) UpdateEntity(name string, updatedData map[string]any) (map[string]any, error) {
	payload := UpdateEntityPayload{
		EntityName:  name,
		UpdatedData: updatedData,
	}
	return c.do(http.MethodPost, "/graph/entity/edit", payload)
}

func (c *Client) DeleteEntity(name string) (map[string]any, error) {
	return c.do(http.MethodDelete, "/documents/delete_entity", DeleteEntityPayload{EntityName: name})
}

func (c *Client) CheckEntityExists(name string) bool {
	url := c.BaseURL + "/graph/entity/exists?name=" + name
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	for k, v := range c.Headers() {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	v, ok := result["exists"]
	return ok && v == true
}

func (c *Client) GetAllEntities() ([]map[string]any, error) {
	raw, err := c.doRaw(http.MethodGet, "/graphs?label=*&max_depth=1&max_nodes=10000", nil)
	if err != nil {
		return nil, err
	}
	var result GraphResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Nodes, nil
}

func (c *Client) GetEntitiesByFilePath(filePath string) ([]string, error) {
	nodes, err := c.GetAllEntities()
	if err != nil {
		return nil, err
	}

	var entityNames []string
	for _, node := range nodes {
		props, ok := node["properties"].(map[string]any)
		if !ok {
			continue
		}
		if fp, ok := props["file_path"].(string); ok && fp == filePath {
			if name, ok := props["entity_name"].(string); ok && name != "" {
				entityNames = append(entityNames, name)
			}
		}
	}
	return entityNames, nil
}

func (c *Client) DeleteEntitiesByFilePath(filePath string) error {
	names, err := c.GetEntitiesByFilePath(filePath)
	if err != nil {
		return fmt.Errorf("get entities by filepath: %w", err)
	}

	for _, name := range names {
		if _, err := c.DeleteEntity(name); err != nil {
			return fmt.Errorf("delete entity %s: %w", name, err)
		}
	}
	return nil
}
