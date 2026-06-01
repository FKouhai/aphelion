package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/coreos/go-systemd/v22/sdjournal"
	"github.com/spf13/cobra"
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
		c.Close(websocket.StatusInternalError, "journal open failed: "+err.Error())
		return
	}
	defer j.Close()
	if err := j.SeekTail(); err != nil {
		c.Close(websocket.StatusInternalError, "seek failed")
		return
	}
	// Rewind to show recent history before following new entries.
	j.PreviousSkip(50)

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
		safe := make(map[string]string, len(entry.Fields))
		for k, v := range entry.Fields {
			safe[k] = strings.ToValidUTF8(v, "�")
		}
		if err := wsjson.Write(ctx, c, safe); err != nil {
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
