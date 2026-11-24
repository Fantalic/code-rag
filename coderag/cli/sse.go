package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func runSSEBridge(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:9622/sse", nil)
	if err != nil {
		return fmt.Errorf("SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect: %w", err)
	}
	defer resp.Body.Close()

	bodyReader := bufio.NewReader(resp.Body)

	endpointLine, err := bodyReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read SSE endpoint line: %w", err)
	}
	endpointLine = strings.TrimSpace(endpointLine)

	if !strings.HasPrefix(endpointLine, "event: endpoint") {
		return fmt.Errorf("expected SSE event: endpoint, got: %s", endpointLine)
	}

	dataLine, err := bodyReader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read SSE data line: %w", err)
	}
	dataLine = strings.TrimSpace(dataLine)
	if !strings.HasPrefix(dataLine, "data: ") {
		return fmt.Errorf("expected SSE data line, got: %s", dataLine)
	}

	messageURL := strings.TrimPrefix(dataLine, "data: ")
	sessionID := extractSessionID(messageURL)
	if sessionID == "" {
		return fmt.Errorf("could not extract session_id from: %s", messageURL)
	}

	log.Printf("SSE session established: %s", sessionID)

	errCh := make(chan error, 2)

	go func() {
		stdin := bufio.NewReader(os.Stdin)
		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					errCh <- fmt.Errorf("stdin read: %w", err)
				}
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			postURL := fmt.Sprintf("http://localhost:9622/messages/?session_id=%s", sessionID)
			postReq, err := http.NewRequestWithContext(ctx, "POST", postURL, strings.NewReader(line))
			if err != nil {
				errCh <- fmt.Errorf("POST request: %w", err)
				return
			}
			postReq.Header.Set("Content-Type", "application/json")

			postResp, err := http.DefaultClient.Do(postReq)
			if err != nil {
				errCh <- fmt.Errorf("POST: %w", err)
				return
			}
			postResp.Body.Close()
		}
	}()

	go func() {
		for {
			line, err := bodyReader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					errCh <- fmt.Errorf("SSE read: %w", err)
				}
				return
			}
			line = strings.TrimSpace(line)

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				fmt.Println(data)
				os.Stdout.Sync()
			}
		}
	}()

	return <-errCh
}

func extractSessionID(messageURL string) string {
	idx := strings.Index(messageURL, "session_id=")
	if idx < 0 {
		return ""
	}
	val := messageURL[idx+len("session_id="):]
	if amp := strings.Index(val, "&"); amp >= 0 {
		val = val[:amp]
	}
	return val
}
