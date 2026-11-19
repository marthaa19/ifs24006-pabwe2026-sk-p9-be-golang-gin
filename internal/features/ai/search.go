package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Searcher menelusuri fakta terkini di mesin pencari.
type Searcher interface {
	Search(query string) ([]Source, error)
}

// HTTPSearcher memanggil SEARCH_ENGINE_ENDPOINT?q=<kata kunci>.
type HTTPSearcher struct {
	Endpoint string
	Client   *http.Client
}

// NewHTTPSearcher membuat HTTPSearcher dengan timeout 10 detik.
func NewHTTPSearcher(endpoint string) *HTTPSearcher {
	return &HTTPSearcher{Endpoint: endpoint, Client: &http.Client{Timeout: 10 * time.Second}}
}

// Search mengembalikan maksimal 5 sumber. Format balasan yang diterima:
// array langsung, {"data":[...]}, atau {"results":[...]}.
func (s *HTTPSearcher) Search(query string) ([]Source, error) {
	if s.Endpoint == "" {
		return nil, fmt.Errorf("endpoint mesin pencari belum dikonfigurasi")
	}
	req, err := http.NewRequest(http.MethodGet, s.Endpoint+"?q="+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mesin pencari membalas status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseSources(body), nil
}

func parseSources(body []byte) []Source {
	var list []Source
	if json.Unmarshal(body, &list) != nil || len(list) == 0 {
		var wrap struct {
			Data    []Source `json:"data"`
			Results []Source `json:"results"`
		}
		_ = json.Unmarshal(body, &wrap)
		list = wrap.Data
		if len(list) == 0 {
			list = wrap.Results
		}
	}
	if len(list) > 5 {
		list = list[:5]
	}
	return list
}
