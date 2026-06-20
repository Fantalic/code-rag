package lightrag

import (
	"net/http"
)

func (c *Client) IngestText(content, fileSource string) error {
	payload := map[string]any{"text": content, "file_source": fileSource}
	_, err := c.do(http.MethodPost, "/documents/text", payload)
	return err
}

func (c *Client) DeleteDocument(fileSource string) error {
	payload := map[string]any{"file_source": fileSource}
	_, err := c.do(http.MethodDelete, "/documents", payload)
	return err
}

func (c *Client) Query(text string) (map[string]any, error) {
	payload := map[string]any{"text": text}
	return c.do(http.MethodPost, "/query/text", payload)
}
