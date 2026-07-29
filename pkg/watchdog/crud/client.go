package crud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	DefaultTimeout = 300 * time.Second
)

type Config struct {
	BaseURL   string
	APIKey    string
	Timeout   time.Duration
	Workspace string
}

func NewConfig() *Config {
	url := os.Getenv("LIGHTRAG_URL")
	if url == "" {
		url = "http://localhost:9621"
	}
	timeout := os.Getenv("LIGHTRAG_TIMEOUT")
	d := DefaultTimeout
	if timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			d = t
		}
	}
	return &Config{
		BaseURL: url,
		APIKey:  os.Getenv("LIGHTRAG_API_KEY"),
		Timeout: d,
	}
}

func (c *Config) Headers() map[string]string {
	h := map[string]string{"Content-Type": "application/json"}
	if c.APIKey != "" {
		h["X-API-Key"] = c.APIKey
	}
	return h
}

func (c *Config) do(method, path string, payload interface{}) (map[string]any, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(b)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	for k, v := range c.Headers() {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return result, nil
}

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

func (c *Config) CreateEntity(name, description, entityType, sourceID, filePath string) (map[string]any, error) {
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

type UpdateEntityPayload struct {
	EntityName   string         `json:"entity_name"`
	UpdatedData  map[string]any `json:"updated_data"`
	AllowRename  bool           `json:"allow_rename"`
}

func (c *Config) UpdateEntity(name string, updatedData map[string]any) (map[string]any, error) {
	payload := UpdateEntityPayload{
		EntityName:  name,
		UpdatedData: updatedData,
	}
	return c.do(http.MethodPost, "/graph/entity/edit", payload)
}

type DeleteEntityPayload struct {
	EntityName string `json:"entity_name"`
}

func (c *Config) DeleteEntity(name string) (map[string]any, error) {
	return c.do(http.MethodDelete, "/documents/delete_entity", DeleteEntityPayload{EntityName: name})
}

func (c *Config) CheckEntityExists(name string) bool {
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

type RelationData struct {
	Description string  `json:"description"`
	Keywords    string  `json:"keywords"`
	Weight      float64 `json:"weight"`
	SourceID    string  `json:"source_id"`
	FilePath    string  `json:"file_path"`
}

type CreateRelationPayload struct {
	SourceEntity string       `json:"source_entity"`
	TargetEntity string       `json:"target_entity"`
	RelationData RelationData `json:"relation_data"`
}

func (c *Config) CreateRelation(source, target, description, keywords string, weight float64, sourceID, filePath string) (map[string]any, error) {
	payload := CreateRelationPayload{
		SourceEntity: source,
		TargetEntity: target,
		RelationData: RelationData{
			Description: description,
			Keywords:    keywords,
			Weight:      weight,
			SourceID:    sourceID,
			FilePath:    filePath,
		},
	}
	return c.do(http.MethodPost, "/graph/relation/create", payload)
}

type UpdateRelationPayload struct {
	SourceID    string         `json:"source_id"`
	TargetID    string         `json:"target_id"`
	UpdatedData map[string]any `json:"updated_data"`
}

func (c *Config) UpdateRelation(source, target string, updatedData map[string]any) (map[string]any, error) {
	payload := UpdateRelationPayload{
		SourceID:    source,
		TargetID:    target,
		UpdatedData: updatedData,
	}
	return c.do(http.MethodPost, "/graph/relation/edit", payload)
}

type DeleteRelationPayload struct {
	SourceEntity string `json:"source_entity"`
	TargetEntity string `json:"target_entity"`
}

func (c *Config) DeleteRelation(source, target string) (map[string]any, error) {
	return c.do(http.MethodDelete, "/documents/delete_relation", DeleteRelationPayload{
		SourceEntity: source,
		TargetEntity: target,
	})
}

type GraphResponse struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

func (c *Config) GetAllEntities() ([]map[string]any, error) {
	url := c.BaseURL + "/graphs?label=*&max_depth=1&max_nodes=10000"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	for k, v := range c.Headers() {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: c.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var result GraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Nodes, nil
}

func (c *Config) SetWorkspace(workspace string) error {
	payload := map[string]string{"workspace": workspace}
	_, err := c.do(http.MethodPost, "/workspace", payload)
	if err == nil {
		c.Workspace = workspace
	}
	return err
}

func (c *Config) ClearAll() error {
	_, err := c.do(http.MethodDelete, "/documents", nil)
	if err != nil {
		return fmt.Errorf("clear all: %w", err)
	}
	log.Printf("All documents, entities, and relations cleared")
	return nil
}

func (c *Config) IngestText(content, fileSource string) error {
	payload := map[string]any{"text": content, "file_source": fileSource}
	_, err := c.do(http.MethodPost, "/documents/text", payload)
	return err
}

func (c *Config) DeleteDocument(fileSource string) error {
	payload := map[string]any{"file_source": fileSource}
	_, err := c.do(http.MethodDelete, "/documents", payload)
	return err
}

func (c *Config) Query(text string) (map[string]any, error) {
	payload := map[string]any{"text": text}
	return c.do(http.MethodPost, "/query/text", payload)
}

func (c *Config) GetEntitiesByFilePath(filePath string) ([]string, error) {
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

func (c *Config) DeleteEntitiesByFilePath(filePath string) error {
	names, err := c.GetEntitiesByFilePath(filePath)
	if err != nil {
		return fmt.Errorf("get entities by filepath: %w", err)
	}

	log.Printf("Found %d entities for file path %s", len(names), filePath)
	for _, name := range names {
		if _, err := c.DeleteEntity(name); err != nil {
			log.Printf("Delete entity %s failed: %v", name, err)
		} else {
			log.Printf("Deleted entity: %s", name)
		}
	}
	return nil
}