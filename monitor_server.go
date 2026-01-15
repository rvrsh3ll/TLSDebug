package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// TrafficEntry represents a captured HTTP request/response
type TrafficEntry struct {
	ID              int
	Timestamp       time.Time
	Method          string
	URL             string
	Host            string
	Path            string
	StatusCode      int
	StatusText      string
	RequestHeaders  map[string][]string
	ResponseHeaders map[string][]string
	RequestBody     string
	ResponseBody    string
	ContentType     string
	Duration        time.Duration
	TLSVersion      string
	ClientAddr      string
}

// TrafficStore holds all captured traffic with thread-safe access
type TrafficStore struct {
	sync.RWMutex
	entries    []TrafficEntry
	nextID     int
	maxEntries int
}

var trafficStore = &TrafficStore{
	entries:    make([]TrafficEntry, 0),
	nextID:     1,
	maxEntries: 1000, // Keep last 1000 entries
}

// AddEntry adds a new traffic entry to the store
func (ts *TrafficStore) AddEntry(entry TrafficEntry) {
	ts.Lock()
	defer ts.Unlock()
	
	entry.ID = ts.nextID
	ts.nextID++
	
	ts.entries = append(ts.entries, entry)
	
	// Keep only the last maxEntries
	if len(ts.entries) > ts.maxEntries {
		ts.entries = ts.entries[len(ts.entries)-ts.maxEntries:]
	}
}

// GetEntries returns all entries (newest first)
func (ts *TrafficStore) GetEntries() []TrafficEntry {
	ts.RLock()
	defer ts.RUnlock()
	
	// Return a copy in reverse order (newest first)
	result := make([]TrafficEntry, len(ts.entries))
	for i, entry := range ts.entries {
		result[len(ts.entries)-1-i] = entry
	}
	
	return result
}

// GetEntry returns a specific entry by ID
func (ts *TrafficStore) GetEntry(id int) *TrafficEntry {
	ts.RLock()
	defer ts.RUnlock()
	
	for _, entry := range ts.entries {
		if entry.ID == id {
			return &entry
		}
	}
	return nil
}

// Clear removes all entries
func (ts *TrafficStore) Clear() {
	ts.Lock()
	defer ts.Unlock()
	
	ts.entries = make([]TrafficEntry, 0)
}

// StartMonitorServer starts the web-based monitoring interface
func StartMonitorServer(port int) {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/entries", handleAPIEntries)
	http.HandleFunc("/api/entry/", handleAPIEntry)
	http.HandleFunc("/api/clear", handleAPIClear)
	http.HandleFunc("/api/stats", handleAPIStats)
	
	addr := fmt.Sprintf(":%d", port)
	log.Printf("[MONITOR] Starting monitor server on http://localhost%s", addr)
	
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("[MONITOR] Server error: %v", err)
		}
	}()
}

// handleIndex serves the main HTML page
func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	htmlPage := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TLS Proxy Monitor</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            padding: 20px;
        }
        
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 25px;
            border-radius: 12px;
            margin-bottom: 25px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        
        .header h1 {
            color: white;
            font-size: 28px;
            margin-bottom: 10px;
        }
        
        .stats {
            display: flex;
            gap: 20px;
            margin-top: 15px;
            flex-wrap: wrap;
        }
        
        .stat-box {
            background: rgba(255,255,255,0.1);
            padding: 12px 20px;
            border-radius: 8px;
            backdrop-filter: blur(10px);
        }
        
        .stat-box .label {
            font-size: 12px;
            opacity: 0.8;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .stat-box .value {
            font-size: 24px;
            font-weight: bold;
            margin-top: 5px;
        }
        
        .controls {
            background: #2a2a2a;
            padding: 20px;
            border-radius: 12px;
            margin-bottom: 20px;
            display: flex;
            gap: 15px;
            flex-wrap: wrap;
            align-items: center;
            box-shadow: 0 2px 4px rgba(0,0,0,0.3);
        }
        
        .controls input[type="text"] {
            flex: 1;
            min-width: 250px;
            padding: 10px 15px;
            border: 2px solid #3a3a3a;
            border-radius: 8px;
            background: #1a1a1a;
            color: #e0e0e0;
            font-size: 14px;
            transition: border-color 0.3s;
        }
        
        .controls input[type="text"]:focus {
            outline: none;
            border-color: #667eea;
        }
        
        .controls button {
            padding: 10px 20px;
            border: none;
            border-radius: 8px;
            background: #667eea;
            color: white;
            cursor: pointer;
            font-size: 14px;
            font-weight: 600;
            transition: all 0.3s;
        }
        
        .controls button:hover {
            background: #5568d3;
            transform: translateY(-1px);
        }
        
        .controls button.danger {
            background: #e74c3c;
        }
        
        .controls button.danger:hover {
            background: #c0392b;
        }
        
        .controls label {
            display: flex;
            align-items: center;
            gap: 8px;
            cursor: pointer;
            user-select: none;
        }
        
        .table-container {
            background: #2a2a2a;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
        }
        
        thead {
            background: #3a3a3a;
        }
        
        th {
            padding: 15px;
            text-align: left;
            font-weight: 600;
            color: #b0b0b0;
            text-transform: uppercase;
            font-size: 12px;
            letter-spacing: 1px;
            border-bottom: 2px solid #4a4a4a;
        }
        
        tbody tr {
            border-bottom: 1px solid #3a3a3a;
            transition: background 0.2s;
            cursor: pointer;
        }
        
        tbody tr:hover {
            background: #333333;
        }
        
        td {
            padding: 15px;
            font-size: 14px;
        }
        
        .method {
            font-weight: bold;
            padding: 4px 10px;
            border-radius: 6px;
            display: inline-block;
            font-size: 12px;
        }
        
        .method.GET { background: #27ae60; color: white; }
        .method.POST { background: #f39c12; color: white; }
        .method.PUT { background: #3498db; color: white; }
        .method.DELETE { background: #e74c3c; color: white; }
        .method.PATCH { background: #9b59b6; color: white; }
        
        .status {
            font-weight: bold;
            padding: 4px 10px;
            border-radius: 6px;
            display: inline-block;
            font-size: 12px;
        }
        
        .status.success { background: #27ae60; color: white; }
        .status.redirect { background: #3498db; color: white; }
        .status.client-error { background: #e67e22; color: white; }
        .status.server-error { background: #e74c3c; color: white; }
        
        .url {
            color: #6eb5ff;
            word-break: break-all;
        }
        
        .timestamp {
            color: #95a5a6;
            font-size: 12px;
        }
        
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.8);
            z-index: 1000;
            padding: 20px;
            overflow-y: auto;
        }
        
        .modal-content {
            background: #2a2a2a;
            max-width: 1200px;
            margin: 40px auto;
            border-radius: 12px;
            padding: 30px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.5);
        }
        
        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 25px;
            padding-bottom: 15px;
            border-bottom: 2px solid #3a3a3a;
        }
        
        .modal-header h2 {
            color: #667eea;
        }
        
        .close-btn {
            background: none;
            border: none;
            color: #95a5a6;
            font-size: 32px;
            cursor: pointer;
            transition: color 0.3s;
        }
        
        .close-btn:hover {
            color: #e74c3c;
        }
        
        .detail-section {
            margin-bottom: 25px;
        }
        
        .detail-section h3 {
            color: #667eea;
            margin-bottom: 12px;
            font-size: 18px;
        }
        
        .detail-grid {
            display: grid;
            grid-template-columns: 150px 1fr;
            gap: 10px;
            background: #1a1a1a;
            padding: 15px;
            border-radius: 8px;
        }
        
        .detail-grid .label {
            color: #95a5a6;
            font-weight: 600;
        }
        
        .detail-grid .value {
            color: #e0e0e0;
        }
        
        .headers-list {
            background: #1a1a1a;
            padding: 15px;
            border-radius: 8px;
            font-family: 'Courier New', monospace;
            font-size: 13px;
        }
        
        .header-item {
            margin-bottom: 8px;
            padding-bottom: 8px;
            border-bottom: 1px solid #3a3a3a;
        }
        
        .header-item:last-child {
            border-bottom: none;
        }
        
        .header-name {
            color: #f39c12;
            font-weight: bold;
        }
        
        .header-value {
            color: #95a5a6;
            margin-left: 10px;
        }
        
        .body-content {
            background: #1a1a1a;
            padding: 15px;
            border-radius: 8px;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            max-height: 400px;
            overflow-y: auto;
            white-space: pre-wrap;
            word-break: break-all;
        }
        
        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: #95a5a6;
        }
        
        .empty-state-icon {
            font-size: 64px;
            margin-bottom: 20px;
            opacity: 0.5;
        }
        
        @media (max-width: 768px) {
            .controls {
                flex-direction: column;
            }
            
            .controls input[type="text"] {
                width: 100%;
            }
            
            .stats {
                flex-direction: column;
            }
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔒 TLS Proxy Monitor</h1>
        <div class="stats">
            <div class="stat-box">
                <div class="label">Total Requests</div>
                <div class="value" id="totalRequests">0</div>
            </div>
            <div class="stat-box">
                <div class="label">Success Rate</div>
                <div class="value" id="successRate">0%</div>
            </div>
            <div class="stat-box">
                <div class="label">Avg Response Time</div>
                <div class="value" id="avgTime">0ms</div>
            </div>
        </div>
    </div>
    
    <div class="controls">
        <input type="text" id="searchBox" placeholder="🔍 Search by URL, host, method, or status...">
        <label>
            <input type="checkbox" id="autoRefresh" checked>
            Auto-refresh (2s)
        </label>
        <button onclick="loadEntries()">🔄 Refresh Now</button>
        <button class="danger" onclick="clearEntries()">🗑️ Clear All</button>
    </div>
    
    <div class="table-container">
        <table>
            <thead>
                <tr>
                    <th>Time</th>
                    <th>Method</th>
                    <th>Host</th>
                    <th>Path</th>
                    <th>Status</th>
                    <th>Duration</th>
                    <th>Type</th>
                </tr>
            </thead>
            <tbody id="trafficTable">
                <tr>
                    <td colspan="7" class="empty-state">
                        <div class="empty-state-icon">📡</div>
                        <div>No traffic captured yet. Make some requests through the proxy!</div>
                    </td>
                </tr>
            </tbody>
        </table>
    </div>
    
    <div id="detailModal" class="modal" onclick="closeModal(event)">
        <div class="modal-content" onclick="event.stopPropagation()">
            <div class="modal-header">
                <h2>Request Details</h2>
                <button class="close-btn" onclick="closeModal()">&times;</button>
            </div>
            <div id="modalBody"></div>
        </div>
    </div>
    
    <script>
        let searchTerm = '';
        let autoRefreshInterval = null;
        
        // Initialize
        document.getElementById('searchBox').addEventListener('input', (e) => {
            searchTerm = e.target.value.toLowerCase();
            loadEntries();
        });
        
        document.getElementById('autoRefresh').addEventListener('change', (e) => {
            if (e.target.checked) {
                startAutoRefresh();
            } else {
                stopAutoRefresh();
            }
        });
        
        function startAutoRefresh() {
            if (!autoRefreshInterval) {
                autoRefreshInterval = setInterval(loadEntries, 2000);
            }
        }
        
        function stopAutoRefresh() {
            if (autoRefreshInterval) {
                clearInterval(autoRefreshInterval);
                autoRefreshInterval = null;
            }
        }
        
        async function loadEntries() {
            try {
                const response = await fetch('/api/entries');
                const entries = await response.json();
                
                const filtered = entries.filter(entry => {
                    if (!searchTerm) return true;
                    return entry.URL.toLowerCase().includes(searchTerm) ||
                           entry.Host.toLowerCase().includes(searchTerm) ||
                           entry.Method.toLowerCase().includes(searchTerm) ||
                           entry.StatusCode.toString().includes(searchTerm);
                });
                
                renderTable(filtered);
                updateStats(entries);
            } catch (error) {
                console.error('Failed to load entries:', error);
            }
        }
        
        function renderTable(entries) {
            const tbody = document.getElementById('trafficTable');
            
            if (entries.length === 0) {
                tbody.innerHTML = `
                    <tr>
                        <td colspan="7" class="empty-state">
                            <div class="empty-state-icon">📡</div>
                            <div>No traffic captured yet. Make some requests through the proxy!</div>
                        </td>
                    </tr>
                `;
                return;
            }
            
            tbody.innerHTML = entries.map(entry => {
                const time = new Date(entry.Timestamp).toLocaleTimeString();
                const statusClass = getStatusClass(entry.StatusCode);
                const duration = entry.Duration ? (entry.Duration / 1000000).toFixed(0) + 'ms' : '-';
                
                return `
                    <tr onclick="showDetails(${entry.ID})">
                        <td class="timestamp">${time}</td>
                        <td><span class="method ${entry.Method}">${entry.Method}</span></td>
                        <td>${escapeHtml(entry.Host)}</td>
                        <td class="url">${escapeHtml(entry.Path)}</td>
                        <td><span class="status ${statusClass}">${entry.StatusCode || '-'}</span></td>
                        <td>${duration}</td>
                        <td>${entry.ContentType || '-'}</td>
                    </tr>
                `;
            }).join('');
        }
        
        function getStatusClass(code) {
            if (code >= 200 && code < 300) return 'success';
            if (code >= 300 && code < 400) return 'redirect';
            if (code >= 400 && code < 500) return 'client-error';
            if (code >= 500) return 'server-error';
            return '';
        }
        
        async function updateStats(entries) {
            document.getElementById('totalRequests').textContent = entries.length;
            
            const successCount = entries.filter(e => e.StatusCode >= 200 && e.StatusCode < 300).length;
            const successRate = entries.length > 0 ? ((successCount / entries.length) * 100).toFixed(1) : 0;
            document.getElementById('successRate').textContent = successRate + '%';
            
            const avgDuration = entries.length > 0
                ? entries.reduce((sum, e) => sum + (e.Duration || 0), 0) / entries.length / 1000000
                : 0;
            document.getElementById('avgTime').textContent = avgDuration.toFixed(0) + 'ms';
        }
        
        async function showDetails(id) {
            try {
                const response = await fetch('/api/entry/' + id);
                const entry = await response.json();
                
                if (!entry) {
                    alert('Entry not found');
                    return;
                }
                
                const modalBody = document.getElementById('modalBody');
                modalBody.innerHTML = `
                    <div class="detail-section">
                        <h3>Request Information</h3>
                        <div class="detail-grid">
                            <div class="label">Method:</div>
                            <div class="value"><span class="method ${entry.Method}">${entry.Method}</span></div>
                            <div class="label">URL:</div>
                            <div class="value">${escapeHtml(entry.URL)}</div>
                            <div class="label">Host:</div>
                            <div class="value">${escapeHtml(entry.Host)}</div>
                            <div class="label">Path:</div>
                            <div class="value">${escapeHtml(entry.Path)}</div>
                            <div class="label">Client:</div>
                            <div class="value">${escapeHtml(entry.ClientAddr)}</div>
                            <div class="label">TLS Version:</div>
                            <div class="value">${entry.TLSVersion || 'N/A'}</div>
                            <div class="label">Timestamp:</div>
                            <div class="value">${new Date(entry.Timestamp).toLocaleString()}</div>
                        </div>
                    </div>
                    
                    ${entry.StatusCode ? `
                    <div class="detail-section">
                        <h3>Response Information</h3>
                        <div class="detail-grid">
                            <div class="label">Status:</div>
                            <div class="value"><span class="status ${getStatusClass(entry.StatusCode)}">${entry.StatusCode} ${escapeHtml(entry.StatusText)}</span></div>
                            <div class="label">Content-Type:</div>
                            <div class="value">${escapeHtml(entry.ContentType)}</div>
                            <div class="label">Duration:</div>
                            <div class="value">${entry.Duration ? (entry.Duration / 1000000).toFixed(2) + 'ms' : 'N/A'}</div>
                        </div>
                    </div>
                    ` : ''}
                    
                    <div class="detail-section">
                        <h3>Request Headers</h3>
                        <div class="headers-list">
                            ${formatHeaders(entry.RequestHeaders)}
                        </div>
                    </div>
                    
                    ${entry.ResponseHeaders ? `
                    <div class="detail-section">
                        <h3>Response Headers</h3>
                        <div class="headers-list">
                            ${formatHeaders(entry.ResponseHeaders)}
                        </div>
                    </div>
                    ` : ''}
                    
                    ${entry.RequestBody ? `
                    <div class="detail-section">
                        <h3>Request Body</h3>
                        <div class="body-content">${escapeHtml(entry.RequestBody)}</div>
                    </div>
                    ` : ''}
                    
                    ${entry.ResponseBody ? `
                    <div class="detail-section">
                        <h3>Response Body</h3>
                        <div class="body-content">${escapeHtml(entry.ResponseBody)}</div>
                    </div>
                    ` : ''}
                `;
                
                document.getElementById('detailModal').style.display = 'block';
            } catch (error) {
                console.error('Failed to load entry details:', error);
                alert('Failed to load entry details');
            }
        }
        
        function formatHeaders(headers) {
            if (!headers) return '<div style="color: #95a5a6;">No headers</div>';
            
            return Object.entries(headers).map(([name, values]) => {
                const valueStr = Array.isArray(values) ? values.join(', ') : values;
                return `<div class="header-item"><span class="header-name">${escapeHtml(name)}:</span><span class="header-value">${escapeHtml(valueStr)}</span></div>`;
            }).join('');
        }
        
        function closeModal(event) {
            if (!event || event.target.id === 'detailModal') {
                document.getElementById('detailModal').style.display = 'none';
            }
        }
        
        async function clearEntries() {
            if (!confirm('Are you sure you want to clear all captured traffic?')) {
                return;
            }
            
            try {
                await fetch('/api/clear', { method: 'POST' });
                loadEntries();
            } catch (error) {
                console.error('Failed to clear entries:', error);
                alert('Failed to clear entries');
            }
        }
        
        function escapeHtml(text) {
            if (!text) return '';
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }
        
        // Start auto-refresh
        startAutoRefresh();
        loadEntries();
    </script>
</body>
</html>`
	
	fmt.Fprint(w, htmlPage)
}

// handleAPIEntries returns all traffic entries as JSON
func handleAPIEntries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	entries := trafficStore.GetEntries()
	json.NewEncoder(w).Encode(entries)
}

// handleAPIEntry returns a specific entry by ID
func handleAPIEntry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Extract ID from path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/entry/")
	var id int
	fmt.Sscanf(idStr, "%d", &id)
	
	entry := trafficStore.GetEntry(id)
	if entry == nil {
		http.NotFound(w, r)
		return
	}
	
	json.NewEncoder(w).Encode(entry)
}

// handleAPIClear clears all entries
func handleAPIClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	trafficStore.Clear()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAPIStats returns traffic statistics
func handleAPIStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	entries := trafficStore.GetEntries()
	
	stats := map[string]interface{}{
		"total":       len(entries),
		"methods":     countByMethod(entries),
		"statusCodes": countByStatusCode(entries),
		"hosts":       countByHost(entries),
	}
	
	json.NewEncoder(w).Encode(stats)
}

func countByMethod(entries []TrafficEntry) map[string]int {
	counts := make(map[string]int)
	for _, entry := range entries {
		counts[entry.Method]++
	}
	return counts
}

func countByStatusCode(entries []TrafficEntry) map[int]int {
	counts := make(map[int]int)
	for _, entry := range entries {
		if entry.StatusCode > 0 {
			counts[entry.StatusCode]++
		}
	}
	return counts
}

func countByHost(entries []TrafficEntry) []map[string]interface{} {
	counts := make(map[string]int)
	for _, entry := range entries {
		counts[entry.Host]++
	}
	
	// Convert to sorted slice
	type hostCount struct {
		host  string
		count int
	}
	var hosts []hostCount
	for host, count := range counts {
		hosts = append(hosts, hostCount{host, count})
	}
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].count > hosts[j].count
	})
	
	// Take top 10
	result := make([]map[string]interface{}, 0)
	for i, hc := range hosts {
		if i >= 10 {
			break
		}
		result = append(result, map[string]interface{}{
			"host":  hc.host,
			"count": hc.count,
		})
	}
	
	return result
}

// Helper function to escape HTML in strings
func escapeHTML(s string) string {
	return html.EscapeString(s)
}
