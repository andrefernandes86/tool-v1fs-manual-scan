(function () {
  const REGIONS = [
    'us-east-1', 'eu-central-1', 'eu-west-2', 'ca-central-1',
    'ap-southeast-1', 'ap-southeast-2', 'ap-northeast-1', 'ap-south-1', 'me-central-1'
  ];

  let currentPath = '';
  let selectedPath = '';
  let currentTaskId = null;
  let pollTimer = null;
  let elapsedTimer = null;
  let scanStartedAt = null;

  function showPane(name) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.pane').forEach(p => p.classList.remove('active'));
    const tab = document.querySelector('.tab[data-tab="' + name + '"]');
    const pane = document.getElementById('pane-' + name);
    if (tab) tab.classList.add('active');
    if (pane) pane.classList.add('active');
  }

  document.querySelectorAll('.tab').forEach(btn => {
    btn.addEventListener('click', () => showPane(btn.dataset.tab));
  });

  async function api(path, options = {}) {
    const res = await fetch(path, {
      headers: options.json ? { 'Content-Type': 'application/json' } : {},
      ...options
    });
    if (!res.ok) {
      const t = await res.text();
      throw new Error(t || res.statusText);
    }
    return res.json();
  }

  function dirListEl() {
    return document.getElementById('dir-list');
  }

  function loadDirs(path) {
    currentPath = path || '';
    const q = path ? '?path=' + encodeURIComponent(path) : '';
    dirListEl().innerHTML = '';
    dirListEl().appendChild(document.createTextNode('Loading…'));
    api('/api/dirs' + q).then(entries => {
      dirListEl().innerHTML = '';
      const displayPath = currentPath || '/';
      document.getElementById('breadcrumb-tail').textContent = displayPath ? ' / ' + displayPath : '';
      entries.forEach(entry => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'dir-item';
        btn.textContent = entry.name + (entry.name === '/' ? '' : '/');
        btn.dataset.path = entry.path;
        btn.addEventListener('click', () => loadDirs(entry.path));
        dirListEl().appendChild(btn);
      });
      updateSelectButton();
    }).catch(err => {
      dirListEl().innerHTML = 'Error: ' + err.message;
    });
  }

  function updateSelectButton() {
    document.getElementById('selected-path').textContent = selectedPath || '—';
    document.getElementById('btn-start-scan').disabled = !selectedPath;
  }

  document.getElementById('btn-select-folder').addEventListener('click', () => {
    selectedPath = currentPath || '/';
    updateSelectButton();
  });

  document.querySelector('.breadcrumb-btn').addEventListener('click', () => {
    selectedPath = '';
    loadDirs('');
    updateSelectButton();
  });

  loadDirs('');

  document.getElementById('btn-start-scan').addEventListener('click', async () => {
    if (!selectedPath) return;
    try {
      const { taskId } = await api('/api/scan/start', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ path: selectedPath })
      });
      currentTaskId = taskId;
      scanStartedAt = null;
      document.getElementById('scan-elapsed').textContent = '0:00';
      document.getElementById('malicious-count').textContent = '0';
      document.getElementById('scan-progress').classList.remove('hidden');
      document.getElementById('malicious-banner').classList.add('hidden');
      startPolling();
    } catch (e) {
      alert('Failed to start scan: ' + e.message);
    }
  });

  function startPolling() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(pollStatus, 1200);
    pollStatus();
  }

  function stopElapsedTimer() {
    if (elapsedTimer) {
      clearInterval(elapsedTimer);
      elapsedTimer = null;
    }
  }

  function formatElapsed(ms) {
    const totalSec = Math.floor(ms / 1000);
    const m = Math.floor(totalSec / 60);
    const s = totalSec % 60;
    return m + ':' + (s < 10 ? '0' : '') + s;
  }

  function updateElapsedDisplay() {
    const el = document.getElementById('scan-elapsed');
    if (!el || !scanStartedAt) return;
    el.textContent = formatElapsed(Date.now() - scanStartedAt);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
    stopElapsedTimer();
  }

  function pollStatus() {
    if (!currentTaskId) return;
    api('/api/scan/status/' + currentTaskId).then(data => {
      if (!scanStartedAt && data.startedAt) {
        scanStartedAt = new Date(data.startedAt).getTime();
        stopElapsedTimer();
        elapsedTimer = setInterval(updateElapsedDisplay, 1000);
        updateElapsedDisplay();
      }
      document.getElementById('current-file').textContent = data.currentFile || '—';
      document.getElementById('scanned-count').textContent = data.scannedCount;
      document.getElementById('total-files').textContent = data.totalFiles;
      document.getElementById('malicious-count').textContent = (data.malicious && data.malicious.length) ? data.malicious.length : 0;
      const pct = data.totalFiles ? (100 * data.scannedCount / data.totalFiles) : 0;
      document.getElementById('progress-fill').style.width = pct + '%';

      const listEl = document.getElementById('malicious-list');
      listEl.innerHTML = '';
      if (data.malicious && data.malicious.length > 0) {
        document.getElementById('malicious-banner').classList.remove('hidden');
        data.malicious.forEach(m => {
          const div = document.createElement('div');
          div.className = 'malicious-item';
          div.innerHTML = '<span class="name">' + escapeHtml(m.fileName) + '</span><br><span class="path">' + escapeHtml(m.filePath) + '</span><br><span class="malware">Malware: ' + escapeHtml(m.malwareName) + '</span>';
          listEl.appendChild(div);
        });
      }

      if (data.finishedAt) {
        stopPolling();
        updateElapsedDisplay();
        const progressEl = document.getElementById('scan-progress');
        if (data.reportPath && !progressEl.querySelector('a[download]')) {
          const a = document.createElement('a');
          a.href = '/api/reports/' + data.reportPath;
          a.download = data.reportPath;
          a.className = 'btn btn-primary';
          a.style.marginTop = '0.5rem';
          a.textContent = 'Download PDF report';
          progressEl.appendChild(a);
        }
        if (data.error && !progressEl.querySelector('p.error-cell')) {
          const errEl = document.createElement('p');
          errEl.className = 'error-cell';
          errEl.textContent = 'Error: ' + data.error;
          progressEl.appendChild(errEl);
        }
      }
    }).catch(() => {});
  }

  function escapeHtml(s) {
    if (s == null || s === undefined) return '';
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }

  document.getElementById('form-config').addEventListener('submit', async (e) => {
    e.preventDefault();
    const apiKey = document.getElementById('input-apikey').value.trim();
    const region = document.getElementById('input-region').value.trim();
    if (!apiKey || !region) {
      alert('API key and region are required.');
      return;
    }
    try {
      await api('/api/config', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ apiKey, region })
      });
      document.getElementById('input-apikey').value = '';
      loadConfig();
      alert('Settings saved.');
    } catch (err) {
      alert('Save failed: ' + err.message);
    }
  });

  function loadConfig() {
    api('/api/config').then(c => {
      const maskedEl = document.getElementById('config-apikey-masked');
      maskedEl.textContent = c.apiKeySet || c.configured ? 'API key is set' : '';
      document.getElementById('input-region').value = c.region || '';
      const action = (c.actionOnMalware === 'quarantine' || c.actionOnMalware === 'delete') ? c.actionOnMalware : 'log';
      const radio = document.querySelector('input[name="actionOnMalware"][value="' + action + '"]');
      if (radio) radio.checked = true;
      document.getElementById('input-quarantine-path').value = c.quarantinePath || '';
      toggleQuarantinePath();
    });
  }

  function toggleQuarantinePath() {
    const wrap = document.getElementById('quarantine-path-wrap');
    const q = document.querySelector('input[name="actionOnMalware"][value="quarantine"]');
    if (wrap && q) wrap.classList.toggle('hidden', !q.checked);
  }

  document.querySelectorAll('input[name="actionOnMalware"]').forEach(function (el) {
    el.addEventListener('change', toggleQuarantinePath);
  });

  document.getElementById('form-scan-action').addEventListener('submit', async (e) => {
    e.preventDefault();
    const action = document.querySelector('input[name="actionOnMalware"]:checked').value;
    const quarantinePath = document.getElementById('input-quarantine-path').value.trim();
    try {
      await api('/api/config/scan-action', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ actionOnMalware: action, quarantinePath: quarantinePath })
      });
      alert('Actions saved.');
    } catch (err) {
      alert('Save failed: ' + err.message);
    }
  });

  loadConfig();

  function loadHistory() {
    api('/api/scan/history').then(list => {
      const el = document.getElementById('history-list');
      if (!list || list.length === 0) {
        el.innerHTML = '<p class="empty-history">No scan history yet.</p>';
        return;
      }
      const table = document.createElement('table');
      table.className = 'history-table';
      table.innerHTML = '<thead><tr><th>Path</th><th>Started</th><th>Files</th><th>Malicious</th><th>Report</th><th>Error</th></tr></thead><tbody></tbody>';
      const tbody = table.querySelector('tbody');
      list.forEach(row => {
        const tr = document.createElement('tr');
        tr.innerHTML =
          '<td>' + escapeHtml(row.path) + '</td>' +
          '<td>' + escapeHtml(row.startedAt) + '</td>' +
          '<td>' + row.scannedCount + ' / ' + row.totalFiles + '</td>' +
          '<td>' + row.maliciousCount + '</td>' +
          '<td>' + (row.reportPath ? '<a href="/api/reports/' + escapeHtml(row.reportPath) + '" download>Download PDF</a>' : '—') + '</td>' +
          '<td class="error-cell">' + (row.error ? escapeHtml(row.error) : '') + '</td>';
        tbody.appendChild(tr);
      });
      el.innerHTML = '';
      el.appendChild(table);
    });
  }

  document.querySelector('.tab[data-tab="history"]').addEventListener('click', loadHistory);

  function loadTestSamplesPath() {
    api('/api/test-samples').then(data => {
      const el = document.getElementById('test-samples-path');
      if (el) el.textContent = data.path || '/data/test-samples';
    }).catch(() => {});
  }

  document.querySelector('.tab[data-tab="settings"]').addEventListener('click', loadTestSamplesPath);

  document.getElementById('btn-test-scan').addEventListener('click', async () => {
    try {
      const data = await api('/api/test-samples');
      const path = data.path || '/data/test-samples';
      const { taskId } = await api('/api/scan/start', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ path: path })
      });
      currentTaskId = taskId;
      scanStartedAt = null;
      document.getElementById('scan-elapsed').textContent = '0:00';
      document.getElementById('malicious-count').textContent = '0';
      showPane('scanner');
      const progressEl = document.getElementById('scan-progress');
      progressEl.classList.remove('hidden');
      progressEl.querySelectorAll('a[download], p.error-cell').forEach(n => n.remove());
      document.getElementById('malicious-banner').classList.add('hidden');
      startPolling();
    } catch (e) {
      alert('Failed to start test scan: ' + e.message + '. Configure API key and region first.');
    }
  });
})();
