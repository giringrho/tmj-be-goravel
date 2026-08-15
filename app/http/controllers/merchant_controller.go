package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"os"
	"path/filepath"

	"github.com/goravel/framework/contracts/http"

	"goravel/app/services/csvprocessor"
)

type MerchantController struct{}

func NewMerchantController() *MerchantController {
	return &MerchantController{}
}

// Process handles multipart file upload, runs the concurrent CSV processor
// synchronously, and returns the aggregated result as JSON.
//
// POST /merchants/process  (multipart form field "files")
func (r *MerchantController) Process(ctx http.Context) http.Response {
	files, err := ctx.Request().Files("files")
	if err != nil {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": fmt.Sprintf("read uploaded files: %v", err),
		})
	}
	if len(files) == 0 {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": "no files uploaded (field name must be \"files\")",
		})
	}

	// Collect temp paths of uploaded files. The framework stores uploads in a
	// temp location; File() returns that path.
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.File())
	}

	res, err := csvprocessor.Process(ctx.Context(), paths, csvprocessor.Config{
		NumWorkers: 4,
	})
	if err != nil {
		return ctx.Response().Status(nethttp.StatusUnprocessableEntity).Json(http.Json{
			"error":  err.Error(),
			"report": res.Report,
		})
	}

	return ctx.Response().Header("Access-Control-Allow-Origin", "*").Success().Json(http.Json{
		"report": res.Report,
		"series": res.Series,
	})
}

// ProcessStream handles multipart file upload and streams Progress events
// back to the client via Server-Sent Events (SSE), followed by a final
// "result" event containing the aggregated output.
//
// POST /merchants/process/stream  (multipart form field "files")
// Response Content-Type: text/event-stream
func (r *MerchantController) ProcessStream(ctx http.Context) http.Response {
	files, err := ctx.Request().Files("files")
	if err != nil {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": fmt.Sprintf("read uploaded files: %v", err),
		})
	}
	if len(files) == 0 {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": "no files uploaded (field name must be \"files\")",
		})
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.File())
	}

	progressCh := make(chan csvprocessor.Progress, 32)

	// Run the processor in a goroutine; stream events as they arrive.
	type procResult struct {
		Res csvprocessor.Result
		Err error
	}
	resultCh := make(chan procResult, 1)

	procCtx, cancel := context.WithCancel(ctx.Context())
	go func() {
		res, err := csvprocessor.Process(procCtx, paths, csvprocessor.Config{
			NumWorkers: 4,
			ProgressCh: progressCh,
		})
		resultCh <- procResult{Res: res, Err: err}
	}()

	// Stream SSE via Goravel's .Stream() API. CORS headers are injected by
	// the Vite proxy (same-origin from browser perspective), so the backend
	// doesn't need to set them on the streaming response.
	return ctx.Response().Stream(nethttp.StatusOK, func(w http.StreamWriter) error {
		defer cancel()

		// Stream progress events until the processor finishes.
		for {
			select {
			case p, ok := <-progressCh:
				if !ok {
					goto done
				}
				data, _ := json.Marshal(p)
				if _, err := w.WriteString(fmt.Sprintf("event: progress\ndata: %s\n\n", data)); err != nil {
					return err
				}
				if err := w.Flush(); err != nil {
					return err
				}
			case pr := <-resultCh:
				payload := map[string]any{
					"report": pr.Res.Report,
					"series": pr.Res.Series,
				}
				if pr.Err != nil {
					payload["error"] = pr.Err.Error()
				}
				data, _ := json.Marshal(payload)
				if _, err := w.WriteString(fmt.Sprintf("event: result\ndata: %s\n\n", data)); err != nil {
					return err
				}
				_ = w.Flush()
				return nil
			}
		}

	done:
		pr := <-resultCh
		payload := map[string]any{
			"report": pr.Res.Report,
			"series": pr.Res.Series,
		}
		if pr.Err != nil {
			payload["error"] = pr.Err.Error()
		}
		data, _ := json.Marshal(payload)
		if _, err := w.WriteString(fmt.Sprintf("event: result\ndata: %s\n\n", data)); err != nil {
			return err
		}
		_ = w.Flush()
		return nil
	})
}

// ProcessDir processes all CSV files in a given directory path (passed as
// JSON body { "dir": "..." }) and returns the result. Useful for testing
// without file upload.
//
// POST /merchants/process-dir  (application/json)
func (r *MerchantController) ProcessDir(ctx http.Context) http.Response {
	var body struct {
		Dir     string `json:"dir"`
		Workers int    `json:"workers"`
	}
	if err := ctx.Request().Bind(&body); err != nil {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": fmt.Sprintf("invalid body: %v", err),
		})
	}
	if body.Dir == "" {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": "dir is required",
		})
	}
	if body.Workers <= 0 {
		body.Workers = 4
	}

	entries, err := os.ReadDir(body.Dir)
	if err != nil {
		return ctx.Response().Status(nethttp.StatusBadRequest).Json(http.Json{
			"error": fmt.Sprintf("read dir: %v", err),
		})
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".csv" {
			files = append(files, filepath.Join(body.Dir, e.Name()))
		}
	}
	if len(files) == 0 {
		return ctx.Response().Status(nethttp.StatusNotFound).Json(http.Json{
			"error": "no CSV files found in directory",
		})
	}

	res, err := csvprocessor.Process(ctx.Context(), files, csvprocessor.Config{
		NumWorkers: body.Workers,
	})
	if err != nil {
		return ctx.Response().Status(nethttp.StatusUnprocessableEntity).Json(http.Json{
			"error":  err.Error(),
			"report": res.Report,
		})
	}

	return ctx.Response().Success().Json(http.Json{
		"report": res.Report,
		"series": res.Series,
	})
}
