package xiaozhiclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVisionForwarderPreservesDeviceMultipartRequestAndResponse(t *testing.T) {
	image := []byte{0xff, 0xd8, 'a', 'n', 'b', 'a', 'n', 0xff, 0xd9}

	var gotDeviceID, gotClientID, gotAuthorization, gotQuestion, gotFilename, gotContentType string
	var gotImage []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/xiaozhi/api/vision" {
			t.Fatalf("unexpected vision request %s %s", r.Method, r.URL.Path)
		}
		gotDeviceID = r.Header.Get("Device-Id")
		gotClientID = r.Header.Get("Client-Id")
		gotAuthorization = r.Header.Get("Authorization")
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("content-type = %q, want multipart/form-data", r.Header.Get("Content-Type"))
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		gotQuestion = r.FormValue("question")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		gotFilename = header.Filename
		gotContentType = header.Header.Get("Content-Type")
		gotImage, err = io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll image: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"answer":"ok"}`))
	}))
	defer srv.Close()

	forwarder := NewVisionForwarder(srv.URL + "/xiaozhi/api/vision")
	resp, err := forwarder.ForwardVisionMultipart(context.Background(), VisionForwardRequest{
		DeviceID:      "9c:13:9e:8b:af:28",
		ClientID:      "client-001",
		Authorization: "Bearer device-token",
		Question:      "请看一下画面",
		FileName:      "camera.jpg",
		ContentType:   "image/jpeg",
		Image:         image,
	})
	if err != nil {
		t.Fatalf("ForwardVisionMultipart: %v", err)
	}

	if gotDeviceID != "9c:13:9e:8b:af:28" || gotClientID != "client-001" || gotAuthorization != "Bearer device-token" {
		t.Fatalf("headers device=%q client=%q auth=%q", gotDeviceID, gotClientID, gotAuthorization)
	}
	if gotQuestion != "请看一下画面" {
		t.Fatalf("question = %q, want original question", gotQuestion)
	}
	if gotFilename != "camera.jpg" || gotContentType != "image/jpeg" {
		t.Fatalf("file metadata filename=%q contentType=%q", gotFilename, gotContentType)
	}
	if !bytes.Equal(gotImage, image) {
		t.Fatalf("image bytes changed: got %v want %v", gotImage, image)
	}
	if resp.StatusCode != http.StatusAccepted || resp.ContentType != "application/json" || string(resp.Body) != `{"answer":"ok"}` {
		t.Fatalf("response = %+v, want upstream status/content-type/body", resp)
	}
}
