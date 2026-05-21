package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.fastly.com"

// APIError represents a non-2xx response from the Fastly API.
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("fastly API responded with %d: %s", e.Code, e.Msg)
	}
	return fmt.Sprintf("fastly API responded with %d", e.Code)
}

// Client is an authenticated Fastly API client.
type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

// NewClient returns a Client with a 30s timeout.
func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}
}

// Service represents a Fastly CDN service.
type Service struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// ListServices returns all services accessible with the configured token,
// following X-Next-Page pagination headers.
func (c *Client) ListServices(ctx context.Context) ([]Service, error) {
	var all []Service
	page := 1
	const maxPages = 1000

	for page <= maxPages {
		u := fmt.Sprintf("%s/service?per_page=100&page=%d", c.baseURL, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Fastly-Key", c.token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode/100 != 2 {
			var apiErr struct {
				Msg string `json:"msg"`
			}
			json.Unmarshal(body, &apiErr)
			return nil, &APIError{Code: resp.StatusCode, Msg: apiErr.Msg}
		}

		var batch []Service
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("decoding service list: %w", err)
		}
		all = append(all, batch...)

		next := resp.Header.Get("X-Next-Page")
		if next == "" {
			break
		}
		nextPage, err := strconv.Atoi(next)
		if err != nil || nextPage <= page {
			break
		}
		page = nextPage
	}

	return all, nil
}

// StatsData holds all numeric fields for a single historical time bucket.
// All numeric fields are float64 for direct Prometheus Gauge use.
// New Fastly fields not listed here are silently ignored by encoding/json.
type StatsData struct {
	StartTime                   uint64  `json:"start_time"`
	Requests                    float64 `json:"requests"`
	Hits                        float64 `json:"hits"`
	Miss                        float64 `json:"miss"`
	Pass                        float64 `json:"pass"`
	Errors                      float64 `json:"errors"`
	Synth                       float64 `json:"synth"`
	Bandwidth                   float64 `json:"bandwidth"`
	RespBodyBytes               float64 `json:"resp_body_bytes"`
	RespHeaderBytes             float64 `json:"resp_header_bytes"`
	BereqBodyBytes              float64 `json:"bereq_body_bytes"`
	BereqHeaderBytes            float64 `json:"bereq_header_bytes"`
	Status1xx                   float64 `json:"status_1xx"`
	Status2xx                   float64 `json:"status_2xx"`
	Status3xx                   float64 `json:"status_3xx"`
	Status4xx                   float64 `json:"status_4xx"`
	Status5xx                   float64 `json:"status_5xx"`
	Status200                   float64 `json:"status_200"`
	Status301                   float64 `json:"status_301"`
	Status302                   float64 `json:"status_302"`
	Status304                   float64 `json:"status_304"`
	Status400                   float64 `json:"status_400"`
	Status401                   float64 `json:"status_401"`
	Status403                   float64 `json:"status_403"`
	Status404                   float64 `json:"status_404"`
	Status410                   float64 `json:"status_410"`
	Status416                   float64 `json:"status_416"`
	Status422                   float64 `json:"status_422"`
	Status503                   float64 `json:"status_503"`
	HitsTime                    float64 `json:"hits_time"`
	MissTime                    float64 `json:"miss_time"`
	TLS                         float64 `json:"tls"`
	TLSv10                      float64 `json:"tls_v10"`
	TLSv11                      float64 `json:"tls_v11"`
	TLSv12                      float64 `json:"tls_v12"`
	TLSv13                      float64 `json:"tls_v13"`
	HTTP2                       float64 `json:"http2"`
	HTTP3                       float64 `json:"http3"`
	IPv6                        float64 `json:"ipv6"`
	Imgopto                     float64 `json:"imgopto"`
	ImgoptoTransforms           float64 `json:"imgopto_transforms"`
	ImgoptoRespBodyBytes        float64 `json:"imgopto_resp_body_bytes"`
	ImgoptoRespHeaderBytes      float64 `json:"imgopto_resp_header_bytes"`
	ComputeRequests             float64 `json:"compute_requests"`
	ComputeExecTimeMs           float64 `json:"compute_execution_time_ms"`
	ComputeRAMUsed              float64 `json:"compute_ram_used"`
	WAFBlocked                  float64 `json:"waf_blocked"`
	WAFLogged                   float64 `json:"waf_logged"`
	WAFPassed                   float64 `json:"waf_passed"`
	AttackReqBodyBytes          float64 `json:"attack_req_body_bytes"`
	AttackReqHeaderBytes        float64 `json:"attack_req_header_bytes"`
	AttackRespSynthBytes        float64 `json:"attack_resp_synth_bytes"`
	Shield                      float64 `json:"shield"`
	ShieldRespBodyBytes         float64 `json:"shield_resp_body_bytes"`
	ShieldRespHeaderBytes       float64 `json:"shield_resp_header_bytes"`
	Otfp                        float64 `json:"otfp"`
	OtfpDeliverTime             float64 `json:"otfp_deliver_time"`
	OtfpManifestRespBodyBytes   float64 `json:"otfp_manifest_resp_body_bytes"`
	OtfpManifestRespHeaderBytes float64 `json:"otfp_manifest_resp_header_bytes"`
	OtfpRespBodyBytes           float64 `json:"otfp_resp_body_bytes"`
	OtfpRespHeaderBytes         float64 `json:"otfp_resp_header_bytes"`
	Video                       float64 `json:"video"`
	PCI                         float64 `json:"pci"`
	Log                         float64 `json:"log"`
	LogBytes                    float64 `json:"log_bytes"`
	RecvSubTime                 float64 `json:"recv_sub_time"`
	RecvSubCount                float64 `json:"recv_sub_count"`
	HashSubTime                 float64 `json:"hash_sub_time"`
	HashSubCount                float64 `json:"hash_sub_count"`
	MissSubTime                 float64 `json:"miss_sub_time"`
	MissSubCount                float64 `json:"miss_sub_count"`
	FetchSubTime                float64 `json:"fetch_sub_time"`
	FetchSubCount               float64 `json:"fetch_sub_count"`
	PassSubTime                 float64 `json:"pass_sub_time"`
	PassSubCount                float64 `json:"pass_sub_count"`
	PipeSubTime                 float64 `json:"pipe_sub_time"`
	PipeSubCount                float64 `json:"pipe_sub_count"`
	DeliverSubTime              float64 `json:"deliver_sub_time"`
	DeliverSubCount             float64 `json:"deliver_sub_count"`
	ErrorSubTime                float64 `json:"error_sub_time"`
	ErrorSubCount               float64 `json:"error_sub_count"`
	HitSubTime                  float64 `json:"hit_sub_time"`
	HitSubCount                 float64 `json:"hit_sub_count"`
	PrehashSubTime              float64 `json:"prehash_sub_time"`
	PrehashSubCount             float64 `json:"prehash_sub_count"`
	PredeliverSubTime           float64 `json:"predeliver_sub_time"`
	PredeliverSubCount          float64 `json:"predeliver_sub_count"`
	HitRatio                    float64 `json:"hit_ratio"`
	OriginOffload               float64 `json:"origin_offload"`
	EdgeRequests                float64 `json:"edge_requests"`
	EdgeRespBodyBytes           float64 `json:"edge_resp_body_bytes"`
	EdgeRespHeaderBytes         float64 `json:"edge_resp_header_bytes"`
	BotChallengesIssued         float64 `json:"bot_challenges_issued"`
	BotChallengesSucceeded      float64 `json:"bot_challenges_succeeded"`
	BotChallengesFailed         float64 `json:"bot_challenges_failed"`
	DDOSActionBlackhole         float64 `json:"ddos_action_blackhole"`
	DDOSActionClose             float64 `json:"ddos_action_close"`
	DDOSActionTarpit            float64 `json:"ddos_action_tarpit"`
}

// statsResponse is the envelope returned by /stats/service/{id}.
type statsResponse struct {
	Status string      `json:"status"`
	Data   []StatsData `json:"data"`
}

// GetStats fetches historical stats for serviceID for the window [from, to].
// Use by=minute for per-minute granularity.
func (c *Client) GetStats(ctx context.Context, serviceID string, from, to time.Time) ([]StatsData, error) {
	params := url.Values{}
	params.Set("by", "minute")
	params.Set("from", strconv.FormatInt(from.Unix(), 10))
	params.Set("to", strconv.FormatInt(to.Unix(), 10))

	u := fmt.Sprintf("%s/stats/service/%s?%s", c.baseURL, url.PathEscape(serviceID), params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Fastly-Key", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 != 2 {
		var apiErr struct {
			Msg string `json:"msg"`
		}
		json.Unmarshal(body, &apiErr)
		return nil, &APIError{Code: resp.StatusCode, Msg: apiErr.Msg}
	}

	var sr statsResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decoding stats for service %s: %w", serviceID, err)
	}
	return sr.Data, nil
}
