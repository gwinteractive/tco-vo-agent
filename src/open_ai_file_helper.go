package tco_vo_agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func UploadFile(apiKey, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	if err := writer.WriteField("purpose", "batch"); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	headers := map[string]string{
		"Content-Type": writer.FormDataContentType(),
	}

	data, err := httpRequest(apiKey, "POST", "https://api.openai.com/v1/files", headers, &body)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}
func DownloadFile(apiKey, fileID string) (string, error) {
	url := fmt.Sprintf("https://api.openai.com/v1/files/%s/content", fileID)
	data, err := httpRequest(apiKey, "GET", url, nil, nil)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func httpRequest(apiKey, method, url string, headers map[string]string, body io.Reader) ([]byte, error) {
	headers["Authorization"] = fmt.Sprintf("Bearer %s", apiKey)
	client := &http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
