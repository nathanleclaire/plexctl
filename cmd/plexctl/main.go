package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/r3labs/sse/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	stopCursorCode            = "\x1b[?25l"
	startCursorCode           = "\x1b[?25h"
	ssePrefixDataSize         = 6
	dirPerm                   = 0o700
	filePerm                  = 0o600
	snippetLen                = 50
	idTruncLen                = 8
	maxSSEBytes               = 1 << 16
	smoothPrintTickerInterval = 3 * time.Millisecond
	smoothPrintBufferSize     = 1024
	tabwriterPadding          = 2
)

type ChatCompletionRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream"`
	MaxTokens *int      `json:"max_tokens,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Thread struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
}

type ThreadStore interface {
	Load(idPrefix string) (*Thread, error)
	Save(th *Thread) error
	List() ([]*Thread, error)
}

type FSStore struct {
	basePath string
}

type sseDelta struct {
	Content string `json:"content"`
}

type sseChoice struct {
	Delta        sseDelta `json:"delta"`
	FinishReason string   `json:"finish_reason"`
}

type sseChunk struct {
	Choices   []sseChoice `json:"choices"`
	Citations []string    `json:"citations"`
}

func NewFSStore() (*FSStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".plexctl", "threads")
	if err := os.MkdirAll(base, dirPerm); err != nil {
		return nil, err
	}
	return &FSStore{basePath: base}, nil
}

func (fs *FSStore) Load(idPrefix string) (*Thread, error) {
	files, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, err
	}
	var match string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), idPrefix) {
			if match != "" {
				return nil, fmt.Errorf(
					"prefix '%s' matched more than one thread",
					idPrefix,
				)
			}
			match = f.Name()
		}
	}
	if match == "" {
		return nil, errors.New("no matching thread found")
	}
	path := filepath.Join(fs.basePath, match)
	data, err := safeReadFile(fs.basePath, path)
	if err != nil {
		return nil, err
	}
	var th Thread
	if err := json.Unmarshal(data, &th); err != nil {
		return nil, err
	}
	return &th, nil
}

func (fs *FSStore) Save(th *Thread) error {
	data, err := json.MarshalIndent(th, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(fs.basePath, th.ID+".json")
	return safeWriteFile(fs.basePath, path, data, filePerm)
}

func (fs *FSStore) List() ([]*Thread, error) {
	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, err
	}
	var result []*Thread
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(fs.basePath, e.Name())
		data, err := safeReadFile(fs.basePath, path)
		if err != nil {
			continue
		}
		var th Thread
		if json.Unmarshal(data, &th) == nil {
			result = append(result, &th)
		}
	}
	return result, nil
}

func safeReadFile(basePath, filePath string) ([]byte, error) {
	absFile, err := checkPath(basePath, filePath)
	if err != nil {
		return nil, err
	}
	// nolint:gosec
	f, err := os.Open(absFile)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			panic(cerr)
		}
	}()
	return io.ReadAll(f)
}

func safeWriteFile(
	basePath, filePath string,
	data []byte,
	perm os.FileMode,
) error {
	absFile, err := checkPath(basePath, filePath)
	if err != nil {
		return err
	}
	return os.WriteFile(absFile, data, perm)
}

func checkPath(basePath, filePath string) (string, error) {
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absFile, absBase+string(os.PathSeparator)) &&
		absFile != absBase {
		return "", fmt.Errorf("unsafe file path: %s", filePath)
	}
	return absFile, nil
}

func newThreadID(messages []Message) string {
	if len(messages) == 0 {
		return base58.Encode([]byte("empty"))
	}
	h := sha256.Sum256([]byte(messages[0].Content + time.Now().String()))
	return base58.Encode(h[:])
}

var debug bool

func buildGetCmd(store ThreadStore, tokenPtr *string) *cobra.Command {
	var (
		model    string
		threadID string
		maxToks  int
	)
	cmd := &cobra.Command{
		Use:   "get <query>",
		Short: "Get a completion for a query from Perplexity",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			th, err := handleThreadLogic(store, threadID, query)
			if err != nil {
				return err
			}
			return streamCompletion(
				*tokenPtr,
				model,
				th,
				store,
				maxToks,
			)
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", "sonar", "Model name")
	cmd.Flags().StringVar(&threadID, "thread", "",
		"Continue an existing thread by ID prefix")
	cmd.Flags().IntVar(&maxToks, "max-tokens", 0, "Max tokens in response")
	return cmd
}

func buildThreadGetCmd(store ThreadStore) *cobra.Command {
	return &cobra.Command{
		Use:   "get <threadid>",
		Short: "Get a thread's messages",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			th, err := store.Load(args[0])
			if err != nil {
				return err
			}
			idPrefix := th.ID
			if len(idPrefix) > idTruncLen {
				idPrefix = idPrefix[:idTruncLen]
			}
			fmt.Printf("THREAD: %s\n\n", idPrefix)
			for i, msg := range th.Messages {
				fmt.Printf(
					"[%d] %s:\n%s\n\n",
					i,
					strings.ToUpper(msg.Role),
					msg.Content,
				)
			}
			return nil
		},
	}
}

func buildThreadCmd(store ThreadStore) *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Manage threads",
		RunE: func(_ *cobra.Command, _ []string) error {
			all, err := store.List()
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(
				os.Stdout,
				0,
				0,
				tabwriterPadding,
				' ',
				0,
			)

			if _, err = fmt.Fprintln(w, "THREAD ID\tFIRST USER MESSAGE"); err != nil {
				return err
			}

			for _, th := range all {
				if len(th.Messages) == 0 {
					continue
				}
				id := th.ID
				if len(id) > idTruncLen {
					id = id[:idTruncLen]
				}
				first := th.Messages[0].Content
				if filter == "" ||
					strings.Contains(th.ID, filter) ||
					strings.Contains(first, filter) {
					if _, err = fmt.Fprintf(w, "%s\t%s\n",
						id, snippet(first)); err != nil {
						return err
					}
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "",
		"Filter by ID or content substring")
	cmd.AddCommand(buildThreadGetCmd(store))
	return cmd
}

func handleThreadLogic(
	store ThreadStore,
	idPrefix, userQuery string,
) (*Thread, error) {
	if idPrefix != "" {
		th, err := store.Load(idPrefix)
		if err != nil {
			return nil, err
		}
		th.Messages = append(th.Messages, Message{"user", userQuery})
		return th, nil
	}
	th := &Thread{Messages: []Message{{Role: "user", Content: userQuery}}}
	th.ID = newThreadID(th.Messages)
	return th, nil
}

func closeBody(resp *http.Response) {
	if cerr := resp.Body.Close(); cerr != nil && debug {
		fmt.Fprintf(
			os.Stderr,
			"Error closing response body: %v\n",
			cerr,
		)
	}
}

// readSSE has been refactored to keep complexity <= 10
func readSSE(reader *sse.EventStreamReader) (string, []string, error) {
	fmt.Print(stopCursorCode)
	defer fmt.Print(startCursorCode)

	var final strings.Builder
	var buf bytes.Buffer
	var finalCitations []string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	charCh := make(chan rune, smoothPrintBufferSize)
	startSmoothPrinter(ctx, &wg, charCh)

	for {
		raw, err := reader.ReadEvent()
		done, checkErr := checkEventEnd(raw, err)
		if checkErr != nil {
			close(charCh)
			wg.Wait()
			return "", nil, checkErr
		}
		if done {
			close(charCh)
			wg.Wait()
			return final.String(), finalCitations, nil
		}
		if err := processSSEChunk(ctx, raw, &buf, &final, &finalCitations, charCh); err != nil {
			close(charCh)
			wg.Wait()
			return "", nil, err
		}
	}
}

// processSSEChunk in a separate function to reduce cyclomatic complexity.
func processSSEChunk(
	ctx context.Context,
	raw []byte,
	buf *bytes.Buffer,
	final *strings.Builder,
	finalCitations *[]string,
	charCh chan rune,
) error {
	if len(raw) == 0 {
		return nil
	}
	dataIdx := bytes.Index(raw, []byte("data: "))
	appendSSEChunk(buf, raw, dataIdx)

	var chunk sseChunk
	if err := json.Unmarshal(buf.Bytes(), &chunk); err != nil {
		// nolint:nilerr
		return nil
	}
	buf.Reset()

	if len(chunk.Citations) > 0 {
		*finalCitations = chunk.Citations
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	txt := chunk.Choices[0].Delta.Content
	final.WriteString(txt)

	for _, r := range txt {
		select {
		case charCh <- r:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func streamCompletion(
	token, model string,
	th *Thread,
	store ThreadStore,
	maxTokens int,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := doCompletionRequest(ctx, token, model, th, maxTokens)
	if err != nil {
		return err
	}
	defer closeBody(resp)

	reader := sse.NewEventStreamReader(resp.Body, maxSSEBytes)
	finalContent, citations, sseErr := readSSE(reader)
	if sseErr != nil {
		return fmt.Errorf("read SSE: %w", sseErr)
	}
	return handleCompletionResponse(store, th, finalContent, citations)
}

// doCompletionRequest is factored out to help shorten streamCompletion.
func doCompletionRequest(
	ctx context.Context,
	token, model string,
	th *Thread,
	maxTokens int,
) (*http.Response, error) {
	reqBody := ChatCompletionRequest{
		Model:    model,
		Messages: th.Messages,
		Stream:   true,
	}
	if maxTokens > 0 {
		reqBody.MaxTokens = &maxTokens
	}
	bodyJSON, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.perplexity.ai/chat/completions",
		bytes.NewReader(bodyJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("request creation: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request execute: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		closeBody(resp)
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}
	return resp, nil
}

// handleCompletionResponse is factored out to help keep streamCompletion short.
func handleCompletionResponse(
	store ThreadStore,
	th *Thread,
	finalContent string,
	citations []string,
) error {
	if finalContent != "" {
		th.Messages = append(th.Messages,
			Message{Role: "assistant", Content: finalContent})
		if err := store.Save(th); err != nil {
			return fmt.Errorf("save thread: %w", err)
		}
	}
	if len(citations) > 0 {
		fmt.Println("\n\nCitations:")
		for i, c := range citations {
			fmt.Printf("[%d] %s\n", i+1, c)
		}
	}
	return nil
}

// startSmoothPrinter prints runes at intervals in a goroutine.
func startSmoothPrinter(
	ctx context.Context,
	wg *sync.WaitGroup,
	charCh <-chan rune,
) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(smoothPrintTickerInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-charCh:
				if !ok {
					return
				}
				fmt.Printf("%c", r)
				<-tick.C
			}
		}
	}()
}

func checkEventEnd(raw []byte, err error) (bool, error) {
	if err != nil && errors.Is(err, io.EOF) {
		if debug {
			fmt.Fprintln(
				os.Stderr,
				"DEBUG: Received EOF from server.",
			)
		}
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("SSE read: %w", err)
	}
	if debug && len(raw) > 0 {
		fmt.Fprintf(os.Stderr, "\nDEBUG: Raw SSE event: %q\n", raw)
	}
	if bytes.Contains(raw, []byte("[DONE]")) {
		if debug {
			fmt.Fprintln(os.Stderr, "DEBUG: Got [DONE] sentinel.")
		}
		return true, nil
	}
	return false, nil
}

func appendSSEChunk(buf *bytes.Buffer, raw []byte, dataIdx int) {
	if dataIdx == -1 {
		buf.Write(raw)
		return
	}
	buf.Write(raw[dataIdx+ssePrefixDataSize:])
}

// parsePartialSSE was unused, so we remove it to satisfy lint (unused).

func snippet(s string) string {
	if len(s) > snippetLen {
		return s[:snippetLen] + "..."
	}
	return s
}

func main() {
	os.Exit(run())
}

func run() int {
	store, err := NewFSStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to init store:", err)
		return 1
	}
	rootCmd := buildRootCmd(store)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	return 0
}

func buildRootCmd(store ThreadStore) *cobra.Command {
	var token string
	viper.SetEnvPrefix("PERPLEXITY")
	viper.AutomaticEnv()

	preRun := func(cmd *cobra.Command, args []string) {
		if token == "" {
			token = viper.GetString("API_TOKEN")
			if token == "" {
				fmt.Fprintln(
					os.Stderr,
					"No token provided. Set via --token or PERPLEXITY_API_TOKEN.",
				)
				os.Exit(1)
			}
		}
	}

	rootCmd := &cobra.Command{Use: "plexctl", PersistentPreRun: preRun}
	rootCmd.PersistentFlags().StringVar(
		&token,
		"token",
		"",
		"Perplexity API token (env PERPLEXITY_API_TOKEN)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&debug,
		"debug",
		false,
		"Enable debug logs to stderr",
	)
	rootCmd.AddCommand(buildGetCmd(store, &token))
	rootCmd.AddCommand(buildThreadCmd(store))
	return rootCmd
}
