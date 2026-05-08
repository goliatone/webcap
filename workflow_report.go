package webcap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// workflowReportFuncs provides template helper functions for arithmetic operations.
var workflowReportFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
}

const (
	workflowStatusSuccess = "success"
	workflowStatusInfo    = "info"
	workflowStatusWarning = "warning"
	workflowStatusError   = "error"
)

const workflowReportStylesheet = `/* Webcap Review Report - Dark Developer Tool Theme */
:root {
  color-scheme: dark;
  --sidebar-width: 240px;
  --sidebar-collapsed: 64px;
  /* Core Backgrounds */
  --bg: #09090b;
  --bg-subtle: #0c0c0e;
  --bg-card: #18181b;
  --bg-elevated: #1f1f23;
  --bg-muted: #27272a;
  --bg-accent: #2d2d32;
  /* Borders */
  --border: #27272a;
  --border-muted: #1f1f23;
  --border-focus: #3f3f46;
  --border-accent: #52525b;
  /* Text */
  --text: #fafafa;
  --text-secondary: #a1a1aa;
  --text-muted: #71717a;
  --text-dim: #52525b;
  /* Accent - Violet */
  --accent: #a78bfa;
  --accent-hover: #c4b5fd;
  --accent-muted: #7c3aed;
  --accent-bg: rgba(167, 139, 250, 0.1);
  --accent-border: rgba(167, 139, 250, 0.3);
  /* Semantic */
  --success: #22c55e;
  --success-bg: rgba(34, 197, 94, 0.1);
  --success-border: rgba(34, 197, 94, 0.3);
  --warning: #f59e0b;
  --warning-bg: rgba(245, 158, 11, 0.1);
  --warning-border: rgba(245, 158, 11, 0.3);
  --error: #ef4444;
  --error-bg: rgba(239, 68, 68, 0.1);
  --error-border: rgba(239, 68, 68, 0.3);
  --info: #3b82f6;
  --info-bg: rgba(59, 130, 246, 0.1);
  --info-border: rgba(59, 130, 246, 0.3);
  /* Radius */
  --radius-sm: 4px;
  --radius: 6px;
  --radius-lg: 8px;
  /* Shadows */
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.4);
  --shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}
* { box-sizing: border-box; }
body {
  margin: 0;
  font-family: "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text);
  background: var(--bg);
  -webkit-font-smoothing: antialiased;
}
code, pre, .mono {
  font-family: "JetBrains Mono", "Fira Code", ui-monospace, monospace;
}
a { color: var(--accent); text-decoration: none; }
a:hover { color: var(--accent-hover); }

/* Layout with Sidebar */
.app { min-height: 100vh; display: flex; }

/* Sidebar */
.sidebar {
  position: fixed; top: 0; left: 0; bottom: 0;
  width: var(--sidebar-width); background: var(--bg-card);
  border-right: 1px solid var(--border);
  display: flex; flex-direction: column;
  transition: width 0.2s ease;
  z-index: 60;
}
.sidebar.collapsed { width: var(--sidebar-collapsed); }
.sidebar-header {
  padding: 16px; border-bottom: 1px solid var(--border);
  display: flex; align-items: center; gap: 12px;
}
.sidebar-logo {
  display: flex; align-items: center; gap: 10px;
  color: var(--text); text-decoration: none;
}
.sidebar-logo-icon { width: 32px; height: 32px; color: var(--accent); flex-shrink: 0; }
.sidebar-logo-text { font-weight: 600; font-size: 15px; white-space: nowrap; overflow: hidden; }
.sidebar-logo-text span { color: var(--accent); }
.sidebar.collapsed .sidebar-logo-text { display: none; }

.sidebar-nav { flex: 1; padding: 12px 8px; display: flex; flex-direction: column; gap: 4px; }
.sidebar-nav-item {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 12px; border-radius: var(--radius);
  color: var(--text-secondary); cursor: pointer;
  transition: all 0.15s; border: none; background: none;
  font-family: inherit; font-size: 13px; font-weight: 500;
  text-align: left; width: 100%;
}
.sidebar-nav-item:hover { background: var(--bg-muted); color: var(--text); }
.sidebar-nav-item.active { background: var(--accent-bg); color: var(--accent); }
.sidebar-nav-icon { width: 20px; height: 20px; flex-shrink: 0; }
.sidebar-nav-label { white-space: nowrap; overflow: hidden; }
.sidebar.collapsed .sidebar-nav-label { display: none; }
.sidebar.collapsed .sidebar-nav-item { justify-content: center; padding: 10px; }

.sidebar-footer {
  padding: 12px 8px; border-top: 1px solid var(--border);
  display: flex; flex-direction: column; gap: 4px;
}
.sidebar-toggle {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 12px; border-radius: var(--radius);
  color: var(--text-muted); cursor: pointer;
  transition: all 0.15s; border: none; background: none;
  font-family: inherit; font-size: 12px; width: 100%;
}
.sidebar-toggle:hover { background: var(--bg-muted); color: var(--text); }
.sidebar-toggle-icon { width: 20px; height: 20px; flex-shrink: 0; transition: transform 0.2s; }
.sidebar.collapsed .sidebar-toggle-icon { transform: rotate(180deg); }
.sidebar-toggle-label { white-space: nowrap; }
.sidebar.collapsed .sidebar-toggle-label { display: none; }
.sidebar.collapsed .sidebar-toggle { justify-content: center; }

/* Main Content */
.main-content {
  flex: 1; margin-left: var(--sidebar-width);
  transition: margin-left 0.2s ease;
  min-height: 100vh; display: flex; flex-direction: column;
}
.sidebar.collapsed ~ .main-content { margin-left: var(--sidebar-collapsed); }
.content { flex: 1; padding: 24px; max-width: 1600px; margin: 0 auto; width: 100%; }

/* Top Bar */
.topbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 24px; background: var(--bg-subtle);
  border-bottom: 1px solid var(--border);
}
.topbar-title { font-size: 14px; font-weight: 500; color: var(--text-secondary); }
.topbar-actions { display: flex; gap: 8px; }

/* Buttons */
.btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 6px;
  padding: 8px 16px; font-size: 13px; font-weight: 500; font-family: inherit;
  border: 1px solid transparent; border-radius: var(--radius);
  cursor: pointer; transition: all 0.15s;
}
.btn-sm { padding: 6px 12px; font-size: 12px; }
.btn-primary { background: var(--accent); color: #000; border-color: var(--accent); }
.btn-primary:hover { background: var(--accent-hover); border-color: var(--accent-hover); }
.btn-secondary { background: var(--bg-muted); color: var(--text); border-color: var(--border-focus); }
.btn-secondary:hover { background: var(--bg-accent); border-color: var(--border-accent); }
.btn-ghost { background: transparent; color: var(--text-secondary); }
.btn-ghost:hover { background: var(--bg-muted); color: var(--text); }

/* Summary Header */
.summary-header { margin-bottom: 24px; }
.summary-header h1 { font-size: 24px; font-weight: 600; margin: 0 0 4px; }
.summary-header .lede { color: var(--text-secondary); font-size: 14px; margin: 0; }
.summary-meta { display: flex; gap: 16px; margin-top: 12px; flex-wrap: wrap; }
.summary-meta-item {
  display: flex; align-items: center; gap: 6px;
  font-size: 12px; color: var(--text-muted);
}
.summary-meta-item .mono { color: var(--text-secondary); }

/* Metric Cards */
.metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px; margin: 20px 0; }
.metric-card {
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius-lg); padding: 16px;
}
.metric-card:hover { border-color: var(--border-focus); }
.metric-label {
  font-size: 11px; font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 8px;
}
.metric-value { font-size: 28px; font-weight: 600; color: var(--text); }
.metric-value.success { color: var(--success); }
.metric-value.warning { color: var(--warning); }
.metric-value.error { color: var(--error); }
.metric-suffix { font-size: 14px; color: var(--text-secondary); margin-left: 4px; }

/* Filter Chips */
.filter-bar { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; align-items: center; }
.filter-chip {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 12px; font-size: 12px; font-weight: 500;
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: 9999px; cursor: pointer; transition: all 0.15s;
  font-family: inherit; color: var(--text-secondary);
}
.filter-chip:hover { border-color: var(--border-focus); color: var(--text); }
.filter-chip.active { background: var(--accent-bg); border-color: var(--accent-border); color: var(--accent); }
.filter-chip-count {
  font-size: 11px; padding: 1px 6px; background: var(--bg-muted);
  border-radius: 9999px; color: var(--text-muted);
}
.filter-chip.active .filter-chip-count { background: var(--accent-border); color: var(--accent); }

/* View Content */
.view-content { display: none; }
.view-content.active { display: block; }

/* Screen Grid */
.screen-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 16px; }
.screen-card {
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius-lg); overflow: hidden; cursor: pointer;
  transition: border-color 0.15s, transform 0.15s;
}
.screen-card:hover { border-color: var(--border-focus); transform: translateY(-2px); }
.screen-card.active { border-color: var(--accent-border); box-shadow: inset 0 0 0 1px var(--accent-border); }
.screen-card.filter-hidden { display: none; }
.screen-card-thumb {
  aspect-ratio: 16/10; background: var(--bg-subtle);
  display: flex; align-items: center; justify-content: center; overflow: hidden;
  position: relative;
}
.screen-card-thumb img { width: 100%; height: 100%; object-fit: cover; }
.screen-card-diff-indicator {
  position: absolute; top: 8px; right: 8px;
  padding: 2px 6px; font-size: 10px; font-weight: 600;
  border-radius: var(--radius-sm); font-family: "JetBrains Mono", monospace;
}
.screen-card-diff-indicator.changed { background: var(--info-bg); color: var(--info); border: 1px solid var(--info-border); }
.screen-card-diff-indicator.unchanged { background: var(--success-bg); color: var(--success); border: 1px solid var(--success-border); }
.screen-card-body { padding: 14px; }
.screen-card-title { font-size: 14px; font-weight: 600; margin: 0 0 6px; }
.screen-card-meta { display: flex; gap: 6px; flex-wrap: wrap; }

/* Pills & Badges */
.pill {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 2px 8px; font-size: 11px; font-weight: 500;
  border-radius: 9999px; background: var(--bg-muted);
  color: var(--text-secondary); border: 1px solid var(--border);
  font-family: "JetBrains Mono", monospace;
  white-space: nowrap; flex-shrink: 0;
}
.pill-accent { background: var(--accent-bg); color: var(--accent); border-color: var(--accent-border); }
.pill-info { background: var(--info-bg); color: var(--info); border-color: var(--info-border); }
.pill-success { background: var(--success-bg); color: var(--success); border-color: var(--success-border); }
.pill-warning { background: var(--warning-bg); color: var(--warning); border-color: var(--warning-border); }
.pill-error { background: var(--error-bg); color: var(--error); border-color: var(--error-border); }
.badge {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 8px; font-size: 11px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.03em;
  border-radius: var(--radius-sm); border: 1px solid transparent;
  white-space: nowrap; flex-shrink: 0;
}
.badge-success { background: var(--success-bg); color: var(--success); border-color: var(--success-border); }
.badge-info { background: var(--info-bg); color: var(--info); border-color: var(--info-border); }
.badge-warning { background: var(--warning-bg); color: var(--warning); border-color: var(--warning-border); }
.badge-error { background: var(--error-bg); color: var(--error); border-color: var(--error-border); }
.badge::before { content: ''; width: 6px; height: 6px; border-radius: 50%; background: currentColor; }

/* Story Matrix */
.story-matrix { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius-lg); overflow: hidden; }
.story-matrix-header { padding: 14px 16px; border-bottom: 1px solid var(--border); background: var(--bg-subtle); }
.story-matrix-header h3 { font-size: 14px; font-weight: 600; margin: 0; }
.story-matrix-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.story-matrix-table th {
  text-align: left; padding: 10px 12px; font-size: 11px; font-weight: 600;
  text-transform: uppercase; letter-spacing: 0.05em;
  color: var(--text-muted); background: var(--bg-subtle); border-bottom: 1px solid var(--border);
}
.story-matrix-table td { padding: 10px 12px; border-bottom: 1px solid var(--border); }
.story-matrix-table tr:last-child td { border-bottom: none; }
.story-matrix-table tr:hover td { background: var(--bg-muted); }

/* Screen Review Detail */
.screen-detail { display: grid; grid-template-columns: 1fr 320px; gap: 20px; }
@media (max-width: 1200px) { .screen-detail { grid-template-columns: 1fr; } }
.screen-detail-main { min-width: 0; }
.screen-detail-sidebar { display: flex; flex-direction: column; gap: 16px; }

/* Compare Area */
.compare-header {
  display: flex; justify-content: space-between; align-items: flex-start;
  margin-bottom: 16px; flex-wrap: wrap; gap: 12px;
}
.compare-title { font-size: 18px; font-weight: 600; margin: 0; }
.compare-controls { display: flex; gap: 8px; flex-wrap: wrap; }
.compare-mode-btn {
  padding: 6px 12px; font-size: 12px; font-weight: 500;
  background: var(--bg-muted); color: var(--text-secondary);
  border: 1px solid var(--border); border-radius: var(--radius);
  cursor: pointer; transition: all 0.15s; font-family: inherit;
}
.compare-mode-btn:hover { background: var(--bg-accent); color: var(--text); }
.compare-mode-btn.active { background: var(--accent-bg); color: var(--accent); border-color: var(--accent-border); }

.compare-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
@media (max-width: 900px) { .compare-grid { grid-template-columns: 1fr; } }
.screen-detail-view[data-compare-mode="diff-only"] .compare-grid { grid-template-columns: 1fr; }
.screen-detail-view[data-compare-mode="diff-only"] .compare-panel[data-panel-kind="current"],
.screen-detail-view[data-compare-mode="diff-only"] .compare-panel[data-panel-kind="reference"] { display: none; }
.compare-panel {
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius-lg); overflow: hidden;
}
.compare-panel-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 14px; background: var(--bg-subtle); border-bottom: 1px solid var(--border);
}
.compare-panel-title { font-size: 13px; font-weight: 600; }
.compare-panel-body { padding: 12px; background: #0a0a0a; }
.compare-panel-body img {
  display: block; width: 100%; height: auto; border-radius: var(--radius);
  cursor: zoom-in;
}
.compare-panel-body .missing {
  padding: 40px 20px; text-align: center; color: var(--text-muted);
  font-size: 13px;
}

/* Warning Block */
.warning-block {
  background: var(--warning-bg); border: 1px solid var(--warning-border);
  border-left: 3px solid var(--warning); border-radius: var(--radius);
  padding: 12px 16px; margin: 16px 0;
}
.warning-block-title { font-size: 13px; font-weight: 600; color: var(--warning); margin: 0 0 4px; }
.warning-block-list { margin: 8px 0 0; padding-left: 20px; font-size: 13px; color: var(--warning); }

/* Evidence Panel */
.evidence-panel {
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius-lg); overflow: hidden;
}
.evidence-panel-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px; background: var(--bg-subtle); border-bottom: 1px solid var(--border);
}
.evidence-panel-title { font-size: 13px; font-weight: 600; display: flex; align-items: center; gap: 8px; }
.evidence-panel-count {
  font-size: 11px; padding: 2px 6px; background: var(--bg-muted);
  border-radius: 9999px; color: var(--text-muted);
}
.evidence-panel-body { max-height: 400px; overflow-y: auto; }
.evidence-item {
  display: flex; align-items: flex-start; gap: 12px;
  padding: 12px 16px; border-bottom: 1px solid var(--border);
  transition: background 0.15s;
}
.evidence-item:hover { background: var(--bg-muted); }
.evidence-item:last-child { border-bottom: none; }
.evidence-checkbox {
  width: 18px; height: 18px; border: 1px solid var(--border-focus);
  border-radius: var(--radius-sm); background: var(--bg-subtle);
  cursor: pointer; flex-shrink: 0; margin-top: 2px;
  display: flex; align-items: center; justify-content: center;
}
.evidence-checkbox.checked { background: var(--accent); border-color: var(--accent); }
.evidence-checkbox.checked::after { content: '✓'; color: #000; font-size: 12px; font-weight: 700; }
.evidence-content { flex: 1; min-width: 0; }
.evidence-id { font-size: 12px; font-weight: 600; color: var(--accent); font-family: "JetBrains Mono", monospace; }
.evidence-text { font-size: 13px; color: var(--text-secondary); margin-top: 2px; }
.evidence-stories { display: flex; gap: 4px; margin-top: 6px; flex-wrap: wrap; }

/* Story View */
.story-layout { display: flex; gap: 20px; }
.story-list {
  display: flex; flex-direction: column; gap: 4px;
  width: 280px; flex-shrink: 0; max-height: calc(100vh - 200px); overflow-y: auto;
}
@media (max-width: 1000px) {
  .story-layout { flex-direction: column; }
  .story-list { width: 100%; max-height: none; }
}
.story-list-item {
  display: flex; align-items: center; gap: 12px; min-width: 0;
  padding: 12px 14px; background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius); cursor: pointer; transition: all 0.15s;
}
.story-list-item:hover { background: var(--bg-muted); border-color: var(--border-focus); }
.story-list-item.active { background: var(--accent-bg); border-color: var(--accent-border); }
.story-list-item-main { flex: 1 1 auto; min-width: 0; overflow: hidden; }
.story-list-item-id { font-size: 12px; font-weight: 600; color: var(--accent); font-family: "JetBrains Mono", monospace; }
.story-list-item-title { font-size: 13px; color: var(--text-secondary); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.story-list-item .pill { margin-left: auto; }

.story-detail {
  flex: 1; background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius-lg); padding: 20px; min-width: 0;
}
.story-detail-header { margin-bottom: 20px; }
.story-detail-id { font-size: 12px; font-weight: 600; color: var(--accent); font-family: "JetBrains Mono", monospace; }
.story-detail-title { font-size: 18px; font-weight: 600; margin: 8px 0; }
.story-detail-section { margin-top: 24px; }
.story-detail-section-title {
  font-size: 12px; font-weight: 600; text-transform: uppercase;
  letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 12px;
}
.acceptance-criteria { list-style: none; padding: 0; margin: 0; }
.acceptance-criteria li {
  display: flex; align-items: flex-start; gap: 12px;
  padding: 10px 0; border-bottom: 1px solid var(--border);
  font-size: 14px; color: var(--text-secondary);
}
.acceptance-criteria li:last-child { border-bottom: none; }

/* Story Screen Comparison Slider */
.story-screen-compare {
  background: var(--bg-subtle); border: 1px solid var(--border);
  border-radius: var(--radius-lg); overflow: hidden; margin-bottom: 16px;
}
.story-screen-compare-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 14px; background: var(--bg-card); border-bottom: 1px solid var(--border);
}
.story-screen-compare-title { font-size: 13px; font-weight: 600; color: var(--text); }
.story-screen-compare-nav { display: flex; gap: 4px; }
.story-screen-compare-nav button {
  padding: 4px 8px; font-size: 11px; background: var(--bg-muted);
  border: 1px solid var(--border); border-radius: var(--radius-sm);
  color: var(--text-secondary); cursor: pointer; font-family: inherit;
}
.story-screen-compare-nav button:hover { background: var(--bg-accent); color: var(--text); }
.story-screen-compare-nav button:disabled { opacity: 0.5; cursor: not-allowed; }

.compare-slider {
  position: relative; aspect-ratio: 16/10; overflow: hidden;
  cursor: ew-resize; user-select: none; touch-action: none;
}
.compare-slider-reference {
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center;
}
.compare-slider-reference img { width: 100%; height: 100%; object-fit: contain; }
.compare-slider-current {
  position: absolute; inset: 0; overflow: hidden;
  clip-path: inset(0 var(--clip-right, 50%) 0 0);
}
.compare-slider-current img { width: 100%; height: 100%; object-fit: contain; }
.compare-slider-handle {
  position: absolute; top: 0; bottom: 0;
  left: calc(100% - var(--clip-right, 50%));
  width: 4px; background: var(--accent);
  cursor: ew-resize; transform: translateX(-50%);
}
.compare-slider-handle::before {
  content: ''; position: absolute; top: 50%; left: 50%;
  transform: translate(-50%, -50%);
  width: 32px; height: 32px; background: var(--accent);
  border-radius: 50%; border: 3px solid var(--bg);
}
.compare-slider-handle::after {
  content: '◀▶'; position: absolute; top: 50%; left: 50%;
  transform: translate(-50%, -50%);
  font-size: 10px; color: var(--bg); font-weight: bold;
}
.compare-slider-labels {
  position: absolute; bottom: 8px; left: 8px; right: 8px;
  display: flex; justify-content: space-between; pointer-events: none;
}
.compare-slider-label {
  padding: 4px 8px; font-size: 11px; font-weight: 600;
  background: rgba(0,0,0,0.7); border-radius: var(--radius-sm);
  color: var(--text);
}

/* Linked Screens */
.linked-screens { display: flex; gap: 12px; flex-wrap: wrap; }
.linked-screen-thumb {
  width: 140px; background: var(--bg-subtle); border: 1px solid var(--border);
  border-radius: var(--radius); overflow: hidden; cursor: pointer;
  transition: border-color 0.15s;
}
.linked-screen-thumb:hover { border-color: var(--accent-border); }
.linked-screen-thumb img { width: 100%; aspect-ratio: 16/10; object-fit: cover; display: block; }
.linked-screen-thumb-label {
  padding: 6px 8px; font-size: 11px; color: var(--text-muted);
  text-align: center; background: var(--bg-card);
}

/* Screen Navigation */
.screen-nav {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 20px; padding-top: 20px; border-top: 1px solid var(--border);
}
.screen-nav-info { font-size: 13px; color: var(--text-muted); }

/* Enhanced Lightbox */
.lightbox {
  position: fixed; inset: 0; z-index: 200;
  background: rgba(9, 9, 11, 0.98); display: none;
  flex-direction: column;
}
.lightbox.active { display: flex; }
.lightbox-toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 20px; background: var(--bg-card);
  border-bottom: 1px solid var(--border);
}
.lightbox-title { font-size: 14px; font-weight: 600; color: var(--text); }
.lightbox-controls { display: flex; gap: 4px; }
.lightbox-mode-btn {
  padding: 6px 12px; font-size: 12px; font-weight: 500;
  background: var(--bg-muted); color: var(--text-secondary);
  border: 1px solid var(--border); border-radius: var(--radius);
  cursor: pointer; transition: all 0.15s; font-family: inherit;
}
.lightbox-mode-btn:hover { background: var(--bg-accent); color: var(--text); }
.lightbox-mode-btn.active { background: var(--accent-bg); color: var(--accent); border-color: var(--accent-border); }
.lightbox-close {
  width: 36px; height: 36px; border-radius: var(--radius);
  background: var(--bg-muted); color: var(--text);
  border: 1px solid var(--border); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  font-size: 18px;
}
.lightbox-close:hover { background: var(--bg-accent); }

.lightbox-content {
  flex: 1; display: flex; align-items: center; justify-content: center;
  padding: 20px; gap: 20px; overflow: auto;
}
.lightbox-panel {
  flex: 1; max-width: 50%; max-height: 100%;
  display: flex; flex-direction: column; align-items: center;
}
.lightbox-panel-label {
  padding: 6px 12px; margin-bottom: 12px;
  font-size: 12px; font-weight: 600; text-transform: uppercase;
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius); color: var(--text-secondary);
}
.lightbox-panel img {
  max-width: 100%; max-height: calc(100vh - 160px);
  border-radius: var(--radius-lg); object-fit: contain;
}
.lightbox[data-mode="current"] .lightbox-panel[data-panel="reference"],
.lightbox[data-mode="reference"] .lightbox-panel[data-panel="current"],
.lightbox[data-mode="diff"] .lightbox-panel[data-panel="current"],
.lightbox[data-mode="diff"] .lightbox-panel[data-panel="reference"] { display: none; }
.lightbox[data-mode="current"] .lightbox-panel,
.lightbox[data-mode="reference"] .lightbox-panel,
.lightbox[data-mode="diff"] .lightbox-panel { max-width: 90%; }
.lightbox[data-mode="side-by-side"] .lightbox-panel[data-panel="diff"] { display: none; }

/* Keyboard Hints Panel */
.kbd-hints {
  position: fixed; bottom: 20px; right: 20px; z-index: 50;
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: var(--radius-lg); padding: 12px 16px;
  font-size: 12px; box-shadow: var(--shadow);
  max-width: 280px;
}
.kbd-hints-toggle {
  position: absolute; top: -10px; right: -10px;
  width: 24px; height: 24px; border-radius: 50%;
  background: var(--bg-muted); border: 1px solid var(--border);
  color: var(--text-muted); cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  font-size: 14px;
}
.kbd-hints-toggle:hover { background: var(--bg-accent); color: var(--text); }
.kbd-hints.collapsed { padding: 8px; }
.kbd-hints.collapsed .kbd-hints-content { display: none; }
.kbd-hints.collapsed .kbd-hints-toggle { position: static; width: auto; height: auto; border-radius: var(--radius); padding: 4px 8px; }
.kbd-hints-title { font-weight: 600; color: var(--text); margin-bottom: 8px; }
.kbd-hints-list { display: flex; flex-direction: column; gap: 6px; }
.kbd-hint { display: flex; align-items: center; gap: 8px; color: var(--text-secondary); }
.kbd {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 22px; padding: 2px 6px; font-size: 11px; font-weight: 500;
  font-family: "JetBrains Mono", monospace;
  background: var(--bg-muted); border: 1px solid var(--border);
  border-bottom-width: 2px; border-radius: var(--radius-sm); color: var(--text-secondary);
}

/* Empty State */
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; padding: 60px 20px; text-align: center;
}
.empty-state-icon { width: 64px; height: 64px; color: var(--text-dim); margin-bottom: 16px; }
.empty-state-title { font-size: 16px; font-weight: 600; color: var(--text); margin-bottom: 8px; }
.empty-state-description { font-size: 14px; color: var(--text-muted); max-width: 400px; }

/* Scrollbar */
::-webkit-scrollbar { width: 8px; height: 8px; }
::-webkit-scrollbar-track { background: var(--bg-subtle); }
::-webkit-scrollbar-thumb { background: var(--bg-accent); border-radius: 4px; }
::-webkit-scrollbar-thumb:hover { background: var(--border-accent); }

/* Utility */
.hidden { display: none !important; }
.flex { display: flex; }
.gap-2 { gap: 8px; }
.gap-4 { gap: 16px; }
.items-center { align-items: center; }
.justify-between { justify-content: space-between; }
.mt-4 { margin-top: 16px; }
.mb-4 { margin-bottom: 16px; }

/* Print Styles */
@media print {
  .sidebar, .kbd-hints, .lightbox, .topbar-actions, .filter-bar { display: none !important; }
  .main-content { margin-left: 0 !important; }
  .app { display: block; }
  .view-content { display: block !important; page-break-inside: avoid; margin-bottom: 40px; }
  .screen-card, .story-list-item, .metric-card { break-inside: avoid; }
  body { background: #fff; color: #000; }
  .summary-header h1, .compare-title, .story-detail-title { color: #000; }
  .pill, .badge { border-color: #ccc; }
}
`

var workflowReportTemplate = template.Must(template.New("workflow-report").Funcs(workflowReportFuncs).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{ .ScenarioLabel }} Review Report</title>
  <link rel="stylesheet" href="styles.css">
</head>
<body>
  <div class="app">
    <!-- Sidebar -->
    <aside class="sidebar" id="sidebar">
      <div class="sidebar-header">
        <a href="#" class="sidebar-logo">
          <svg class="sidebar-logo-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 9h6v6H9z"/>
          </svg>
          <span class="sidebar-logo-text">QA <span>Review</span></span>
        </a>
      </div>
      <nav class="sidebar-nav">
        <button type="button" class="sidebar-nav-item active" data-nav="overview">
          <svg class="sidebar-nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
            <rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/>
          </svg>
          <span class="sidebar-nav-label">Overview</span>
        </button>
        <button type="button" class="sidebar-nav-item" data-nav="screens">
          <svg class="sidebar-nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="3" y="3" width="18" height="18" rx="2"/>
            <circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/>
          </svg>
          <span class="sidebar-nav-label">Screens</span>
        </button>
        <button type="button" class="sidebar-nav-item" data-nav="stories">
          <svg class="sidebar-nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/>
            <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>
          </svg>
          <span class="sidebar-nav-label">Stories</span>
        </button>
      </nav>
      <div class="sidebar-footer">
        <button type="button" class="sidebar-toggle" id="sidebar-toggle">
          <svg class="sidebar-toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="11 17 6 12 11 7"/><polyline points="18 17 13 12 18 7"/>
          </svg>
          <span class="sidebar-toggle-label">Collapse</span>
        </button>
      </div>
    </aside>

    <!-- Main Content -->
    <div class="main-content">
      <div class="topbar">
        <div class="topbar-title">{{ .ScenarioLabel }}</div>
        <div class="topbar-actions">
          <button type="button" class="btn btn-sm btn-secondary" onclick="window.print()">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 6 2 18 2 18 9"/><path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"/>
              <rect x="6" y="14" width="12" height="8"/>
            </svg>
            Print / Save PDF
          </button>
        </div>
      </div>

      <main class="content">
        <!-- Summary Header -->
        <div class="summary-header">
          <h1>{{ .ScenarioLabel }}</h1>
          <p class="lede">{{ .ScenarioDescription }}</p>
          <div class="summary-meta">
            <span class="summary-meta-item">
              <span class="badge badge-{{ .ReportStatus.Level }}">{{ .ReportStatus.Label }}</span>
            </span>
            <span class="summary-meta-item">
              <span class="mono">Generated {{ .GeneratedAt }}</span>
            </span>
            <span class="summary-meta-item">
              <span class="mono">{{ .BaseURL }}</span>
            </span>
            {{ if .ReportStatus.Summary }}
            <span class="summary-meta-item">{{ .ReportStatus.Summary }}</span>
            {{ end }}
          </div>
        </div>

        <!-- Metrics -->
        <div class="metrics">
          <div class="metric-card">
            <div class="metric-label">Screens</div>
            <div class="metric-value">{{ .ScreenCount }}</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Stories</div>
            <div class="metric-value">{{ .StoryCount }}</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Review</div>
            <div class="metric-value{{ if gt .ReviewCount 0 }} success{{ end }}">{{ .ReviewCount }}</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Attention</div>
            <div class="metric-value{{ if gt .AttentionCount 0 }} warning{{ end }}">{{ .AttentionCount }}</div>
          </div>
          <div class="metric-card">
            <div class="metric-label">Missing</div>
            <div class="metric-value{{ if gt .MissingCurrent 0 }} error{{ end }}">{{ add .MissingCurrent .MissingReference }}</div>
          </div>
        </div>

        <!-- Overview View -->
        <div id="view-overview" class="view-content active">
          <!-- Filter Chips -->
          <div class="filter-bar">
            <button type="button" class="filter-chip active" data-filter="all">
              All <span class="filter-chip-count">{{ .ScreenCount }}</span>
            </button>
            <button type="button" class="filter-chip" data-filter="success">
              Ready <span class="filter-chip-count" data-count-success>0</span>
            </button>
            <button type="button" class="filter-chip" data-filter="info">
              Review <span class="filter-chip-count" data-count-info>0</span>
            </button>
            <button type="button" class="filter-chip" data-filter="warning">
              Warning <span class="filter-chip-count" data-count-warning>0</span>
            </button>
            <button type="button" class="filter-chip" data-filter="error">
              Error <span class="filter-chip-count" data-count-error>0</span>
            </button>
          </div>

          <div style="display: grid; grid-template-columns: 1fr 380px; gap: 20px;">
            <div>
              {{ if eq .ScreenCount 0 }}
              <div class="empty-state">
                <svg class="empty-state-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                  <rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 9h6v6H9z"/>
                </svg>
                <div class="empty-state-title">No Screens Captured</div>
                <div class="empty-state-description">Run the capture workflow to generate screen comparisons.</div>
              </div>
              {{ else }}
              <div class="screen-grid">
                {{ range $index, $entry := .Entries }}
                <div class="screen-card" data-screen-index="{{ $index }}" data-status="{{ $entry.Status.Level }}"
                     data-current="{{ $entry.CurrentImageRelative }}" data-reference="{{ $entry.ReferenceImageRelative }}" data-diff="{{ $entry.DiffImageRelative }}"
                     onclick="showScreen({{ $index }})">
                  <div class="screen-card-thumb">
                    {{ if $entry.CurrentImageRelative }}<img src="{{ $entry.CurrentImageRelative }}" alt="{{ $entry.Label }}">{{ end }}
                    {{ if and $entry.DiffEntry $entry.DiffEntry.Changed }}
                    <span class="screen-card-diff-indicator changed">{{ printf "%.1f%%" $entry.DiffEntry.ChangedPercent }}</span>
                    {{ else if $entry.DiffEntry }}
                    <span class="screen-card-diff-indicator unchanged">✓</span>
                    {{ end }}
                  </div>
                  <div class="screen-card-body">
                    <div class="screen-card-title">{{ $entry.Label }}</div>
                    <div class="screen-card-meta">
                      <span class="pill pill-{{ $entry.Status.Level }}">{{ $entry.Status.Label }}</span>
                    </div>
                  </div>
                </div>
                {{ end }}
              </div>
              {{ end }}
            </div>
            <div>
              {{ if gt .StoryCount 0 }}
              <div class="story-matrix">
                <div class="story-matrix-header">
                  <h3>Story Matrix</h3>
                </div>
                <table class="story-matrix-table">
                  <thead>
                    <tr><th>Story</th><th>Screens</th><th>Status</th></tr>
                  </thead>
                  <tbody>
                    {{ range .Stories }}
                    <tr onclick="showStory('{{ .StoryID }}')" style="cursor: pointer;">
                      <td><span style="color: var(--accent); font-family: 'JetBrains Mono', monospace; font-size: 12px;">{{ .StoryID }}</span></td>
                      <td>{{ len .ScreenIDs }}</td>
                      <td><span class="badge badge-{{ .Status.Level }}">{{ .Status.Label }}</span></td>
                    </tr>
                    {{ end }}
                  </tbody>
                </table>
              </div>
              {{ end }}
            </div>
          </div>
        </div>

        <!-- Screens View -->
        <div id="view-screens" class="view-content">
          {{ if eq .ScreenCount 0 }}
          <div class="empty-state">
            <svg class="empty-state-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/>
            </svg>
            <div class="empty-state-title">No Screens to Review</div>
            <div class="empty-state-description">Screens will appear here after capture.</div>
          </div>
          {{ else }}
          {{ range $index, $entry := .Entries }}
          <div class="screen-detail-view" data-screen-index="{{ $index }}" data-screen-id="{{ $entry.ScreenID }}"
               data-current="{{ $entry.CurrentImageRelative }}" data-reference="{{ $entry.ReferenceImageRelative }}" data-diff="{{ $entry.DiffImageRelative }}"
               data-compare-mode="side-by-side"{{ if ne $index 0 }} style="display: none;"{{ end }}>
            <div class="screen-detail">
              <div class="screen-detail-main">
                <div class="compare-header">
                  <div>
                    <h2 class="compare-title">{{ $entry.Label }}</h2>
                    <div class="screen-card-meta" style="margin-top: 8px;">
                      <span class="pill">{{ $entry.ScreenID }}</span>
                      <span class="pill pill-{{ $entry.Status.Level }}">{{ $entry.Status.Label }}</span>
                      {{ if and $entry.DiffEntry $entry.DiffEntry.Changed }}<span class="pill pill-info">{{ printf "%.2f%%" $entry.DiffEntry.ChangedPercent }} changed</span>{{ end }}
                      {{ range $entry.PrimaryStories }}<span class="pill pill-accent">{{ .ID }}</span>{{ end }}
                    </div>
                  </div>
                  <div class="compare-controls">
                    <button type="button" class="compare-mode-btn active" data-screen-index="{{ $index }}" data-mode="side-by-side">Side by Side</button>
                    <button type="button" class="compare-mode-btn" data-screen-index="{{ $index }}" data-mode="diff-only">Diff Only</button>
                    <button type="button" class="btn btn-sm btn-secondary" onclick="openLightboxForScreen({{ $index }})">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/>
                        <line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/>
                      </svg>
                      Expand
                    </button>
                  </div>
                </div>

                {{ if $entry.Warnings }}
                <div class="warning-block">
                  <div class="warning-block-title">{{ len $entry.Warnings }} Warning(s)</div>
                  <ul class="warning-block-list">
                    {{ range $entry.Warnings }}<li><strong>{{ .Code }}</strong>: {{ .Message }}</li>{{ end }}
                  </ul>
                </div>
                {{ end }}

                <div class="compare-grid">
                  <div class="compare-panel" data-panel-kind="current">
                    <div class="compare-panel-header"><span class="compare-panel-title">Current</span></div>
                    <div class="compare-panel-body">
                      {{ if $entry.CurrentImageRelative }}<img src="{{ $entry.CurrentImageRelative }}" alt="Current" onclick="openLightboxForScreen({{ $index }}, 'current')">
                      {{ else }}<div class="missing">Current capture missing</div>{{ end }}
                    </div>
                  </div>
                  <div class="compare-panel" data-panel-kind="reference">
                    <div class="compare-panel-header"><span class="compare-panel-title">Reference</span></div>
                    <div class="compare-panel-body">
                      {{ if $entry.ReferenceImageRelative }}<img src="{{ $entry.ReferenceImageRelative }}" alt="Reference" onclick="openLightboxForScreen({{ $index }}, 'reference')">
                      {{ else }}<div class="missing">Reference missing</div>{{ end }}
                    </div>
                  </div>
                  <div class="compare-panel" data-panel-kind="diff">
                    <div class="compare-panel-header"><span class="compare-panel-title">Diff</span></div>
                    <div class="compare-panel-body">
                      {{ if $entry.DiffImageRelative }}<img src="{{ $entry.DiffImageRelative }}" alt="Diff" onclick="openLightboxForScreen({{ $index }}, 'diff')">
                      {{ else }}<div class="missing">No diff</div>{{ end }}
                    </div>
                  </div>
                </div>

                <div class="screen-nav">
                  <div class="screen-nav-info">Screen {{ add $index 1 }} of {{ $.ScreenCount }}</div>
                  <div class="flex gap-2">
                    {{ if gt $index 0 }}<button class="btn btn-secondary btn-sm" onclick="showScreen({{ sub $index 1 }})">← Prev</button>{{ end }}
                    {{ if lt (add $index 1) $.ScreenCount }}<button class="btn btn-secondary btn-sm" onclick="showScreen({{ add $index 1 }})">Next →</button>{{ end }}
                  </div>
                </div>
              </div>

              <div class="screen-detail-sidebar">
                {{ if $entry.ExpectedElements }}
                <div class="evidence-panel">
                  <div class="evidence-panel-header">
                    <span class="evidence-panel-title">Expected <span class="evidence-panel-count">{{ len $entry.ExpectedElements }}</span></span>
                  </div>
                  <div class="evidence-panel-body">
                    {{ range $entry.ExpectedElements }}
                    <div class="evidence-item">
                      <div class="evidence-checkbox" onclick="this.classList.toggle('checked')"></div>
                      <div class="evidence-content"><div class="evidence-text">{{ . }}</div></div>
                    </div>
                    {{ end }}
                  </div>
                </div>
                {{ end }}

                {{ if $entry.Evidence }}
                <div class="evidence-panel">
                  <div class="evidence-panel-header">
                    <span class="evidence-panel-title">Evidence <span class="evidence-panel-count">{{ len $entry.Evidence }}</span></span>
                  </div>
                  <div class="evidence-panel-body">
                    {{ range $entry.Evidence }}
                    <div class="evidence-item">
                      <div class="evidence-checkbox" onclick="this.classList.toggle('checked')"></div>
                      <div class="evidence-content">
                        <div class="evidence-id">{{ .ID }}</div>
                        <div class="evidence-text">{{ .Text }}</div>
                      </div>
                    </div>
                    {{ end }}
                  </div>
                </div>
                {{ end }}

                {{ if $entry.Notes }}
                <div class="evidence-panel">
                  <div class="evidence-panel-header">
                    <span class="evidence-panel-title">Notes</span>
                  </div>
                  <div class="evidence-panel-body">
                    {{ range $entry.Notes }}
                    <div class="evidence-item">
                      <div class="evidence-content"><div class="evidence-text">{{ . }}</div></div>
                    </div>
                    {{ end }}
                  </div>
                </div>
                {{ end }}

                {{ if $entry.Annotations }}
                <div class="evidence-panel">
                  <div class="evidence-panel-header">
                    <span class="evidence-panel-title">Annotations</span>
                  </div>
                  <div class="evidence-panel-body">
                    {{ range $entry.Annotations }}
                    <div class="evidence-item">
                      <div class="evidence-content"><div class="evidence-text">{{ . }}</div></div>
                    </div>
                    {{ end }}
                  </div>
                </div>
                {{ end }}
              </div>
            </div>
          </div>
          {{ end }}
          {{ end }}
        </div>

        <!-- Stories View -->
        <div id="view-stories" class="view-content">
          {{ if eq .StoryCount 0 }}
          <div class="empty-state">
            <svg class="empty-state-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>
            </svg>
            <div class="empty-state-title">No Stories Defined</div>
            <div class="empty-state-description">Add stories to your scenario to track acceptance criteria.</div>
          </div>
          {{ else }}
          <div class="story-layout">
            <div class="story-list">
              {{ range $index, $story := .Stories }}
              <div class="story-list-item{{ if eq $index 0 }} active{{ end }}" data-story="{{ $story.StoryID }}" onclick="selectStory('{{ $story.StoryID }}')">
                <div class="story-list-item-main">
                  <div class="story-list-item-id">{{ $story.StoryID }}</div>
                  <div class="story-list-item-title">{{ $story.Story.Title }}</div>
                </div>
                <span class="pill pill-{{ $story.Status.Level }}">{{ $story.Status.Label }}</span>
              </div>
              {{ end }}
            </div>
            <div style="flex: 1; min-width: 0;">
              {{ range $storyIndex, $story := .Stories }}
              <div class="story-detail" data-story-detail="{{ $story.StoryID }}" data-story-index="{{ $storyIndex }}"{{ if ne $storyIndex 0 }} style="display: none;"{{ end }}>
                <div class="story-detail-header">
                  <div class="story-detail-id">{{ $story.StoryID }}</div>
                  <h2 class="story-detail-title">{{ $story.Story.Title }}</h2>
                  <div class="screen-card-meta">
                    <span class="badge badge-{{ $story.Status.Level }}">{{ $story.Status.Label }}</span>
                  </div>
                </div>

                {{ if $story.ScreenIDs }}
                <div class="story-detail-section" data-story-compare-section>
                  <div class="story-detail-section-title">Screen Comparison</div>
                  <div class="story-screen-compare" data-story="{{ $story.StoryID }}">
                    <div class="story-screen-compare-header">
                      <span class="story-screen-compare-title" data-screen-label>Select a screen</span>
                      <div class="story-screen-compare-nav">
                        <button type="button" data-action="prev-screen" disabled>←</button>
                        <span data-screen-counter>0/0</span>
                        <button type="button" data-action="next-screen" disabled>→</button>
                      </div>
                    </div>
                    <div class="compare-slider" data-slider style="--clip-right: 50%">
                      <div class="compare-slider-reference"><img src="" alt="Reference" data-img-reference></div>
                      <div class="compare-slider-current"><img src="" alt="Current" data-img-current></div>
                      <div class="compare-slider-handle"></div>
                      <div class="compare-slider-labels">
                        <span class="compare-slider-label">Current</span>
                        <span class="compare-slider-label">Reference</span>
                      </div>
                    </div>
                  </div>
                  <div class="linked-screens" style="margin-top: 12px;">
                    {{ range $screenIdx, $screenID := $story.ScreenIDs }}
                    <div class="linked-screen-thumb" data-screen-id="{{ $screenID }}" data-thumb-index="{{ $screenIdx }}" onclick="setStoryCompareScreen('{{ $story.StoryID }}', {{ $screenIdx }})">
                      <img src="" alt="{{ $screenID }}" data-thumb-img>
                      <div class="linked-screen-thumb-label">{{ $screenID }}</div>
                    </div>
                    {{ end }}
                  </div>
                </div>
                {{ end }}

                {{ if $story.Story.AcceptanceCriteria }}
                <div class="story-detail-section">
                  <div class="story-detail-section-title">Acceptance Criteria</div>
                  <ul class="acceptance-criteria">
                    {{ range $story.Story.AcceptanceCriteria }}
                    <li>
                      <div class="evidence-checkbox" onclick="this.classList.toggle('checked')"></div>
                      <span>{{ . }}</span>
                    </li>
                    {{ end }}
                  </ul>
                </div>
                {{ end }}

                {{ if $story.MissingPaths }}
                <div class="warning-block">
                  <div class="warning-block-title">Missing Assets</div>
                  <ul class="warning-block-list">
                    {{ range $story.MissingPaths }}<li>{{ . }}</li>{{ end }}
                  </ul>
                </div>
                {{ end }}
              </div>
              {{ end }}
            </div>
          </div>
          {{ end }}
        </div>
      </main>
    </div>
  </div>

  <!-- Enhanced Lightbox -->
  <div class="lightbox" id="lightbox" data-mode="side-by-side">
    <div class="lightbox-toolbar">
      <div class="lightbox-title" id="lightbox-title">Screen Comparison</div>
      <div class="lightbox-controls">
        <button type="button" class="lightbox-mode-btn" data-lb-mode="current">Current</button>
        <button type="button" class="lightbox-mode-btn" data-lb-mode="reference">Reference</button>
        <button type="button" class="lightbox-mode-btn active" data-lb-mode="side-by-side">Side by Side</button>
        <button type="button" class="lightbox-mode-btn" data-lb-mode="diff">Diff</button>
      </div>
      <button type="button" class="lightbox-close" onclick="closeLightbox()">×</button>
    </div>
    <div class="lightbox-content">
      <div class="lightbox-panel" data-panel="current">
        <div class="lightbox-panel-label">Current</div>
        <img id="lightbox-current" src="" alt="Current">
      </div>
      <div class="lightbox-panel" data-panel="reference">
        <div class="lightbox-panel-label">Reference</div>
        <img id="lightbox-reference" src="" alt="Reference">
      </div>
      <div class="lightbox-panel" data-panel="diff">
        <div class="lightbox-panel-label">Diff</div>
        <img id="lightbox-diff" src="" alt="Diff">
      </div>
    </div>
  </div>

  <!-- Keyboard Hints -->
  <div class="kbd-hints" id="kbd-hints">
    <button type="button" class="kbd-hints-toggle" onclick="toggleKbdHints()">?</button>
    <div class="kbd-hints-content">
      <div class="kbd-hints-title">Keyboard Shortcuts</div>
      <div class="kbd-hints-list">
        <div class="kbd-hint"><kbd class="kbd">1</kbd> Overview</div>
        <div class="kbd-hint"><kbd class="kbd">2</kbd> Screens</div>
        <div class="kbd-hint"><kbd class="kbd">3</kbd> Stories</div>
        <div class="kbd-hint"><kbd class="kbd">←</kbd><kbd class="kbd">→</kbd> Navigate screens</div>
        <div class="kbd-hint"><kbd class="kbd">Esc</kbd> Close lightbox</div>
        <div class="kbd-hint"><kbd class="kbd">C</kbd><kbd class="kbd">R</kbd><kbd class="kbd">S</kbd><kbd class="kbd">D</kbd> Lightbox modes</div>
      </div>
    </div>
  </div>

  <script>
    // State
    const sidebar = document.getElementById('sidebar');
    const lightbox = document.getElementById('lightbox');
    const screenCards = document.querySelectorAll('.screen-card');
    const screenDetailViews = document.querySelectorAll('.screen-detail-view');
    const filterChips = document.querySelectorAll('.filter-chip');
    let activeScreenIndex = 0;
    let activeFilter = 'all';

    // Sidebar
    const sidebarCollapsed = localStorage.getItem('sidebar-collapsed') === 'true';
    if (sidebarCollapsed) sidebar.classList.add('collapsed');

    document.getElementById('sidebar-toggle').addEventListener('click', () => {
      sidebar.classList.toggle('collapsed');
      localStorage.setItem('sidebar-collapsed', sidebar.classList.contains('collapsed'));
    });

    // Navigation
    function switchView(viewName) {
      document.querySelectorAll('.sidebar-nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.nav === viewName);
      });
      document.querySelectorAll('.view-content').forEach(content => {
        content.classList.toggle('active', content.id === 'view-' + viewName);
      });
    }

    document.querySelectorAll('.sidebar-nav-item').forEach(item => {
      item.addEventListener('click', () => switchView(item.dataset.nav));
    });

    // Filter chips
    function updateFilterCounts() {
      const counts = { success: 0, info: 0, warning: 0, error: 0 };
      screenCards.forEach(card => {
        const status = card.dataset.status;
        if (counts[status] !== undefined) counts[status]++;
      });
      Object.keys(counts).forEach(key => {
        const el = document.querySelector('[data-count-' + key + ']');
        if (el) el.textContent = counts[key];
      });
    }
    updateFilterCounts();

    function applyFilter(filter) {
      activeFilter = filter;
      filterChips.forEach(chip => chip.classList.toggle('active', chip.dataset.filter === filter));
      screenCards.forEach(card => {
        const show = filter === 'all' || card.dataset.status === filter;
        card.classList.toggle('filter-hidden', !show);
      });
    }

    filterChips.forEach(chip => {
      chip.addEventListener('click', () => applyFilter(chip.dataset.filter));
    });

    // Screen navigation
    function showScreen(index) {
      if (index < 0 || index >= screenDetailViews.length) return;
      activeScreenIndex = index;
      switchView('screens');
      screenDetailViews.forEach((el, i) => el.style.display = i === index ? 'block' : 'none');
      screenCards.forEach((card, i) => card.classList.toggle('active', i === index));
    }

    function showScreenByID(screenID) {
      const view = Array.from(screenDetailViews).find(el => el.dataset.screenId === screenID);
      if (view) showScreen(Number.parseInt(view.dataset.screenIndex, 10));
    }

    function setCompareMode(index, mode) {
      const view = screenDetailViews[index];
      if (!view) return;
      view.dataset.compareMode = mode;
      view.querySelectorAll('.compare-mode-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.mode === mode);
      });
    }

    document.querySelectorAll('.compare-mode-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        const index = Number.parseInt(btn.dataset.screenIndex, 10);
        if (!Number.isNaN(index)) setCompareMode(index, btn.dataset.mode);
      });
    });

    // Story selection
    function selectStory(storyID) {
      document.querySelectorAll('.story-list-item').forEach(el => {
        el.classList.toggle('active', el.dataset.story === storyID);
      });
      document.querySelectorAll('[data-story-detail]').forEach(el => {
        el.style.display = el.dataset.storyDetail === storyID ? 'block' : 'none';
      });
      initStoryCompare(storyID);
    }

    function showStory(storyID) {
      switchView('stories');
      selectStory(storyID);
    }

    // Story comparison slider - build screen data from detail views
    const storyScreenData = {};
    screenDetailViews.forEach(view => {
      const id = view.dataset.screenId;
      if (id) {
        storyScreenData[id] = {
          current: view.dataset.current || '',
          reference: view.dataset.reference || '',
          diff: view.dataset.diff || ''
        };
      }
    });

    // Track active story compare screen index per story
    const storyCompareIndex = {};

    function initStoryCompare(storyID) {
      const section = document.querySelector('.story-detail[data-story-detail="' + storyID + '"] [data-story-compare-section]');
      if (!section) return;
      const container = section.querySelector('.story-screen-compare');
      const thumbs = section.querySelectorAll('.linked-screen-thumb');
      if (!container || thumbs.length === 0) return;

      // Populate thumbnails with images
      thumbs.forEach((thumb, idx) => {
        const screenID = thumb.dataset.screenId;
        const data = storyScreenData[screenID];
        const img = thumb.querySelector('[data-thumb-img]');
        if (img) {
          img.src = data ? data.current : '';
          img.alt = screenID;
        }
      });

      const storedIndex = storyCompareIndex[storyID];
      const initialIndex = Number.isInteger(storedIndex) && storedIndex >= 0 && storedIndex < thumbs.length
        ? storedIndex
        : 0;

      storyCompareIndex[storyID] = initialIndex;
      setStoryCompareScreen(storyID, initialIndex);
    }

    function setStoryCompareScreen(storyID, index) {
      const section = document.querySelector('.story-detail[data-story-detail="' + storyID + '"] [data-story-compare-section]');
      if (!section) return;
      const container = section.querySelector('.story-screen-compare');
      const thumbs = section.querySelectorAll('.linked-screen-thumb');
      if (!container || index < 0 || index >= thumbs.length) return;

      const screenID = thumbs[index].dataset.screenId;
      const data = storyScreenData[screenID];
      storyCompareIndex[storyID] = index;

      // Update slider images
      const imgCurrent = container.querySelector('[data-img-current]');
      const imgReference = container.querySelector('[data-img-reference]');
      if (imgCurrent) imgCurrent.src = data ? data.current : '';
      if (imgReference) imgReference.src = data ? data.reference : '';

      // Update header
      const labelEl = container.querySelector('[data-screen-label]');
      const counterEl = container.querySelector('[data-screen-counter]');
      if (labelEl) labelEl.textContent = screenID || 'Unknown';
      if (counterEl) counterEl.textContent = (index + 1) + '/' + thumbs.length;

      // Update nav buttons
      const prevBtn = container.querySelector('[data-action="prev-screen"]');
      const nextBtn = container.querySelector('[data-action="next-screen"]');
      if (prevBtn) {
        prevBtn.disabled = index === 0;
        prevBtn.onclick = () => setStoryCompareScreen(storyID, index - 1);
      }
      if (nextBtn) {
        nextBtn.disabled = index >= thumbs.length - 1;
        nextBtn.onclick = () => setStoryCompareScreen(storyID, index + 1);
      }

      // Highlight active thumbnail
      thumbs.forEach((t, i) => {
        t.style.borderColor = i === index ? 'var(--accent)' : '';
        t.style.boxShadow = i === index ? '0 0 0 2px var(--accent-border)' : '';
      });

      // Reset slider position
      const slider = container.querySelector('.compare-slider');
      if (slider) slider.style.setProperty('--clip-right', '50%');
    }

    // Slider drag functionality
    function initSliders() {
      document.querySelectorAll('.compare-slider').forEach(slider => {
        let isDragging = false;

        const updateSlider = (clientX) => {
          const rect = slider.getBoundingClientRect();
          if (rect.width === 0) return;
          const x = Math.max(0, Math.min(rect.width, clientX - rect.left));
          const percent = 100 - (x / rect.width) * 100;
          slider.style.setProperty('--clip-right', percent.toFixed(1) + '%');
        };

        const onMouseDown = (e) => {
          e.preventDefault();
          isDragging = true;
          updateSlider(e.clientX);
        };

        const onMouseMove = (e) => {
          if (!isDragging) return;
          e.preventDefault();
          updateSlider(e.clientX);
        };

        const onMouseUp = () => {
          isDragging = false;
        };

        // Mouse events
        slider.addEventListener('mousedown', onMouseDown);
        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);

        // Touch events for mobile
        slider.addEventListener('touchstart', (e) => {
          isDragging = true;
          updateSlider(e.touches[0].clientX);
        }, { passive: true });
        document.addEventListener('touchmove', (e) => {
          if (!isDragging) return;
          updateSlider(e.touches[0].clientX);
        }, { passive: true });
        document.addEventListener('touchend', onMouseUp);
      });
    }
    initSliders();

    // Enhanced Lightbox
    function openLightboxForScreen(index, mode = 'side-by-side') {
      const view = screenDetailViews[index];
      if (!view) return;
      const title = view.querySelector('.compare-title')?.textContent || 'Screen';
      document.getElementById('lightbox-title').textContent = title;
      document.getElementById('lightbox-current').src = view.dataset.current || '';
      document.getElementById('lightbox-reference').src = view.dataset.reference || '';
      document.getElementById('lightbox-diff').src = view.dataset.diff || '';
      setLightboxMode(mode);
      lightbox.classList.add('active');
    }

    function setLightboxMode(mode) {
      lightbox.dataset.mode = mode;
      document.querySelectorAll('.lightbox-mode-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.lbMode === mode);
      });
    }

    document.querySelectorAll('.lightbox-mode-btn').forEach(btn => {
      btn.addEventListener('click', () => setLightboxMode(btn.dataset.lbMode));
    });

    function closeLightbox() {
      lightbox.classList.remove('active');
    }

    lightbox.addEventListener('click', (e) => {
      if (e.target === lightbox || e.target.classList.contains('lightbox-content')) closeLightbox();
    });

    // Keyboard hints
    function toggleKbdHints() {
      document.getElementById('kbd-hints').classList.toggle('collapsed');
    }

    // Keyboard navigation
    document.addEventListener('keydown', (e) => {
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

      if (e.key === 'Escape') { closeLightbox(); return; }

      if (lightbox.classList.contains('active')) {
        if (e.key === 'c' || e.key === 'C') setLightboxMode('current');
        if (e.key === 'r' || e.key === 'R') setLightboxMode('reference');
        if (e.key === 's' || e.key === 'S') setLightboxMode('side-by-side');
        if (e.key === 'd' || e.key === 'D') setLightboxMode('diff');
        return;
      }

      if (e.key === '1') switchView('overview');
      if (e.key === '2') switchView('screens');
      if (e.key === '3') switchView('stories');
      if (e.key === 'ArrowLeft' && document.getElementById('view-screens').classList.contains('active')) {
        showScreen(Math.max(0, activeScreenIndex - 1));
      }
      if (e.key === 'ArrowRight' && document.getElementById('view-screens').classList.contains('active')) {
        showScreen(Math.min(screenDetailViews.length - 1, activeScreenIndex + 1));
      }
    });

    // Initialize
    if (screenDetailViews.length > 0) {
      showScreen(0);
      setCompareMode(0, 'side-by-side');
    }
    document.querySelectorAll('[data-story-detail]').forEach((el, i) => {
      if (i === 0) initStoryCompare(el.dataset.storyDetail);
    });
  </script>
</body>
</html>`))

func (s *Service) GenerateWorkflowReport(ctx context.Context, req WorkflowReportRequest) (WorkflowReportResult, error) {
	if s == nil {
		return WorkflowReportResult{}, newCaptureError(CodeCapture, "generate_workflow_report", "webcap service is not configured", nil)
	}
	scenario := req.Scenario
	if err := normalizeWorkflowScenario(&scenario, s.workflow); err != nil {
		return WorkflowReportResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return WorkflowReportResult{}, wrapCaptureError("generate_workflow_report", err)
	}

	reportPath := filepath.Join(scenario.Artifacts.ReportDir, "index.html")
	metadataPath := filepath.Join(scenario.Artifacts.ReportDir, "report.json")
	stylesheetPath := filepath.Join(scenario.Artifacts.ReportDir, "styles.css")

	entries := make([]WorkflowReportEntry, 0, len(scenario.Screens))
	for _, screen := range scenario.Screens {
		entry, err := s.buildWorkflowReportEntry(ctx, scenario, screen)
		if err != nil {
			return WorkflowReportResult{}, err
		}
		entries = append(entries, entry)
	}
	stories := buildWorkflowStoryReports(entries, scenario.Stories)

	result := WorkflowReportResult{
		ScenarioID:   scenario.ID,
		ScenarioPath: scenario.SourcePath,
		ReportFormat: scenario.Environment.ReportFormat,
		ReportPath:   reportPath,
		MetadataPath: metadataPath,
		CurrentDir:   scenario.Artifacts.CurrentDir,
		DiffDir:      scenario.Artifacts.DiffDir,
		Entries:      entries,
		Stories:      stories,
		CreatedAt:    s.now(),
	}
	result.Status = workflowReviewStatusForReport(result.Entries)

	reportView, err := stageWorkflowReportAssets(scenario, result)
	if err != nil {
		return WorkflowReportResult{}, err
	}
	var rendered bytes.Buffer
	if renderErr := workflowReportTemplate.Execute(&rendered, reportView); renderErr != nil {
		return WorkflowReportResult{}, wrapCaptureError("generate_workflow_report_html", renderErr)
	}
	if writeErr := writeFile(reportPath, rendered.Bytes()); writeErr != nil {
		return WorkflowReportResult{}, writeErr
	}
	if writeErr := writeFile(stylesheetPath, []byte(workflowReportStylesheet)); writeErr != nil {
		return WorkflowReportResult{}, writeErr
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return WorkflowReportResult{}, wrapCaptureError("generate_workflow_report_json", err)
	}
	if err := writeFile(metadataPath, append(encoded, '\n')); err != nil {
		return WorkflowReportResult{}, err
	}
	return result, nil
}

func (s *Service) buildWorkflowReportEntry(ctx context.Context, scenario WorkflowScenario, screen WorkflowScreen) (WorkflowReportEntry, error) {
	targetURL, err := workflowScreenURL(scenario, screen, s.workflow)
	if err != nil {
		return WorkflowReportEntry{}, err
	}
	currentPath := filepath.Join(scenario.Artifacts.CurrentDir, screen.OutputName+"."+defaultImageFormat)
	currentMetadata := currentPath + ".json"
	entry := WorkflowReportEntry{
		ScreenID:          screen.ID,
		Label:             screen.Label,
		Route:             screen.Route,
		TargetURL:         targetURL,
		CurrentImagePath:  currentPath,
		CurrentMetadata:   currentMetadata,
		ReferenceImage:    screen.ReferenceImage,
		ExpectedElements:  append([]string(nil), screen.ExpectedElements...),
		Evidence:          append([]WorkflowEvidenceItem(nil), screen.Evidence...),
		Notes:             append([]string(nil), screen.Notes...),
		Annotations:       append([]string(nil), screen.Annotations...),
		PrimaryStories:    workflowStoriesForIDs(scenario.Stories, screen.PrimaryStories),
		SupportingStories: workflowStoriesForIDs(scenario.Stories, screen.SupportingStories),
	}
	if _, statErr := os.Stat(currentPath); statErr != nil {
		entry.MissingCurrent = true
		entry.Warnings = append(entry.Warnings, CaptureWarning{Code: string(CodeValidation), Message: "current capture image is missing"})
	}
	if _, statErr := os.Stat(screen.ReferenceImage); statErr != nil {
		entry.MissingReference = true
		entry.Warnings = append(entry.Warnings, CaptureWarning{Code: string(CodeValidation), Message: "reference image is missing"})
	}
	if !entry.MissingCurrent {
		if payload, readErr := os.ReadFile(currentMetadata); readErr == nil {
			var capture CaptureResult
			if unmarshalErr := json.Unmarshal(payload, &capture); unmarshalErr == nil {
				entry.CurrentCapture = &capture
				entry.Warnings = append(entry.Warnings, capture.Warnings...)
			}
		}
	}
	if entry.MissingCurrent || entry.MissingReference {
		entry.Status = workflowReviewStatusForEntry(entry)
		return entry, nil
	}

	comparison := screen.Comparison
	entry.ComparisonMode = comparison.Mode
	comparedCurrentPath, comparedReferencePath, err := prepareWorkflowComparisonImages(entry, comparison, scenario.Artifacts.DiffDir)
	if err != nil {
		entry.Warnings = append(entry.Warnings, errorWarning(err))
		comparedCurrentPath = entry.CurrentImagePath
		comparedReferencePath = entry.ReferenceImage
	}
	entry.ComparedCurrentImagePath = comparedCurrentPath
	entry.ComparedReferenceImagePath = comparedReferencePath

	diffPath := filepath.Join(scenario.Artifacts.DiffDir, screen.OutputName+"."+defaultImageFormat)
	diffMetadata := diffPath + ".json"
	diffResult, err := s.Diff(ctx, DiffRequest{
		BasePath:     entry.ComparedCurrentImagePath,
		ComparePath:  entry.ComparedReferenceImagePath,
		OutputPath:   diffPath,
		MetadataPath: diffMetadata,
	})
	if err != nil {
		entry.Warnings = append(entry.Warnings, errorWarning(err))
		entry.Status = workflowReviewStatusForEntry(entry)
		return entry, nil
	}
	entry.DiffImagePath = diffResult.OutputPath
	entry.DiffMetadataPath = diffResult.MetadataPath
	entry.DiffEntry = diffResult.Entry
	entry.DiffSummary = &diffResult.Summary
	entry.Status = workflowReviewStatusForEntry(entry)
	return entry, nil
}

func workflowStoriesForIDs(index map[string]WorkflowStory, ids []string) []WorkflowStory {
	if len(ids) == 0 {
		return nil
	}
	out := make([]WorkflowStory, 0, len(ids))
	for _, id := range ids {
		story, ok := index[id]
		if !ok {
			continue
		}
		out = append(out, story)
	}
	return out
}

func buildWorkflowStoryReports(entries []WorkflowReportEntry, stories map[string]WorkflowStory) []WorkflowStoryReport {
	keys := make([]string, 0, len(stories))
	for key := range stories {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	entryIndex := make(map[string]WorkflowReportEntry, len(entries))
	for _, entry := range entries {
		entryIndex[entry.ScreenID] = entry
	}
	out := make([]WorkflowStoryReport, 0, len(keys))
	for _, key := range keys {
		report := WorkflowStoryReport{
			StoryID: key,
			Story:   stories[key],
		}
		screenSeen := map[string]struct{}{}
		evidenceSeen := map[string]struct{}{}
		for _, entry := range entries {
			for _, story := range append(append([]WorkflowStory(nil), entry.PrimaryStories...), entry.SupportingStories...) {
				if story.ID != key {
					continue
				}
				if _, exists := screenSeen[entry.ScreenID]; !exists {
					report.ScreenIDs = append(report.ScreenIDs, entry.ScreenID)
					screenSeen[entry.ScreenID] = struct{}{}
				}
				if entry.MissingCurrent {
					report.MissingPaths = append(report.MissingPaths, entry.ScreenID+":current")
				}
				if entry.MissingReference {
					report.MissingPaths = append(report.MissingPaths, entry.ScreenID+":reference")
				}
			}
			for _, item := range entry.Evidence {
				if !slices.Contains(item.Stories, key) {
					continue
				}
				if _, exists := evidenceSeen[item.ID]; exists {
					continue
				}
				report.Evidence = append(report.Evidence, item)
				evidenceSeen[item.ID] = struct{}{}
			}
		}
		report.Status = workflowReviewStatusForStory(report, entryIndex)
		out = append(out, report)
	}
	return out
}

type workflowReportTemplateView struct {
	ScenarioLabel       string
	ScenarioDescription string
	BaseURL             string
	GeneratedAt         string
	ScreenCount         int
	StoryCount          int
	ReviewCount         int
	AttentionCount      int
	MissingCurrent      int
	MissingReference    int
	ReportStatus        WorkflowReviewStatus
	Entries             []workflowReportTemplateEntry
	Stories             []WorkflowStoryReport
}

type workflowReportTemplateEntry struct {
	WorkflowReportEntry
	CurrentImageRelative   string
	ReferenceImageRelative string
	DiffImageRelative      string
}

func stageWorkflowReportAssets(scenario WorkflowScenario, result WorkflowReportResult) (workflowReportTemplateView, error) {
	view := workflowReportTemplateView{
		ScenarioLabel:       scenario.Label,
		ScenarioDescription: scenario.Description,
		BaseURL:             scenario.Environment.BaseURL,
		GeneratedAt:         result.CreatedAt.Format("Jan 2, 2006 at 3:04 PM"),
		ScreenCount:         len(result.Entries),
		StoryCount:          len(result.Stories),
		ReportStatus:        result.Status,
		Stories:             result.Stories,
	}
	reportDir := filepath.Dir(result.ReportPath)
	view.Entries = make([]workflowReportTemplateEntry, 0, len(result.Entries))
	for _, entry := range result.Entries {
		if entry.MissingCurrent {
			view.MissingCurrent++
		}
		if entry.MissingReference {
			view.MissingReference++
		}
		switch entry.Status.Level {
		case workflowStatusInfo:
			view.ReviewCount++
		case workflowStatusWarning, workflowStatusError:
			view.AttentionCount++
		}
		currentRelative, err := stageWorkflowReportAsset(reportDir, "current", firstNonEmpty(entry.ComparedCurrentImagePath, entry.CurrentImagePath))
		if err != nil {
			return workflowReportTemplateView{}, err
		}
		referenceRelative, err := stageWorkflowReportAsset(reportDir, "reference", firstNonEmpty(entry.ComparedReferenceImagePath, entry.ReferenceImage))
		if err != nil {
			return workflowReportTemplateView{}, err
		}
		diffRelative, err := stageWorkflowReportAsset(reportDir, "diff", entry.DiffImagePath)
		if err != nil {
			return workflowReportTemplateView{}, err
		}
		view.Entries = append(view.Entries, workflowReportTemplateEntry{
			WorkflowReportEntry:    entry,
			CurrentImageRelative:   currentRelative,
			ReferenceImageRelative: referenceRelative,
			DiffImageRelative:      diffRelative,
		})
	}
	return view, nil
}

func workflowReviewStatusForEntry(entry WorkflowReportEntry) WorkflowReviewStatus {
	switch {
	case entry.MissingCurrent || entry.MissingReference:
		return WorkflowReviewStatus{
			Level:   workflowStatusError,
			Label:   "Missing Assets",
			Summary: "This screen is missing one or more required images.",
		}
	case len(entry.Warnings) > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusWarning,
			Label:   "Capture Issues",
			Summary: fmt.Sprintf("%d warning(s) require attention.", len(entry.Warnings)),
		}
	case entry.DiffEntry != nil && entry.DiffEntry.Changed:
		return WorkflowReviewStatus{
			Level:   workflowStatusInfo,
			Label:   "Needs Review",
			Summary: fmt.Sprintf("%.2f%% of compared pixels changed.", entry.DiffEntry.ChangedPercent),
		}
	default:
		return WorkflowReviewStatus{
			Level:   workflowStatusSuccess,
			Label:   "Ready",
			Summary: "No capture warnings or visual diffs detected.",
		}
	}
}

func workflowReviewStatusForStory(report WorkflowStoryReport, entryIndex map[string]WorkflowReportEntry) WorkflowReviewStatus {
	if len(report.MissingPaths) > 0 {
		return WorkflowReviewStatus{
			Level:   workflowStatusError,
			Label:   "Missing Assets",
			Summary: fmt.Sprintf("%d linked asset gap(s) require attention.", len(report.MissingPaths)),
		}
	}

	infoCount := 0
	warningCount := 0
	errorCount := 0
	for _, screenID := range report.ScreenIDs {
		entry, ok := entryIndex[screenID]
		if !ok {
			continue
		}
		switch entry.Status.Level {
		case workflowStatusError:
			errorCount++
		case workflowStatusWarning:
			warningCount++
		case workflowStatusInfo:
			infoCount++
		}
	}

	switch {
	case errorCount > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusError,
			Label:   "Missing Assets",
			Summary: fmt.Sprintf("%d linked screen(s) have blocking asset issues.", errorCount),
		}
	case warningCount > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusWarning,
			Label:   "Capture Issues",
			Summary: fmt.Sprintf("%d linked screen(s) have capture warnings.", warningCount),
		}
	case infoCount > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusInfo,
			Label:   "Needs Review",
			Summary: fmt.Sprintf("%d linked screen(s) show visual changes.", infoCount),
		}
	default:
		return WorkflowReviewStatus{
			Level:   workflowStatusSuccess,
			Label:   "Ready",
			Summary: "All linked screens are present and diff clean.",
		}
	}
}

func workflowReviewStatusForReport(entries []WorkflowReportEntry) WorkflowReviewStatus {
	reviewCount := 0
	attentionCount := 0
	missingCount := 0
	for _, entry := range entries {
		switch entry.Status.Level {
		case workflowStatusInfo:
			reviewCount++
		case workflowStatusWarning:
			attentionCount++
		case workflowStatusError:
			attentionCount++
			missingCount++
		}
	}

	switch {
	case missingCount > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusError,
			Label:   "Attention Required",
			Summary: fmt.Sprintf("%d screen(s) need attention, including %d with missing assets.", attentionCount, missingCount),
		}
	case attentionCount > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusWarning,
			Label:   "Attention Required",
			Summary: fmt.Sprintf("%d screen(s) have capture warnings.", attentionCount),
		}
	case reviewCount > 0:
		return WorkflowReviewStatus{
			Level:   workflowStatusInfo,
			Label:   "Needs Review",
			Summary: fmt.Sprintf("%d screen(s) show visual changes.", reviewCount),
		}
	default:
		return WorkflowReviewStatus{
			Level:   workflowStatusSuccess,
			Label:   "Ready",
			Summary: "No capture warnings, missing assets, or visual diffs detected.",
		}
	}
}

func stageWorkflowReportAsset(reportDir, kind, sourcePath string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", nil
	}
	payload, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", wrapCaptureError("stage_workflow_report_asset", err)
	}
	targetPath := filepath.Join(reportDir, "assets", sanitizeName(kind), filepath.Base(sourcePath))
	if err := writeFile(targetPath, payload); err != nil {
		return "", err
	}
	return workflowRelativePath(reportDir, targetPath), nil
}

func workflowRelativePath(fromDir, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	relative, err := filepath.Rel(fromDir, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(relative)
}
