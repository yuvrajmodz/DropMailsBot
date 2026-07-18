package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

const (
	ollamaExecutable = "ollama"
	modelName        = "smollm2:360m"
)

func InitializeLocalLLM() error {
	printInfo("Checking Ollama...")

	ollamaInstalled, err := isOllamaInstalled()
	if err != nil {
		return fmt.Errorf("failed to check Ollama installation: %w", err)
	}

	if !ollamaInstalled {
		printError("Local LLM not installed.")
		if !promptUser("Do you want to install it? (y/n): ") {
			os.Exit(0)
		}

		printInfo("Installing Ollama...")
		if err := installOllama(); err != nil {
			return fmt.Errorf("failed to install Ollama: %w", err)
		}

		printInfo("Verifying installation...")
		if err := verifyOllamaInstallation(); err != nil {
			printError("Systematic Errors to install Local LLM")
			return fmt.Errorf("Ollama verification failed: %w", err)
		}
		printSuccess("Ollama installed successfully.")
	}

	printInfo("Checking Local LLM...")
	modelExists, err := isModelInstalled()
	if err != nil {
		return fmt.Errorf("failed to check model installation: %w", err)
	}

	if !modelExists {
		printError("Local LLM model not installed.")
		if !promptUser("Do you want to install it? (y/n): ") {
			os.Exit(0)
		}

		printInfo(fmt.Sprintf("Installing %s...", modelName))
		if err := installModel(); err != nil {
			return fmt.Errorf("failed to install model: %w", err)
		}

		printInfo("Verifying installation...")
		if err := verifyModelInstallation(); err != nil {
			printError("Systematic Errors to install Local LLM")
			return fmt.Errorf("model verification failed: %w", err)
		}
		printSuccess(fmt.Sprintf("%s installed successfully.", modelName))
	}

	printSuccess("Local LLM Ready.")
	return nil
}

func isOllamaInstalled() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ollamaExecutable, "--version")
	cmd.Stdout = nil
	cmd.Stderr = nil

	err := cmd.Run()
	if err != nil {
		return false, nil
	}

	return true, nil
}

func installOllama() error {
	osName := runtime.GOOS

	switch osName {
	case "linux":
		return installOllamaLinux()
	case "darwin":
		return fmt.Errorf("macOS installation not yet implemented")
	case "windows":
		return fmt.Errorf("Windows installation not yet implemented")
	default:
		return fmt.Errorf("unsupported operating system: %s", osName)
	}
}

func installOllamaLinux() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "curl", "-fsSL", "https://ollama.com/install.sh")
	cmd.Stdout = nil
	cmd.Stderr = nil

	curlOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to download Ollama installation script: %w", err)
	}

	cmd = exec.CommandContext(ctx, "sh", "-c", string(curlOutput))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute Ollama installation script: %w", err)
	}

	return nil
}

func verifyOllamaInstallation() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ollamaExecutable, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ollama --version failed: %w, output: %s", err, string(output))
	}

	if len(output) == 0 {
		return fmt.Errorf("ollama --version returned empty output")
	}

	return nil
}

func isModelInstalled() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ollamaExecutable, "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to list models: %w", err)
	}

	return strings.Contains(string(output), modelName), nil
}

func installModel() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, ollamaExecutable, "pull", modelName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull model %s: %w", modelName, err)
	}

	return nil
}

func verifyModelInstallation() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ollamaExecutable, "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list models for verification: %w", err)
	}

	if !strings.Contains(string(output), modelName) {
		return fmt.Errorf("model %s not found in Ollama model list", modelName)
	}

	return nil
}

func promptUser(message string) bool {
	fmt.Print(colorRed + message + colorReset)
	
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(input)
	return input == "y" || input == "Y"
}

func printInfo(message string) {
	fmt.Println(colorCyan + message + colorReset)
}

func printSuccess(message string) {
	fmt.Println(colorGreen + message + colorReset)
}

func printError(message string) {
	fmt.Println(colorRed + message + colorReset)
}

func printWarning(message string) {
	fmt.Println(colorYellow + message + colorReset)
}

func executeCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command %s failed: %w", name, err)
	}
	return string(output), nil
}

func executeCommandWithStreams(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}
	
	go func() {
		io.Copy(os.Stdout, stdout)
	}()
	
	go func() {
		io.Copy(os.Stderr, stderr)
	}()
	
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	
	return nil
}
