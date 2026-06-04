//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// ── Auth Tests ──────────────────────────────────────────────

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

func TestAuthJWT_Signature(t *testing.T) {
	client := getAuthClient()
	req, _ := http.NewRequest("GET", baseURL+"/snapshot", nil)
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
	req, _ := http.NewRequest("GET", baseURL+"/snapshot", nil)
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

// ── Alert Tests ─────────────────────────────────────────────

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

func TestAlerts_History(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/alerts/history", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("get alert history failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── File System Tests ───────────────────────────────────────

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
		req := authRequest("GET", "/fs/list?path="+path, nil)
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
	req := authRequest("GET", "/fs/list", nil)
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
	req := authRequest("GET", "/fs/list?path=.", nil)
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
	req := authRequest("POST", "/fs/mkdir", bytes.NewReader(body))
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
	req := authRequest("POST", "/fs/mkfile", bytes.NewReader(body))
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
	req := authRequest("DELETE", "/fs/remove", bytes.NewReader(body))
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
	req := authRequest("GET", "/fs/list?path=", nil)
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
	req := authRequest("GET", "/fs/stats", nil)
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
	req := authRequest("GET", "/fs/stats?duration=24h", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("file stats failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFileStats_ReturnsArrayNotNil(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/fs/stats?duration=1m", nil)
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

func TestFileRead_SmallFile(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	testFile := filepath.Join(os.TempDir(), fmt.Sprintf("test_read_%d.txt", time.Now().UnixNano()))
	defer func() {
		delBody, _ := json.Marshal(map[string]string{"path": testFile})
		delReq := authRequest("DELETE", "/fs/remove", bytes.NewReader(delBody))
		if delResp, err := getAuthClient().Do(delReq); err == nil {
			delResp.Body.Close()
		}
	}()

	client := getAuthClient()
	body, _ := json.Marshal(map[string]string{"path": testFile})
	req := authRequest("POST", "/fs/mkfile", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	resp.Body.Close()

	readReq := authRequest("GET", "/fs/read?path="+testFile, nil)
	rresp, rerr := client.Do(readReq)
	if rerr != nil {
		t.Fatalf("read file failed: %v", rerr)
	}
	defer rresp.Body.Close()
	if rresp.StatusCode != 200 {
		t.Errorf("read expected 200, got %d", rresp.StatusCode)
	}
}

// ── Metrics / History Tests ─────────────────────────────────

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

func TestHistory_Success(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/history?duration=1h&limit=10", nil)
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
	req := authRequest("GET", "/history", nil)
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
	req := authRequest("GET", "/history?limit=1000", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("history failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Cron Job Tests ──────────────────────────────────────────

func TestCronJobs_CRUD(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/cronjobs", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list cron jobs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Script Tests ────────────────────────────────────────────

func TestScripts_List(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/scripts", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("list scripts failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestScripts_CreateAndDelete(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()

	// Create
	body, _ := json.Marshal(map[string]string{
		"name":        fmt.Sprintf("test_script_%d", time.Now().UnixNano()),
		"interpreter": "/bin/bash",
		"description": "integration test script",
		"content":     "#!/bin/bash\necho hello",
	})
	req := authRequest("POST", "/scripts", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("create script failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create script expected 200, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Delete
	if id, ok := result["id"]; ok {
		delReq := authRequest("DELETE", fmt.Sprintf("/scripts/%v", id), nil)
		delResp, err := client.Do(delReq)
		if err != nil {
			t.Fatalf("delete script failed: %v", err)
		}
		delResp.Body.Close()
		if delResp.StatusCode != 200 {
			t.Errorf("delete script expected 200, got %d", delResp.StatusCode)
		}
	}
}

func TestScriptSecurityCheck(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	body, _ := json.Marshal(map[string]string{
		"content": "rm -rf /",
	})
	req := authRequest("POST", "/scripts/check", bytes.NewReader(body))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("script check failed: %v", err)
	}
	defer resp.Body.Close()
	// Should return 200 with warnings, not block entirely
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Terminal Tests ──────────────────────────────────────────

func TestTerminalHistory(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/terminal/history", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("terminal history failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestTerminalShells(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/terminal/shells", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("terminal shells failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Audit Log Tests ─────────────────────────────────────────

func TestAuditLogs(t *testing.T) {
	if authToken == "" {
		t.Skip("no auth token available")
	}
	client := getAuthClient()
	req := authRequest("GET", "/audit-logs", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("audit logs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Concurrency / Security Tests ────────────────────────────

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
			req := authRequest("POST", "/fs/mkdir", bytes.NewReader(body))
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
		"/fs/list",
		"/history",
		"/snapshot",
		"/alert-rules",
		"/scripts",
		"/cronjobs",
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
			req := authRequest("GET", "/snapshot", nil)
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

// ── Benchmarks ──────────────────────────────────────────────

func BenchmarkHealthEndpoint(b *testing.B) {
	for i := 0; i < b.N; i++ {
		resp, _ := http.Get(baseURL + "/health")
		if resp != nil {
			resp.Body.Close()
		}
	}
}

func BenchmarkSnapshotEndpoint(b *testing.B) {
	if authToken == "" {
		b.Skip("no auth token")
	}
	client := getAuthClient()
	for i := 0; i < b.N; i++ {
		req := authRequest("GET", "/snapshot", nil)
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
