package weather

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTrip func(*http.Request) (*http.Response, error)

func TestWeatherEnvironment(t *testing.T) {
	for _, tc := range []struct {
		mode, key, want string
		enabled         bool
	}{
		{"", "", "noncommercial", true},
		{"", "test-key", "customer", true},
		{"off", "test-key", "off", false},
		{"customer", "", "customer", false},
		{"noncommercial", "", "noncommercial", true},
		{"invalid", "", "invalid", false},
	} {
		t.Run(tc.mode+"-"+tc.want, func(t *testing.T) {
			t.Setenv("WORKSERVICE_WEATHER_MODE", tc.mode)
			t.Setenv("WORKSERVICE_WEATHER_API_KEY", tc.key)
			c := FromEnv()
			if c.Mode != tc.want || c.Enabled() != tc.enabled {
				t.Fatalf("unexpected config: mode=%s enabled=%v", c.Mode, c.Enabled())
			}
		})
	}
}

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestCitiesDailyCacheAndForecast(t *testing.T) {
	s := New(Config{Mode: "noncommercial"})
	calls := 0
	s.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if r.URL.Host == "geocoding-api.open-meteo.com" {
			if q.Get("countryCode") != "BY" {
				t.Fatal("missing country filter")
			}
			return response(`{"results":[{"id":1,"name":"Минск","admin1":"Минск","latitude":53.9,"longitude":27.5667,"country_code":"BY"},{"id":2,"name":"Other","country_code":"US"}]}`), nil
		}
		if q.Get("timezone") != "Europe/Minsk" || q.Get("daily") == "" {
			t.Fatal("missing daily variables")
		}
		date := q.Get("start_date")
		return response(fmt.Sprintf(`{"daily":{"time":[%q],"temperature_2m_min":[4.5],"temperature_2m_max":[12.3],"weather_code":[61],"precipitation_sum":[2.1]}}`, date)), nil
	})
	cities, err := s.Cities(context.Background(), "Минск")
	if err != nil || len(cities) != 1 || cities[0].Name != "Минск" {
		t.Fatal(cities, err)
	}
	_, _ = s.Cities(context.Background(), "Минск")
	if calls != 1 {
		t.Fatal("city cache miss")
	}
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	for _, date := range []string{"2026-09-21", "2026-10-26", "2026-10-27"} {
		days, err := s.Days(context.Background(), "Минск", 53.9, 27.56, date, date, now)
		if err != nil {
			t.Fatal(err)
		}
		day, ok := days[date]
		if !ok || !strings.Contains(day.Text, "4.5") || !strings.Contains(day.Source, "не замер") {
			t.Fatal(days)
		}
		if date == "2026-10-27" && day.Kind != "forecast" {
			t.Fatal("today was treated as completed observation")
		}
	}
	_, err = s.Days(context.Background(), "Минск", 53, 27, "2026-10-28", "2026-10-28", now)
	if err == nil {
		t.Fatal("future weather accepted")
	}
}
func TestWeatherNullsAndSecretErrors(t *testing.T) {
	s := New(Config{Mode: "customer", Key: "TEST-SECRET"})
	s.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "customer-archive-api.open-meteo.com" || r.URL.Query().Get("apikey") != "TEST-SECRET" {
			t.Fatal("commercial endpoint")
		}
		return response(`{"daily":{"time":["2026-09-21"],"temperature_2m_min":[null],"temperature_2m_max":[null],"weather_code":[null]}}`), nil
	})
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	days, err := s.Days(context.Background(), "Минск", 53, 27, "2026-09-21", "2026-09-21", now)
	if err != nil || len(days) != 0 {
		t.Fatal("fabricated zero-degree weather", days, err)
	}
	s.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("upstream %s", r.URL.String()) })
	_, err = s.Days(context.Background(), "Минск", 54, 27, "2026-09-20", "2026-09-20", now)
	if err == nil || strings.Contains(err.Error(), "TEST-SECRET") {
		t.Fatal("upstream key exposed", err)
	}
	disabled := New(Config{Mode: "off"})
	if _, err = disabled.Cities(context.Background(), "Минск"); err == nil {
		t.Fatal("unconfigured provider enabled")
	}
}
