// package main

// import (
// 	"archive/tar"
// 	"bytes"
// 	"context"
// 	"fmt"
// 	"io"
// 	"log"
// 	"os"
// 	"regexp"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/docker/docker/api/types"
// 	"github.com/docker/docker/client"
// 	"github.com/gofiber/fiber/v2"
// )

// type TestCase struct {
// 	Input  string `json:"input"`
// 	Output string `json:"output"`
// }

// type Submission struct {
// 	Language    string     `json:"language"`
// 	Code        string     `json:"code"`
// 	ExecuteCode string     `json:"execute_code"`
// 	TestCases   []TestCase `json:"test_cases"`
// }

// var languageCommands = map[string]string{
// 	"python": "python3 /app/solution.py",
// 	"go":     "go run /app/solution.go",
// }

// func getFileName(language string) string {
// 	if language == "python" {
// 		return "solution.py"
// 	} else if language == "go" {
// 		return "solution.go"
// 	}
// 	return "solution.txt"
// }

// func createTarFile(fileName, content string) (*bytes.Buffer, error) {
// 	buffer := new(bytes.Buffer)
// 	tarWriter := tar.NewWriter(buffer)
// 	defer tarWriter.Close()

// 	hdr := &tar.Header{
// 		Name: fileName,
// 		Mode: 0600,
// 		Size: int64(len(content)),
// 	}
// 	if err := tarWriter.WriteHeader(hdr); err != nil {
// 		return nil, fmt.Errorf("Tar header yozishda xatolik: %v", err)
// 	}
// 	if _, err := tarWriter.Write([]byte(content)); err != nil {
// 		return nil, fmt.Errorf("Tar fayl yozishda xatolik: %v", err)
// 	}

// 	return buffer, nil
// }

// func fileConnect(cli *client.Client, containerName, fileName, containerPath, content string) error {
// 	tarBuffer, err := createTarFile(fileName, content)
// 	if err != nil {
// 		return fmt.Errorf("Tar fayl yaratishda xatolik: %v", err)
// 	}

// 	return cli.CopyToContainer(context.Background(), containerName, containerPath, tarBuffer, types.CopyToContainerOptions{})
// }

// func getMemoryUsage() (int, error) {
// 	data, err := os.ReadFile("/proc/self/status")
// 	if err != nil {
// 		return 0, fmt.Errorf("Memory usage o'qib bo'lmadi: %v", err)
// 	}

// 	re := regexp.MustCompile(`VmRSS:\s+(\d+) kB`)
// 	matches := re.FindStringSubmatch(string(data))
// 	if len(matches) < 2 {
// 		return 0, fmt.Errorf("Memory usage topilmadi")
// 	}
// 	return strconv.Atoi(matches[1])
// }

// func generateFullCode(language, userCode, executeCode string, testCases []TestCase) string {
// 	switch language {
// 	case "python":
// 		var builder strings.Builder
// 		builder.WriteString(userCode + "\n\n")

// 		if len(testCases) > 0 {
// 			// Test rejimi
// 			builder.WriteString("if __name__ == '__main__':\n")
// 			builder.WriteString("    import sys\n")
// 			builder.WriteString("    if len(sys.argv) > 1 and sys.argv[1] == 'test':\n")

// 			// Test holatlari uchun kod
// 			for i, testCase := range testCases {
// 				builder.WriteString(fmt.Sprintf("        # Test %d\n", i+1))
// 				builder.WriteString(fmt.Sprintf("        input_values = %s\n", testCase.Input))
// 				builder.WriteString(fmt.Sprintf("        expected_output = %s\n", testCase.Output))

// 				// Execute codeni test qilish uchun moslashtirish
// 				testExecute := strings.ReplaceAll(executeCode, "input()", fmt.Sprintf("'%s'", testCase.Input))
// 				builder.WriteString(fmt.Sprintf("        %s\n", testExecute))

// 				// Natijani tekshirish
// 				builder.WriteString(fmt.Sprintf("        if result != expected_output:\n"))
// 				builder.WriteString(fmt.Sprintf("            print(f\"Test %d failed: expected {expected_output}, got {result}\")\n", i+1))
// 				builder.WriteString(fmt.Sprintf("        else:\n"))
// 				builder.WriteString(fmt.Sprintf("            print(f\"Test %d passed\")\n", i+1))
// 			}
// 		} else {
// 			// Oddiy ishga tushirish rejimi
// 			builder.WriteString("if __name__ == '__main__':\n")
// 			builder.WriteString(fmt.Sprintf("    %s\n", executeCode))
// 		}
// 		return builder.String()

// 	case "go":
// 		var builder strings.Builder
// 		builder.WriteString("package main\n\n")
// 		builder.WriteString("import (\n")
// 		builder.WriteString("    \"fmt\"\n")
// 		builder.WriteString("    \"os\"\n")
// 		builder.WriteString("    \"strconv\"\n")
// 		builder.WriteString(")\n\n")
// 		builder.WriteString(userCode + "\n\n")
// 		builder.WriteString("func main() {\n")

// 		if len(testCases) > 0 {
// 			// Test rejimi
// 			builder.WriteString("    if len(os.Args) > 1 && os.Args[1] == \"test\" {\n")
// 			for i, testCase := range testCases {
// 				builder.WriteString(fmt.Sprintf("        // Test %d\n", i+1))
// 				builder.WriteString(fmt.Sprintf("        input := %s\n", testCase.Input))
// 				builder.WriteString("        expected := " + testCase.Output + "\n")

// 				// Execute codeni test qilish uchun moslashtirish
// 				testExecute := strings.ReplaceAll(executeCode, "fmt.Scanln", "func() { return input }")
// 				builder.WriteString("        " + testExecute + "\n")
// 			}
// 			builder.WriteString("    } else {\n")
// 			builder.WriteString("        " + executeCode + "\n")
// 			builder.WriteString("    }\n")
// 		} else {
// 			// Oddiy ishga tushirish rejimi
// 			builder.WriteString("    " + executeCode + "\n")
// 		}
// 		builder.WriteString("}")
// 		return builder.String()

// 	default:
// 		return userCode + "\n\n" + executeCode
// 	}
// }

// func executeCode(cli *client.Client, containerName, command string, isTest bool) (string, float64, int, error) {
// 	startTime := time.Now()

// 	fullCommand := command
// 	if isTest {
// 		fullCommand += " test" // Test rejimini ishga tushirish
// 	}

// 	execConfig := types.ExecConfig{
// 		AttachStdout: true,
// 		AttachStderr: true,
// 		Tty:          false,
// 		Cmd:          []string{"sh", "-c", fullCommand},
// 	}

// 	execIDResp, err := cli.ContainerExecCreate(context.Background(), containerName, execConfig)
// 	if err != nil {
// 		return "", 0, 0, fmt.Errorf("Exec yaratishda xatolik: %v", err)
// 	}

// 	resp, err := cli.ContainerExecAttach(context.Background(), execIDResp.ID, types.ExecStartCheck{})
// 	if err != nil {
// 		return "", 0, 0, fmt.Errorf("Exec attach qilishda xatolik: %v", err)
// 	}
// 	defer resp.Close()

// 	var outputBuffer bytes.Buffer
// 	if _, err := io.Copy(&outputBuffer, resp.Reader); err != nil {
// 		return "", 0, 0, fmt.Errorf("Natijani o'qishda xatolik: %v", err)
// 	}

// 	executionTime := time.Since(startTime).Seconds()
// 	memoryUsage, _ := getMemoryUsage()

// 	return outputBuffer.String(), executionTime, memoryUsage, nil
// }

// func parseTestOutput(output string, testCases []TestCase) []map[string]interface{} {
// 	var results []map[string]interface{}

// 	for i, testCase := range testCases {
// 		result := map[string]interface{}{
// 			"test_case": i + 1,
// 			"input":     testCase.Input,
// 			"expected":  testCase.Output,
// 			"status":    "unknown",
// 			"output":    "",
// 		}

// 		// Python uchun test natijalarini tahlil qilish
// 		if strings.Contains(output, fmt.Sprintf("Test %d passed", i+1)) {
// 			result["status"] = "passed"
// 			result["output"] = testCase.Output
// 		} else if strings.Contains(output, fmt.Sprintf("Test %d failed", i+1)) {
// 			result["status"] = "failed"
// 			re := regexp.MustCompile(fmt.Sprintf(`Test %d failed: expected .+, got (.+)`, i+1))
// 			if match := re.FindStringSubmatch(output); len(match) > 1 {
// 				result["output"] = match[1]
// 			}
// 		}

// 		results = append(results, result)
// 	}

// 	return results
// }

// func main() {
// 	app := fiber.New()
// 	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		log.Fatalf("Docker client yaratishda xatolik: %v", err)
// 	}
// 	defer cli.Close()

// 	app.Post("/execute", func(c *fiber.Ctx) error {
// 		var submission Submission
// 		if err := c.BodyParser(&submission); err != nil {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Invalid JSON format",
// 			})
// 		}

// 		command, exists := languageCommands[submission.Language]
// 		if !exists {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"success": false,
// 				"error":   "Unsupported language",
// 			})
// 		}

// 		containerName := fmt.Sprintf("%s-app", submission.Language)
// 		fileName := getFileName(submission.Language)

// 		fullCode := generateFullCode(submission.Language, submission.Code, submission.ExecuteCode, submission.TestCases)
// 		if err := fileConnect(cli, containerName, fileName, "/app/", fullCode); err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   err.Error(),
// 			})
// 		}

// 		isTest := len(submission.TestCases) > 0
// 		output, execTime, memoryUsage, err := executeCode(cli, containerName, command, isTest)
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"success": false,
// 				"error":   err.Error(),
// 			})
// 		}

// 		response := fiber.Map{
// 			"success": true,
// 			"time":    execTime,
// 			"memory":  memoryUsage,
// 			"output":  output,
// 		}

// 		if isTest {
// 			testResults := parseTestOutput(output, submission.TestCases)
// 			response["test_results"] = testResults

// 			// Test statistikasi
// 			passed := 0
// 			for _, result := range testResults {
// 				if result["status"] == "passed" {
// 					passed++
// 				}
// 			}

// 			response["summary"] = fiber.Map{
// 				"total_tests": len(submission.TestCases),
// 				"passed":      passed,
// 				"failed":      len(submission.TestCases) - passed,
// 			}
// 		}

// 		return c.JSON(response)
// 	})

// 	log.Fatal(app.Listen(":3000"))
// }
