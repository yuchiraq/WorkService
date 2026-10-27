// Package weather retrieves explicitly attributed, modelled daily weather. It
// never treats a forecast as an observation at the construction site.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type City struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Region    string  `json:"region"`
	Label     string  `json:"label"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
type Day struct {
	Date   string `json:"date"`
	Text   string `json:"text"`
	Source string `json:"source"`
	Kind   string `json:"kind"`
}
type Config struct{ Key, Mode string }

func FromEnv() Config {
	c := Config{Key: strings.TrimSpace(os.Getenv("WORKSERVICE_WEATHER_API_KEY")), Mode: strings.TrimSpace(os.Getenv("WORKSERVICE_WEATHER_MODE"))}
	if c.Mode == "" {
		if c.Key != "" {
			c.Mode = "customer"
		} else {
			c.Mode = "noncommercial"
		}
	}
	return c
}
func (c Config) Enabled() bool {
	return c.Mode == "noncommercial" || (c.Mode == "customer" && c.Key != "")
}
func (c Config) Reason() string {
	if c.Enabled() {
		return ""
	}
	if c.Mode == "off" {
		return "Автоматическая погода отключена (WORKSERVICE_WEATHER_MODE=off). Поле можно заполнить вручную"
	}
	return "Проверьте WORKSERVICE_WEATHER_MODE: noncommercial, customer или off. Для customer требуется WORKSERVICE_WEATHER_API_KEY"
}

type cached struct {
	at   time.Time
	body []byte
}
type Service struct {
	config Config
	client *http.Client
	mu     sync.Mutex
	cache  map[string]cached
	slots  chan struct{}
}

func New(c Config) *Service {
	return &Service{config: c, client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirect disabled") }}, cache: map[string]cached{}, slots: make(chan struct{}, 4)}
}

func (s *Service) fetch(ctx context.Context, host, path string, query url.Values, target any) error {
	if !s.config.Enabled() {
		return fmt.Errorf("%s", s.config.Reason())
	}
	if s.config.Mode == "customer" {
		host = "customer-" + host
		query.Set("apikey", s.config.Key)
	}
	u := url.URL{Scheme: "https", Host: host, Path: path, RawQuery: query.Encode()}
	key := u.String()
	s.mu.Lock()
	entry, ok := s.cache[key]
	s.mu.Unlock()
	if ok && time.Since(entry.at) < time.Hour {
		return json.Unmarshal(entry.body, target)
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return fmt.Errorf("Получение погоды прервано")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("Не удалось подготовить запрос погоды")
	}
	req.Header.Set("User-Agent", "WorkService/1.0 (object-document-weather)")
	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Сервис погоды не ответил. Запись можно сохранить без автоматической погоды")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Сервис погоды вернул ошибку (%d). Проверьте ключ, лимиты и доступность данных", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if err != nil || len(body) > 2*1024*1024 {
		return fmt.Errorf("Слишком большой или неполный ответ сервиса погоды")
	}
	if err = json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("Не удалось прочитать данные сервиса погоды")
	}
	s.mu.Lock()
	if len(s.cache) >= 128 {
		oldest := ""
		var at time.Time
		for k, v := range s.cache {
			if oldest == "" || v.at.Before(at) {
				oldest, at = k, v.at
			}
		}
		delete(s.cache, oldest)
	}
	s.cache[key] = cached{time.Now(), body}
	s.mu.Unlock()
	return nil
}

func (s *Service) Cities(ctx context.Context, name string) ([]City, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		return []City{}, nil
	}
	var data struct {
		Results []struct {
			ID                   int64 `json:"id"`
			Name, Admin1, Admin2 string
			CountryCode          string `json:"country_code"`
			Latitude, Longitude  float64
		} `json:"results"`
	}
	if err := s.fetch(ctx, "geocoding-api.open-meteo.com", "/v1/search", url.Values{"name": {name}, "count": {"8"}, "language": {"ru"}, "countryCode": {"BY"}}, &data); err != nil {
		return nil, err
	}
	result := []City{}
	for _, r := range data.Results {
		if !strings.EqualFold(r.CountryCode, "BY") || !validCoordinates(r.Latitude, r.Longitude) {
			continue
		}
		region := r.Admin1
		if r.Admin2 != "" && r.Admin2 != r.Admin1 {
			region += " / " + r.Admin2
		}
		label := r.Name
		if region != "" {
			label += ", " + region
		}
		result = append(result, City{r.ID, r.Name, region, label, r.Latitude, r.Longitude})
	}
	return result, nil
}

func validCoordinates(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
func (s *Service) Days(ctx context.Context, city string, lat, lon float64, from, to string, now time.Time) (map[string]Day, error) {
	if !validCoordinates(lat, lon) {
		return nil, fmt.Errorf("Выберите город из списка")
	}
	start, e := time.Parse("2006-01-02", from)
	end, e2 := time.Parse("2006-01-02", to)
	todayText := now.In(time.FixedZone("Minsk", 3*3600)).Format("2006-01-02")
	today, _ := time.Parse("2006-01-02", todayText)
	if e != nil || e2 != nil || start.After(end) || start.Year() < 1940 || end.Sub(start) > 366*24*time.Hour || end.After(today) {
		return nil, fmt.Errorf("Погода доступна с 1940 года, не позднее сегодняшнего дня; период — до года")
	}
	cut := today.AddDate(0, 0, -5)
	result := map[string]Day{}
	type span struct {
		from, to         time.Time
		host, path, kind string
	}
	spans := []span{}
	if start.Before(cut) {
		last := end
		if !last.Before(cut) {
			last = cut.AddDate(0, 0, -1)
		}
		spans = append(spans, span{start, last, "archive-api.open-meteo.com", "/v1/archive", "reanalysis"})
	}
	if !end.Before(cut) {
		first := start
		if first.Before(cut) {
			first = cut
		}
		spans = append(spans, span{first, end, "api.open-meteo.com", "/v1/forecast", "recent-model"})
	}
	for _, period := range spans {
		var data struct {
			Daily struct {
				Time          []string   `json:"time"`
				Min           []*float64 `json:"temperature_2m_min"`
				Max           []*float64 `json:"temperature_2m_max"`
				Code          []*int     `json:"weather_code"`
				Precipitation []*float64 `json:"precipitation_sum"`
			} `json:"daily"`
		}
		query := url.Values{"latitude": {strconv.FormatFloat(lat, 'f', 6, 64)}, "longitude": {strconv.FormatFloat(lon, 'f', 6, 64)}, "start_date": {period.from.Format("2006-01-02")}, "end_date": {period.to.Format("2006-01-02")}, "daily": {"temperature_2m_min,temperature_2m_max,weather_code,precipitation_sum"}, "timezone": {"Europe/Minsk"}, "temperature_unit": {"celsius"}, "precipitation_unit": {"mm"}}
		if err := s.fetch(ctx, period.host, period.path, query, &data); err != nil {
			return nil, err
		}
		d := data.Daily
		for i, date := range d.Time {
			if date < from || date > to || i >= len(d.Min) || i >= len(d.Max) || i >= len(d.Code) || d.Min[i] == nil || d.Max[i] == nil || d.Code[i] == nil {
				continue
			}
			if *d.Min[i] < -100 || *d.Max[i] > 70 || *d.Min[i] > *d.Max[i] {
				continue
			}
			text := fmt.Sprintf("%.1f…%.1f °C; %s (за сутки)", *d.Min[i], *d.Max[i], weatherLabel(*d.Code[i]))
			kind := period.kind
			if i < len(d.Precipitation) && d.Precipitation[i] != nil && *d.Precipitation[i] >= 0 {
				text += fmt.Sprintf("; осадки %.1f мм", *d.Precipitation[i])
			}
			if date == todayText {
				text = "Предварительный прогноз: " + text
				kind = "forecast"
			}
			result[date] = Day{Date: date, Text: text, Kind: kind, Source: "Open-Meteo · " + city + " · " + date + " · модельные данные, не замер на объекте"}
		}
	}
	return result, nil
}
func weatherLabel(code int) string {
	switch {
	case code == 0:
		return "ясно"
	case code >= 1 && code <= 3:
		return "облачно"
	case code == 45 || code == 48:
		return "туман"
	case code >= 51 && code <= 57:
		return "морось"
	case code >= 61 && code <= 67:
		return "дождь"
	case code >= 71 && code <= 77:
		return "снег"
	case code >= 80 && code <= 82:
		return "ливень"
	case code == 85 || code == 86:
		return "снегопад"
	case code >= 95 && code <= 99:
		return "гроза"
	}
	return "погодный код " + strconv.Itoa(code)
}
