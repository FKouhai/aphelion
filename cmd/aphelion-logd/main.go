package main

import (
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/spf13/cobra"
	"net/http"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "aphelion-logd",
	Short: "log daemon that exposes systemd journal entries over a websocket",
	RunE:  run,
}

var (
	logdAddr    string
	agentAddr   string
	metricsAddr string
	cgroupBase  string
)

func init() {
	rootCmd.Flags().StringVar(&logdAddr, "addr", "0.0.0.0:7374", "TCP address for the QMP gateway")
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer c.CloseNow()

	j, err := sdjournal.NewJournal()
	if err != nil {
		c.Close(websocket.StatusInternalError, "journal open failed")
		return
	}
	defer j.Close()
	if err := j.SeekTail(); err != nil {
		c.Close(websocket.StatusInternalError, "seek failed")
		return
	}
	// flush entries at the tail position
	j.Previous()

	ctx := r.Context()
	for {
		n, err := j.Next()
		if err != nil {
			break
		}
		if n == 0 {
			// No new entry yet; block until we get one or client disconects
			if status := j.Wait(sdjournal.IndefiniteWait); status < 0 {
				break
			}
			continue
		}
		entry, err := j.GetEntry()
		if err != nil {
			continue
		}
		if err := wsjson.Write(ctx, c, entry.Fields); err != nil {
			break
		}
	}
	c.Close(websocket.StatusNormalClosure, "")
}

func run(cmd *cobra.Command, args []string) error {
	http.HandleFunc("/logs", logsHandler)
	return http.ListenAndServe(logdAddr, nil)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
