package servers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"slicer-api/slicer"
)

func RunHTTP(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/slice", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read error: %v", err), http.StatusBadRequest)
			return
		}
		if len(data) == 0 {
			http.Error(w, "no input data received", http.StatusBadRequest)
			return
		}

		app := os.Getenv("SLICER_APP")
		if app == "" {
			http.Error(w, "SLICER_APP path is not set", http.StatusInternalServerError)
			return
		}

		outFilePath, sErr := slicer.Slice(data, app)
		if sErr != nil {
			http.Error(w, fmt.Sprintf("slicing failed: %v", sErr), http.StatusInternalServerError)
			return
		}

		file, err := os.Open(outFilePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("open output file error: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		defer os.RemoveAll(filepath.Dir(outFilePath))

		w.Header().Set("Content-Type", "application/3mf")
		w.Header().Set("Content-Disposition", "attachment; filename=output.gcode.3mf")
		http.ServeContent(w, r, "output.gcode.3mf", time.Now(), file)
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	log.Printf("HTTP server listening on %s", addr)
	return srv.ListenAndServe()
}
