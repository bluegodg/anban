package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/preflight"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

type preflightConfig struct {
	ManagerBaseURL  string
	ManagerAPIToken string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("anban-preflight", flag.ContinueOnError)
	flags.SetOutput(stderr)
	deviceID := flags.String("device-id", os.Getenv("ANBAN_PREFLIGHT_DEVICE_ID"), "xiaozhi manager device ID/name to check")
	gatePassed := flags.Bool("xiaozhi-gate-passed", envBool("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED"), "confirm Gate A: original xiaozhi voice loop was manually verified")
	allowMissingDeviceID := flags.Bool("allow-missing-device-id", envBool("ANBAN_PREFLIGHT_ALLOW_MISSING_DEVICE_ID"), "allow manager-only preflight when the xiaozhi device ID is not known yet")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := loadPreflightConfig()
	if err != nil {
		fmt.Fprintf(stderr, "preflight config failed: %v\n", err)
		return 1
	}

	client := xiaozhiclient.NewHTTPClient(cfg.ManagerBaseURL, cfg.ManagerAPIToken)
	report := preflight.Run(context.Background(), client, *deviceID)
	fmt.Fprintln(stdout, preflight.FormatReport(report))
	if report.Failed() {
		return 1
	}
	if !*gatePassed {
		fmt.Fprintln(stderr, "preflight Gate A not confirmed: run original xiaozhi wake/respond/interrupt first, then pass --xiaozhi-gate-passed or set ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED=true.")
		return 1
	}
	if strings.TrimSpace(*deviceID) == "" && !*allowMissingDeviceID {
		fmt.Fprintln(stderr, "preflight device ID not provided: pass -device-id or set ANBAN_PREFLIGHT_DEVICE_ID so manager can verify this real device; use --allow-missing-device-id only for manager-only network/token checks.")
		return 1
	}
	return 0
}

func loadPreflightConfig() (preflightConfig, error) {
	cfg := preflightConfig{
		ManagerBaseURL:  strings.TrimSpace(os.Getenv("ANBAN_MANAGER_BASE_URL")),
		ManagerAPIToken: strings.TrimSpace(os.Getenv("ANBAN_MANAGER_API_TOKEN")),
	}
	if cfg.ManagerBaseURL == "" {
		return preflightConfig{}, fmt.Errorf("config: ANBAN_MANAGER_BASE_URL 必填")
	}
	if cfg.ManagerAPIToken == "" {
		return preflightConfig{}, fmt.Errorf("config: ANBAN_MANAGER_API_TOKEN 必填")
	}
	if config.IsPlaceholderValue(cfg.ManagerAPIToken) {
		return preflightConfig{}, fmt.Errorf("config: ANBAN_MANAGER_API_TOKEN 不能使用示例占位值")
	}
	return cfg, nil
}

func envBool(key string) bool {
	switch strings.TrimSpace(os.Getenv(key)) {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	default:
		return false
	}
}
