package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"aphelion/pkg/connect"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <host> <vm>",
	Short: "Stream journal logs from a VM",
	Args:  cobra.ExactArgs(2),
	RunE:  runLogs,
}

func runLogs(cmd *cobra.Command, args []string) error {
	hostName, vmName := args[0], args[1]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	host, err := cfg.ByName(hostName)
	if err != nil {
		return err
	}

	conn, err := connect.Dial(*host)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", host.DisplayName, err)
	}
	defer conn.Close()

	ag, err := conn.Agent(agentPort)
	if err != nil {
		return err
	}
	defer ag.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lr, err := ag.Logs(ctx, vmName, logdPort)
	if err != nil {
		return fmt.Errorf("opening log stream for %s: %w", vmName, err)
	}
	defer lr.Close()

	scanner := bufio.NewScanner(lr)
	for scanner.Scan() {
		var fields map[string]string
		if err := json.Unmarshal(scanner.Bytes(), &fields); err != nil {
			continue
		}
		fmt.Fprintln(os.Stdout, formatLogEntry(fields))
	}
	return scanner.Err()
}

func formatLogEntry(fields map[string]string) string {
	ts := formatLogTimestamp(fields["__REALTIME_TIMESTAMP"])
	unit := fields["SYSLOG_IDENTIFIER"]
	if unit == "" {
		unit = fields["_SYSTEMD_UNIT"]
	}
	msg := fields["MESSAGE"]
	if unit != "" {
		return fmt.Sprintf("%s %s: %s", ts, unit, msg)
	}
	return fmt.Sprintf("%s %s", ts, msg)
}

func formatLogTimestamp(raw string) string {
	us, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return raw
	}
	return time.UnixMicro(us).Local().Format("Jan 02 15:04:05")
}
