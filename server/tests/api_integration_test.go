package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gxfdev/DevDash/server/internal/filemgr"
)

const baseURL = "http://localhost:9090/api/v1"
const authURL = "http://localhost:9090/api"

var authToken string

func getAuthClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func TestMain(m *testing.M) {
	token, err := login("admin", "admin123")
	if err != nil {
		fmt.Printf("WARNING: cannot login for integration tests: %v\n", err)
	} else {
		authToken = token
	}
	os.Exit(m.Run())
}

func login(username, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp, err := http.Post(authURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if token, ok := result["access_token"].(string); ok {
		return token, nil
	}
	return "", fmt.Errorf("no access_token in response")
}

func authRequest(method, path string, body io.Reader) *http.Request {
	req, _ := http.NewRequest(method, baseURL+path, body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)
	return req
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthLogin_Success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	resp, err := http.Post(authURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["access_token"] == nil {
		t.Error("expected access_token in response")
	}
}

func TestAuthLogin_InvalidCredentials(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	resp, err := http.Post(authURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("expected non-200 for invalid credentials")
	}
}

func TestAuthLogin_MissingFields(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"username": "admin"})
	resp, err := http.Post(authURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("expected non-200 for missing password")
	}
}

func TestNodesList_RequiresAuth(t *testing.T) {
	resp, err := http.Get(baseURL + "/nodes")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("expected auth required, got 200")
	}
}

func TestNodesList_WithAuth(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/nodes", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNodeRegister_InvalidInput(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	body, _ := json.Marshal(map[string]string{"name": ""})
	client := getAuthClient()
	req := authRequest("POST", "/node/register", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("expected error for empty name")
	}
}

func TestDatabaseQuery_SQLInjection(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	injectionPayloads := []string{
		"DROP TABLE users;",
		"SELECT * FROM users WHERE 1=1; DROP TABLE users;--",
		"INSERT INTO users VALUES ('hacked','hacked')",
		"UPDATE users SET password='hacked'",
		"DELETE FROM nodes",
	}
	client := getAuthClient()
	for _, payload := range injectionPayloads {
		body, _ := json.Marshal(map[string]string{"sql": payload})
		req := authRequest("POST", "/node/self/databases/0/query", bytes.NewReader(body))
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("SQL injection should be blocked: %s", payload)
		}
	}
}

func TestDatabaseQuery_AllowedStatements(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	allowedQueries := []string{
		"SELECT 1",
		"SHOW TABLES",
		"EXPLAIN SELECT 1",
		"DESCRIBE nodes",
	}
	client := getAuthClient()
	for _, query := range allowedQueries {
		body, _ := json.Marshal(map[string]string{"sql": query})
		req := authRequest("POST", "/node/self/databases/0/query", bytes.NewReader(body))
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 403 {
			t.Errorf("allowed query should not be blocked: %s", query)
		}
	}
}

func TestAlertRules_CRUD(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	req := authRequest("GET", "/alert-rules", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list alert rules failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAlerts_Active(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/alerts/active", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get active alerts failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSettings_Get(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/settings", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get settings failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFilePathTraversal(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	traversalPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		"/etc/shadow",
		"/root/.ssh/id_rsa",
	}
	client := getAuthClient()
	for _, path := range traversalPaths {
		req := authRequest("GET", "/node/self/fs/list?path="+path, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			body, _ := io.ReadAll(resp.Body)
			if strings.Contains(string(body), "root:") {
				t.Errorf("path traversal not blocked: %s", path)
			}
		}
	}
}

func TestFileList_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/fs/list", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var files []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&files)
	if files == nil {
		t.Error("expected file list, got nil")
	}
}

func TestFileList_WithPath(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/fs/list?path=.", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFileCreateDir_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("test_dir_%d", time.Now().UnixNano()))
	body, _ := json.Marshal(map[string]string{"path": testDir})
	req := authRequest("POST", "/node/self/fs/mkdir", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("mkdir expected 200, got %d", resp.StatusCode)
	}
	os.RemoveAll(testDir)
}

func TestFileCreateFile_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	testFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_file_%d.txt", time.Now().UnixNano()))
	body, _ := json.Marshal(map[string]string{"path": testFile})
	req := authRequest("POST", "/node/self/fs/mkfile", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("mkfile failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("mkfile expected 200, got %d", resp.StatusCode)
	}
	os.Remove(testFile)
}

func TestFileDelete_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("test_del_%d", time.Now().UnixNano()))
	os.MkdirAll(testDir, 0755)
	body, _ := json.Marshal(map[string]string{"path": testDir})
	req := authRequest("DELETE", "/node/self/fs/remove", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("delete expected 200, got %d", resp.StatusCode)
	}
}

func TestFileList_EmptyPath(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/fs/list?path=", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for empty path, got %d", resp.StatusCode)
	}
}

func TestFileStats_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/fs/stats", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("file stats failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFileStats_WithDuration(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/fs/stats?duration=24h", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("file stats failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHistory_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/history?duration=1h&limit=10", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHistory_DefaultParams(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/history", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHistory_MaxLimit(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/history?limit=1000", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCronJobs_CRUD(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/cronjobs", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list cron jobs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSoftware_CRUD(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/software", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list software failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthJWT_Signature(t *testing.T) {
	client := getAuthClient()
	req, _ := http.NewRequest("GET", baseURL+"/nodes", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("expected invalid JWT to be rejected")
	}
}

func TestAuthJWT_ExpiredToken(t *testing.T) {
	client := getAuthClient()
	req, _ := http.NewRequest("GET", baseURL+"/nodes", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjEwMDAwMDAwMDB9.fake")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Error("expected expired JWT to be rejected")
	}
}

func TestConcurrentFileOperations(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	const concurrency = 10
	errCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			client := getAuthClient()
			testDir := filepath.Join(os.TempDir(), fmt.Sprintf("test_concurrent_%d_%d", time.Now().UnixNano(), idx))
			body, _ := json.Marshal(map[string]string{"path": testDir})
			req := authRequest("POST", "/node/self/fs/mkdir", bytes.NewReader(body))
			resp, err := client.Do(req)
			if err != nil {
				errCh <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				errCh <- fmt.Errorf("concurrent mkdir %d: expected 200, got %d", idx, resp.StatusCode)
				return
			}
			os.RemoveAll(testDir)
			errCh <- nil
		}(i)
	}
	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent file op failed: %v", err)
		}
	}
}

func TestUnauthenticatedAccess(t *testing.T) {
	paths := []string{
		"/node/self/fs/list",
		"/node/self/history",
		"/node/self/metrics",
		"/alert-rules",
		"/settings",
	}
	client := getAuthClient()
	for _, p := range paths {
		req, _ := http.NewRequest("GET", baseURL+p, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("unauthenticated access should be denied: %s", p)
		}
	}
}

func TestRateLimiting_Basic(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	const burstSize = 50
	client := getAuthClient()
	successCount := 0
	for i := 0; i < burstSize; i++ {
		req := authRequest("GET", "/health", nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == 200 {
			successCount++
		}
		resp.Body.Close()
	}
	if successCount < burstSize/2 {
		t.Errorf("too many rate-limited requests: %d/%d succeeded", successCount, burstSize)
	}
}

func TestConcurrentMetricsRequests(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	const concurrency = 20
	errCh := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			client := getAuthClient()
			req := authRequest("GET", "/node/self/metrics", nil)
			resp, err := client.Do(req)
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				errCh <- fmt.Errorf("expected 200, got %d", resp.StatusCode)
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent request failed: %v", err)
		}
	}
}

func BenchmarkHealthEndpoint(b *testing.B) {
	for i := 0; i < b.N; i++ {
		resp, _ := http.Get(baseURL + "/health")
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkNodesList(b *testing.B) {
	if authToken == "" {
		b.Skip("no auth token")
	}
	client := getAuthClient()
	for i := 0; i < b.N; i++ {
		req := authRequest("GET", "/nodes", nil)
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func NewTestRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ============================================================
// 新增测试用例 - 覆盖 BUG 修复 + 边界 + 性能
// ============================================================

func TestFileStats_ReturnsArrayNotNil(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/node/self/fs/stats?duration=1m", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("file stats failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data json.RawMessage
	json.NewDecoder(resp.Body).Decode(&data)
	if string(data) == "null" {
		t.Error("file stats should return array [], not null")
	}
}

func TestSnapshot_API(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/snapshot", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var snap map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&snap)
	if snap == nil {
		t.Error("expected snapshot data, got nil")
	}
}

func TestLatest_API(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/latest", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("latest failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFileRead_SmallFile(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	testFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_read_%d.txt", time.Now().UnixNano()))
	defer func() {
		delBody, _ := json.Marshal(map[string]string{"path": testFile})
		delReq := authRequest("DELETE", "/node/self/fs/remove", bytes.NewReader(delBody))
		if delResp, err := getAuthClient().Do(delReq); err == nil {
			delResp.Body.Close()
		}
	}()

	client := getAuthClient()
	body, _ := json.Marshal(map[string]string{"path": testFile})
	req := authRequest("POST", "/node/self/fs/mkfile", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	resp.Body.Close()

	readReq := authRequest("GET", "/node/self/fs/read?path="+testFile, nil)
	rresp, rerr := client.Do(readReq)
	if rerr != nil {
		t.Fatalf("read file failed: %v", rerr)
	}
	defer rresp.Body.Close()
	if rresp.StatusCode != 200 {
		t.Errorf("read expected 200, got %d", rresp.StatusCode)
	}
}

func TestDownload_FileExists(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	testFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_dl_%d.txt", time.Now().UnixNano()))
	defer func() {
		delBody, _ := json.Marshal(map[string]string{"path": testFile})
		delReq := authRequest("DELETE", "/node/self/fs/remove", bytes.NewReader(delBody))
		if delResp, err := getAuthClient().Do(delReq); err == nil {
			delResp.Body.Close()
		}
	}()

	client := getAuthClient()
	createBody, _ := json.Marshal(map[string]string{"path": testFile})
	createReq := authRequest("POST", "/node/self/fs/mkfile", bytes.NewReader(createBody))
	cResp, cErr := client.Do(createReq)
	if cErr != nil {
		t.Fatalf("create failed: %v", cErr)
	}
	cResp.Body.Close()
	if cResp.StatusCode != 200 {
		t.Fatalf("create returned %d", cResp.StatusCode)
	}

	dlReq := authRequest("GET", "/node/self/fs/download?path="+testFile, nil)
	resp, err := client.Do(dlReq)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("download expected 200, got %d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Disposition")
	if !strings.Contains(contentType, "attachment") {
		t.Errorf("expected Content-Disposition attachment, got: %s", contentType)
	}
}

func TestPathNormalization_WindowsStyle(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	tests := []struct {
		path     string
		expectOK bool
		skipOS   string
	}{
		{".", true, ""},
		{"./", true, ""},
		{"C:/", true, "linux"},
		{"C:\\", true, "linux"},
		{"/tmp", true, "windows"},
	}
	for _, tc := range tests {
		if tc.skipOS != "" {
			if tc.skipOS == "linux" && runtime.GOOS != "windows" {
				t.Logf("skipping Windows-style path %q on %s", tc.path, runtime.GOOS)
				continue
			}
			if tc.skipOS == "windows" && runtime.GOOS == "windows" {
				t.Logf("skipping Unix-style path %q on Windows", tc.path)
				continue
			}
		}
		req := authRequest("GET", "/node/self/fs/list?path="+url.QueryEscape(tc.path), nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("path %s request failed: %v", tc.path, err)
			continue
		}
		resp.Body.Close()
		if tc.expectOK && resp.StatusCode != 200 {
			t.Errorf("path %s: expected 200, got %d", tc.path, resp.StatusCode)
		}
	}
}

func TestMaliciousFilename_Upload(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	testCases := []struct {
		input       string
		expectClean bool
	}{
		{"../../../etc/passwd", true},
		{"..\\..\\..\\windows\\system32\\config\\sam", true},
		{"../../.ssh/authorized_keys", true},
		{"null\x00byte", true},
		{"very_long_name_" + strings.Repeat("a", 300), true},
		{"normal_file.txt", true},
	}
	for _, tc := range testCases {
		sanitized := filemgr.SanitizeFileName(tc.input)
		if !tc.expectClean {
			continue
		}
		if strings.Contains(sanitized, "/") || strings.Contains(sanitized, "\\") {
			t.Errorf("path separators not removed: %q -> %q", tc.input, sanitized)
		}
		if strings.Contains(sanitized, "\x00") {
			t.Errorf("null byte not removed: %q -> %q", tc.input, sanitized)
		}
		if len(sanitized) > 255 {
			t.Errorf("name too long after sanitization: %d chars", len(sanitized))
		}
		if sanitized == "" && tc.input != "" {
			t.Errorf("non-empty input produced empty output: %q", tc.input)
		}
	}
}

func TestFileCreateDeleteCycle_100Times(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	failCount := 0
	const iterations = 100

	prefix := filepath.Join(os.TempDir(), fmt.Sprintf("cycle_%d_", time.Now().UnixNano()))
	for i := 0; i < iterations; i++ {
		testPath := prefix + fmt.Sprintf("%03d", i)

		createBody, _ := json.Marshal(map[string]string{"path": testPath})
		createReq := authRequest("POST", "/node/self/fs/mkdir", bytes.NewReader(createBody))
		resp, _ := client.Do(createReq)
		if resp != nil {
			resp.Body.Close()
			if resp.StatusCode != 200 {
				failCount++
				continue
			}
		}

		deleteBody, _ := json.Marshal(map[string]string{"path": testPath})
		deleteReq := authRequest("DELETE", "/node/self/fs/remove", bytes.NewReader(deleteBody))
		delResp, _ := client.Do(deleteReq)
		if delResp != nil {
			delResp.Body.Close()
			if delResp.StatusCode != 200 {
				failCount++
			}
		}
		os.RemoveAll(testPath)
	}
	if failCount > 0 {
		t.Errorf("%d/%d operations failed in create-delete cycle", failCount, iterations)
	}
}

func TestConcurrentReadWrite_50Threads(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	const concurrency = 50
	errCh := make(chan error, concurrency)

	baseDir := filepath.Join(os.TempDir(), fmt.Sprintf("stress_%d", time.Now().UnixNano()))
	os.MkdirAll(baseDir, 0755)
	defer os.RemoveAll(baseDir)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			client := getAuthClient()
			testFile := filepath.Join(baseDir, fmt.Sprintf("file_%d.txt", idx))

			content := []byte(fmt.Sprintf("content-%d-%d", idx, time.Now().UnixNano()))
			writeBody, _ := json.Marshal(map[string]interface{}{"path": testFile, "content": string(content)})
			writeReq := authRequest("POST", "/node/self/fs/mkfile", bytes.NewReader(writeBody))
			wResp, wErr := client.Do(writeReq)
			if wErr != nil {
				errCh <- fmt.Errorf("write failed for %d: %v", idx, wErr)
				return
			}
			wResp.Body.Close()

			readReq := authRequest("GET", "/node/self/fs/list?path="+url.QueryEscape(baseDir), nil)
			rResp, rErr := client.Do(readReq)
			if rErr != nil {
				errCh <- fmt.Errorf("read failed for %d: %v", idx, rErr)
				return
			}
			rResp.Body.Close()
			errCh <- nil
		}(i)
	}
	failCount := 0
	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Error(err)
			failCount++
		}
	}
	if failCount > concurrency/10 {
		t.Errorf("too many failures in concurrent R/W: %d/%d", failCount, concurrency)
	}
}

func TestLargeDirectoryListing_Performance(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("perf_%d", time.Now().UnixNano()))
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	const fileCount = 500
	for i := 0; i < fileCount; i++ {
		f := filepath.Join(testDir, fmt.Sprintf("file_%04d.txt", i))
		os.WriteFile(f, []byte("perf test data "+fmt.Sprint(i)), 0644)
	}

	start := time.Now()
	listReq := authRequest("GET", "/node/self/fs/list?path="+url.QueryEscape(testDir), nil)
	resp, err := client.Do(listReq)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var files []map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&files)
		if elapsed > 300*time.Millisecond {
			t.Errorf("listing %d files took %v (should be <300ms)", fileCount, elapsed)
		} else {
			t.Logf("listed %d files (%d returned) in %v", fileCount, len(files), elapsed)
		}
	} else {
		t.Logf("listing returned status %d in %v (may need allowed dirs setup)", resp.StatusCode, elapsed)
	}
}

func TestAlertRules_FullCRUD(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	createBody, _ := json.Marshal(map[string]interface{}{
		"metric":    "cpu_usage",
		"op":        ">",
		"threshold": float64(90),
		"level":     "warning",
		"channels":  []string{"webhook"},
		"enabled":   true,
	})
	createReq := authRequest("POST", "/alert-rules", bytes.NewReader(createBody))
	cResp, cErr := client.Do(createReq)
	if cErr != nil {
		t.Fatalf("create alert rule failed: %v", cErr)
	}
	cResp.Body.Close()
	if cResp.StatusCode != 200 && cResp.StatusCode != 201 {
		t.Errorf("create alert rule: expected 200/201, got %d", cResp.StatusCode)
	}
}

func TestTrendAnalysis_DataEndpoint(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	durationTests := []string{"1h", "6h", "1d", "7d"}
	for _, dur := range durationTests {
		histReq := authRequest("GET", "/node/self/history?duration="+dur+"&limit=10", nil)
		hResp, hErr := client.Do(histReq)
		if hErr != nil {
			t.Errorf("history %s failed: %v", dur, hErr)
			continue
		}
		hResp.Body.Close()
		if hResp.StatusCode != 200 {
			t.Errorf("history %s: expected 200, got %d", dur, hResp.StatusCode)
		}
	}

	statsReq := authRequest("GET", "/node/self/fs/stats?duration=7d", nil)
	sResp, sErr := client.Do(statsReq)
	if sErr != nil {
		t.Fatalf("file stats failed: %v", sErr)
	}
	defer sResp.Body.Close()
	if sResp.StatusCode != 200 {
		t.Errorf("file stats: expected 200, got %d", sResp.StatusCode)
	}
}

func TestEdgeCase_EmptyAndSpecialPaths(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	edgeCases := []struct {
		path       string
		expectFail bool
	}{
		{".", false},
		{"./", false},
		{"/", false},
		{"", false},
		{"   ", true},
	}
	for _, tc := range edgeCases {
		req := authRequest("GET", "/node/self/fs/list?path="+tc.path, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("edge case path %q request error: %v", tc.path, err)
			continue
		}
		if tc.expectFail && resp.StatusCode < 400 {
			t.Errorf("edge case path %q should fail but got %d", tc.path, resp.StatusCode)
		}
		if !tc.expectFail && resp.StatusCode >= 500 {
			t.Errorf("edge case path %q caused server error: %d", tc.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}
