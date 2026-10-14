// Command seed posts sample CallRail calls to a running callprocessor, so you
// can try the API without CallRail.
//
// Run it from the project root:
//
//	go run ./cmd/seed                                  # local: http://localhost:8080
//	go run ./cmd/seed -url http://<ALB hostname>       # EKS
//	go run ./cmd/seed -n 50 -concurrency 10
//
// Call ids are fixed (CALseed0001, CALseed0002, ...), so running it twice
// reports "unchanged" instead of creating duplicates.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// postResult is the outcome of sending one call.
type postResult struct {
	id         string
	statusCode int
	result     string // "inserted", "updated" or "unchanged", from the API
	err        error
	elapsed    time.Duration
}

// apiResponse is the JSON body POST /api/calls returns on success.
type apiResponse struct {
	Status string `json:"status"`
	Result string `json:"result"`
	ID     string `json:"id"`
}

var (
	customerNames  = []string{"JOHN SMITH", "MARIA GARCIA", "WEI CHEN", "AISHA KHAN", "LIAM O'BRIEN", "SOFIA ROSSI", "DAVID KIM", "FATIMA ALI"}
	customerCities = []string{
		"Salt Lake City, UT", "Denver, CO", "Phoenix, AZ", "Boise, ID",
		"Las Vegas, NV", "Portland, OR", "Seattle, WA", "Albuquerque, NM",
	}
)

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "base URL of the callprocessor API")
	count := flag.Int("n", 20, "number of calls to send")
	concurrency := flag.Int("concurrency", 5, "how many requests to send at once")
	samplePath := flag.String("sample", "data/callrail_sample.json", "sample CallRail payload to base the calls on")
	resend := flag.Bool("resend", true, "re-send a few calls afterwards to show the unchanged and updated results")
	flag.Parse()

	url := strings.TrimRight(*baseURL, "/")
	client := &http.Client{Timeout: 15 * time.Second}

	sample, err := loadSample(*samplePath)
	if err != nil {
		log.Fatalf("loading sample: %v", err)
	}

	calls := make([]map[string]any, *count)
	for i := range calls {
		calls[i] = makeCall(sample, i)
	}

	fmt.Printf("Sending %d calls to %s (%d at a time)\n\n", len(calls), url, *concurrency)
	start := time.Now()
	results := postAll(client, url, calls, *concurrency)
	printResults(results)
	fmt.Printf("Took %s\n", time.Since(start).Round(time.Millisecond))

	failed := countFailures(results)

	if *resend && len(calls) >= 4 {
		// The first two exactly as before: the API should report "unchanged".
		// The next two with a field changed: it should report "updated".
		again := []map[string]any{calls[0], calls[1], modified(calls[2]), modified(calls[3])}

		fmt.Printf("\nRe-sending 4 calls: 2 identical, 2 with a changed duration\n\n")
		results = postAll(client, url, again, *concurrency)
		printResults(results)
		failed += countFailures(results)
	}

	fmt.Println()
	if err := printFirstPage(client, url); err != nil {
		log.Printf("GET /api/calls: %v", err)
		failed++
	}

	if failed > 0 {
		os.Exit(1)
	}
}

func loadSample(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sample map[string]any
	if err := json.Unmarshal(data, &sample); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	return sample, nil
}

// makeCall returns a copy of the sample with a unique id and some fields
// varied, so the stored calls don't all look the same.
func makeCall(sample map[string]any, i int) map[string]any {
	call := maps.Clone(sample) // shallow copy: only top-level fields are replaced below

	city, state, _ := strings.Cut(customerCities[i%len(customerCities)], ", ")
	duration := 30 + (i*47)%600 // between 30 seconds and 10 minutes
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC).Add(time.Duration(i) * 37 * time.Minute)

	call["id"] = fmt.Sprintf("CALseed%04d", i+1)
	call["customer_name"] = customerNames[i%len(customerNames)]
	call["customer_city"] = city
	call["customer_state"] = state
	call["customer_phone_number"] = fmt.Sprintf("+1555555%04d", i+1)
	call["duration"] = duration
	call["formatted_duration"] = fmt.Sprintf("%dm %ds", duration/60, duration%60)
	call["start_time"] = start.Format(time.RFC3339)
	call["answered"] = i%5 != 0 // every fifth call was missed
	call["first_call"] = i%3 == 0
	return call
}

// modified returns a copy of call with its duration changed by one second.
func modified(call map[string]any) map[string]any {
	changed := maps.Clone(call)
	changed["duration"] = call["duration"].(int) + 1
	return changed
}

// postAll sends every call, up to concurrency at a time, and returns the
// results in the same order as calls.
func postAll(client *http.Client, url string, calls []map[string]any, concurrency int) []postResult {
	results := make([]postResult, len(calls))

	var g errgroup.Group
	g.SetLimit(concurrency) // g.Go blocks while this many goroutines are running

	for i, call := range calls {
		g.Go(func() error {
			// Each goroutine writes only its own slot, so no locking is needed.
			results[i] = postCall(client, url, call)
			// Failures are recorded in the result rather than returned, so one
			// failed call doesn't stop the others.
			return nil
		})
	}
	g.Wait()

	return results
}

// postCall uses a named result (res) so the deferred function can set
// res.elapsed after whichever return statement runs.
func postCall(client *http.Client, url string, call map[string]any) (res postResult) {
	res.id = call["id"].(string)
	start := time.Now()
	defer func() { res.elapsed = time.Since(start) }()

	body, err := json.Marshal(call)
	if err != nil {
		res.err = err
		return res
	}

	resp, err := client.Post(url+"/api/calls", "application/json", bytes.NewReader(body))
	if err != nil {
		res.err = err
		return res
	}
	defer resp.Body.Close()

	res.statusCode = resp.StatusCode
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		res.err = err
		return res
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		res.err = fmt.Errorf("%s", strings.TrimSpace(string(respBody)))
		return res
	}

	var parsed apiResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		res.err = fmt.Errorf("unexpected response %q: %w", respBody, err)
		return res
	}
	res.result = parsed.Result
	return res
}

func printResults(results []postResult) {
	counts := map[string]int{}
	for _, r := range results {
		if r.err != nil {
			counts["failed"]++
			fmt.Printf("  %-12s  %3d  FAILED: %v\n", r.id, r.statusCode, r.err)
			continue
		}
		counts[r.result]++
		fmt.Printf("  %-12s  %3d  %-9s  %6.1fms\n", r.id, r.statusCode, r.result, float64(r.elapsed.Microseconds())/1000)
	}

	var summary []string
	for _, key := range []string{"inserted", "updated", "unchanged", "failed"} {
		if counts[key] > 0 {
			summary = append(summary, fmt.Sprintf("%d %s", counts[key], key))
		}
	}
	fmt.Printf("\n  %s\n", strings.Join(summary, ", "))
}

func countFailures(results []postResult) int {
	n := 0
	for _, r := range results {
		if r.err != nil {
			n++
		}
	}
	return n
}

// printFirstPage fetches the newest few calls to show they were stored.
func printFirstPage(client *http.Client, url string) error {
	resp, err := client.Get(url + "/api/calls?limit=5")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var page struct {
		Data []struct {
			ID           string `json:"id"`
			CustomerName string `json:"customer_name"`
		} `json:"data"`
		Total      int64 `json:"total"`
		TotalPages int64 `json:"total_pages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return err
	}

	fmt.Printf("GET /api/calls?limit=5: %d calls stored in total, %d pages. Newest:\n", page.Total, page.TotalPages)
	for _, c := range page.Data {
		fmt.Printf("  %-12s  %s\n", c.ID, c.CustomerName)
	}
	return nil
}
