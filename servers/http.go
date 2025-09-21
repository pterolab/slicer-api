package servers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slicer-api/slicer"
	"strconv"
	"strings"
	"time"
)

// FilamentInfo represents the filament usage data extracted from G-code
type FilamentInfo struct {
	FilamentUsedMM  float64 `json:"filament_used_mm"`
	FilamentUsedCM3 float64 `json:"filament_used_cm3"`
	FilamentUsedG   float64 `json:"filament_used_g"`
	FilamentCost    float64 `json:"filament_cost"`
	FilamantModelTime string    `json:"model_printing_time,omitempty"`
	FilamantTotalTime string    `json:"total_estimated_time,omitempty"`
}

// parseFloatArray parses a comma-separated string of floats and returns a slice
func parseFloatArray(s string) []float64 {
	parts := strings.Split(s, ",")
	var result []float64
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if val, err := strconv.ParseFloat(part, 64); err == nil {
			result = append(result, val)
		}
	}
	return result
}

// extractFilamentInfo reads a G-code file and extracts filament usage information
func extractFilamentInfo(sliceResponse *slicer.SlicingResponse) (*FilamentInfo, error) {
	file, err := os.Open(sliceResponse.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	type TimeInfo struct {
		ModelPrintingTime      string `json:"model_printing_time,omitempty"`
		TotalEstimatedTime     string `json:"total_estimated_time,omitempty"`
	}

	info := &FilamentInfo{}
	timeInfo := &TimeInfo{}
	scanner := bufio.NewScanner(file)

	// Regular expressions to match the filament info lines
	regexPatterns := map[string]*regexp.Regexp{
		"mm":   regexp.MustCompile(`; filament used \[mm\] = (.+)`),
		"cm3":  regexp.MustCompile(`; filament used \[cm3\] = (.+)`),
		"g":    regexp.MustCompile(`; filament used \[g\] = (.+)`),
		"cost": regexp.MustCompile(`; filament cost = (.+)`),
		"model_time": regexp.MustCompile(`; model printing time: ([\dhms\s]+)`),
		"total_time": regexp.MustCompile(`; total estimated time: ([\dhms\s]+)`),
	}

	for scanner.Scan() {
		line := scanner.Text()

		if match := regexPatterns["mm"].FindStringSubmatch(line); len(match) > 1 {
			arr := parseFloatArray(match[1])
			if len(arr) > 0 {
				info.FilamentUsedMM = arr[0]
			}
		} else if match := regexPatterns["cm3"].FindStringSubmatch(line); len(match) > 1 {
			arr := parseFloatArray(match[1])
			if len(arr) > 0 {
				info.FilamentUsedCM3 = arr[0]
			}
		} else if match := regexPatterns["g"].FindStringSubmatch(line); len(match) > 1 {
			arr := parseFloatArray(match[1])
			if len(arr) > 0 {
				info.FilamentUsedG = arr[0]
			}
		} else if match := regexPatterns["cost"].FindStringSubmatch(line); len(match) > 1 {
			arr := parseFloatArray(match[1])
			if len(arr) > 0 {
				info.FilamentCost = arr[0]
			}
		} else if match := regexPatterns["model_time"].FindStringSubmatch(line); len(match) > 1 {
			timeInfo.ModelPrintingTime = strings.TrimSpace(match[1])
		} else if match := regexPatterns["total_time"].FindStringSubmatch(line); len(match) > 1 {
			timeInfo.TotalEstimatedTime = strings.TrimSpace(match[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	// Merge timeInfo into FilamentInfo by marshaling/unmarshaling
	// Or, you can extend FilamentInfo to include these fields directly.
	// Here, we marshal both and merge into a map for flexibility.
	resultMap := make(map[string]interface{})
	b1, _ := json.Marshal(info)
	b2, _ := json.Marshal(timeInfo)
	json.Unmarshal(b1, &resultMap)
	json.Unmarshal(b2, &resultMap)

	finalBytes, _ := json.Marshal(resultMap)
	finalInfo := &FilamentInfo{}
	json.Unmarshal(finalBytes, finalInfo)

	fmt.Printf("Deleting dir %s after extracting info\n", sliceResponse.OutputDir)
	os.RemoveAll(sliceResponse.OutputDir)
	return finalInfo, nil
}

func RunHTTP(addr string) error {
	mux := http.NewServeMux()
	
	// Original slice endpoint
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
		
		sliceResponse, sErr := slicer.Slice(data, app)
		if sErr != nil {
			http.Error(w, fmt.Sprintf("slicing failed: %v", sErr), http.StatusInternalServerError)
			return
		}
		
		file, err := os.Open(sliceResponse.OutputPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("open output file error: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		// Uncomment these lines if you want to clean up temporary files
		// defer os.RemoveAll(filepath.Dir(outFilePath))
		
		fmt.Println("Output file: ", sliceResponse.OutputPath)
		w.Header().Set("Content-Type", "application/gcode")
		w.Header().Set("Content-Disposition", "attachment; filename=plate_1.gcode")
		http.ServeContent(w, r, "plate_1.gcode", time.Now(), file)
	})

	// New endpoint to analyze G-code and return filament info as JSON
	mux.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if a file path is provided in the request body
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read error: %v", err), http.StatusBadRequest)
			return
		}

		filePath := strings.TrimSpace(string(data))
		if filePath == "" {
			filePath = "plate_1.gcode" // Default file name
		}

		// Extract filament information from the G-code file
		sliceResponse := &slicer.SlicingResponse{
			OutputPath: filePath,
			OutputDir:  filepath.Dir(filePath),
		}
		filamentInfo, err := extractFilamentInfo(sliceResponse)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to extract filament info: %v", err), http.StatusInternalServerError)
			return
		}

		// Set response headers for JSON
		w.Header().Set("Content-Type", "application/json")
		
		// Encode and send the JSON response
		if err := json.NewEncoder(w).Encode(filamentInfo); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode JSON: %v", err), http.StatusInternalServerError)
			return
		}
	})

	// Enhanced slice endpoint that also returns filament analysis
	mux.HandleFunc("/slice-with-analysis", func(w http.ResponseWriter, r *http.Request) {
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
		
		sliceResponse, sErr := slicer.Slice(data, app)
		if sErr != nil {
			http.Error(w, fmt.Sprintf("slicing failed: %v", sErr), http.StatusInternalServerError)
			return
		}
		
		// Extract filament information and times
		filamentInfo, err := extractFilamentInfo(sliceResponse)
		if err != nil {
			log.Printf("Warning: failed to extract filament info: %v", err)
			// Continue serving the file even if analysis fails
		}

		// Unmarshal again to get time fields from the result map
		var resultMap map[string]interface{}
		b, _ := json.Marshal(filamentInfo)
		json.Unmarshal(b, &resultMap)

		// Create response structure
		type SliceResponse struct {
			FilamentInfo        *FilamentInfo `json:"filament_info,omitempty"`
			Message             string        `json:"message"`
		}

		response := SliceResponse{
			FilamentInfo:       filamentInfo,
			Message:            "Slicing completed successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode JSON response: %v", err), http.StatusInternalServerError)
			return
		}
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	log.Printf("HTTP server listening on %s", addr)
	log.Printf("Available endpoints:")
	log.Printf("  POST /slice - Original slicing endpoint (returns G-code file)")
	log.Printf("  POST /analyze - Analyze G-code file and return filament info as JSON")
	log.Printf("  POST /slice-with-analysis - Slice and return analysis as JSON")
	
	return srv.ListenAndServe()
}