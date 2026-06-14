// Package preflight contains non-invasive checks for the Scheme C deployment gate.
package preflight

import (
	"context"
	"fmt"
	"strings"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type Status string

const (
	StatusPass   Status = "pass"
	StatusFail   Status = "fail"
	StatusSkip   Status = "skip"
	StatusManual Status = "manual"
)

type Check struct {
	Name   string
	Status Status
	Detail string
}

type Report struct {
	Checks []Check
}

func (r Report) Failed() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}

type ManagerAccessChecker interface {
	CheckManagerAccess(ctx context.Context) error
}

type DeviceStatusReader interface {
	ManagerAccessChecker
	GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error)
}

func Run(ctx context.Context, client DeviceStatusReader, deviceID string) Report {
	deviceID = strings.TrimSpace(deviceID)
	checks := []Check{
		{
			Name:   "xiaozhi-only voice loop",
			Status: StatusManual,
			Detail: "Gate A: 先在未依赖 anban 的情况下完成原版小智唤醒、回应、打断；此项不能由 anban 自动验证。",
		},
		{
			Name:   "anban optionality smoke",
			Status: StatusManual,
			Detail: "Gate D: 完成安伴最小联调后，停掉 anban，再确认设备仍可继续原版小智对话；此项不能由 anban 自动验证。",
		},
	}
	if err := client.CheckManagerAccess(ctx); err != nil {
		checks = append(checks, Check{
			Name:   "xiaozhi manager OpenAPI access",
			Status: StatusFail,
			Detail: "manager OpenAPI/token 检查失败: " + err.Error(),
		})
		return Report{Checks: checks}
	}
	checks = append(checks, Check{
		Name:   "xiaozhi manager OpenAPI access",
		Status: StatusPass,
		Detail: "manager OpenAPI 可访问，API Token 已被接受。",
	})

	if deviceID == "" {
		checks = append(checks, Check{
			Name:   "xiaozhi manager device status",
			Status: StatusSkip,
			Detail: "未提供设备 ID，跳过 manager 设备在线检查。设置 -device-id 或 ANBAN_PREFLIGHT_DEVICE_ID 后可检查。",
		})
		return Report{Checks: checks}
	}

	status, err := client.GetDeviceStatus(ctx, deviceID)
	if err != nil {
		checks = append(checks, Check{
			Name:   "xiaozhi manager device status",
			Status: StatusFail,
			Detail: "manager OpenAPI/token/设备查询失败: " + err.Error(),
		})
		return Report{Checks: checks}
	}
	if !status.Online {
		checks = append(checks, Check{
			Name:   "xiaozhi manager device status",
			Status: StatusFail,
			Detail: fmt.Sprintf("设备 %s 已找到但当前离线。先让设备在 xiaozhi manager 里在线，再联调安伴。", firstNonEmpty(status.DeviceID, deviceID)),
		})
		return Report{Checks: checks}
	}

	detail := fmt.Sprintf("设备 %s 在线", firstNonEmpty(status.DeviceID, deviceID))
	if !status.LastActiveAt.IsZero() {
		detail += "; last_active_at=" + status.LastActiveAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	checks = append(checks, Check{
		Name:   "xiaozhi manager device status",
		Status: StatusPass,
		Detail: detail,
	})
	return Report{Checks: checks}
}

func FormatReport(report Report) string {
	var b strings.Builder
	for _, check := range report.Checks {
		fmt.Fprintf(&b, "[%s] %s - %s\n", strings.ToUpper(string(check.Status)), check.Name, check.Detail)
	}
	return strings.TrimRight(b.String(), "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
