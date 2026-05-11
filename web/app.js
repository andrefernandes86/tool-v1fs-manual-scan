(function () {
  const MAX_SCAN_FOLDERS = 32;

  let currentPath = '';
  let selectedPaths = [];
  let currentTaskId = null;
  let pollTimer = null;
  let elapsedTimer = null;
  let scanStartedAt = null;
  let runningInContainer = false;
  const MAX_SCAN_TAGS_UI = 32;
  let scanTagChips = [];

  async function updateScannerStatus() {
    const chip = document.getElementById('scanner-status-chip');
    if (!chip) return;

    try {
      const response = await fetch('/api/scanner/status');
      const data = await response.json();

      chip.classList.remove('header-chip-checking', 'header-chip-available', 'header-chip-unavailable');
      const textEl = chip.querySelector('.status-text');

      if (data.available) {
        chip.classList.add('header-chip-available');
        textEl.textContent = 'Scanner Available';
      } else {
        chip.classList.add('header-chip-unavailable');
        textEl.textContent = 'Scanner Unavailable';
      }
    } catch (err) {
      chip.classList.remove('header-chip-checking', 'header-chip-available', 'header-chip-unavailable');
      chip.classList.add('header-chip-unavailable');
      const textEl = chip.querySelector('.status-text');
      textEl.textContent = 'Scanner Unavailable';
    }
  }

  function startScannerStatusCheck() {
    updateScannerStatus();
  }

  function showPane(name) {
    document.querySelectorAll('.pane').forEach(p => p.classList.remove('active'));
    const tab = document.querySelector('.side-nav-item[data-tab="' + name + '"]');
    const pane = document.getElementById('pane-' + name);
    document.querySelectorAll('.side-nav-item').forEach(btn => btn.classList.remove('side-nav-item-active'));
    if (tab) tab.classList.add('side-nav-item-active');
    if (pane) pane.classList.add('active');
  }

  document.querySelectorAll('.side-nav-item').forEach(btn => {
    const tabName = btn.dataset.tab;
    if (!tabName) return;
    btn.addEventListener('click', () => showPane(tabName));
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

  function confirmScanRootIfNeeded(p) {
    const norm = String(p).replace(/\\/g, '/');
    if (norm === '/' || norm === '//') {
      let msg =
        'You are including the filesystem root (/). Scans are recursive (all subfolders). On Linux, only top-level /proc, /sys, /dev, and /run are skipped. Continue?';
      if (runningInContainer) {
        msg =
          'You selected /. In Docker that is only this container (small)—not your Mac/PC files or USB drives unless you mounted them.\n\nTo scan your stuff: stop the container, run docker again with -v (host path:container path), then add the container path under Scan targets—e.g. -v /Users/andre:/mnt/data and scan /mnt/data, or on Linux -v /:/host:ro and scan /host.\n\nContinue with / anyway?';
      }
      return window.confirm(msg);
    }
    if (/^[a-zA-Z]:\/?$/.test(norm)) {
      return window.confirm('You selected a drive root. This may scan the entire volume. Continue?');
    }
    return true;
  }

  function renderScanTargets() {
    const el = document.getElementById('scan-targets');
    if (!el) return;
    el.innerHTML = '';
    if (selectedPaths.length === 0) {
      const ph = document.createElement('span');
      ph.className = 'scan-targets-placeholder';
      ph.textContent = 'No folders selected. Tick checkboxes beside folders or use the buttons below.';
      el.appendChild(ph);
      return;
    }
    selectedPaths.forEach(function (p) {
      const chip = document.createElement('span');
      chip.className = 'scan-target-chip';
      const pathSpan = document.createElement('span');
      pathSpan.className = 'scan-target-chip-path';
      pathSpan.textContent = p;
      const rm = document.createElement('button');
      rm.type = 'button';
      rm.className = 'scan-target-chip-remove';
      rm.setAttribute('aria-label', 'Remove folder');
      rm.textContent = '\u00d7';
      rm.addEventListener('click', function () {
        selectedPaths = selectedPaths.filter(function (x) {
          return x !== p;
        });
        renderScanTargets();
        updateSelectButton();
      });
      chip.appendChild(pathSpan);
      chip.appendChild(rm);
      el.appendChild(chip);
    });
    syncDirectoryListCheckboxes();
  }

  function updateSelectButton() {
    document.getElementById('btn-start-scan').disabled = selectedPaths.length === 0;
  }

  function syncDirectoryListCheckboxes() {
    const root = dirListEl();
    if (!root) return;
    root.querySelectorAll('input.dir-scan-checkbox').forEach(function (input) {
      const p = input.getAttribute('data-path');
      if (p) input.checked = selectedPaths.indexOf(p) >= 0;
    });
  }

  /** Returns true if the checkbox state should stay as requested. */
  function togglePathFromCheckbox(absPath, wantOn) {
    if (wantOn) {
      if (!confirmScanRootIfNeeded(absPath)) {
        return false;
      }
      if (selectedPaths.indexOf(absPath) >= 0) {
        return true;
      }
      if (selectedPaths.length >= MAX_SCAN_FOLDERS) {
        alert('Maximum ' + MAX_SCAN_FOLDERS + ' folders per scan.');
        return false;
      }
      selectedPaths.push(absPath);
    } else {
      selectedPaths = selectedPaths.filter(function (x) {
        return x !== absPath;
      });
    }
    renderScanTargets();
    updateSelectButton();
    return true;
  }

  function showScanTargetSummary(text) {
    const wrap = document.getElementById('scan-target-summary');
    const val = document.getElementById('scan-target-summary-value');
    if (!wrap || !val) return;
    val.textContent = text || '';
    wrap.classList.toggle('hidden', !text);
  }

  function prepareProgressUI() {
    scanStartedAt = null;
    document.getElementById('scan-elapsed').textContent = '0:00';
    document.getElementById('malicious-count').textContent = '0';
    document.getElementById('scan-error-count').textContent = '0';
    const lastErr = document.getElementById('scan-last-error');
    if (lastErr) {
      lastErr.textContent = '';
      lastErr.classList.add('hidden');
    }
    const scanHint = document.getElementById('scan-hint-banner');
    if (scanHint) {
      scanHint.textContent = '';
      scanHint.classList.add('hidden');
    }
    document.getElementById('scan-progress').classList.remove('hidden');
    document.getElementById('malicious-banner').classList.add('hidden');
    const progressEl = document.getElementById('scan-progress');
    progressEl.querySelectorAll('a[download], p.error-cell').forEach(function (n) {
      n.remove();
    });
  }

  function startScanSession(taskId, targetLabel, scanHint) {
    currentTaskId = taskId;
    prepareProgressUI();
    showScanTargetSummary(targetLabel || '');
    const hintEl = document.getElementById('scan-hint-banner');
    if (hintEl && scanHint) {
      hintEl.textContent = scanHint;
      hintEl.classList.remove('hidden');
    }
    startPolling();
  }

  function loadDirs(path) {
    currentPath = path != null ? path : '';
    const q = currentPath !== '' ? '?path=' + encodeURIComponent(currentPath) : '';
    const upBtn = document.getElementById('btn-dir-up');
    dirListEl().innerHTML = '';
    dirListEl().appendChild(document.createTextNode('Loading…'));
    api('/api/dirs' + q).then(data => {
      const entries = data.entries || [];
      const serverPath = data.currentPath != null ? data.currentPath : currentPath;
      currentPath = serverPath;
      dirListEl().innerHTML = '';
      const tail = document.getElementById('breadcrumb-tail');
      if (currentPath === '') {
        tail.textContent = '';
        tail.classList.add('breadcrumb-path-muted');
      } else {
        tail.textContent = ' → ' + currentPath;
        tail.classList.remove('breadcrumb-path-muted');
      }
      if (upBtn) {
        upBtn.disabled = !data.canGoUp;
        upBtn.onclick = () => {
          if (data.canGoUp) {
            loadDirs(data.upPath != null ? data.upPath : '');
          }
        };
      }
      if (entries.length === 0) {
        const empty = document.createElement('p');
        empty.className = 'dir-list-empty';
        empty.textContent = currentPath === '' ? 'No locations found.' : 'No subfolders here.';
        dirListEl().appendChild(empty);
      } else {
        entries.forEach(function (entry) {
          const row = document.createElement('div');
          row.className = 'dir-item-row';

          const checkLabel = document.createElement('label');
          checkLabel.className = 'dir-item-check';
          const cb = document.createElement('input');
          cb.type = 'checkbox';
          cb.className = 'dir-scan-checkbox';
          cb.setAttribute('data-path', entry.path);
          cb.checked = selectedPaths.indexOf(entry.path) >= 0;
          cb.setAttribute('aria-label', 'Include ' + entry.path + ' in scan');
          cb.addEventListener('click', function (e) {
            e.stopPropagation();
          });
          cb.addEventListener('change', function () {
            if (!togglePathFromCheckbox(entry.path, cb.checked)) {
              cb.checked = false;
            }
          });
          checkLabel.appendChild(cb);
          row.appendChild(checkLabel);

          const nav = document.createElement('button');
          nav.type = 'button';
          nav.className = 'dir-item dir-item-nav';
          const label = entry.name === '/' ? 'Root' : entry.name;
          nav.innerHTML =
            '<span class="dir-item-icon" aria-hidden="true"></span><span class="dir-item-label">' +
            escapeHtml(label) +
            '</span>';
          nav.setAttribute('aria-label', 'Open folder ' + entry.path);
          nav.addEventListener('click', function () {
            loadDirs(entry.path);
          });
          row.appendChild(nav);

          dirListEl().appendChild(row);
        });
      }
    }).catch(err => {
      dirListEl().innerHTML = '';
      const p = document.createElement('p');
      p.className = 'dir-list-error';
      p.textContent = 'Error: ' + err.message;
      dirListEl().appendChild(p);
      if (upBtn) upBtn.disabled = true;
    });
  }

  document.getElementById('btn-add-scan-target').addEventListener('click', function () {
    if (currentPath === '') {
      alert('Open a folder first: choose a location, then open the directory you want to add.');
      return;
    }
    if (!confirmScanRootIfNeeded(currentPath)) return;
    if (selectedPaths.indexOf(currentPath) >= 0) return;
    if (selectedPaths.length >= MAX_SCAN_FOLDERS) {
      alert('Maximum ' + MAX_SCAN_FOLDERS + ' folders per scan.');
      return;
    }
    selectedPaths.push(currentPath);
    renderScanTargets();
    updateSelectButton();
  });

  document.getElementById('btn-use-only-folder').addEventListener('click', function () {
    if (currentPath === '') {
      alert('Open a folder first: choose a location, then open the directory you want to scan.');
      return;
    }
    if (!confirmScanRootIfNeeded(currentPath)) return;
    selectedPaths = [currentPath];
    renderScanTargets();
    updateSelectButton();
  });

  document.getElementById('btn-clear-scan-targets').addEventListener('click', function () {
    selectedPaths = [];
    renderScanTargets();
    updateSelectButton();
  });

  document.querySelector('.breadcrumb-btn').addEventListener('click', function () {
    loadDirs('');
  });

  loadDirs('');

  document.getElementById('btn-start-scan').addEventListener('click', async function () {
    if (selectedPaths.length === 0) return;
    const defaultName =
      selectedPaths.length === 1 ? selectedPaths[0] : selectedPaths.length + ' folders';
    const reportNameInput = window.prompt(
      'Optional report name to help identify this scan later:',
      defaultName
    );
    if (reportNameInput === null) return;
    const reportName = reportNameInput.trim();
    try {
      const res = await api('/api/scan/start', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ paths: selectedPaths, reportName: reportName })
      });
      startScanSession(res.taskId, selectedPaths.join('; '), res.scanHint);
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
      document.getElementById('scan-error-count').textContent = data.scanErrors || 0;
      const lastErr = document.getElementById('scan-last-error');
      if (lastErr) {
        if (data.lastScanError) {
          lastErr.textContent = 'Latest scan error: ' + data.lastScanError;
          lastErr.classList.remove('hidden');
        } else {
          lastErr.textContent = '';
          lastErr.classList.add('hidden');
        }
      }
      const pct = data.totalFiles ? (100 * data.scannedCount / data.totalFiles) : 0;
      document.getElementById('progress-fill').style.width = pct + '%';
      if (data.path) {
        showScanTargetSummary(data.path);
      }
      const detailsEl = document.getElementById('scan-details');
      if (detailsEl && scanStartedAt) {
        const finishedAtMs = data.finishedAt ? new Date(data.finishedAt).getTime() : null;
        const nowMs = finishedAtMs || Date.now();
        const elapsedSec = Math.max(0, (nowMs - scanStartedAt) / 1000);
        let fps = 0;
        if (elapsedSec > 0) {
          fps = data.scannedCount / elapsedSec;
        }
        detailsEl.classList.remove('hidden');
        document.getElementById('stat-fps').textContent = fps ? fps.toFixed(1) : '—';
        document.getElementById('stat-scanned').textContent = data.scannedCount;
        document.getElementById('stat-total').textContent = data.totalFiles;
        let etaText = '—';
        if (fps > 0 && data.totalFiles > data.scannedCount) {
          const remaining = data.totalFiles - data.scannedCount;
          const secLeft = remaining / fps;
          const m = Math.floor(secLeft / 60);
          const s = Math.floor(secLeft % 60);
          etaText = m + 'm ' + (s < 10 ? '0' + s : s);
        }
        document.getElementById('stat-eta').textContent = etaText;
      }

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
    const scannerType = (document.querySelector('input[name="scannerType"]:checked') || {}).value || 'saas';
    const apiKey = document.getElementById('input-apikey').value.trim();
    const region = document.getElementById('input-region').value.trim();
    const localScannerUrl = (document.getElementById('input-local-scanner-url') || {}).value ? document.getElementById('input-local-scanner-url').value.trim() : '';
    const localScannerApiKey = (document.getElementById('input-local-scanner-apikey') || {}).value ? document.getElementById('input-local-scanner-apikey').value.trim() : '';
    const localScannerProtocol = 'grpc';
    if (scannerType === 'saas') {
      if (!apiKey || !region) {
        alert('API key and region are required for SaaS scanner.');
        return;
      }
    } else if (!localScannerUrl) {
      alert('Local scanner URL is required for local scanner.');
      return;
    }
    try {
      await api('/api/config', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ apiKey, region, scannerType, localScannerUrl, localScannerApiKey, localScannerProtocol, localScannerTls: false })
      });
      document.getElementById('input-apikey').value = '';
      loadConfig();
      alert('Scanner settings saved.');
    } catch (err) {
      alert('Save failed: ' + err.message);
    }
  });

  function applyRuntimeFromConfig(c) {
    runningInContainer = !!(c && c.runningInContainer);
    const banner = document.getElementById('scanner-container-hint');
    if (banner && c && c.containerScanRootHint) {
      const textEl = banner.querySelector('.scanner-container-hint-text');
      if (textEl) textEl.textContent = c.containerScanRootHint;
      banner.classList.toggle('hidden', !runningInContainer);
      const toggle = banner.querySelector('.collapsible-banner-toggle');
      if (toggle && !toggle.hasListener) {
        toggle.hasListener = true;
        toggle.addEventListener('click', function () {
          const content = document.getElementById('scanner-container-content');
          const isExpanded = toggle.getAttribute('aria-expanded') === 'true';
          toggle.setAttribute('aria-expanded', !isExpanded);
          content.classList.toggle('hidden');
        });
      }
    } else if (banner) {
      banner.classList.add('hidden');
    }
  }

  function scanTagsForSave() {
    return scanTagChips.slice();
  }

  function renderScanTagChips() {
    const wrap = document.getElementById('scan-tags-chips');
    const entry = document.getElementById('input-scan-tag-entry');
    if (!wrap) return;
    wrap.innerHTML = '';
    scanTagChips.forEach(function (tag, index) {
      const chip = document.createElement('span');
      chip.className = 'scan-tag-chip';
      chip.setAttribute('data-index', String(index));
      const lab = document.createElement('span');
      lab.className = 'scan-tag-chip-label';
      lab.textContent = tag;
      const rm = document.createElement('button');
      rm.type = 'button';
      rm.className = 'scan-tag-chip-remove';
      rm.setAttribute('aria-label', 'Remove tag ' + tag);
      rm.textContent = '\u00D7';
      rm.addEventListener('click', function () {
        scanTagChips.splice(index, 1);
        renderScanTagChips();
      });
      chip.appendChild(lab);
      chip.appendChild(rm);
      wrap.appendChild(chip);
    });
    if (entry) {
      entry.disabled = scanTagChips.length >= MAX_SCAN_TAGS_UI;
      entry.placeholder =
        scanTagChips.length >= MAX_SCAN_TAGS_UI ? 'Maximum tags reached' : 'Type a tag, press Enter';
    }
  }

  function tryAddScanTagFromInput() {
    const entry = document.getElementById('input-scan-tag-entry');
    if (!entry || entry.disabled) return;
    let t = entry.value.trim();
    if (!t) return;
    if (t.length > 128) t = t.slice(0, 128);
    if (/[\u0000-\u001F\u007F]/.test(t)) {
      alert('Tags cannot contain control characters.');
      return;
    }
    if (scanTagChips.indexOf(t) >= 0) {
      entry.value = '';
      return;
    }
    if (scanTagChips.length >= MAX_SCAN_TAGS_UI) return;
    scanTagChips.push(t);
    entry.value = '';
    renderScanTagChips();
  }

  function wireScanTagEntry() {
    const entry = document.getElementById('input-scan-tag-entry');
    if (!entry || entry.dataset.wired) return;
    entry.dataset.wired = '1';
    entry.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        tryAddScanTagFromInput();
        return;
      }
      if (e.key === 'Backspace' && !entry.value && scanTagChips.length > 0) {
        scanTagChips.pop();
        renderScanTagChips();
      }
    });
  }

  function loadConfig() {
    api('/api/config').then(c => {
      applyRuntimeFromConfig(c);
      const maskedEl = document.getElementById('config-apikey-masked');
      maskedEl.textContent = c.apiKeySet || c.configured ? 'API key is set' : '';
      document.getElementById('input-region').value = c.region || '';
      const scannerType = c.scannerType === 'local' ? 'local' : 'saas';
      const modeRadio = document.querySelector('input[name="scannerType"][value="' + scannerType + '"]');
      if (modeRadio) modeRadio.checked = true;
      const localUrlEl = document.getElementById('input-local-scanner-url');
      if (localUrlEl) localUrlEl.value = c.localScannerUrl || '';
      toggleScannerFields();
      const action = (c.actionOnMalware === 'quarantine' || c.actionOnMalware === 'delete') ? c.actionOnMalware : 'log';
      const radio = document.querySelector('input[name="actionOnMalware"][value="' + action + '"]');
      if (radio) radio.checked = true;
      document.getElementById('input-quarantine-path').value = c.quarantinePath || '';
      const hashEl = document.getElementById('input-hash-enabled');
      if (hashEl) hashEl.checked = !!c.hashEnabled;
      const pmlEl = document.getElementById('input-predictive-ml');
      if (pmlEl) pmlEl.checked = !!c.predictiveML;
      const maxScansEl = document.getElementById('input-max-concurrent-scans');
      if (maxScansEl) maxScansEl.value = (typeof c.maxConcurrentScans === 'number' && c.maxConcurrentScans > 0) ? String(c.maxConcurrentScans) : '';
      const reportModeEl = document.getElementById('input-report-mode');
      if (reportModeEl) reportModeEl.value = c.reportMode === 'all' ? 'all' : 'stats';
      scanTagChips = Array.isArray(c.scanTags) ? c.scanTags.slice() : [];
      renderScanTagChips();
      wireScanTagEntry();
      toggleQuarantinePath();
    });
  }

  function toggleQuarantinePath() {
    const wrap = document.getElementById('quarantine-path-wrap');
    const q = document.querySelector('input[name="actionOnMalware"][value="quarantine"]');
    if (wrap && q) wrap.classList.toggle('hidden', !q.checked);
  }

  function updateLocalScannerHints() {
    const mode = (document.querySelector('input[name="scannerType"]:checked') || {}).value || 'saas';
    if (mode !== 'local') return;
    const endpointInput = document.getElementById('input-local-scanner-url');
    if (endpointInput) endpointInput.placeholder = 'host:port (e.g. 192.168.200.71:50051)';
  }

  function toggleScannerFields() {
    const mode = (document.querySelector('input[name="scannerType"]:checked') || {}).value || 'saas';
    const saasWrap = document.getElementById('scanner-saas-wrap');
    const localWrap = document.getElementById('scanner-local-wrap');
    if (saasWrap) saasWrap.classList.toggle('hidden', mode !== 'saas');
    if (localWrap) localWrap.classList.toggle('hidden', mode !== 'local');
    updateLocalScannerHints();
  }

  document.querySelectorAll('input[name="actionOnMalware"]').forEach(function (el) {
    el.addEventListener('change', toggleQuarantinePath);
  });
  document.querySelectorAll('input[name="scannerType"]').forEach(function (el) {
    el.addEventListener('change', toggleScannerFields);
  });
  const btnTestScanner = document.getElementById('btn-test-scanner');
  if (btnTestScanner) {
    btnTestScanner.addEventListener('click', async () => {
      try {
        const res = await api('/api/scanner/test', { method: 'POST', json: true, body: JSON.stringify({}) });
        alert(res.message || 'Scanner responded successfully.');
      } catch (err) {
        alert('Scanner test failed: ' + err.message);
      }
    });
  }
  const btnCompatScanner = document.getElementById('btn-compat-scanner');
  if (btnCompatScanner) {
    btnCompatScanner.addEventListener('click', async () => {
      try {
        const res = await api('/api/scanner/compat', { method: 'POST', json: true, body: JSON.stringify({}) });
        alert('✓ Malware detection works!\n\n' + (res.message || 'Malware test file was detected as malicious.'));
      } catch (err) {
        alert('✗ Malware detection failed:\n\n' + err.message);
      }
    });
  }

  const tabButtons = document.querySelectorAll('.tab-button');
  tabButtons.forEach(button => {
    button.addEventListener('click', () => {
      const tabId = button.getAttribute('data-tab');
      tabButtons.forEach(btn => btn.classList.remove('tab-button-active'));
      document.querySelectorAll('.tab-content').forEach(content => content.classList.remove('tab-content-active'));
      button.classList.add('tab-button-active');
      document.getElementById(tabId).classList.add('tab-content-active');
    });
  });

  document.getElementById('form-scan-action').addEventListener('submit', async (e) => {
    e.preventDefault();
    const action = document.querySelector('input[name="actionOnMalware"]:checked').value;
    const quarantinePath = document.getElementById('input-quarantine-path').value.trim();
    const hashEl = document.getElementById('input-hash-enabled');
    const hashEnabled = !!(hashEl && hashEl.checked);
    const pmlEl = document.getElementById('input-predictive-ml');
    const predictiveML = !!(pmlEl && pmlEl.checked);
    const reportModeEl = document.getElementById('input-report-mode');
    const reportMode = (reportModeEl && reportModeEl.value === 'all') ? 'all' : 'stats';
    const maxScansEl = document.getElementById('input-max-concurrent-scans');
    let maxConcurrentScans = 0;
    if (maxScansEl && maxScansEl.value.trim() !== '') {
      const n = parseInt(maxScansEl.value.trim(), 10);
      if (!isNaN(n) && n >= 0 && n <= 1000) maxConcurrentScans = n;
    }
    try {
      await api('/api/config/scan-action', {
        method: 'POST',
        json: true,
        body: JSON.stringify({
          actionOnMalware: action,
          quarantinePath: quarantinePath,
          maxConcurrentScans: maxConcurrentScans,
          hashEnabled: hashEnabled,
          predictiveML: predictiveML,
          reportMode: reportMode,
          scanTags: scanTagsForSave()
        })
      });
      alert('Actions saved.');
    } catch (err) {
      alert('Save failed: ' + err.message);
    }
  });

  renderScanTagChips();
  wireScanTagEntry();
  loadConfig();
  startScannerStatusCheck();
  renderScanTargets();
  updateSelectButton();

  function loadHistory() {
    api('/api/scan/history').then(list => {
      const el = document.getElementById('history-list');
      if (!list || list.length === 0) {
        el.innerHTML = '<p class="empty-history">No scan history yet.</p>';
        return;
      }
      const table = document.createElement('table');
      table.className = 'history-table';
      table.innerHTML = '<thead><tr><th>Report name</th><th>Path</th><th>Started</th><th>Files</th><th>Malicious</th><th>Report</th><th>Error</th></tr></thead><tbody></tbody>';
      const tbody = table.querySelector('tbody');
      list.forEach(row => {
        const tr = document.createElement('tr');
        tr.innerHTML =
          '<td>' + escapeHtml(row.reportName || '') + '</td>' +
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

  const historyNav = document.querySelector('.side-nav-item[data-tab="history"]');
  if (historyNav) historyNav.addEventListener('click', loadHistory);

  function loadTestSamplesPath() {
    api('/api/test-samples').then(data => {
      const el = document.getElementById('test-samples-path');
      if (el) el.textContent = data.path || '/data/test-samples';
    }).catch(() => {});
  }

  const settingsNav = document.querySelector('.side-nav-item[data-tab="settings"]');
  if (settingsNav) settingsNav.addEventListener('click', loadTestSamplesPath);
  async function runTestSample(sample) {
    const destDir = document.getElementById('input-test-dest').value.trim();
    if (!destDir) {
      alert('Destination folder is required.');
      return;
    }
    const defaultName = (sample === 'eicar' ? 'Malware test' : 'Clean test') + ' - ' + destDir;
    const reportNameInput = window.prompt('Optional report name for this test scan:', defaultName);
    if (reportNameInput === null) return;
    const reportName = reportNameInput.trim();
    try {
      const { taskId, scanPath } = await api('/api/test-scan', {
        method: 'POST',
        json: true,
        body: JSON.stringify({ sample, destDir, reportName: reportName })
      });
      showPane('scanner');
      startScanSession(taskId, scanPath || destDir);
    } catch (e) {
      alert('Failed to start test scan: ' + e.message);
    }
  }

  const btnEicar = document.getElementById('btn-test-eicar');
  if (btnEicar) {
    btnEicar.addEventListener('click', () => runTestSample('eicar'));
  }
  const btnClean = document.getElementById('btn-test-clean');
  if (btnClean) {
    btnClean.addEventListener('click', () => runTestSample('clean'));
  }
})();
