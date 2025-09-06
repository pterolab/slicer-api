package slicer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

type SlicingResult struct {
	ErrorString string `json:"error_string"`
	PlateIndex  int    `json:"plate_index"`
	ReturnCode  int    `json:"return_code"`
}

type SlicingError struct {
	Result SlicingResult
}

func (e *SlicingError) Error() string {
	return fmt.Sprintf("%s (Plate: %d, Code: %d)", e.Result.ErrorString, e.Result.PlateIndex, e.Result.ReturnCode)
}

// Slice slices a 3MF file using the specified slicer path
// It takes the file content as a byte slice as well as the path to the slicer.
// It returns the path to the sliced output file (3mf) or an error if something went wrong.
// The error may be of type SlicingError.
// It will automatically create a temporary directory for input and output files.
// The caller is responsible for reading the file and cleaning up the temporary directory.
func Slice(file []byte, app string) (string, error) {
	// Create a temporary directory to store input and output files
	dir := uuid.New().String()
	errDir := os.Mkdir(dir, 0777)
	if errDir != nil {
		return "", fmt.Errorf("error creating temp dir: %w", errDir)
	}

	// Use absolute path for better reliability
	absDir, err := filepath.Abs(dir)
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("error getting absolute path: %w", err)
	}

	inputPath := filepath.Join(absDir, "input.3mf")
	errFile := os.WriteFile(inputPath, file, 0644) // Changed from 0444 to 0644
	if errFile != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("error writing input file: %w", errFile)
	}

	// Prepare the args to run the slicer
	args := make([]string, 0)
	args = append(args, "--export-3mf", "output.3mf")
	args = append(args, "--slice", "1")
	args = append(args, "--outputdir", absDir)
	args = append(args, "--allow-newer-file")
	args = append(args, inputPath)

	cmd := exec.Command(app, args...)
	cmd.Dir = absDir // Set working directory
	
	// Capture both stdout and stderr for better debugging
	output, err := cmd.CombinedOutput()
	
	fmt.Printf("Running command: %s %v\n", app, args)
	fmt.Printf("Working directory: %s\n", absDir)
	
	if err != nil {
		fmt.Printf("Command output: %s\n", string(output))
		fmt.Printf("Command error: %v\n", err)
		
		// Check if the slicer executable exists and is executable
		if _, statErr := os.Stat(app); os.IsNotExist(statErr) {
			os.RemoveAll(dir)
			return "", fmt.Errorf("slicer executable not found: %s", app)
		}
		
		// Read the result file if it exists to provide more information about the error
		resultPath := filepath.Join(absDir, "result.json")
		if _, statErr := os.Stat(resultPath); statErr == nil {
			f, openErr := os.Open(resultPath)
			if openErr == nil {
				defer f.Close()
				var result SlicingResult
				if jsonErr := json.NewDecoder(f).Decode(&result); jsonErr == nil {
					os.RemoveAll(dir)
					return "", &SlicingError{Result: result}
				}
			}
		}
		
		os.RemoveAll(dir)
		return "", fmt.Errorf("error running command (exit code: %v): %s", err, string(output))
	}

	// Verify the output file was created
	outputPath := filepath.Join(absDir, "plate_1.gcode")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		os.RemoveAll(dir)
		return "", fmt.Errorf("output file was not created: %s", outputPath)
	}

	fmt.Printf("Command succeeded. Output: %s\n", string(output))
	return outputPath, nil
}

// Helper function to check if slicer is available and get version info
func CheckSlicer(app string) error {
	if _, err := os.Stat(app); os.IsNotExist(err) {
		return fmt.Errorf("slicer executable not found: %s", app)
	}
	
	// Try to run with --help or --version to test if it's working
	cmd := exec.Command(app, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("slicer not working properly: %v, output: %s", err, string(output))
	}
	
	fmt.Printf("Slicer appears to be working. Help output length: %d bytes\n", len(output))
	return nil
}