package vision

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const visionMCPTimeout = 8 * time.Second
const lookCaptureTimeout = 30 * time.Second
const defaultCaptureRetention = 30 * 24 * time.Hour
const maxVisionImageBytes = 10 << 20

type Service struct {
	xc                   xiaozhiclient.Client
	store                *Store
	visionForwarder      visionForwarder
	mediaRoot            string
	captureTimeout       time.Duration
	captureRetention     time.Duration
	maxCapturesPerDevice int
	greetingTrigger      sharedtypes.ProactiveGreetingTrigger
	mindSink             MindSink
	logger               *log.Logger
	mu                   sync.Mutex
	presenceByDevice     map[string]Presence
}

type visionForwarder interface {
	ForwardVisionMultipart(ctx context.Context, req xiaozhiclient.VisionForwardRequest) (xiaozhiclient.VisionForwardResponse, error)
}

func NewService(xc xiaozhiclient.Client, triggers ...sharedtypes.ProactiveGreetingTrigger) *Service {
	var trigger sharedtypes.ProactiveGreetingTrigger
	if len(triggers) > 0 {
		trigger = triggers[0]
	}
	return &Service{
		xc:                   xc,
		greetingTrigger:      trigger,
		captureTimeout:       lookCaptureTimeout,
		captureRetention:     defaultCaptureRetention,
		maxCapturesPerDevice: 100,
		logger:               log.Default(),
		presenceByDevice:     make(map[string]Presence),
	}
}

func (s *Service) UseStore(store *Store) {
	s.store = store
}

func (s *Service) UseMediaRoot(root string) {
	s.mediaRoot = strings.TrimSpace(root)
}

func (s *Service) UseVisionForwarder(forwarder visionForwarder) {
	s.visionForwarder = forwarder
}

func (s *Service) UseCaptureTimeout(timeout time.Duration) {
	if timeout > 0 {
		s.captureTimeout = timeout
	}
}

func (s *Service) UseRetentionDays(days int) {
	if days > 0 {
		s.captureRetention = time.Duration(days) * 24 * time.Hour
	}
}

func (s *Service) UseMaxCapturesPerDevice(max int) {
	if max > 0 {
		s.maxCapturesPerDevice = max
	}
}

func (s *Service) UseMindSink(sink MindSink) {
	s.mindSink = sink
}

func (s *Service) UseLogger(logger *log.Logger) {
	if logger != nil {
		s.logger = logger
	}
}

func (s *Service) Look(ctx context.Context, req LookRequest) (CaptureDTO, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return CaptureDTO{}, ErrInvalidInput
	}
	if s.store == nil {
		return CaptureDTO{}, ErrStoreUnavailable
	}
	if pending, err := s.store.FindPendingCapture(ctx, deviceID); err == nil {
		return captureDTO(pending), ErrCaptureInProgress
	} else if !errors.Is(err, ErrNotFound) {
		return CaptureDTO{}, err
	}

	captureID, err := newCaptureID()
	if err != nil {
		return CaptureDTO{}, err
	}
	now := time.Now().UTC()
	capture := Capture{
		CaptureID: captureID,
		DeviceID:  deviceID,
		Status:    CaptureStatusPending,
		Presence:  PresenceUnknown,
		ExpiresAt: now.Add(s.captureRetentionDuration()),
	}
	if err := s.store.CreateCapture(ctx, &capture); err != nil {
		return CaptureDTO{}, err
	}

	callCtx, cancel := s.withLookCaptureTimeout(ctx)
	defer cancel()
	_, err = s.xc.CallDeviceMCPTool(callCtx, deviceID, DefaultCaptureTool, map[string]any{
		"question": buildLookQuestion(captureID),
	})
	if err != nil {
		if latest, latestErr := s.store.GetCapture(ctx, deviceID, captureID); latestErr == nil && latest.Status != CaptureStatusPending {
			return captureDTO(latest), nil
		}
		capture.Status = CaptureStatusFailed
		capture.FailureCode = captureFailureCode(err)
		capture.FailureMessage = captureFailureMessage(err)
		s.logCaptureFailure(capture, "mcp_call", capture.FailureCode)
		_ = s.store.UpdateCapture(ctx, &capture)
		return captureDTO(capture), err
	}

	latest, err := s.store.GetCapture(ctx, deviceID, captureID)
	if err != nil {
		return captureDTO(capture), nil
	}
	return captureDTO(latest), nil
}

func (s *Service) HandleDeviceVisionUpload(ctx context.Context, req DeviceVisionUpload) (xiaozhiclient.VisionForwardResponse, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" || strings.TrimSpace(req.Question) == "" || len(req.Image) == 0 {
		return xiaozhiclient.VisionForwardResponse{}, ErrImageUploadInvalid
	}
	if len(req.Image) > maxVisionImageBytes {
		return xiaozhiclient.VisionForwardResponse{}, ErrImageTooLarge
	}
	contentType := strings.TrimSpace(req.ContentType)
	if !isSupportedImageContentType(contentType) || !isValidImageBytes(contentType, req.Image) {
		return xiaozhiclient.VisionForwardResponse{}, ErrImageUploadInvalid
	}
	if s.visionForwarder == nil {
		return xiaozhiclient.VisionForwardResponse{}, ErrStoreUnavailable
	}

	captureID, cleanedQuestion, marked := extractCaptureMarker(req.Question)
	forwardReq := xiaozhiclient.VisionForwardRequest{
		DeviceID:      deviceID,
		ClientID:      req.ClientID,
		Authorization: req.Authorization,
		Question:      req.Question,
		FileName:      req.FileName,
		ContentType:   contentType,
		Image:         req.Image,
	}

	var capture Capture
	var shouldSave bool
	if marked && s.store != nil && s.mediaRoot != "" {
		if found, err := s.store.GetCapture(ctx, deviceID, captureID); err == nil && found.Status == CaptureStatusPending {
			capture = found
			shouldSave = true
			forwardReq.Question = cleanedQuestion
			if err := s.saveCaptureImage(ctx, &capture, contentType, req.Image); err != nil {
				return xiaozhiclient.VisionForwardResponse{}, err
			}
		}
	}

	resp, err := s.visionForwarder.ForwardVisionMultipart(ctx, forwardReq)
	if !shouldSave {
		return resp, err
	}
	if err != nil {
		capture.Status = CaptureStatusPartial
		capture.FailureCode = "vision_analysis_failed"
		capture.FailureMessage = visionAnalysisFailureMessage
		s.logCaptureFailure(capture, "vision_analysis", capture.FailureCode)
		_ = s.store.UpdateCapture(ctx, &capture)
		return resp, err
	}

	analysis := parseVisionAnalysis(resp.Body)
	capture.AnalysisSummary = analysis.Summary
	capture.Presence = analysis.Presence
	capture.AnalysisRaw = string(resp.Body)
	concernsRaw, _ := json.Marshal(analysis.Concerns)
	capture.ConcernsJSON = string(concernsRaw)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		capture.Status = CaptureStatusPartial
		capture.FailureCode = "vision_analysis_failed"
		capture.FailureMessage = visionAnalysisFailureMessage
	} else {
		capture.Status = CaptureStatusSucceeded
		capture.FailureCode = ""
		capture.FailureMessage = ""
	}
	if err := s.store.UpdateCapture(ctx, &capture); err != nil {
		return resp, err
	}
	return resp, nil
}

func (s *Service) ReadCaptureImage(ctx context.Context, deviceID, captureID string) (CaptureImage, error) {
	deviceID = strings.TrimSpace(deviceID)
	captureID = strings.TrimSpace(captureID)
	if deviceID == "" || captureID == "" {
		return CaptureImage{}, ErrInvalidInput
	}
	if s.store == nil || s.mediaRoot == "" {
		return CaptureImage{}, ErrStoreUnavailable
	}
	capture, err := s.store.GetCapture(ctx, deviceID, captureID)
	if err != nil {
		return CaptureImage{}, err
	}
	if capture.Status == CaptureStatusExpired {
		return CaptureImage{}, ErrCaptureExpired
	}
	if capture.ImageRelativePath == "" {
		return CaptureImage{}, ErrNotFound
	}
	path := filepath.Join(s.mediaRoot, filepath.FromSlash(capture.ImageRelativePath))
	data, err := os.ReadFile(path)
	if err != nil {
		return CaptureImage{}, err
	}
	return CaptureImage{
		Bytes:       data,
		ContentType: capture.ImageContentType,
		Size:        int64(len(data)),
		SHA256:      capture.ImageSHA256,
	}, nil
}

func (s *Service) ReanalyzeCapture(ctx context.Context, req ReanalyzeRequest) (CaptureDTO, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	captureID := strings.TrimSpace(req.CaptureID)
	if deviceID == "" || captureID == "" {
		return CaptureDTO{}, ErrInvalidInput
	}
	if s.store == nil || s.mediaRoot == "" || s.visionForwarder == nil {
		return CaptureDTO{}, ErrStoreUnavailable
	}
	capture, err := s.store.GetCapture(ctx, deviceID, captureID)
	if err != nil {
		return CaptureDTO{}, err
	}
	image, err := s.ReadCaptureImage(ctx, deviceID, captureID)
	if err != nil {
		return CaptureDTO{}, err
	}

	resp, err := s.visionForwarder.ForwardVisionMultipart(ctx, xiaozhiclient.VisionForwardRequest{
		DeviceID:    deviceID,
		Question:    buildReanalyzeQuestion(),
		FileName:    filepath.Base(capture.ImageRelativePath),
		ContentType: image.ContentType,
		Image:       image.Bytes,
	})
	if err != nil {
		capture.Status = CaptureStatusPartial
		capture.FailureCode = "vision_analysis_failed"
		capture.FailureMessage = visionAnalysisFailureMessage
		_ = s.store.UpdateCapture(ctx, &capture)
		return captureDTO(capture), err
	}

	analysis := parseVisionAnalysis(resp.Body)
	capture.AnalysisSummary = analysis.Summary
	capture.Presence = analysis.Presence
	capture.AnalysisRaw = string(resp.Body)
	concernsRaw, _ := json.Marshal(analysis.Concerns)
	capture.ConcernsJSON = string(concernsRaw)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		capture.Status = CaptureStatusPartial
		capture.FailureCode = "vision_analysis_failed"
		capture.FailureMessage = visionAnalysisFailureMessage
	} else {
		capture.Status = CaptureStatusSucceeded
		capture.FailureCode = ""
		capture.FailureMessage = ""
	}
	if err := s.store.UpdateCapture(ctx, &capture); err != nil {
		return CaptureDTO{}, err
	}
	return captureDTO(capture), nil
}

func (s *Service) GetCapture(ctx context.Context, deviceID, captureID string) (CaptureDTO, error) {
	deviceID = strings.TrimSpace(deviceID)
	captureID = strings.TrimSpace(captureID)
	if deviceID == "" || captureID == "" {
		return CaptureDTO{}, ErrInvalidInput
	}
	if s.store == nil {
		return CaptureDTO{}, ErrStoreUnavailable
	}
	capture, err := s.store.GetCapture(ctx, deviceID, captureID)
	if err != nil {
		return CaptureDTO{}, err
	}
	return captureDTO(capture), nil
}

func (s *Service) ListCaptures(ctx context.Context, req CaptureListRequest) ([]CaptureDTO, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, ErrInvalidInput
	}
	if s.store == nil {
		return nil, ErrStoreUnavailable
	}
	req.DeviceID = deviceID
	captures, err := s.store.ListCaptures(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]CaptureDTO, 0, len(captures))
	for _, capture := range captures {
		out = append(out, captureDTO(capture))
	}
	return out, nil
}

func (s *Service) FinalizeTimedOutCaptures(ctx context.Context, now time.Time) (int, error) {
	if s.store == nil {
		return 0, ErrStoreUnavailable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	timeout := s.captureTimeout
	if timeout <= 0 {
		timeout = lookCaptureTimeout
	}
	captures, err := s.store.ListPendingCapturesCreatedBefore(ctx, now.UTC().Add(-timeout))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, capture := range captures {
		capture.Status = CaptureStatusFailed
		capture.FailureCode = "capture_timeout"
		capture.FailureMessage = "拍摄超时，请稍后重试"
		s.logCaptureFailure(capture, "capture_timeout", capture.FailureCode)
		if err := s.store.UpdateCapture(ctx, &capture); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Service) ExpireCaptures(ctx context.Context, now time.Time) (int, error) {
	if s.store == nil {
		return 0, ErrStoreUnavailable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	captures, err := s.store.ListCapturesExpiredBefore(ctx, now.UTC())
	if err != nil {
		return 0, err
	}
	count := 0
	for _, capture := range captures {
		if err := s.expireCapture(ctx, &capture); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Service) PruneExcessCaptures(ctx context.Context) (int, error) {
	if s.store == nil {
		return 0, ErrStoreUnavailable
	}
	max := s.maxCapturesPerDevice
	if max <= 0 {
		max = 100
	}
	deviceIDs, err := s.store.ListDeviceIDsWithActiveCaptures(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, deviceID := range deviceIDs {
		captures, err := s.store.ListActiveCapturesBeyondLimit(ctx, deviceID, max)
		if err != nil {
			return count, err
		}
		for _, capture := range captures {
			if err := s.expireCapture(ctx, &capture); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

func (s *Service) expireCapture(ctx context.Context, capture *Capture) error {
	if capture.ImageRelativePath != "" && s.mediaRoot != "" {
		path := filepath.Join(s.mediaRoot, filepath.FromSlash(capture.ImageRelativePath))
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	capture.Status = CaptureStatusExpired
	capture.ImageRelativePath = ""
	capture.ImageContentType = ""
	capture.ImageSize = 0
	capture.ImageSHA256 = ""
	return s.store.UpdateCapture(ctx, capture)
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

	callCtx, cancel := withVisionMCPTimeout(ctx)
	defer cancel()

	raw, err := s.xc.CallDeviceMCPTool(callCtx, deviceID, tool, req.Args)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{
		DeviceID: deviceID,
		Tool:     tool,
		Raw:      raw,
	}, nil
}

func (s *Service) CaptureAndObservePresence(ctx context.Context, req CaptureRequest) (PresenceCheckResult, error) {
	workflowCtx, cancel := withVisionMCPTimeout(ctx)
	defer cancel()

	capture, err := s.Capture(workflowCtx, req)
	if err != nil {
		return PresenceCheckResult{}, err
	}

	presence, err := parsePresence(capture.Raw)
	if err != nil {
		return PresenceCheckResult{Capture: capture}, err
	}
	observation, err := s.ObservePresence(workflowCtx, PresenceObservationRequest{
		DeviceID: capture.DeviceID,
		Presence: presence,
	})
	return PresenceCheckResult{
		Capture:     capture,
		Observation: observation,
	}, err
}

func (s *Service) PollPresence(ctx context.Context, deviceID string) (PresencePollResult, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return PresencePollResult{}, ErrInvalidInput
	}

	workflowCtx, cancel := withVisionMCPTimeout(ctx)
	defer cancel()

	result := PresencePollResult{DeviceID: deviceID}
	status, err := s.xc.GetDeviceStatus(workflowCtx, deviceID)
	if err != nil {
		return result, err
	}
	if !status.Online {
		result.Skipped = true
		result.SkipReason = "device offline"
		return result, nil
	}

	check, err := s.CaptureAndObservePresence(workflowCtx, CaptureRequest{DeviceID: deviceID})
	result.Check = check
	return result, err
}

func (s *Service) ObservePresence(ctx context.Context, req PresenceObservationRequest) (PresenceObservationResult, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	presence := normalizePresence(req.Presence)
	if deviceID == "" || presence == PresenceUnknown {
		return PresenceObservationResult{}, ErrInvalidInput
	}

	observedAt := req.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	s.mu.Lock()
	previous := s.presenceByDevice[deviceID]
	if previous == "" {
		previous = PresenceUnknown
	}
	s.presenceByDevice[deviceID] = presence
	shouldTrigger := previous == PresenceNoOne && presence == PresenceSomeone
	s.mu.Unlock()

	result := PresenceObservationResult{
		DeviceID:         deviceID,
		PreviousPresence: previous,
		Presence:         presence,
		ObservedAt:       observedAt,
	}
	if s.mindSink != nil {
		eventType := "presence_absent"
		if presence == PresenceSomeone {
			eventType = "presence_seen"
		}
		if err := s.mindSink.IngestMindEvent(ctx, MindEvent{
			DeviceID: deviceID,
			Type:     eventType,
			Summary:  "视觉 presence 进入安伴心智",
			Payload:  map[string]any{"presence": string(presence)},
		}); err != nil {
			return result, err
		}
		return result, nil
	}
	if !shouldTrigger || s.greetingTrigger == nil {
		return result, nil
	}

	greeting, err := s.greetingTrigger.TriggerProactiveGreeting(ctx, deviceID)
	result.Greeting = &greeting
	if errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.TriggeredGreeting = true
	return result, nil
}

func withVisionMCPTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= visionMCPTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, visionMCPTimeout)
}

func (s *Service) withLookCaptureTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := s.captureTimeout
	if timeout <= 0 {
		timeout = lookCaptureTimeout
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func (s *Service) captureRetentionDuration() time.Duration {
	if s.captureRetention <= 0 {
		return defaultCaptureRetention
	}
	return s.captureRetention
}

func captureFailureCode(err error) string {
	switch {
	case errors.Is(err, xiaozhiclient.ErrDeviceOffline):
		return "device_offline"
	case errors.Is(err, xiaozhiclient.ErrUpstreamTimeout):
		return "capture_timeout"
	case errors.Is(err, xiaozhiclient.ErrMCPToolUnavailable):
		return "camera_tool_unavailable"
	default:
		return "camera_tool_unavailable"
	}
}

const visionAnalysisFailureMessage = "图片已保存，但画面分析暂时失败"

func captureFailureMessage(err error) string {
	switch captureFailureCode(err) {
	case "device_offline":
		return "设备当前离线，请确认设备已联网"
	case "capture_timeout":
		return "拍摄超时，请稍后重试"
	default:
		return "摄像头暂时不可用，请稍后重试"
	}
}

func (s *Service) logCaptureFailure(capture Capture, stage, category string) {
	if s.logger == nil {
		return
	}
	s.logger.Printf(
		"vision capture failure captureId=%s device=%s stage=%s category=%s",
		capture.CaptureID,
		deviceHash(capture.DeviceID),
		stage,
		category,
	)
}

func newCaptureID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "cap_" + hex.EncodeToString(b[:]), nil
}

func buildLookQuestion(captureID string) string {
	return fmt.Sprintf(`[[ANBAN_CAPTURE:%s]] 请拍摄当前画面并观察老人状态。请尽量返回 JSON，字段包括 summary、presence、concerns；presence 只能是 someone、no_one 或 unknown。`, captureID)
}

func buildReanalyzeQuestion() string {
	return `请重新观察这张图片里的老人状态。请尽量返回 JSON，字段包括 summary、presence、concerns；presence 只能是 someone、no_one 或 unknown。`
}

func extractCaptureMarker(question string) (captureID string, cleaned string, ok bool) {
	const prefix = "[[ANBAN_CAPTURE:"
	start := strings.Index(question, prefix)
	if start < 0 {
		return "", question, false
	}
	afterPrefix := question[start+len(prefix):]
	end := strings.Index(afterPrefix, "]]")
	if end < 0 {
		return "", question, false
	}
	captureID = strings.TrimSpace(afterPrefix[:end])
	if captureID == "" {
		return "", question, false
	}
	cleaned = strings.TrimSpace(question[:start] + afterPrefix[end+len("]]"):])
	return captureID, cleaned, true
}

func (s *Service) saveCaptureImage(ctx context.Context, capture *Capture, contentType string, image []byte) error {
	now := time.Now().UTC()
	sum := sha256.Sum256(image)
	relDir := filepath.ToSlash(filepath.Join("vision", deviceHash(capture.DeviceID), now.Format("2006"), now.Format("01")))
	ext := ".jpg"
	if contentType == "image/png" {
		ext = ".png"
	}
	relPath := filepath.ToSlash(filepath.Join(relDir, capture.CaptureID+ext))
	absDir := filepath.Join(s.mediaRoot, filepath.FromSlash(relDir))
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return err
	}
	finalPath := filepath.Join(s.mediaRoot, filepath.FromSlash(relPath))
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, image, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	capture.ImageRelativePath = relPath
	capture.ImageContentType = contentType
	capture.ImageSize = int64(len(image))
	capture.ImageSHA256 = hex.EncodeToString(sum[:])
	capture.CapturedAt = &now
	return s.store.UpdateCapture(ctx, capture)
}

func deviceHash(deviceID string) string {
	sum := sha256.Sum256([]byte(deviceID))
	return hex.EncodeToString(sum[:8])
}

func isSupportedImageContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg", "image/png":
		return true
	default:
		return false
	}
}

func isValidImageBytes(contentType string, image []byte) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return len(image) >= 4 && image[0] == 0xff && image[1] == 0xd8 && image[len(image)-2] == 0xff && image[len(image)-1] == 0xd9
	case "image/png":
		return len(image) >= 8 && bytes.Equal(image[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	default:
		return false
	}
}

func parseVisionAnalysis(body []byte) CaptureAnalysis {
	body = []byte(strings.TrimSpace(string(body)))
	if len(body) == 0 {
		return CaptureAnalysis{Presence: PresenceUnknown}
	}
	var text string
	if err := json.Unmarshal(body, &text); err == nil {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
			return parseVisionAnalysis([]byte(text))
		}
		return CaptureAnalysis{Summary: text, Presence: PresenceUnknown}
	}
	var payload struct {
		Summary  string          `json:"summary"`
		Presence Presence        `json:"presence"`
		Concerns json.RawMessage `json:"concerns"`
		Data     json.RawMessage `json:"data"`
		Result   json.RawMessage `json:"result"`
		Response json.RawMessage `json:"response"`
		Text     string          `json:"text"`
		Content  []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return CaptureAnalysis{Summary: string(body), Presence: PresenceUnknown}
	}
	hasDirectFields := payload.Summary != "" || payload.Presence != "" || len(payload.Concerns) > 0
	if !hasDirectFields {
		for _, nested := range []json.RawMessage{payload.Data, payload.Result, payload.Response} {
			if len(nested) > 0 && string(nested) != "null" {
				return parseVisionAnalysis(nested)
			}
		}
		if strings.TrimSpace(payload.Text) != "" {
			return parseVisionAnalysis([]byte(jsonString(payload.Text)))
		}
		for _, item := range payload.Content {
			if strings.TrimSpace(item.Text) != "" {
				return parseVisionAnalysis([]byte(jsonString(item.Text)))
			}
		}
	}
	analysis := CaptureAnalysis{
		Summary:  strings.TrimSpace(payload.Summary),
		Presence: normalizePresence(payload.Presence),
		Concerns: parseConcerns(payload.Concerns),
	}
	return analysis
}

func parseConcerns(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var concerns []string
	if err := json.Unmarshal(raw, &concerns); err == nil {
		return concerns
	}
	return nil
}

func jsonString(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func captureDTO(capture Capture) CaptureDTO {
	analysis := CaptureAnalysis{
		Summary:  capture.AnalysisSummary,
		Presence: capture.Presence,
		Concerns: parseConcerns(json.RawMessage(capture.ConcernsJSON)),
	}
	return CaptureDTO{
		CaptureID:      capture.CaptureID,
		DeviceID:       capture.DeviceID,
		Status:         capture.Status,
		CapturedAt:     capture.CapturedAt,
		ImageURL:       captureImageURL(capture),
		Analysis:       analysis,
		FailureCode:    capture.FailureCode,
		FailureMessage: capture.FailureMessage,
	}
}

func captureImageURL(capture Capture) string {
	if capture.ImageRelativePath == "" {
		return ""
	}
	return "/api/vision/captures/" + capture.CaptureID + "/image"
}

func parsePresence(raw json.RawMessage) (Presence, error) {
	var payload struct {
		Presence Presence `json:"presence"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return PresenceUnknown, ErrPresenceUnavailable
	}
	presence := normalizePresence(payload.Presence)
	if presence == PresenceUnknown {
		return PresenceUnknown, ErrPresenceUnavailable
	}
	return presence, nil
}

func normalizePresence(presence Presence) Presence {
	switch presence {
	case PresenceSomeone, PresenceNoOne:
		return presence
	default:
		return PresenceUnknown
	}
}
