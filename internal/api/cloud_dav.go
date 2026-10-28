package api

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/webdav"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"project/internal/security"
	"project/internal/storage"
)

type davWindow struct {
	count int
	until time.Time
}
type CloudDAVService struct {
	origin   string
	proxies  []netip.Prefix
	mu       sync.Mutex
	attempts map[string]davWindow
	locks    map[string]webdav.LockSystem
	slots    chan struct{}
}

func NewCloudDAVService() *CloudDAVService {
	s := &CloudDAVService{attempts: map[string]davWindow{}, locks: map[string]webdav.LockSystem{}, slots: make(chan struct{}, 8)}
	raw := strings.TrimSpace(os.Getenv("WORKSERVICE_DAV_ORIGIN"))
	u, err := url.Parse(raw)
	if err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && (u.Path == "" || u.Path == "/") && u.RawQuery == "" && u.Fragment == "" {
		s.origin = "https://" + u.Host
	}
	for _, raw := range strings.Split(os.Getenv("WORKSERVICE_DAV_TRUSTED_PROXIES"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			ip, e := netip.ParseAddr(raw)
			if e != nil {
				s.origin = ""
				break
			}
			p = netip.PrefixFrom(ip, ip.BitLen())
		}
		if p.Bits() == 0 {
			s.origin = ""
			break
		}
		s.proxies = append(s.proxies, p)
	}
	return s
}

func (s *CloudDAVService) secure(r *http.Request) bool {
	if s.origin == "" || "https://"+r.Host != s.origin {
		return false
	}
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip, e := netip.ParseAddr(host)
	if err != nil || e != nil || r.Header.Get("X-Forwarded-Proto") != "https" {
		return false
	}
	for _, p := range s.proxies {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

// Bounded counters use the actual peer, never an untrusted forwarded IP.
func (s *CloudDAVService) allow(key string, limit int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.attempts {
		if now.After(v.until) {
			delete(s.attempts, k)
		}
	}
	v, exists := s.attempts[key]
	if !exists {
		if len(s.attempts) >= 4096 {
			return false
		}
		v.until = now.Add(time.Minute)
	}
	v.count++
	s.attempts[key] = v
	return v.count <= limit
}

type canonicalLocks struct{ webdav.LockSystem }

func davLockName(name string) string {
	return cases.Fold().String(norm.NFC.String(path.Clean("/" + name)))
}
func (l canonicalLocks) Create(now time.Time, d webdav.LockDetails) (string, error) {
	d.Root = davLockName(d.Root)
	d.Duration = 5 * time.Minute
	return l.LockSystem.Create(now, d)
}
func (l canonicalLocks) Confirm(now time.Time, a, b string, conditions ...webdav.Condition) (func(), error) {
	if a != "" {
		a = davLockName(a)
	}
	if b != "" {
		b = davLockName(b)
	}
	return l.LockSystem.Confirm(now, a, b, conditions...)
}
func (l canonicalLocks) Refresh(now time.Time, token string, _ time.Duration) (webdav.LockDetails, error) {
	return l.LockSystem.Refresh(now, token, 5*time.Minute)
}

func validDAVPath(p string) bool {
	if p != "/dav" && !strings.HasPrefix(p, "/dav/") {
		return false
	}
	if len(p) > 8192 || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return !strings.Contains(p, "//")
}

func (s *CloudDAVService) Serve(c *gin.Context) {
	r, w := c.Request, c.Writer
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; frame-ancestors 'none'")
	if !s.secure(r) {
		c.String(403, "WebDAV requires the configured HTTPS endpoint")
		return
	}
	if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		c.Status(403)
		return
	}
	if !validDAVPath(r.URL.Path) {
		c.Status(400)
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	username, password, ok := r.BasicAuth()
	if len(username) > 180 || len(password) > 128 {
		c.Status(401)
		return
	}
	if !s.allow("peer:"+host, 1200) {
		c.Header("Retry-After", "60")
		c.Status(429)
		return
	}
	device, err := storage.AuthenticateCloudDevice(username, password)
	if !ok || err != nil {
		if !s.allow("failed:"+host, 30) {
			c.Header("Retry-After", "60")
			c.Status(429)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="WorkService Cloud", charset="UTF-8"`)
		c.Status(401)
		return
	}
	read := r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" || r.Method == "PROPFIND"
	switch r.Method {
	case "GET", "HEAD", "OPTIONS", "PROPFIND", "PUT", "MKCOL", "DELETE", "COPY", "MOVE", "LOCK", "UNLOCK", "PROPPATCH":
	default:
		c.Status(405)
		return
	}
	if device.ReadOnly && !read {
		c.Status(403)
		return
	}
	if r.Method == "LOCK" && !s.allow("locks:"+device.OwnerID, 30) {
		c.Header("Retry-After", "60")
		c.Status(429)
		return
	}
	if r.Method == "PROPFIND" && r.Header.Get("Depth") != "0" && r.Header.Get("Depth") != "1" {
		c.Data(403, "application/xml; charset=utf-8", []byte(`<D:error xmlns:D="DAV:"><D:propfind-finite-depth/></D:error>`))
		return
	}
	if r.Method == "COPY" || r.Method == "MOVE" {
		dst, e := url.Parse(r.Header.Get("Destination"))
		if e != nil || dst.User != nil || dst.RawQuery != "" || dst.Fragment != "" || !validDAVPath(dst.Path) || (dst.Host != "" && (dst.Host != r.Host || dst.Scheme != "https")) {
			c.Status(400)
			return
		}
		srcName, dstName := davLockName(r.URL.Path), davLockName(dst.Path)
		if srcName == "/dav" || dstName == "/dav" || srcName == dstName || strings.HasPrefix(dstName, srcName+"/") || strings.HasPrefix(srcName, dstName+"/") {
			c.Status(403)
			return
		}
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		c.Header("Retry-After", "5")
		c.Status(503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(10 * time.Minute))
	defer controller.SetReadDeadline(time.Time{})
	limit := int64(1 << 20)
	if r.Method == "PUT" {
		limit = storage.CloudMaxFileBytes()
	}
	if r.ContentLength > limit {
		c.Status(413)
		return
	}
	r = r.WithContext(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	fs := storage.NewCloudDAV(device.OwnerID, !read)
	defer fs.Close()
	if !read {
		info, e := fs.Stat(ctx, strings.TrimPrefix(r.URL.Path, "/dav"))
		etag := ""
		if e == nil {
			if tagged, ok := info.(webdav.ETager); ok {
				etag, _ = tagged.ETag(ctx)
			}
		}
		if match := r.Header.Get("If-Match"); match != "" && !davETagMatches(match, etag) {
			c.Status(412)
			return
		}
		if match := r.Header.Get("If-None-Match"); match != "" && davETagMatches(match, etag) {
			c.Status(412)
			return
		}
	}
	s.mu.Lock()
	locks := s.locks[device.OwnerID]
	if locks == nil {
		locks = canonicalLocks{webdav.NewMemLS()}
		s.locks[device.OwnerID] = locks
	}
	s.mu.Unlock()
	handler := webdav.Handler{Prefix: "/dav", FileSystem: fs, LockSystem: locks}
	if read {
		if r.Method == "GET" || r.Method == "HEAD" {
			w.Header().Set("Content-Disposition", "attachment")
		}
		handler.ServeHTTP(w, r)
		return
	}
	// Do not send success before an entire upload/copy/move is durably committed.
	response := &davResponse{header: make(http.Header), status: 200}
	handler.ServeHTTP(response, r)
	if response.status >= 200 && response.status < 300 {
		if _, err = storage.AuthenticateCloudDevice(username, password); err != nil {
			c.Status(401)
			return
		}
		if err = fs.Commit(ctx); err != nil {
			cloudError(c, err)
			return
		}
		security.LogEvent("cloud_dav_write", fmt.Sprintf("owner=%s device=%s method=%s", device.OwnerID, device.ID, r.Method))
	}
	for key, values := range response.header {
		w.Header()[key] = values
	}
	c.Data(response.status, response.header.Get("Content-Type"), response.body.Bytes())
}

func davETagMatches(header, etag string) bool {
	if etag == "" {
		return false
	}
	for _, value := range strings.Split(header, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || value == etag {
			return true
		}
	}
	return false
}

type davResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *davResponse) Header() http.Header    { return w.header }
func (w *davResponse) WriteHeader(status int) { w.status = status }
func (w *davResponse) Write(p []byte) (int, error) {
	if w.body.Len()+len(p) > 1<<20 {
		return 0, fmt.Errorf("DAV response too large")
	}
	return w.body.Write(p)
}

func (s *CloudDAVService) Devices(c *gin.Context) {
	u, err := storage.GetUserByID(c.GetString("userID"))
	if err != nil {
		c.Status(403)
		return
	}
	c.JSON(200, gin.H{"devices": storage.ListCloudDevices(u.ID), "url": s.origin + "/dav/", "enabled": s.origin != "", "secure": s.secure(c.Request), "username": u.Username})
}
func (s *CloudDAVService) CreateDevice(c *gin.Context) {
	if !s.secure(c.Request) {
		c.JSON(403, gin.H{"error": "Подключения доступны только после настройки защищённого HTTPS-адреса на сервере"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input struct {
		Name     string `json:"name"`
		ReadOnly bool   `json:"readOnly"`
		Days     int    `json:"days"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(400)
		return
	}
	u, err := storage.GetUserByID(c.GetString("userID"))
	if err != nil || !s.allow("create:"+u.ID, 5) {
		c.Status(429)
		return
	}
	if _, err = storage.ValidateUser(u.Username, input.Password); err != nil {
		c.JSON(403, gin.H{"error": "Неверный текущий пароль аккаунта"})
		return
	}
	device, password, err := storage.CreateCloudDevice(u.ID, input.Name, input.ReadOnly, input.Days)
	if err != nil {
		cloudError(c, err)
		return
	}
	security.LogEvent("cloud_device_created", "owner="+u.ID+" device="+device.ID)
	c.JSON(201, gin.H{"device": device, "password": password})
}
func (s *CloudDAVService) RevokeDevice(c *gin.Context) {
	owner := c.GetString("userID")
	if err := storage.RevokeCloudDevice(owner, c.Param("id")); err != nil {
		cloudError(c, err)
		return
	}
	security.LogEvent("cloud_device_revoked", "owner="+owner+" device="+c.Param("id"))
	c.Status(204)
}
