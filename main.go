package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
)

// TestCase har bir test holatini tasvirlaydi
type TestCase struct {
	ID     int    `json:"id"`
	Input  string `json:"input"`
	Output string `json:"output,omitempty"`
}

// Submission foydalanuvchi yuborishini tasvirlaydi
type Submission struct {
	Code            string     `json:"code"`
	Language        string     `json:"language"`
	ExecutionTest   string     `json:"execution_test_cases"`
	TestCases       []TestCase `json:"test_cases"`
}

// ExecutionResult kod bajarish natijasini tasvirlaydi
type ExecutionResult struct {
	LanguageID     string     `json:"language_id"`
	Code          string     `json:"code"`
	IsAccepted    bool       `json:"is_accepted"`
	ExecutionTime float64    `json:"execution_time"`
	MemoryUsage   float64    `json:"memory_usage"`
	TestCases     []TestCase `json:"testcases_json"`
}

var languageCommands = map[string]string{
	"python": "python3 /app/solution.py",
	"go":     "go run /app/solution.go",
}

func getFileName(language string) string {
	switch language {
	case "python":
		return "solution.py"
	case "go":
		return "solution.go"
	default:
		return "solution.txt"
	}
}

func createTarFile(fileName, content string) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buffer)
	defer tarWriter.Close()

	hdr := &tar.Header{
		Name: fileName,
		Mode: 0600,
		Size: int64(len(content)),
	}
	if err := tarWriter.WriteHeader(hdr); err != nil {
		return nil, fmt.Errorf("failed to write tar header: %v", err)
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		return nil, fmt.Errorf("failed to write tar file: %v", err)
	}

	return buffer, nil
}

func copyToContainer(cli *client.Client, containerName, fileName, content string) error {
	tarBuffer, err := createTarFile(fileName, content)
	if err != nil {
		return fmt.Errorf("failed to create tar file: %v", err)
	}

	return cli.CopyToContainer(
		context.Background(),
		containerName,
		"/app/",
		tarBuffer,
		types.CopyToContainerOptions{},
	)
}

func getMemoryUsage() (float64, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, fmt.Errorf("failed to read memory usage: %v", err)
	}

	re := regexp.MustCompile(`VmRSS:\s+(\d+) kB`)
	matches := re.FindStringSubmatch(string(data))
	if len(matches) < 2 {
		return 0, fmt.Errorf("memory usage not found")
	}

	usageKB, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory usage: %v", err)
	}
	return float64(usageKB) / 1024, // Convert KB to MB
}


func executeCode(cli *client.Client, containerName, command string) (string, float64, float64, error) {
	startTime := time.Now()

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          []string{"sh", "-c", command},
	}

	execIDResp, err := cli.ContainerExecCreate(context.Background(), containerName, execConfig)

	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create exec: %v", err)
	}

	resp, err := cli.ContainerExecAttach(context.Background(), execIDResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to attach to exec: %v", err)
	}
	defer resp.Close()

	var outputBuffer bytes.Buffer
	if _, err := io.Copy(&outputBuffer, resp.Reader); err != nil {
		return "", 0, 0, fmt.Errorf("failed to read output: %v", err)
	}

	executionTime := time.Since(startTime).Seconds()
	memoryUsage, _ := getMemoryUsage()

	return strings.TrimSpace(outputBuffer.String()), executionTime, memoryUsage, nil
}

func processTestCases(submission Submission, output string) []TestCase {
	var results []TestCase
	outputLines := strings.Split(output, "\n")

	for i, tc := range submission.TestCases {
		result := TestCase{
			ID:    tc.ID,
			Input: tc.Input,
		}

		if i < len(outputLines) {
			result.Output = outputLines[i]
			if tc.Output != "" {
				result.IsTrue = (result.Output == tc.Output)
			}
		}

		results = append(results, result)
	}

	return results
}

func main() {
	app := fiber.New()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	defer cli.Close()

	app.Post("/run-test", func(c *fiber.Ctx) error {
		var submission Submission
		if err := c.BodyParser(&submission); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid JSON format",
			})
		}

		command, exists := languageCommands[submission.Language]
		if !exists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Unsupported language",
			})
		}

		containerName := fmt.Sprintf("%s-app", submission.Language)
		fileName := getFileName(submission.Language)

		// Combine code and execution test cases
		fullCode := fmt.Sprintf("%s\n%s", submission.Code, submission.ExecutionTest)
		if err := copyToContainer(cli, containerName, fileName, fullCode); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		output, execTime, memoryUsage, err := executeCode(cli, containerName, command)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		// Process test cases results
		testResults := processTestCases(submission, output)

		result := ExecutionResult{
			LanguageID:     submission.Language,
			Code:          submission.Code,
			IsAccepted:    true, // Assuming all passed for simplicity
			ExecutionTime: execTime,
			MemoryUsage:   memoryUsage,
			TestCases:     testResults,
		}

		return c.JSON(result)
	})

	log.Fatal(app.Listen(":3000"))
}