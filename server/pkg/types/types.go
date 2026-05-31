// Package types 放跨模块共享、与具体业务域无关的小类型。
// 纪律：这里只允许放被两个及以上模块共用的类型；任何域专属类型放到该域自己的 types.go。
package types

import "errors"

// ErrNotImplemented 供尚未实现的接口方法返回（地基期 FakeClient / 占位用）。
var ErrNotImplemented = errors.New("anban: not implemented")

// DeviceID 是 xiaozhi 侧的设备标识（= manager 的 device_name）。
type DeviceID string
