package vision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
	"github.com/gin-gonic/gin"
)

func TestHandlerLookUsesBoundDeviceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(newVisionTestStore(t))
	r := gin.New()
	r.Use(accountDeviceContext("dev-bound"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/look", strings.NewReader(`{"deviceId":"dev-from-body"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/vision/look status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if xc.gotDeviceID != "dev-bound" {
		t.Fatalf("MCP deviceID = %q, want bound device", xc.gotDeviceID)
	}
	var payload CaptureDTO
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal look: %v", err)
	}
	if payload.DeviceID != "dev-bound" || payload.CaptureID == "" || payload.Status != CaptureStatusPending {
		t.Fatalf("payload = %+v, want pending capture for bound device", payload)
	}
}

func TestHandlerLookReturnsClassifiedXiaozhiFailureCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{err: xiaozhiclient.ErrDeviceOffline})
	svc.UseStore(newVisionTestStore(t))
	r := gin.New()
	r.Use(accountDeviceContext("dev-bound"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/look", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("POST /api/vision/look status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		Error   string     `json:"error"`
		Capture CaptureDTO `json:"capture"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal look error: %v", err)
	}
	if payload.Error != "device_offline" || payload.Capture.FailureCode != "device_offline" {
		t.Fatalf("payload = %+v, want device_offline at HTTP boundary", payload)
	}
}

func TestHandlerLookDoesNotExposeUpstreamFailureDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{err: errors.New("xiaozhi manager POST http://internal.example/mcp -> 500: token=SUPERSECRET")})
	svc.UseStore(newVisionTestStore(t))
	r := gin.New()
	r.Use(accountDeviceContext("dev-bound"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/look", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("POST /api/vision/look status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "SUPERSECRET") || strings.Contains(w.Body.String(), "internal.example") {
		t.Fatalf("response leaked upstream failure details: %s", w.Body.String())
	}
	var payload struct {
		Error   string     `json:"error"`
		Capture CaptureDTO `json:"capture"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal look error: %v", err)
	}
	if payload.Error != "camera_tool_unavailable" || payload.Capture.FailureMessage != "摄像头暂时不可用，请稍后重试" {
		t.Fatalf("payload = %+v, want sanitized actionable failure", payload)
	}
}

func TestHandlerCaptureImageReturnsRawBytesForBoundDevice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{
			StatusCode:  http.StatusOK,
			ContentType: "application/json",
			Body:        []byte(`{"summary":"老人正在沙发上休息","presence":"someone","concerns":[]}`),
		},
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(newVisionTestStore(t))
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)

	look, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	question, _ := xc.gotArgs["question"].(string)
	image := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9}
	if _, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:    "dev-001",
		Question:    question,
		FileName:    "camera.jpg",
		ContentType: "image/jpeg",
		Image:       image,
	}); err != nil {
		t.Fatalf("HandleDeviceVisionUpload: %v", err)
	}

	r := gin.New()
	r.Use(accountDeviceContext("dev-001"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))
	req := httptest.NewRequest(http.MethodGet, "/api/vision/captures/"+look.CaptureID+"/image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET image status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), image) {
		t.Fatalf("image bytes = %v, want original", w.Body.Bytes())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want image/jpeg", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Content-Length") == "" {
		t.Fatal("Content-Length is empty")
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "private") {
		t.Fatalf("Cache-Control = %q, want private cache policy", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", w.Header().Get("X-Content-Type-Options"))
	}
}

func TestHandlerExpiredCaptureOperationsReturnCaptureExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	visionStore := newVisionTestStore(t)
	expired := Capture{
		CaptureID: "cap_expired",
		DeviceID:  "dev-001",
		Status:    CaptureStatusExpired,
		Presence:  PresenceUnknown,
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := visionStore.CreateCapture(context.Background(), &expired); err != nil {
		t.Fatalf("CreateCapture: %v", err)
	}

	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(&fakeVisionForwarder{})
	r := gin.New()
	r.Use(accountDeviceContext("dev-001"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	for _, request := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/vision/captures/cap_expired/image"},
		{method: http.MethodPost, path: "/api/vision/captures/cap_expired/reanalyze"},
	} {
		req := httptest.NewRequest(request.method, request.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusGone {
			t.Fatalf("%s %s status = %d, want 410; body=%s", request.method, request.path, w.Code, w.Body.String())
		}
		var payload struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal expired response: %v", err)
		}
		if payload.Error != "capture_expired" {
			t.Fatalf("%s %s error = %q, want capture_expired", request.method, request.path, payload.Error)
		}
	}
}

func TestHandlerCaptureImageDoesNotRevealAnotherDevicesCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	visionStore := newVisionTestStore(t)
	capture := Capture{
		CaptureID: "cap_private",
		DeviceID:  "dev-owner",
		Status:    CaptureStatusSucceeded,
		Presence:  PresenceSomeone,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := visionStore.CreateCapture(context.Background(), &capture); err != nil {
		t.Fatalf("CreateCapture: %v", err)
	}
	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	r := gin.New()
	r.Use(accountDeviceContext("dev-other"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/vision/captures/cap_private/image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "capture_not_found") {
		t.Fatalf("cross-device image status = %d body=%s, want 404 capture_not_found", w.Code, w.Body.String())
	}
}

func TestHandlerReanalyzeCaptureUsesBoundDeviceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{},
		err:  errors.New("vlm unavailable"),
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(newVisionTestStore(t))
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)

	look, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	question, _ := xc.gotArgs["question"].(string)
	image := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9}
	if _, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:    "dev-001",
		Question:    question,
		FileName:    "camera.jpg",
		ContentType: "image/jpeg",
		Image:       image,
	}); err == nil {
		t.Fatal("HandleDeviceVisionUpload error = nil, want partial capture setup")
	}

	forwarder.err = nil
	forwarder.resp = xiaozhiclient.VisionForwardResponse{
		StatusCode:  http.StatusOK,
		ContentType: "application/json",
		Body:        []byte(`{"summary":"老人正在窗边站着","presence":"someone","concerns":[]}`),
	}

	r := gin.New()
	r.Use(accountDeviceContext("dev-001"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))
	req := httptest.NewRequest(http.MethodPost, "/api/vision/captures/"+look.CaptureID+"/reanalyze", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST reanalyze status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var payload CaptureDTO
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal reanalyze: %v", err)
	}
	if payload.Status != CaptureStatusSucceeded || payload.Analysis.Summary != "老人正在窗边站着" {
		t.Fatalf("payload = %+v, want reanalyzed capture", payload)
	}
}

func TestHandlerListCapturesUsesBoundDeviceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	visionStore := newVisionTestStore(t)
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	for _, capture := range []Capture{
		{CaptureID: "cap_bound", DeviceID: "dev-bound", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
		{CaptureID: "cap_other", DeviceID: "dev-other", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(time.Minute)},
	} {
		capture := capture
		if err := visionStore.CreateCapture(context.Background(), &capture); err != nil {
			t.Fatalf("CreateCapture: %v", err)
		}
	}
	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)
	r := gin.New()
	r.Use(accountDeviceContext("dev-bound"))
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodGet, "/api/vision/captures?deviceId=dev-other", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET captures status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var payload []CaptureDTO
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal captures: %v", err)
	}
	if len(payload) != 1 || payload[0].CaptureID != "cap_bound" {
		t.Fatalf("payload = %+v, want only bound device capture", payload)
	}
}

func TestHandlerDeviceVisionUploadRequiresIngressTokenAndCompletesCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{
			StatusCode:  http.StatusOK,
			ContentType: "text/plain",
			Body:        []byte(`{"summary":"老人正在沙发上休息","presence":"someone","concerns":[]}`),
		},
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(newVisionTestStore(t))
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)

	look, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	question, _ := xc.gotArgs["question"].(string)

	r := gin.New()
	NewHandler(svc).RegisterDeviceRoutes(r.Group("/api"), "secret-token")

	image := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9}
	body, contentType := newDeviceVisionMultipart(t, question, image)
	req := httptest.NewRequest(http.MethodPost, "/api/device/vision?ingress_token=secret-token", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Device-Id", "dev-001")
	req.Header.Set("Client-Id", "client-001")
	req.Header.Set("Authorization", "Bearer device-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/device/vision status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != string(forwarder.resp.Body) {
		t.Fatalf("body = %s, want upstream body", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("content-type = %q, want upstream content-type", w.Header().Get("Content-Type"))
	}

	saved, err := svc.GetCapture(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture: %v", err)
	}
	if saved.Status != CaptureStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", saved.Status)
	}

	body, contentType = newDeviceVisionMultipart(t, question, image)
	req = httptest.NewRequest(http.MethodPost, "/api/device/vision?ingress_token=wrong", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Device-Id", "dev-001")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want 401", w.Code)
	}
}

func TestHandlerCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{raw: json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg"}`)})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(`{
		"deviceId":"dev-001",
		"tool":"camera.capture",
		"args":{"quality":"low"}
	}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/vision/capture status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var payload CaptureResult
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal capture: %v", err)
	}
	if payload.DeviceID != "dev-001" || payload.Tool != "camera.capture" {
		t.Fatalf("payload = %+v, want dev-001 camera.capture", payload)
	}
	if string(payload.Raw) != `{"imageUrl":"https://example.test/capture.jpg"}` {
		t.Fatalf("raw = %s", payload.Raw)
	}
}

func newDeviceVisionMultipart(t *testing.T, question string, image []byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("question", question); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	part, err := writer.CreateFormFile("file", "camera.jpg")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatalf("write image: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func accountDeviceContext(deviceID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(sharedtypes.GinContextAuthMode, "account")
		c.Set(sharedtypes.GinContextDeviceID, deviceID)
		c.Next()
	}
}

func TestHandlerCaptureRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{not-json`},
		{name: "missing device", body: `{"deviceId":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandlerCaptureReturnsBadGatewayWhenMCPFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{err: errors.New("manager unavailable")})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/capture", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerCheckPresenceCapturesAndObserves(t *testing.T) {
	gin.SetMode(gin.TestMode)
	xc := &visionClient{raw: json.RawMessage(`{"presence":"no_one"}`)}
	trigger := &fakeGreetingTrigger{}
	svc := NewService(xc, trigger)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST check-presence no_one status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	xc.raw = json.RawMessage(`{"presence":"someone"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST check-presence someone status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want one return greeting", trigger.calls)
	}

	var payload PresenceCheckResult
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal check presence: %v", err)
	}
	if !payload.Observation.TriggeredGreeting {
		t.Fatalf("payload = %+v, want triggered greeting", payload)
	}
}

func TestHandlerCheckPresenceReturnsBadGatewayWhenPresenceMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{raw: json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg"}`)})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	req := httptest.NewRequest(http.MethodPost, "/api/vision/check-presence", strings.NewReader(`{"deviceId":"dev-001"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerObservePresenceTriggersGreeting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	trigger := &fakeGreetingTrigger{}
	svc := NewService(&visionClient{}, trigger)
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	for _, body := range []string{
		`{"deviceId":"dev-001","presence":"no_one"}`,
		`{"deviceId":"dev-001","presence":"someone"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/vision/presence", strings.NewReader(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("POST /api/vision/presence status = %d, want 200; body=%s", w.Code, w.Body.String())
		}
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want one no_one -> someone greeting", trigger.calls)
	}
}

func TestHandlerObservePresenceRejectsBadRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(&visionClient{}, &fakeGreetingTrigger{})
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"))

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{not-json`},
		{name: "missing device", body: `{"deviceId":"","presence":"someone"}`},
		{name: "bad presence", body: `{"deviceId":"dev-001","presence":"maybe"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/vision/presence", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
		})
	}
}
