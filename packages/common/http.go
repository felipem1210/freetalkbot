package common

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

type PostHttpReq struct {
	Url           string
	Headers       map[string]string
	FormParams    map[string]string
	JsonBody      map[string]string
	FileParamName string
	FilePath      string
}

type Response struct {
	RecipientId string `json:"recipient_id"`
	Text        string `json:"text"`
}

type Responses []Response

func (r *PostHttpReq) SendPost(ct string) (io.ReadCloser, error) {
	var requestBody bytes.Buffer
	var ctContent string

	switch ct {
	case "form-data":
		// Create a buffer and a writer for the multipart/form-data
		writer := multipart.NewWriter(&requestBody)
		// Add form parameters
		for key, val := range r.FormParams {
			err := writer.WriteField(key, val)
			if err != nil {
				return nil, fmt.Errorf("error writing form field: %w", err)
			}
		}
		// If a file path is provided, add the file to the form
		if r.FilePath != "" && r.FileParamName != "" {
			file, err := os.Open(r.FilePath)
			if err != nil {
				return nil, fmt.Errorf("error opening file: %w", err)
			}
			defer file.Close()

			part, err := writer.CreateFormFile(r.FileParamName, filepath.Base(file.Name()))
			if err != nil {
				return nil, fmt.Errorf("error creating form file: %w", err)
			}
			_, err = io.Copy(part, file)
			if err != nil {
				return nil, fmt.Errorf("error copying file content: %w", err)
			}
		}

		// Close the writer to finish writing the request body
		err := writer.Close()
		if err != nil {
			return nil, fmt.Errorf("error closing writer: %w", err)
		}
		ctContent = writer.FormDataContentType()

	case "json":
		jsonData, err := json.Marshal(r.JsonBody)
		requestBody = *bytes.NewBuffer(jsonData)
		if err != nil {
			return nil, fmt.Errorf("error converting data to JSON: %s", err)
		}
		ctContent = "application/json"
	}

	// Create a POST request
	req, err := http.NewRequest("POST", r.Url, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", ctContent)

	for key, value := range r.Headers {
		req.Header.Set(key, value)
	}

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error response from server: %s", body)
	}

	return resp.Body, nil
}

// processResponseBody reads and processes the body of an HTTP response.
func ProcessResponseString(respBody io.ReadCloser) (string, error) {
	// Ensure the response body is closed after reading
	defer respBody.Close()

	// Read the body
	bodyBytes, err := io.ReadAll(respBody)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Convert the body to a string
	bodyString := string(bodyBytes)

	return bodyString, nil
}

// // processJSONResponse reads and processes the JSON body of an HTTP response.
func (r Responses) ProcessJSONResponse(respBody io.ReadCloser) (Responses, error) {
	// Ensure the response body is closed after reading
	defer respBody.Close()

	if respBody == nil {
		return r, fmt.Errorf("received a nil response body")
	}

	// Read the body
	bodyBytes, err := io.ReadAll(respBody)
	if err != nil {
		return r, fmt.Errorf("error reading response body: %w", err)
	}

	err = json.Unmarshal(bodyBytes, &r)

	if err != nil {
		return r, fmt.Errorf("error unmarshaling JSON: %w", err)
	}
	return r, nil
}
