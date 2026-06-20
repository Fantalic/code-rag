package lightrag

import (
	"net/http"
)

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

type UpdateRelationPayload struct {
	SourceID    string         `json:"source_id"`
	TargetID    string         `json:"target_id"`
	UpdatedData map[string]any `json:"updated_data"`
}

type DeleteRelationPayload struct {
	SourceEntity string `json:"source_entity"`
	TargetEntity string `json:"target_entity"`
}

func (c *Client) CreateRelation(source, target, description, keywords string, weight float64, sourceID, filePath string) (map[string]any, error) {
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

func (c *Client) UpdateRelation(source, target string, updatedData map[string]any) (map[string]any, error) {
	payload := UpdateRelationPayload{
		SourceID:    source,
		TargetID:    target,
		UpdatedData: updatedData,
	}
	return c.do(http.MethodPost, "/graph/relation/edit", payload)
}

func (c *Client) DeleteRelation(source, target string) (map[string]any, error) {
	return c.do(http.MethodDelete, "/documents/delete_relation", DeleteRelationPayload{
		SourceEntity: source,
		TargetEntity: target,
	})
}
