package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"shachiku/internal/config"

	"github.com/playwright-community/playwright-go"
)

var (
	pwRun      *playwright.Playwright
	pwBrowser  playwright.Browser
	pwPage     playwright.Page
	pwInitOnce sync.Once
	pwInitErr  error
)

func initPlaywright() error {
	pwInitOnce.Do(func() {
		// Install Playwright browsers (will only download if missing)
		err := playwright.Install()
		if err != nil {
			pwInitErr = fmt.Errorf("failed to install playwright browsers: %w", err)
			return
		}

		pwRun, err = playwright.Run()
		if err != nil {
			pwInitErr = fmt.Errorf("failed to run playwright: %w", err)
			return
		}

		pwBrowser, err = pwRun.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(false),
		})
		if err != nil {
			pwInitErr = fmt.Errorf("failed to launch chromium: %w", err)
			return
		}

		context, err := pwBrowser.NewContext()
		if err != nil {
			pwInitErr = fmt.Errorf("failed to create context: %w", err)
			return
		}

		pwPage, err = context.NewPage()
		if err != nil {
			pwInitErr = fmt.Errorf("failed to create page: %w", err)
			return
		}
	})
	return pwInitErr
}

func performPlaywrightCommand(args string) string {
	err := initPlaywright()
	if err != nil {
		return fmt.Sprintf("Playwright initialization failed: %v", err)
	}

	var payload struct {
		Action   string `json:"action"`
		URL      string `json:"url"`
		Selector string `json:"selector"`
		Text     string `json:"text"`
		FilePath string `json:"file_path"` // for screenshot
	}

	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return fmt.Sprintf("Error parsing playwright arguments. Ensure it is a valid JSON: %v", err)
	}

	var output string

	switch payload.Action {
	case "goto":
		if payload.URL == "" {
			return "Error: URL is required for goto action."
		}
		if _, err := pwPage.Goto(payload.URL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return fmt.Sprintf("Error navigating to %s: %v", payload.URL, err)
		}

		// Optionally wait a few seconds for network requests to settle (for SPAs), but don't fail if it times out
		pwPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateNetworkidle,
			Timeout: playwright.Float(5000), // 5 seconds max wait for idle
		})

		title, _ := pwPage.Title()
		locator := pwPage.Locator("body")
		innerText, err := locator.InnerText()
		contentStr := ""
		if err == nil {
			if len(innerText) > 2000 {
				contentStr = innerText[:2000] + "\n... [Content truncated due to length]"
			} else {
				contentStr = innerText
			}
		}

		output = fmt.Sprintf("Successfully navigated to %s. Page title: %s\nPage Content:\n%s", payload.URL, title, contentStr)

	case "click":
		if payload.Selector == "" {
			return "Error: selector is required for click action."
		}
		if err := pwPage.Locator(payload.Selector).Click(); err != nil {
			return fmt.Sprintf("Error clicking selector %s: %v", payload.Selector, err)
		}
		output = fmt.Sprintf("Successfully clicked selector %s", payload.Selector)

	case "type":
		if payload.Selector == "" || payload.Text == "" {
			return "Error: selector and text are required for type action."
		}
		if err := pwPage.Locator(payload.Selector).Fill(payload.Text); err != nil {
			return fmt.Sprintf("Error typing into selector %s: %v", payload.Selector, err)
		}
		output = fmt.Sprintf("Successfully typed into selector %s", payload.Selector)

	case "screenshot":
		filename := payload.FilePath
		if filename == "" {
			filename = filepath.Join(config.GetDataDir(), "tmp", "screenshot.png")
		}
		os.MkdirAll(filepath.Dir(filename), 0755)
		screenshot, err := pwPage.Screenshot(playwright.PageScreenshotOptions{
			FullPage: playwright.Bool(true),
		})
		if err != nil {
			return fmt.Sprintf("Error taking screenshot: %v", err)
		}

		if err := os.WriteFile(filename, screenshot, 0644); err != nil {
			return fmt.Sprintf("Error saving screenshot: %v", err)
		}
		output = fmt.Sprintf("Successfully took screenshot and saved to %s", filename)

	case "evaluate":
		// execute arbitrary js
		if payload.Text == "" {
			return "Error: text (JS code) is required for evaluate action."
		}
		res, err := pwPage.Evaluate(payload.Text)
		if err != nil {
			return fmt.Sprintf("Error evaluating js: %v", err)
		}
		output = fmt.Sprintf("Result: %v", res)

	case "close":
		err := pwBrowser.Close()
		pwInitOnce = sync.Once{} // allow re-init
		if err != nil {
			return fmt.Sprintf("Error closing browser: %v", err)
		}
		output = "Successfully closed the browser."

	default:
		return fmt.Sprintf("Error: unknown action '%s'", payload.Action)
	}

	return output
}
