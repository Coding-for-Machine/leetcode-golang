package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gofiber/fiber/v2"
)

type TestCase struct {
	ID     int    `json:"id"`
	Input  string `json:"input"`
	Output string `json:"output,omitempty"`
	IsTrue bool   `json:"is_true,omitempty"`
}

type Submission struct {
	Code          string     `json:"code"`
	Language      string     `json:"language"`
	ExecutionTest string     `json:"execution_test_cases"`
	TestCases     []TestCase `json:"test_cases"`
}

type ExecutionResult struct {
	LanguageID    string     `json:"language_id"`
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
		return nil, fmt.Errorf("tar header yozishda xato: %v", err)
	}
	if _, err := tarWriter.Write([]byte(content)); err != nil {
		return nil, fmt.Errorf("tar faylga yozishda xato: %v", err)
	}

	return buffer, nil
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
		return "", 0, 0, fmt.Errorf("exec yaratishda xato: %v", err)
	}

	resp, err := cli.ContainerExecAttach(context.Background(), execIDResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", 0, 0, fmt.Errorf("execga ulanishda xato: %v", err)
	}
	defer resp.Close()

	var outputBuffer bytes.Buffer
	if _, err := io.Copy(&outputBuffer, resp.Reader); err != nil {
		return "", 0, 0, fmt.Errorf("natijani o'qishda xato: %v", err)
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
	app := fiber.New(fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
	})

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Docker klientini yaratishda xato: %v", err)
	}
	defer cli.Close()

	app.Post("/run-test", func(c *fiber.Ctx) error {
		var submission Submission
		if err := c.BodyParser(&submission); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Noto'g'ri JSON formati"})
		}

		command, exists := languageCommands[submission.Language]
		if !exists {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Qo'llab-quvvatlanmaydigan dasturlash tili"})
		}

		containerName := fmt.Sprintf("%s-app", submission.Language)
		fileName := getFileName(submission.Language)
		fullCode := fmt.Sprintf("%s\n%s", submission.Code, submission.ExecutionTest)

		if err := copyToContainer(cli, containerName, fileName, fullCode); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		output, execTime, memoryUsage, err := executeCode(cli, containerName, command)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		result := ExecutionResult{
			LanguageID:    submission.Language,
			Code:          submission.Code,
			IsAccepted:    true,
			ExecutionTime: execTime,
			MemoryUsage:   memoryUsage,
			TestCases:     processTestCases(submission, output),
		}
		return c.JSON(result)
	})

	log.Println("Server 3000-portda ishga tushmoqda...")
	log.Fatal(app.Listen(":3000"))
}
