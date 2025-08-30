package slicer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

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
	//create a temporary directory to store input and output files
	dir := uuid.New().String()
	errDir := os.Mkdir(dir, 0777)
	if errDir != nil {
		return "", fmt.Errorf("error creating temp dir: %w", errDir)
	}

	errFile := os.WriteFile(fmt.Sprintf("%s/input.3mf", dir), file, 0444)
	if errFile != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("error writing input file: %w", errFile)
	}

	//prepare the args to run the slicer
	args := make([]string, 0)

	args = append(args, "--export-3mf", "output.3mf")

	args = append(args, "--slice", "1")

	args = append(args, "--outputdir", dir)

	//this needed to be set to allow slicing, without it it would not slice
	args = append(args, "--allow-newer-file")

	args = append(args, fmt.Sprintf("%s/input.3mf", dir))

	cmd := exec.Command(app, args...)

	err := cmd.Run()

	if err != nil {
		//read the result file if it exist to provide more information about the error
		//(not all details are included)
		resultPath := fmt.Sprintf("%s/result.json", dir)
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
		return "", fmt.Errorf("error running command: %w", err)
	}

	return fmt.Sprintf("%s/output.3mf", dir), nil
}
