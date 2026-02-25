package browser

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"goclaw/internal/skill"
)

// Navigate is a skill that navigates to a URL and extracts page content.
type Navigate struct{}

func (n *Navigate) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "browse_web",
		Description: "Navigate to a URL using a headless browser. Extracts the page title and visible text content. Use this to read web pages, check websites, or gather information from the internet.",
		Parameters: map[string]skill.ParamSchema{
			"url": {
				Type:        "string",
				Description: "The full URL to navigate to (e.g., https://example.com)",
			},
			"extract": {
				Type:        "string",
				Description: "What to extract: 'text' (visible text, default), 'html' (raw HTML), 'title' (page title only)",
				Enum:        []string{"text", "html", "title"},
			},
		},
		Required: []string{"url"},
	}
}

func (n *Navigate) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	url := args["url"]
	if url == "" {
		return skill.ToolResult{Error: "url argument is required"}, nil
	}

	extract := args["extract"]
	if extract == "" {
		extract = "text"
	}

	// 30-second timeout for browser operations.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create a new browser context.
	allocCtx, allocCancel := chromedp.NewContext(ctx)
	defer allocCancel()

	var title, content string

	tasks := chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Title(&title),
	}

	switch extract {
	case "html":
		tasks = append(tasks, chromedp.OuterHTML("html", &content))
	case "title":
		// Title already captured above.
	default: // "text"
		tasks = append(tasks, chromedp.Text("body", &content, chromedp.NodeVisible))
	}

	if err := chromedp.Run(allocCtx, tasks...); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("browser error: %v", err)}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\nURL: %s\n", title, url))

	if extract != "title" {
		// Truncate very long content.
		const maxLen = 30_000
		if len(content) > maxLen {
			content = content[:maxLen] + "\n\n... [content truncated]"
		}
		sb.WriteString(fmt.Sprintf("\n--- Content ---\n%s", content))
	}

	return skill.ToolResult{Output: sb.String()}, nil
}

// Screenshot is a skill that takes a screenshot of a web page.
type Screenshot struct{}

func (s *Screenshot) Definition() skill.ToolDefinition {
	return skill.ToolDefinition{
		Name:        "screenshot_web",
		Description: "Take a screenshot of a web page. Returns the file path of the saved PNG screenshot.",
		Parameters: map[string]skill.ParamSchema{
			"url": {
				Type:        "string",
				Description: "The URL to screenshot",
			},
			"output_path": {
				Type:        "string",
				Description: "Path where to save the screenshot PNG",
			},
		},
		Required: []string{"url", "output_path"},
	}
}

func (s *Screenshot) Execute(ctx context.Context, args map[string]string) (skill.ToolResult, error) {
	url := args["url"]
	outputPath := args["output_path"]

	if url == "" || outputPath == "" {
		return skill.ToolResult{Error: "url and output_path arguments are required"}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewContext(ctx)
	defer allocCancel()

	var buf []byte
	if err := chromedp.Run(allocCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("screenshot error: %v", err)}, nil
	}

	if err := writeFile(outputPath, buf); err != nil {
		return skill.ToolResult{Error: fmt.Sprintf("save error: %v", err)}, nil
	}

	return skill.ToolResult{Output: fmt.Sprintf("Screenshot saved to: %s (%d bytes)", outputPath, len(buf))}, nil
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
