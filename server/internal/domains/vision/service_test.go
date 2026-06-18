package vision

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

func TestServiceLookCreatesPendingCaptureAndCallsCameraWithCaptureMarker(t *testing.T) {
	visionStore := newVisionTestStore(t)
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(visionStore)

	result, err := svc.Look(context.Background(), LookRequest{DeviceID: " dev-001 "})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	if result.CaptureID == "" {
		t.Fatal("CaptureID is empty")
	}
	if result.Status != CaptureStatusPending {
		t.Fatalf("status = %q, want pending", result.Status)
	}
	if xc.gotDeviceID != "dev-001" || xc.gotTool != DefaultCaptureTool {
		t.Fatalf("MCP call device=%q tool=%q, want trimmed device and default camera", xc.gotDeviceID, xc.gotTool)
	}
	question, _ := xc.gotArgs["question"].(string)
	if !strings.Contains(question, "[[ANBAN_CAPTURE:"+result.CaptureID+"]]") {
		t.Fatalf("question = %q, want capture marker for %s", question, result.CaptureID)
	}
	if !strings.Contains(question, "summary") || !strings.Contains(question, "presence") || !strings.Contains(question, "concerns") {
		t.Fatalf("question = %q, want structured observation fields", question)
	}

	saved, err := svc.GetCapture(context.Background(), "dev-001", result.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture: %v", err)
	}
	if saved.CaptureID != result.CaptureID || saved.Status != CaptureStatusPending || saved.DeviceID != "dev-001" {
		t.Fatalf("saved = %+v, want pending capture for dev-001", saved)
	}
}

func TestServiceLookRejectsSecondCaptureWhileDeviceHasPendingCapture(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(newVisionTestStore(t))

	first, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("first Look: %v", err)
	}
	second, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if !errors.Is(err, ErrCaptureInProgress) {
		t.Fatalf("second Look error = %v, want ErrCaptureInProgress", err)
	}
	if second.CaptureID != first.CaptureID || second.Status != CaptureStatusPending {
		t.Fatalf("second = %+v, want existing pending capture %+v", second, first)
	}
	if xc.mcpCalls != 1 {
		t.Fatalf("MCP calls = %d, want one camera invocation", xc.mcpCalls)
	}
}

func TestServiceLookMapsXiaozhiErrorsToCaptureFailureCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
	}{
		{name: "offline", err: xiaozhiclient.ErrDeviceOffline, wantCode: "device_offline"},
		{name: "tool unavailable", err: xiaozhiclient.ErrMCPToolUnavailable, wantCode: "camera_tool_unavailable"},
		{name: "timeout", err: xiaozhiclient.ErrUpstreamTimeout, wantCode: "capture_timeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visionStore := newVisionTestStore(t)
			svc := NewService(&visionClient{err: tt.err})
			svc.UseStore(visionStore)

			result, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if result.Status != CaptureStatusFailed || result.FailureCode != tt.wantCode {
				t.Fatalf("result = %+v, want failed %s", result, tt.wantCode)
			}
			saved, err := svc.GetCapture(context.Background(), "dev-001", result.CaptureID)
			if err != nil {
				t.Fatalf("GetCapture: %v", err)
			}
			if saved.FailureCode != tt.wantCode {
				t.Fatalf("saved failure code = %q, want %q", saved.FailureCode, tt.wantCode)
			}
		})
	}
}

func TestServiceLookLogsCaptureFailureWithoutRawDeviceOrUpstreamDetails(t *testing.T) {
	var logs bytes.Buffer
	svc := NewService(&visionClient{err: errors.New("upstream token=SUPERSECRET")})
	svc.UseStore(newVisionTestStore(t))
	svc.UseLogger(log.New(&logs, "", 0))

	result, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-sensitive"})
	if err == nil {
		t.Fatal("Look error = nil, want upstream failure")
	}
	entry := logs.String()
	for _, want := range []string{
		"captureId=" + result.CaptureID,
		"device=" + deviceHash("dev-sensitive"),
		"stage=mcp_call",
		"category=camera_tool_unavailable",
	} {
		if !strings.Contains(entry, want) {
			t.Fatalf("log = %q, want %q", entry, want)
		}
	}
	if strings.Contains(entry, "dev-sensitive") || strings.Contains(entry, "SUPERSECRET") {
		t.Fatalf("log leaked raw device or upstream details: %q", entry)
	}
}

func TestServiceLookUsesConfiguredRetentionDays(t *testing.T) {
	visionStore := newVisionTestStore(t)
	svc := NewService(&visionClient{raw: json.RawMessage(`{"ok":true}`)})
	svc.UseStore(visionStore)
	svc.UseRetentionDays(7)

	start := time.Now().UTC()
	result, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	saved, err := visionStore.GetCapture(context.Background(), "dev-001", result.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture: %v", err)
	}
	minExpiresAt := start.Add(7 * 24 * time.Hour)
	maxExpiresAt := time.Now().UTC().Add(7*24*time.Hour + time.Minute)
	if saved.ExpiresAt.Before(minExpiresAt) || saved.ExpiresAt.After(maxExpiresAt) {
		t.Fatalf("ExpiresAt = %s, want about 7 days from now", saved.ExpiresAt)
	}
}

func TestServiceDeviceVisionUploadCompletesMarkedLookCapture(t *testing.T) {
	visionStore := newVisionTestStore(t)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{
			StatusCode:  http.StatusOK,
			ContentType: "application/json",
			Body:        []byte(`{"summary":"老人正在沙发上休息","presence":"someone","concerns":[]}`),
		},
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)

	look, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	question, _ := xc.gotArgs["question"].(string)
	image := []byte{0xff, 0xd8, 'a', 'n', 'b', 'a', 'n', 0xff, 0xd9}

	resp, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:      "dev-001",
		ClientID:      "client-001",
		Authorization: "Bearer device-token",
		Question:      question,
		FileName:      "camera.jpg",
		ContentType:   "image/jpeg",
		Image:         image,
	})
	if err != nil {
		t.Fatalf("HandleDeviceVisionUpload: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(resp.Body) != string(forwarder.resp.Body) {
		t.Fatalf("response = %+v, want upstream response", resp)
	}
	if forwarder.got.DeviceID != "dev-001" || forwarder.got.ClientID != "client-001" || forwarder.got.Authorization != "Bearer device-token" {
		t.Fatalf("forwarded headers = %+v", forwarder.got)
	}
	if strings.Contains(forwarder.got.Question, "[[ANBAN_CAPTURE:") {
		t.Fatalf("forwarded question still contains capture marker: %q", forwarder.got.Question)
	}
	if !bytes.Equal(forwarder.got.Image, image) {
		t.Fatalf("forwarded image changed: got %v want %v", forwarder.got.Image, image)
	}

	saved, err := svc.GetCapture(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture: %v", err)
	}
	if saved.Status != CaptureStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", saved.Status)
	}
	if saved.Analysis.Summary != "老人正在沙发上休息" || saved.Analysis.Presence != PresenceSomeone {
		t.Fatalf("analysis = %+v, want parsed upstream observation", saved.Analysis)
	}

	storedImage, err := svc.ReadCaptureImage(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("ReadCaptureImage: %v", err)
	}
	if storedImage.ContentType != "image/jpeg" || !bytes.Equal(storedImage.Bytes, image) {
		t.Fatalf("stored image = contentType %q bytes %v", storedImage.ContentType, storedImage.Bytes)
	}
	sum := sha256.Sum256(image)
	if storedImage.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("stored image sha = %q, want original bytes hash", storedImage.SHA256)
	}
}

func TestServiceDeviceVisionUploadRejectsInvalidBytesDeclaredAsJPEG(t *testing.T) {
	forwarder := &fakeVisionForwarder{}
	svc := NewService(&visionClient{})
	svc.UseVisionForwarder(forwarder)

	_, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:    "dev-001",
		Question:    "请看看画面",
		FileName:    "camera.jpg",
		ContentType: "image/jpeg",
		Image:       []byte("not an image"),
	})
	if !errors.Is(err, ErrImageUploadInvalid) {
		t.Fatalf("error = %v, want ErrImageUploadInvalid", err)
	}
	if forwarder.got.DeviceID != "" {
		t.Fatalf("invalid image was forwarded: %+v", forwarder.got)
	}
}

func TestServiceDeviceVisionUploadForwardsUnmarkedRequestWithoutSavingCapture(t *testing.T) {
	visionStore := newVisionTestStore(t)
	forwarder := &fakeVisionForwarder{resp: xiaozhiclient.VisionForwardResponse{
		StatusCode:  http.StatusOK,
		ContentType: "text/plain",
		Body:        []byte("画面里有一把椅子"),
	}}
	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)
	image := []byte{0xff, 0xd8, 0xff, 0xe0, 'v', 'o', 'i', 'c', 'e', 0xff, 0xd9}

	resp, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:      "dev-001",
		ClientID:      "client-001",
		Authorization: "Bearer device-token",
		Question:      "请描述你看到了什么",
		FileName:      "voice.jpg",
		ContentType:   "image/jpeg",
		Image:         image,
	})
	if err != nil {
		t.Fatalf("HandleDeviceVisionUpload: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(resp.Body) != "画面里有一把椅子" {
		t.Fatalf("response = %+v, want transparent upstream response", resp)
	}
	if forwarder.got.Question != "请描述你看到了什么" || !bytes.Equal(forwarder.got.Image, image) {
		t.Fatalf("forwarded request changed: %+v", forwarder.got)
	}
	captures, err := svc.ListCaptures(context.Background(), CaptureListRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("ListCaptures: %v", err)
	}
	if len(captures) != 0 {
		t.Fatalf("captures = %+v, want no record for ordinary voice vision", captures)
	}
}

func TestServiceDeviceVisionUploadParsesWrappedJSONStringAnalysis(t *testing.T) {
	visionStore := newVisionTestStore(t)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{
			StatusCode:  http.StatusOK,
			ContentType: "application/json",
			Body:        []byte(`{"data":"{\"summary\":\"老人没有出现在画面里\",\"presence\":\"no_one\",\"concerns\":[\"灯光较暗\"]}"}`),
		},
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)

	look, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	question, _ := xc.gotArgs["question"].(string)
	if _, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:    "dev-001",
		Question:    question,
		FileName:    "camera.jpg",
		ContentType: "image/jpeg",
		Image:       []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9},
	}); err != nil {
		t.Fatalf("HandleDeviceVisionUpload: %v", err)
	}

	saved, err := svc.GetCapture(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture: %v", err)
	}
	if saved.Analysis.Summary != "老人没有出现在画面里" || saved.Analysis.Presence != PresenceNoOne {
		t.Fatalf("analysis = %+v, want parsed wrapped JSON string", saved.Analysis)
	}
	if len(saved.Analysis.Concerns) != 1 || saved.Analysis.Concerns[0] != "灯光较暗" {
		t.Fatalf("concerns = %+v, want parsed concern", saved.Analysis.Concerns)
	}
}

func TestServiceLookKeepsCompletedUploadWhenMCPReturnsErrorAfterDeviceUpload(t *testing.T) {
	visionStore := newVisionTestStore(t)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{
			StatusCode:  http.StatusOK,
			ContentType: "application/json",
			Body:        []byte(`{"summary":"老人正在沙发上休息","presence":"someone","concerns":[]}`),
		},
	}
	xc := &visionClient{err: errors.New("mcp call closed after upload")}
	svc := NewService(xc)
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)
	xc.onMCP = func(ctx context.Context, deviceID, tool string, args map[string]any) {
		question, _ := args["question"].(string)
		_, _ = svc.HandleDeviceVisionUpload(ctx, DeviceVisionUpload{
			DeviceID:    deviceID,
			Question:    question,
			FileName:    "camera.jpg",
			ContentType: "image/jpeg",
			Image:       []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9},
		})
	}

	result, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look error = %v, want completed upload to win over trailing MCP error", err)
	}
	if result.Status != CaptureStatusSucceeded {
		t.Fatalf("result status = %q, want succeeded capture preserved", result.Status)
	}
	saved, err := svc.GetCapture(context.Background(), "dev-001", result.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture: %v", err)
	}
	if saved.Status != CaptureStatusSucceeded || saved.Analysis.Summary != "老人正在沙发上休息" {
		t.Fatalf("saved = %+v, want completed upload preserved", saved)
	}
}

func TestServiceReanalyzeCaptureUsesSavedImageAndUpdatesPartialCapture(t *testing.T) {
	var logs bytes.Buffer
	visionStore := newVisionTestStore(t)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{},
		err:  errors.New("vlm unavailable"),
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)
	svc.UseLogger(log.New(&logs, "", 0))

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
		t.Fatal("HandleDeviceVisionUpload error = nil, want analysis failure")
	}

	partial, err := svc.GetCapture(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture partial: %v", err)
	}
	if partial.Status != CaptureStatusPartial {
		t.Fatalf("status = %q, want partial after analysis failure", partial.Status)
	}
	for _, want := range []string{
		"captureId=" + look.CaptureID,
		"device=" + deviceHash("dev-001"),
		"stage=vision_analysis",
		"category=vision_analysis_failed",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("log = %q, want %q", logs.String(), want)
		}
	}

	forwarder.err = nil
	forwarder.resp = xiaozhiclient.VisionForwardResponse{
		StatusCode:  http.StatusOK,
		ContentType: "application/json",
		Body:        []byte(`{"summary":"老人坐在沙发上看电视","presence":"someone","concerns":["桌边有水杯"]}`),
	}
	result, err := svc.ReanalyzeCapture(context.Background(), ReanalyzeRequest{
		DeviceID:  " dev-001 ",
		CaptureID: look.CaptureID,
	})
	if err != nil {
		t.Fatalf("ReanalyzeCapture: %v", err)
	}
	if result.Status != CaptureStatusSucceeded {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if result.Analysis.Summary != "老人坐在沙发上看电视" || result.Analysis.Presence != PresenceSomeone {
		t.Fatalf("analysis = %+v, want updated VLM analysis", result.Analysis)
	}
	if len(result.Analysis.Concerns) != 1 || result.Analysis.Concerns[0] != "桌边有水杯" {
		t.Fatalf("concerns = %+v, want parsed concern", result.Analysis.Concerns)
	}
	if strings.Contains(forwarder.got.Question, "[[ANBAN_CAPTURE:") {
		t.Fatalf("reanalyze question contains capture marker: %q", forwarder.got.Question)
	}
	if !bytes.Equal(forwarder.got.Image, image) {
		t.Fatalf("reanalyze image changed: got %v want %v", forwarder.got.Image, image)
	}
}

func TestServiceListCapturesReturnsRecentCapturesForDevice(t *testing.T) {
	visionStore := newVisionTestStore(t)
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	captures := []Capture{
		{CaptureID: "cap_old", DeviceID: "dev-001", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-time.Hour)},
		{CaptureID: "cap_other", DeviceID: "dev-002", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
		{CaptureID: "cap_new", DeviceID: "dev-001", Status: CaptureStatusPartial, Presence: PresenceUnknown, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now},
	}
	for i := range captures {
		if err := visionStore.CreateCapture(context.Background(), &captures[i]); err != nil {
			t.Fatalf("CreateCapture %s: %v", captures[i].CaptureID, err)
		}
	}
	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)

	list, err := svc.ListCaptures(context.Background(), CaptureListRequest{DeviceID: " dev-001 ", Limit: 10})
	if err != nil {
		t.Fatalf("ListCaptures: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v, want 2 captures for dev-001", list)
	}
	if list[0].CaptureID != "cap_new" || list[1].CaptureID != "cap_old" {
		t.Fatalf("list order = %+v, want newest dev-001 first", list)
	}
}

func TestServiceFinalizeTimedOutCapturesFailsOnlyStalePendingCaptures(t *testing.T) {
	var logs bytes.Buffer
	visionStore := newVisionTestStore(t)
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	captures := []Capture{
		{CaptureID: "cap_old_pending", DeviceID: "dev-old", Status: CaptureStatusPending, Presence: PresenceUnknown, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-31 * time.Second)},
		{CaptureID: "cap_fresh_pending", DeviceID: "dev-fresh", Status: CaptureStatusPending, Presence: PresenceUnknown, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-5 * time.Second)},
		{CaptureID: "cap_done", DeviceID: "dev-done", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-time.Hour)},
	}
	for i := range captures {
		if err := visionStore.CreateCapture(context.Background(), &captures[i]); err != nil {
			t.Fatalf("CreateCapture %s: %v", captures[i].CaptureID, err)
		}
	}
	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)
	svc.UseCaptureTimeout(30 * time.Second)
	svc.UseLogger(log.New(&logs, "", 0))

	count, err := svc.FinalizeTimedOutCaptures(context.Background(), now)
	if err != nil {
		t.Fatalf("FinalizeTimedOutCaptures: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want one timed-out capture", count)
	}
	old, err := svc.GetCapture(context.Background(), "dev-old", "cap_old_pending")
	if err != nil {
		t.Fatalf("GetCapture old: %v", err)
	}
	if old.Status != CaptureStatusFailed || old.FailureCode != "capture_timeout" {
		t.Fatalf("old = %+v, want failed capture_timeout", old)
	}
	for _, want := range []string{
		"captureId=cap_old_pending",
		"device=" + deviceHash("dev-old"),
		"stage=capture_timeout",
		"category=capture_timeout",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("log = %q, want %q", logs.String(), want)
		}
	}
	fresh, err := svc.GetCapture(context.Background(), "dev-fresh", "cap_fresh_pending")
	if err != nil {
		t.Fatalf("GetCapture fresh: %v", err)
	}
	if fresh.Status != CaptureStatusPending {
		t.Fatalf("fresh status = %q, want pending", fresh.Status)
	}
	done, err := svc.GetCapture(context.Background(), "dev-done", "cap_done")
	if err != nil {
		t.Fatalf("GetCapture done: %v", err)
	}
	if done.Status != CaptureStatusSucceeded {
		t.Fatalf("done status = %q, want succeeded", done.Status)
	}
}

func TestServiceExpireCapturesRemovesExpiredImageAndMarksCaptureExpired(t *testing.T) {
	visionStore := newVisionTestStore(t)
	forwarder := &fakeVisionForwarder{
		resp: xiaozhiclient.VisionForwardResponse{
			StatusCode:  http.StatusOK,
			ContentType: "application/json",
			Body:        []byte(`{"summary":"老人正在沙发上休息","presence":"someone","concerns":[]}`),
		},
	}
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)
	svc.UseStore(visionStore)
	svc.UseMediaRoot(t.TempDir())
	svc.UseVisionForwarder(forwarder)

	look, err := svc.Look(context.Background(), LookRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Look: %v", err)
	}
	question, _ := xc.gotArgs["question"].(string)
	if _, err := svc.HandleDeviceVisionUpload(context.Background(), DeviceVisionUpload{
		DeviceID:    "dev-001",
		Question:    question,
		FileName:    "camera.jpg",
		ContentType: "image/jpeg",
		Image:       []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xff, 0xd9},
	}); err != nil {
		t.Fatalf("HandleDeviceVisionUpload: %v", err)
	}
	capture, err := visionStore.GetCapture(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture setup: %v", err)
	}
	now := time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC)
	capture.ExpiresAt = now.Add(-time.Second)
	if err := visionStore.UpdateCapture(context.Background(), &capture); err != nil {
		t.Fatalf("UpdateCapture setup: %v", err)
	}

	count, err := svc.ExpireCaptures(context.Background(), now)
	if err != nil {
		t.Fatalf("ExpireCaptures: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want one expired capture", count)
	}
	expired, err := svc.GetCapture(context.Background(), "dev-001", look.CaptureID)
	if err != nil {
		t.Fatalf("GetCapture expired: %v", err)
	}
	if expired.Status != CaptureStatusExpired || expired.ImageURL != "" {
		t.Fatalf("expired = %+v, want expired without image URL", expired)
	}
	if _, err := svc.ReadCaptureImage(context.Background(), "dev-001", look.CaptureID); !errors.Is(err, ErrCaptureExpired) {
		t.Fatalf("ReadCaptureImage err = %v, want ErrCaptureExpired after expiration", err)
	}
}

func TestServicePruneExcessCapturesExpiresOldestCapturesPerDevice(t *testing.T) {
	visionStore := newVisionTestStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	captures := []Capture{
		{CaptureID: "cap_oldest", DeviceID: "dev-001", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-3 * time.Hour)},
		{CaptureID: "cap_middle", DeviceID: "dev-001", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-2 * time.Hour)},
		{CaptureID: "cap_newest", DeviceID: "dev-001", Status: CaptureStatusPartial, Presence: PresenceUnknown, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-time.Hour)},
		{CaptureID: "cap_other", DeviceID: "dev-002", Status: CaptureStatusSucceeded, Presence: PresenceSomeone, ExpiresAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-4 * time.Hour)},
	}
	for i := range captures {
		if err := visionStore.CreateCapture(context.Background(), &captures[i]); err != nil {
			t.Fatalf("CreateCapture %s: %v", captures[i].CaptureID, err)
		}
	}
	svc := NewService(&visionClient{})
	svc.UseStore(visionStore)
	svc.UseMaxCapturesPerDevice(2)

	count, err := svc.PruneExcessCaptures(context.Background())
	if err != nil {
		t.Fatalf("PruneExcessCaptures: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want one pruned capture", count)
	}
	oldest, err := svc.GetCapture(context.Background(), "dev-001", "cap_oldest")
	if err != nil {
		t.Fatalf("GetCapture oldest: %v", err)
	}
	if oldest.Status != CaptureStatusExpired {
		t.Fatalf("oldest status = %q, want expired", oldest.Status)
	}
	middle, err := svc.GetCapture(context.Background(), "dev-001", "cap_middle")
	if err != nil {
		t.Fatalf("GetCapture middle: %v", err)
	}
	newest, err := svc.GetCapture(context.Background(), "dev-001", "cap_newest")
	if err != nil {
		t.Fatalf("GetCapture newest: %v", err)
	}
	if middle.Status != CaptureStatusSucceeded || newest.Status != CaptureStatusPartial {
		t.Fatalf("kept statuses middle=%q newest=%q, want unchanged", middle.Status, newest.Status)
	}
	other, err := svc.GetCapture(context.Background(), "dev-002", "cap_other")
	if err != nil {
		t.Fatalf("GetCapture other: %v", err)
	}
	if other.Status != CaptureStatusSucceeded {
		t.Fatalf("other status = %q, want unchanged", other.Status)
	}
}

func TestServiceCaptureCallsDeviceMCPTool(t *testing.T) {
	xc := &visionClient{
		raw: json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg","presence":"someone"}`),
	}
	svc := NewService(xc)

	result, err := svc.Capture(context.Background(), CaptureRequest{
		DeviceID: " dev-001 ",
		Tool:     "camera.capture",
		Args:     map[string]any{"quality": "low"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if xc.gotDeviceID != "dev-001" {
		t.Fatalf("deviceID = %q, want trimmed dev-001", xc.gotDeviceID)
	}
	if xc.gotTool != "camera.capture" {
		t.Fatalf("tool = %q, want camera.capture", xc.gotTool)
	}
	if xc.gotArgs["quality"] != "low" {
		t.Fatalf("args = %+v, want quality low", xc.gotArgs)
	}
	if result.DeviceID != "dev-001" || result.Tool != "camera.capture" {
		t.Fatalf("result = %+v, want dev-001 camera.capture", result)
	}
	if string(result.Raw) != `{"imageUrl":"https://example.test/capture.jpg","presence":"someone"}` {
		t.Fatalf("raw = %s", result.Raw)
	}
}

func TestServiceCaptureUsesDefaultTool(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"ok":true}`)}
	svc := NewService(xc)

	result, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if xc.gotTool != DefaultCaptureTool {
		t.Fatalf("tool = %q, want default %q", xc.gotTool, DefaultCaptureTool)
	}
	if result.Tool != DefaultCaptureTool {
		t.Fatalf("result tool = %q, want default %q", result.Tool, DefaultCaptureTool)
	}
}

func TestServiceCaptureBoundsMCPCallForPRDLatency(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"presence":"someone"}`)}
	svc := NewService(xc)

	if _, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !xc.gotDeadline {
		t.Fatal("MCP call context has no deadline; PRD #7 requires vision trigger latency <= 8s")
	}
	remaining := time.Until(xc.deadline)
	if remaining <= 0 || remaining > 8*time.Second {
		t.Fatalf("MCP call deadline remaining = %s, want within 8s", remaining)
	}
}

func TestServiceCaptureRejectsMissingDeviceID(t *testing.T) {
	svc := NewService(&visionClient{})

	_, err := svc.Capture(context.Background(), CaptureRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceCaptureAndObservePresenceUsesMCPPresenceSignal(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"imageUrl":"https://example.test/empty.jpg","presence":"no_one"}`)}
	trigger := &fakeGreetingTrigger{result: sharedtypes.ProactiveGreetingResult{Status: "played", Text: "王阿姨，回来啦"}}
	svc := NewService(xc, trigger)
	ctx := context.Background()

	empty, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("CaptureAndObservePresence no_one: %v", err)
	}
	if empty.Observation.Presence != PresenceNoOne || empty.Observation.TriggeredGreeting {
		t.Fatalf("empty observation = %+v, want no_one without greeting", empty.Observation)
	}
	if string(empty.Capture.Raw) != `{"imageUrl":"https://example.test/empty.jpg","presence":"no_one"}` {
		t.Fatalf("capture raw = %s", empty.Capture.Raw)
	}

	xc.raw = json.RawMessage(`{"imageUrl":"https://example.test/return.jpg","presence":"someone"}`)
	returned, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("CaptureAndObservePresence someone: %v", err)
	}
	if !returned.Observation.TriggeredGreeting {
		t.Fatalf("returned observation = %+v, want greeting triggered", returned.Observation)
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want one return greeting", trigger.calls)
	}
}

func TestServiceCaptureAndObservePresenceBoundsWholePRDLatency(t *testing.T) {
	xc := &visionClient{raw: json.RawMessage(`{"presence":"no_one"}`)}
	trigger := &fakeGreetingTrigger{}
	svc := NewService(xc, trigger)
	ctx := context.Background()

	if _, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("CaptureAndObservePresence no_one: %v", err)
	}
	xc.raw = json.RawMessage(`{"presence":"someone"}`)
	if _, err := svc.CaptureAndObservePresence(ctx, CaptureRequest{DeviceID: "dev-001"}); err != nil {
		t.Fatalf("CaptureAndObservePresence someone: %v", err)
	}

	if !trigger.gotDeadline {
		t.Fatal("greeting trigger context has no deadline; PRD #7 requires capture plus trigger latency <= 8s")
	}
	remaining := time.Until(trigger.deadline)
	if remaining <= 0 || remaining > 8*time.Second {
		t.Fatalf("greeting trigger deadline remaining = %s, want within the shared 8s vision budget", remaining)
	}
}

func TestServiceCaptureAndObservePresenceRequiresPresenceSignal(t *testing.T) {
	tests := []json.RawMessage{
		json.RawMessage(`{"imageUrl":"https://example.test/capture.jpg"}`),
		json.RawMessage(`{"presence":"maybe"}`),
		json.RawMessage(`not-json`),
	}
	for _, raw := range tests {
		t.Run(string(raw), func(t *testing.T) {
			svc := NewService(&visionClient{raw: raw}, &fakeGreetingTrigger{})
			_, err := svc.CaptureAndObservePresence(context.Background(), CaptureRequest{DeviceID: "dev-001"})
			if !errors.Is(err, ErrPresenceUnavailable) {
				t.Fatalf("err = %v, want ErrPresenceUnavailable", err)
			}
		})
	}
}

func TestServicePollPresenceSkipsOfflineDevice(t *testing.T) {
	xc := &visionClient{
		status: xiaozhiclient.DeviceStatus{DeviceID: "dev-001", Online: false},
		raw:    json.RawMessage(`{"presence":"someone"}`),
	}
	svc := NewService(xc, &fakeGreetingTrigger{})

	result, err := svc.PollPresence(context.Background(), " dev-001 ")
	if err != nil {
		t.Fatalf("PollPresence offline: %v", err)
	}
	if !result.Skipped || result.SkipReason != "device offline" {
		t.Fatalf("result = %+v, want skipped device offline", result)
	}
	if result.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want trimmed dev-001", result.DeviceID)
	}
	if xc.statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want one status check", xc.statusCalls)
	}
	if xc.mcpCalls != 0 {
		t.Fatalf("mcpCalls = %d, want no camera call for offline device", xc.mcpCalls)
	}
}

func TestServicePollPresenceCapturesOnlineDevice(t *testing.T) {
	xc := &visionClient{
		status: xiaozhiclient.DeviceStatus{DeviceID: "dev-001", Online: true},
		raw:    json.RawMessage(`{"presence":"no_one"}`),
	}
	svc := NewService(xc, &fakeGreetingTrigger{})

	result, err := svc.PollPresence(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("PollPresence online: %v", err)
	}
	if result.Skipped {
		t.Fatalf("result = %+v, want online device captured", result)
	}
	if result.Check.Observation.Presence != PresenceNoOne {
		t.Fatalf("presence = %q, want no_one", result.Check.Observation.Presence)
	}
	if xc.statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want one status check", xc.statusCalls)
	}
	if xc.mcpCalls != 1 || xc.gotTool != DefaultCaptureTool {
		t.Fatalf("mcpCalls = %d tool=%q, want one default camera call", xc.mcpCalls, xc.gotTool)
	}
}

func TestServiceObservePresenceTriggersGreetingWhenSomeoneReturns(t *testing.T) {
	trigger := &fakeGreetingTrigger{
		result: sharedtypes.ProactiveGreetingResult{
			Status: "played",
			Text:   "王阿姨，回来啦，今天过得怎么样？",
		},
	}
	svc := NewService(&visionClient{}, trigger)
	ctx := context.Background()

	first, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: " dev-001 ", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence first someone: %v", err)
	}
	if first.TriggeredGreeting {
		t.Fatalf("first observation triggered greeting, want no startup greeting: %+v", first)
	}

	left, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceNoOne})
	if err != nil {
		t.Fatalf("ObservePresence no one: %v", err)
	}
	if left.PreviousPresence != PresenceSomeone || left.Presence != PresenceNoOne {
		t.Fatalf("left result = %+v, want someone -> no_one", left)
	}

	returned, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence returned: %v", err)
	}
	if !returned.TriggeredGreeting {
		t.Fatalf("returned result = %+v, want greeting triggered", returned)
	}
	if returned.PreviousPresence != PresenceNoOne || returned.Presence != PresenceSomeone {
		t.Fatalf("returned result = %+v, want no_one -> someone", returned)
	}
	if returned.Greeting == nil || returned.Greeting.Status != "played" {
		t.Fatalf("greeting = %+v, want played greeting", returned.Greeting)
	}
	if trigger.calls != 1 || trigger.deviceIDs[0] != "dev-001" {
		t.Fatalf("trigger calls = %d deviceIDs=%+v, want one trimmed dev-001", trigger.calls, trigger.deviceIDs)
	}
}

func TestServiceObservePresenceEmitsMindEventWhenSinkConfigured(t *testing.T) {
	sink := &fakeVisionMindSink{}
	trigger := &fakeGreetingTrigger{}
	svc := NewService(&visionClient{}, trigger)
	svc.UseMindSink(sink)
	_, err := svc.ObservePresence(context.Background(), PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want 1", sink.events)
	}
	if sink.events[0].Type != "presence_seen" {
		t.Fatalf("event = %+v, want presence_seen", sink.events[0])
	}
	if trigger.calls != 0 {
		t.Fatalf("trigger calls = %d, want Mind to decide greeting", trigger.calls)
	}
}

type fakeVisionMindSink struct{ events []MindEvent }

func (f *fakeVisionMindSink) IngestMindEvent(ctx context.Context, event MindEvent) error {
	f.events = append(f.events, event)
	return nil
}

func TestServiceObservePresenceDoesNotRepeatGreetingWhileStillSomeone(t *testing.T) {
	trigger := &fakeGreetingTrigger{}
	svc := NewService(&visionClient{}, trigger)
	ctx := context.Background()

	_, _ = svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceNoOne})
	_, _ = svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	still, err := svc.ObservePresence(ctx, PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence still someone: %v", err)
	}
	if still.TriggeredGreeting {
		t.Fatalf("still result = %+v, want no repeated greeting", still)
	}
	if trigger.calls != 1 {
		t.Fatalf("trigger calls = %d, want only the no_one -> someone transition", trigger.calls)
	}
}

func TestServiceObservePresenceRejectsBadInput(t *testing.T) {
	svc := NewService(&visionClient{}, &fakeGreetingTrigger{})

	tests := []PresenceObservationRequest{
		{Presence: PresenceSomeone},
		{DeviceID: "dev-001", Presence: "maybe"},
	}
	for _, req := range tests {
		if _, err := svc.ObservePresence(context.Background(), req); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("ObservePresence(%+v) err = %v, want ErrInvalidInput", req, err)
		}
	}
}

type visionClient struct {
	xiaozhiclient.FakeClient
	raw         json.RawMessage
	err         error
	status      xiaozhiclient.DeviceStatus
	statusErr   error
	statusCalls int
	mcpCalls    int
	gotDeviceID string
	gotTool     string
	gotArgs     map[string]any
	gotDeadline bool
	deadline    time.Time
	onMCP       func(ctx context.Context, deviceID, tool string, args map[string]any)
}

func (c *visionClient) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	c.statusCalls++
	status := c.status
	if status.DeviceID == "" {
		status.DeviceID = deviceID
	}
	return status, c.statusErr
}

func (c *visionClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	c.mcpCalls++
	c.gotDeviceID = deviceID
	c.gotTool = tool
	c.gotArgs = args
	c.deadline, c.gotDeadline = ctx.Deadline()
	if c.onMCP != nil {
		c.onMCP(ctx, deviceID, tool, args)
	}
	return c.raw, c.err
}

type fakeGreetingTrigger struct {
	result      sharedtypes.ProactiveGreetingResult
	err         error
	calls       int
	deviceIDs   []string
	gotDeadline bool
	deadline    time.Time
}

func (f *fakeGreetingTrigger) TriggerProactiveGreeting(ctx context.Context, deviceID string) (sharedtypes.ProactiveGreetingResult, error) {
	f.calls++
	f.deviceIDs = append(f.deviceIDs, deviceID)
	f.deadline, f.gotDeadline = ctx.Deadline()
	return f.result, f.err
}

func newVisionTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	visionStore := NewStore(st.DB)
	if err := visionStore.AutoMigrate(); err != nil {
		t.Fatalf("migrate vision store: %v", err)
	}
	return visionStore
}

type fakeVisionForwarder struct {
	resp xiaozhiclient.VisionForwardResponse
	err  error
	got  xiaozhiclient.VisionForwardRequest
}

func (f *fakeVisionForwarder) ForwardVisionMultipart(ctx context.Context, req xiaozhiclient.VisionForwardRequest) (xiaozhiclient.VisionForwardResponse, error) {
	f.got = req
	return f.resp, f.err
}
