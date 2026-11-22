// Command aws_docs searches AWS documentation for the aws-docs skill.
//
// It mirrors the `search_documentation` tool of the awslabs AWS Documentation
// MCP server (github.com/awslabs/mcp) using only the Go standard library, so no
// dependencies need fetching — just the Go toolchain to build.
//
// Reading a page is handled by the skill via WebFetch on the page's `.md`
// sibling (see SKILL.md); only search needs code because it is a POST the
// WebFetch tool cannot make.
//
// NOTE: `proxy.search.docs.aws.com` is an internal AWS docs API and is not a
// documented public contract — it may change without notice. If search stops
// returning results, SKILL.md documents a WebSearch fallback.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	searchAPIURL = "https://proxy.search.docs.aws.com/search"
	userAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 " +
		"ModelContextProtocol/skill (AWS Documentation Skill)"
	timeout = 30 * time.Second
	// excerptMax caps the context line of each rendered result, and
	// errorBodyMax caps the response-body snippet in HTTP error messages.
	excerptMax   = 300
	errorBodyMax = 200

	noResultsMsg = "No results found."
)

// stringSlice is a repeatable string flag (e.g. --product a --product b).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// uuidV4 returns a random RFC 4122 version 4 UUID built from crypto/rand.
func uuidV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// textExcerpt mirrors the fields of a suggestion's excerpt we render.
type textExcerpt struct {
	Title          string `json:"title"`
	Link           string `json:"link"`
	Summary        string `json:"summary"`
	SuggestionBody string `json:"suggestionBody"`
	Metadata       struct {
		SEOAbstract string `json:"seo_abstract"`
	} `json:"metadata"`
}

// searchResponse mirrors the fields of the search API response we render.
type searchResponse struct {
	Suggestions []struct {
		TextExcerptSuggestion *textExcerpt `json:"textExcerptSuggestion"`
	} `json:"suggestions"`
}

// search POSTs a query to the AWS docs search API and returns the raw response
// body so callers can either format it or print it verbatim (--json).
func search(phrase, intent string, products, guides []string) ([]byte, error) {
	session, err := uuidV4()
	if err != nil {
		return nil, err
	}

	contextAttributes := []map[string]string{
		{"key": "domain", "value": "docs.aws.amazon.com"},
	}
	for _, product := range products {
		contextAttributes = append(contextAttributes,
			map[string]string{"key": "aws-docs-search-product", "value": product})
	}
	for _, guide := range guides {
		contextAttributes = append(contextAttributes,
			map[string]string{"key": "aws-docs-search-guide", "value": guide})
	}

	body := map[string]any{
		"textQuery":            map[string]string{"input": phrase},
		"contextAttributes":    contextAttributes,
		"acceptSuggestionBody": "RawText",
		"locales":              []string{"en_us"},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(searchAPIURL)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("session", session)
	u.RawQuery = query.Encode()
	if intent != "" {
		// whitespace-collapse the intent, then append as a query param
		collapsed := strings.Join(strings.Fields(intent), " ")
		u.RawQuery += "&search_intent=" + url.QueryEscape(collapsed)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-MCP-Session-Id", session)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		snippet := truncate(collapse(string(data)), errorBodyMax)
		if snippet == "" {
			return nil, fmt.Errorf("HTTP %d from search API", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d from search API: %s", resp.StatusCode, snippet)
	}
	return data, nil
}

// collapse reduces runs of whitespace in s to single spaces.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate shortens s to at most max runes, appending an ellipsis if cut.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

// contextFor picks the best available context string for a suggestion.
func contextFor(e textExcerpt) string {
	for _, candidate := range []string{e.Metadata.SEOAbstract, e.Summary, e.SuggestionBody} {
		if candidate != "" {
			return truncate(collapse(candidate), excerptMax)
		}
	}
	return ""
}

// formatResults renders suggestions as readable text, one block per result.
func formatResults(data []byte, limit int) (string, error) {
	var parsed searchResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}

	var blocks []string
	rank := 0
	for _, suggestion := range parsed.Suggestions {
		if len(blocks) >= limit {
			break
		}
		e := suggestion.TextExcerptSuggestion
		if e == nil {
			continue
		}
		rank++
		title := e.Title
		if title == "" {
			title = "(untitled)"
		}
		block := fmt.Sprintf("%d. %s\n   %s", rank, title, e.Link)
		if ctx := contextFor(*e); ctx != "" {
			block += "\n   " + ctx
		}
		blocks = append(blocks, block)
	}

	if len(blocks) == 0 {
		return noResultsMsg, nil
	}
	return strings.Join(blocks, "\n\n"), nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func runSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: aws_docs search <phrase> [flags]")
		fmt.Fprintln(os.Stderr, "\nSearch official AWS documentation (docs.aws.amazon.com).")
		fmt.Fprintln(os.Stderr, "\nFlags:")
		fs.PrintDefaults()
	}

	limit := fs.Int("limit", 5, "Max results (1-50, default 5)")
	intent := fs.String("intent", "", "Natural-language search intent (no PII)")
	var products stringSlice
	var guides stringSlice
	fs.Var(&products, "product", `Filter by product, e.g. "Amazon Simple Storage Service" (repeatable)`)
	fs.Var(&guides, "guide", `Filter by guide type, e.g. "User Guide", "API Reference" (repeatable)`)
	asJSON := fs.Bool("json", false, "Print the raw JSON response")

	// Go's flag package stops at the first non-flag argument, so parse
	// iteratively to allow the phrase to appear before or between flags
	// (e.g. `search "..." --limit 5`).
	var positionals []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		positionals = append(positionals, fs.Arg(0))
		rest = fs.Args()[1:]
	}

	if len(positionals) == 0 {
		fmt.Fprintln(os.Stderr, "aws-docs search: a search phrase is required")
		fs.Usage()
		return 2
	}
	phrase := strings.Join(positionals, " ")

	n := clamp(*limit, 1, 50)
	data, err := search(phrase, *intent, products, guides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aws-docs search: %v\n", err)
		return 1
	}

	if *asJSON {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			// Fall back to the raw bytes if they aren't valid JSON.
			os.Stdout.Write(data)
		} else {
			fmt.Println(pretty.String())
		}
		return 0
	}

	out, err := formatResults(data, n)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aws-docs search: %v\n", err)
		return 1
	}
	if out == noResultsMsg && (len(products) > 0 || len(guides) > 0) {
		out += "\nHint: --product/--guide values must match AWS's exact catalogue" +
			" names (e.g. \"Amazon Simple Storage Service\"). Retry without filters."
	}
	fmt.Println(out)
	return 0
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: aws_docs <command> [args]")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	fmt.Fprintln(os.Stderr, "  search <phrase> [flags]   Search AWS documentation")
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "search":
		os.Exit(runSearch(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "aws_docs: unknown command %q\n", args[0])
		usage()
		os.Exit(2)
	}
}
