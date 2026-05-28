// Package api provides a minimal client for the Fastly historical stats API.
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

// Error represents a non-2xx response from the Fastly API.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string {
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

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the default Fastly API base URL.
// Primarily useful in tests and when targeting non-standard environments.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// NewClient returns a Client with a 30s timeout.
func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
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
			return nil, &Error{Code: resp.StatusCode, Msg: apiErr.Msg}
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

	// Next-Gen WAF
	NGWAFBotAnalysisRequestCount float64 `json:"ngwaf_bot_analysis_request_count"`
	NGWAFRequestsAllowedCount    float64 `json:"ngwaf_requests_allowed_count"`
	NGWAFRequestsBlockedCount    float64 `json:"ngwaf_requests_blocked_count"`
	NGWAFRequestsChallengedCount float64 `json:"ngwaf_requests_challenged_count"`
	NGWAFRequestsLoggedCount     float64 `json:"ngwaf_requests_logged_count"`
	NGWAFRequestsTimeoutCount    float64 `json:"ngwaf_requests_timeout_count"`
	NGWAFRequestsTotalCount      float64 `json:"ngwaf_requests_total_count"`
	NGWAFRequestsUnknownCount    float64 `json:"ngwaf_requests_unknown_count"`

	// AI Accelerator
	AIAEstimatedTimeSavedMs float64 `json:"aia_estimated_time_saved_ms"`
	AIAOriginUsageTokens    float64 `json:"aia_origin_usage_tokens"`
	AIARequests             float64 `json:"aia_requests"`
	AIAResponseUsageTokens  float64 `json:"aia_response_usage_tokens"`
	AIAStatus1xx            float64 `json:"aia_status_1xx"`
	AIAStatus2xx            float64 `json:"aia_status_2xx"`
	AIAStatus3xx            float64 `json:"aia_status_3xx"`
	AIAStatus4xx            float64 `json:"aia_status_4xx"`
	AIAStatus5xx            float64 `json:"aia_status_5xx"`

	// All-sources aggregates
	AllEdgeHitRequests  float64 `json:"all_edge_hit_requests"`
	AllEdgeMissRequests float64 `json:"all_edge_miss_requests"`
	AllErrorRequests    float64 `json:"all_error_requests"`
	AllHitRequests      float64 `json:"all_hit_requests"`
	AllMissRequests     float64 `json:"all_miss_requests"`
	AllPassRequests     float64 `json:"all_pass_requests"`
	AllStatus1xx        float64 `json:"all_status_1xx"`
	AllStatus2xx        float64 `json:"all_status_2xx"`
	AllStatus3xx        float64 `json:"all_status_3xx"`
	AllStatus4xx        float64 `json:"all_status_4xx"`
	AllStatus5xx        float64 `json:"all_status_5xx"`
	AllSynthRequests    float64 `json:"all_synth_requests"`

	// API Discovery
	APIDiscoveryRequestsCount float64 `json:"api_discovery_requests_count"`

	// Attack (extended)
	AttackBlockedReqBodyBytes   float64 `json:"attack_blocked_req_body_bytes"`
	AttackBlockedReqHeaderBytes float64 `json:"attack_blocked_req_header_bytes"`
	AttackLoggedReqBodyBytes    float64 `json:"attack_logged_req_body_bytes"`
	AttackLoggedReqHeaderBytes  float64 `json:"attack_logged_req_header_bytes"`
	AttackPassedReqBodyBytes    float64 `json:"attack_passed_req_body_bytes"`
	AttackPassedReqHeaderBytes  float64 `json:"attack_passed_req_header_bytes"`

	// Bandwidth aliases
	BodySize   float64 `json:"body_size"`
	HeaderSize float64 `json:"header_size"`

	// Bot challenge tokens
	BotChallengeCompleteTokensChecked  float64 `json:"bot_challenge_complete_tokens_checked"`
	BotChallengeCompleteTokensDisabled float64 `json:"bot_challenge_complete_tokens_disabled"`
	BotChallengeCompleteTokensFailed   float64 `json:"bot_challenge_complete_tokens_failed"`
	BotChallengeCompleteTokensIssued   float64 `json:"bot_challenge_complete_tokens_issued"`
	BotChallengeCompleteTokensPassed   float64 `json:"bot_challenge_complete_tokens_passed"`
	BotChallengeStarts                 float64 `json:"bot_challenge_starts"`

	// Bot edge requests by category
	BotEdgeRequestsAccessibilityCount            float64 `json:"bot_edge_requests_accessibility_count"`
	BotEdgeRequestsAICrawlerCount                float64 `json:"bot_edge_requests_ai_crawler_count"`
	BotEdgeRequestsAIFetcherCount                float64 `json:"bot_edge_requests_ai_fetcher_count"`
	BotEdgeRequestsAnalyzedCount                 float64 `json:"bot_edge_requests_analyzed_count"`
	BotEdgeRequestsContentFetcherCount           float64 `json:"bot_edge_requests_content_fetcher_count"`
	BotEdgeRequestsDetectedCount                 float64 `json:"bot_edge_requests_detected_count"`
	BotEdgeRequestsMonitoringCount               float64 `json:"bot_edge_requests_monitoring_count"`
	BotEdgeRequestsOnlineMarketingCount          float64 `json:"bot_edge_requests_online_marketing_count"`
	BotEdgeRequestsPagePreviewCount              float64 `json:"bot_edge_requests_page_preview_count"`
	BotEdgeRequestsPlatformIntegrationsCount     float64 `json:"bot_edge_requests_platform_integrations_count"`
	BotEdgeRequestsResearchCount                 float64 `json:"bot_edge_requests_research_count"`
	BotEdgeRequestsSearchEngineCrawlerCount      float64 `json:"bot_edge_requests_search_engine_crawler_count"`
	BotEdgeRequestsSearchEngineOptimizationCount float64 `json:"bot_edge_requests_search_engine_optimization_count"`
	BotEdgeRequestsSecurityToolsCount            float64 `json:"bot_edge_requests_security_tools_count"`
	BotEdgeRequestsVerifiedCount                 float64 `json:"bot_edge_requests_verified_count"`
	BotRequestsTotalCount                        float64 `json:"bot_requests_total_count"`

	// Compute (extended)
	ComputeBereqBodyBytes              float64 `json:"compute_bereq_body_bytes"`
	ComputeBereqErrors                 float64 `json:"compute_bereq_errors"`
	ComputeBereqHeaderBytes            float64 `json:"compute_bereq_header_bytes"`
	ComputeBereqs                      float64 `json:"compute_bereqs"`
	ComputeBerespBodyBytes             float64 `json:"compute_beresp_body_bytes"`
	ComputeBerespHeaderBytes           float64 `json:"compute_beresp_header_bytes"`
	ComputeCacheOperationsCount        float64 `json:"compute_cache_operations_count"`
	ComputeGlobalsLimitExceeded        float64 `json:"compute_globals_limit_exceeded"`
	ComputeGuestErrors                 float64 `json:"compute_guest_errors"`
	ComputeHandoff                     float64 `json:"compute_handoff"`
	ComputeHeapLimitExceeded           float64 `json:"compute_heap_limit_exceeded"`
	ComputePlatformInternalError       float64 `json:"compute_platform_internal_error"`
	ComputePlatformInvalidRequestError float64 `json:"compute_platform_invalid_request_error"`
	ComputeReqBodyBytes                float64 `json:"compute_req_body_bytes"`
	ComputeReqHeaderBytes              float64 `json:"compute_req_header_bytes"`
	ComputeRequestTimeBilledMs         float64 `json:"compute_request_time_billed_ms"`
	ComputeRequestTimeMs               float64 `json:"compute_request_time_ms"`
	ComputeResourceLimitExceeded       float64 `json:"compute_resource_limit_exceeded"`
	ComputeRespBodyBytes               float64 `json:"compute_resp_body_bytes"`
	ComputeRespHeaderBytes             float64 `json:"compute_resp_header_bytes"`
	ComputeRespStatus103               float64 `json:"compute_resp_status_103"`
	ComputeRespStatus1xx               float64 `json:"compute_resp_status_1xx"`
	ComputeRespStatus200               float64 `json:"compute_resp_status_200"`
	ComputeRespStatus204               float64 `json:"compute_resp_status_204"`
	ComputeRespStatus206               float64 `json:"compute_resp_status_206"`
	ComputeRespStatus2xx               float64 `json:"compute_resp_status_2xx"`
	ComputeRespStatus301               float64 `json:"compute_resp_status_301"`
	ComputeRespStatus302               float64 `json:"compute_resp_status_302"`
	ComputeRespStatus304               float64 `json:"compute_resp_status_304"`
	ComputeRespStatus3xx               float64 `json:"compute_resp_status_3xx"`
	ComputeRespStatus400               float64 `json:"compute_resp_status_400"`
	ComputeRespStatus401               float64 `json:"compute_resp_status_401"`
	ComputeRespStatus403               float64 `json:"compute_resp_status_403"`
	ComputeRespStatus404               float64 `json:"compute_resp_status_404"`
	ComputeRespStatus416               float64 `json:"compute_resp_status_416"`
	ComputeRespStatus429               float64 `json:"compute_resp_status_429"`
	ComputeRespStatus4xx               float64 `json:"compute_resp_status_4xx"`
	ComputeRespStatus500               float64 `json:"compute_resp_status_500"`
	ComputeRespStatus501               float64 `json:"compute_resp_status_501"`
	ComputeRespStatus502               float64 `json:"compute_resp_status_502"`
	ComputeRespStatus503               float64 `json:"compute_resp_status_503"`
	ComputeRespStatus504               float64 `json:"compute_resp_status_504"`
	ComputeRespStatus505               float64 `json:"compute_resp_status_505"`
	ComputeRespStatus530               float64 `json:"compute_resp_status_530"`
	ComputeRespStatus5xx               float64 `json:"compute_resp_status_5xx"`
	ComputeRuntimeErrors               float64 `json:"compute_runtime_errors"`
	ComputeSandboxes                   float64 `json:"compute_sandboxes"`
	ComputeServiceBereqError           float64 `json:"compute_service_bereq_error"`
	ComputeServiceChainError           float64 `json:"compute_service_chain_error"`
	ComputeServiceLimitsError          float64 `json:"compute_service_limits_error"`
	ComputeServiceMemoryExceededError  float64 `json:"compute_service_memory_exceeded_error"`
	ComputeServiceResourceLimitsError  float64 `json:"compute_service_resource_limits_error"`
	ComputeServiceRuntimeError         float64 `json:"compute_service_runtime_error"`
	ComputeServiceTimeoutError         float64 `json:"compute_service_timeout_error"`
	ComputeServiceVcpuExceededError    float64 `json:"compute_service_vcpu_exceeded_error"`
	ComputeStackLimitExceeded          float64 `json:"compute_stack_limit_exceeded"`

	// DDoS (extended)
	DDOSActionDowngrade                 float64 `json:"ddos_action_downgrade"`
	DDOSActionDowngradedConnections     float64 `json:"ddos_action_downgraded_connections"`
	DDOSActionLimitStreamsConnections   float64 `json:"ddos_action_limit_streams_connections"`
	DDOSActionLimitStreamsRequests      float64 `json:"ddos_action_limit_streams_requests"`
	DDOSActionTarpitAccept              float64 `json:"ddos_action_tarpit_accept"`
	DDOSProtectionRequestsAllowCount    float64 `json:"ddos_protection_requests_allow_count"`
	DDOSProtectionRequestsDetectCount   float64 `json:"ddos_protection_requests_detect_count"`
	DDOSProtectionRequestsMitigateCount float64 `json:"ddos_protection_requests_mitigate_count"`

	// DNS
	DNSBillableResponsesCount    float64 `json:"dns_billable_responses_count"`
	DNSNonbillableResponsesCount float64 `json:"dns_nonbillable_responses_count"`

	// Edge (extended)
	EdgeHitRequests         float64 `json:"edge_hit_requests"`
	EdgeHitRespBodyBytes    float64 `json:"edge_hit_resp_body_bytes"`
	EdgeHitRespHeaderBytes  float64 `json:"edge_hit_resp_header_bytes"`
	EdgeMissRequests        float64 `json:"edge_miss_requests"`
	EdgeMissRespBodyBytes   float64 `json:"edge_miss_resp_body_bytes"`
	EdgeMissRespHeaderBytes float64 `json:"edge_miss_resp_header_bytes"`

	// Fanout
	FanoutBereqBodyBytes    float64 `json:"fanout_bereq_body_bytes"`
	FanoutBereqHeaderBytes  float64 `json:"fanout_bereq_header_bytes"`
	FanoutBerespBodyBytes   float64 `json:"fanout_beresp_body_bytes"`
	FanoutBerespHeaderBytes float64 `json:"fanout_beresp_header_bytes"`
	FanoutConnTimeMs        float64 `json:"fanout_conn_time_ms"`
	FanoutRecvPublishes     float64 `json:"fanout_recv_publishes"`
	FanoutReqBodyBytes      float64 `json:"fanout_req_body_bytes"`
	FanoutReqHeaderBytes    float64 `json:"fanout_req_header_bytes"`
	FanoutRespBodyBytes     float64 `json:"fanout_resp_body_bytes"`
	FanoutRespHeaderBytes   float64 `json:"fanout_resp_header_bytes"`
	FanoutSendPublishes     float64 `json:"fanout_send_publishes"`

	// Image Optimizer (extended)
	ImgoptoAvifCount             float64 `json:"imgopto_avif_count"`
	ImgoptoComputeRequests       float64 `json:"imgopto_compute_requests"`
	ImgoptoGifCount              float64 `json:"imgopto_gif_count"`
	ImgoptoJpegCount             float64 `json:"imgopto_jpeg_count"`
	ImgoptoJpegxlCount           float64 `json:"imgopto_jpegxl_count"`
	ImgoptoMp4Count              float64 `json:"imgopto_mp4_count"`
	ImgoptoPngCount              float64 `json:"imgopto_png_count"`
	ImgoptoShield                float64 `json:"imgopto_shield"`
	ImgoptoShieldRespBodyBytes   float64 `json:"imgopto_shield_resp_body_bytes"`
	ImgoptoShieldRespHeaderBytes float64 `json:"imgopto_shield_resp_header_bytes"`
	ImgoptoSvgCount              float64 `json:"imgopto_svg_count"`
	ImgoptoWebpCount             float64 `json:"imgopto_webp_count"`

	// Image Video
	Imgvideo                      float64 `json:"imgvideo"`
	ImgvideoFrames                float64 `json:"imgvideo_frames"`
	ImgvideoRespBodyBytes         float64 `json:"imgvideo_resp_body_bytes"`
	ImgvideoRespHeaderBytes       float64 `json:"imgvideo_resp_header_bytes"`
	ImgvideoShield                float64 `json:"imgvideo_shield"`
	ImgvideoShieldFrames          float64 `json:"imgvideo_shield_frames"`
	ImgvideoShieldRespBodyBytes   float64 `json:"imgvideo_shield_resp_body_bytes"`
	ImgvideoShieldRespHeaderBytes float64 `json:"imgvideo_shield_resp_header_bytes"`

	// KV Store
	KVStoreClassAOperations float64 `json:"kv_store_class_a_operations"`
	KVStoreClassBOperations float64 `json:"kv_store_class_b_operations"`

	// Response body bytes by type
	HitRespBodyBytes  float64 `json:"hit_resp_body_bytes"`
	MissRespBodyBytes float64 `json:"miss_resp_body_bytes"`
	PassRespBodyBytes float64 `json:"pass_resp_body_bytes"`
	PassTime          float64 `json:"pass_time"`

	// Object size buckets
	ObjectSize1k   float64 `json:"object_size_1k"`
	ObjectSize10k  float64 `json:"object_size_10k"`
	ObjectSize100k float64 `json:"object_size_100k"`
	ObjectSize1m   float64 `json:"object_size_1m"`
	ObjectSize10m  float64 `json:"object_size_10m"`
	ObjectSize100m float64 `json:"object_size_100m"`
	ObjectSize1g   float64 `json:"object_size_1g"`

	// Object Storage
	ObjectStorageClassAOperationsCount float64 `json:"object_storage_class_a_operations_count"`
	ObjectStorageClassBOperationsCount float64 `json:"object_storage_class_b_operations_count"`
	ObjectStoreClassAOperations        float64 `json:"object_store_class_a_operations"`
	ObjectStoreClassBOperations        float64 `json:"object_store_class_b_operations"`

	// Origin (extended)
	OriginCacheFetchRespBodyBytes   float64 `json:"origin_cache_fetch_resp_body_bytes"`
	OriginCacheFetchRespHeaderBytes float64 `json:"origin_cache_fetch_resp_header_bytes"`
	OriginCacheFetches              float64 `json:"origin_cache_fetches"`
	OriginFetchBodyBytes            float64 `json:"origin_fetch_body_bytes"`
	OriginFetchHeaderBytes          float64 `json:"origin_fetch_header_bytes"`
	OriginFetchRespBodyBytes        float64 `json:"origin_fetch_resp_body_bytes"`
	OriginFetchRespHeaderBytes      float64 `json:"origin_fetch_resp_header_bytes"`
	OriginFetches                   float64 `json:"origin_fetches"`
	OriginRevalidations             float64 `json:"origin_revalidations"`

	// OTFP (extended)
	OtfpManifests             float64 `json:"otfp_manifests"`
	OtfpShieldRespBodyBytes   float64 `json:"otfp_shield_resp_body_bytes"`
	OtfpShieldRespHeaderBytes float64 `json:"otfp_shield_resp_header_bytes"`
	OtfpShieldTime            float64 `json:"otfp_shield_time"`

	// Pipe
	Pipe float64 `json:"pipe"`

	// Request bytes
	ReqBodyBytes   float64 `json:"req_body_bytes"`
	ReqHeaderBytes float64 `json:"req_header_bytes"`

	// Request behavior
	RequestCollapseUnusableCount float64 `json:"request_collapse_unusable_count"`
	RequestCollapseUsableCount   float64 `json:"request_collapse_usable_count"`
	RequestDeniedGetHeadBody     float64 `json:"request_denied_get_head_body"`
	Restarts                     float64 `json:"restarts"`

	// Segblock
	SegblockOriginFetches float64 `json:"segblock_origin_fetches"`
	SegblockShieldFetches float64 `json:"segblock_shield_fetches"`

	// Shield (extended)
	ShieldCacheFetches         float64 `json:"shield_cache_fetches"`
	ShieldFetchBodyBytes       float64 `json:"shield_fetch_body_bytes"`
	ShieldFetchHeaderBytes     float64 `json:"shield_fetch_header_bytes"`
	ShieldFetchRespBodyBytes   float64 `json:"shield_fetch_resp_body_bytes"`
	ShieldFetchRespHeaderBytes float64 `json:"shield_fetch_resp_header_bytes"`
	ShieldFetches              float64 `json:"shield_fetches"`
	ShieldHitRequests          float64 `json:"shield_hit_requests"`
	ShieldHitRespBodyBytes     float64 `json:"shield_hit_resp_body_bytes"`
	ShieldHitRespHeaderBytes   float64 `json:"shield_hit_resp_header_bytes"`
	ShieldMissRequests         float64 `json:"shield_miss_requests"`
	ShieldMissRespBodyBytes    float64 `json:"shield_miss_resp_body_bytes"`
	ShieldMissRespHeaderBytes  float64 `json:"shield_miss_resp_header_bytes"`
	ShieldRevalidations        float64 `json:"shield_revalidations"`

	// Status codes (extended)
	Status204 float64 `json:"status_204"`
	Status206 float64 `json:"status_206"`
	Status406 float64 `json:"status_406"`
	Status429 float64 `json:"status_429"`
	Status500 float64 `json:"status_500"`
	Status501 float64 `json:"status_501"`
	Status502 float64 `json:"status_502"`
	Status504 float64 `json:"status_504"`
	Status505 float64 `json:"status_505"`
	Status530 float64 `json:"status_530"`

	// TLS (extended)
	TLSHandshakeSentBytes float64 `json:"tls_handshake_sent_bytes"`

	// Traffic (extended)
	Uncacheable float64 `json:"uncacheable"`
	Upgrade     float64 `json:"upgrade"`

	// WebSocket
	WebsocketBereqBodyBytes    float64 `json:"websocket_bereq_body_bytes"`
	WebsocketBereqHeaderBytes  float64 `json:"websocket_bereq_header_bytes"`
	WebsocketBerespBodyBytes   float64 `json:"websocket_beresp_body_bytes"`
	WebsocketBerespHeaderBytes float64 `json:"websocket_beresp_header_bytes"`
	WebsocketConnTimeMs        float64 `json:"websocket_conn_time_ms"`
	WebsocketReqBodyBytes      float64 `json:"websocket_req_body_bytes"`
	WebsocketReqHeaderBytes    float64 `json:"websocket_req_header_bytes"`
	WebsocketRespBodyBytes     float64 `json:"websocket_resp_body_bytes"`
	WebsocketRespHeaderBytes   float64 `json:"websocket_resp_header_bytes"`
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
		return nil, &Error{Code: resp.StatusCode, Msg: apiErr.Msg}
	}

	var sr statsResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("decoding stats for service %s: %w", serviceID, err)
	}
	return sr.Data, nil
}
