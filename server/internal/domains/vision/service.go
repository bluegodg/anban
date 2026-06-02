package vision

import (
	"context"
	"strings"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type Service struct {
	xc xiaozhiclient.Client
}

func NewService(xc xiaozhiclient.Client) *Service {
	return &Service{xc: xc}
}

func (s *Service) Capture(ctx context.Context, req CaptureRequest) (CaptureResult, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return CaptureResult{}, ErrInvalidInput
	}

	tool := strings.TrimSpace(req.Tool)
	if tool == "" {
		tool = DefaultCaptureTool
	}

	raw, err := s.xc.CallDeviceMCPTool(ctx, deviceID, tool, req.Args)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{
		DeviceID: deviceID,
		Tool:     tool,
		Raw:      raw,
	}, nil
}
