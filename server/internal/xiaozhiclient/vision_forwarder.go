package xiaozhiclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

type VisionForwardRequest struct {
	DeviceID      string
	ClientID      string
	Authorization string
	Question      string
	FileName      string
	ContentType   string
	Image         []byte
}

type VisionForwardResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

type VisionHTTPForwarder struct {
	visionURL string
	hc        *http.Client
}

func NewVisionForwarder(visionURL string) *VisionHTTPForwarder {
	return &VisionHTTPForwarder{
		visionURL: strings.TrimSpace(visionURL),
		hc:        &http.Client{Timeout: 30 * time.Second},
	}
}

func (f *VisionHTTPForwarder) ForwardVisionMultipart(ctx context.Context, req VisionForwardRequest) (VisionForwardResponse, error) {
	if f == nil || f.visionURL == "" {
		return VisionForwardResponse{}, fmt.Errorf("xiaozhi vision url is empty")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("question", req.Question); err != nil {
		return VisionForwardResponse{}, err
	}

	fileName := strings.TrimSpace(req.FileName)
	if fileName == "" {
		fileName = "image"
	}
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": fileName,
	}))
	partHeader.Set("Content-Type", contentType)
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return VisionForwardResponse{}, err
	}
	if _, err := part.Write(req.Image); err != nil {
		return VisionForwardResponse{}, err
	}
	if err := writer.Close(); err != nil {
		return VisionForwardResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.visionURL, &body)
	if err != nil {
		return VisionForwardResponse{}, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if req.DeviceID != "" {
		httpReq.Header.Set("Device-Id", req.DeviceID)
	}
	if req.ClientID != "" {
		httpReq.Header.Set("Client-Id", req.ClientID)
	}
	if req.Authorization != "" {
		httpReq.Header.Set("Authorization", req.Authorization)
	}

	resp, err := f.hc.Do(httpReq)
	if err != nil {
		return VisionForwardResponse{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return VisionForwardResponse{}, err
	}
	return VisionForwardResponse{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        respBody,
	}, nil
}
