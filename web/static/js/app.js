/* gohour web UI — app.js
 * All application logic extracted from base.html.
 * Alpine.js stores/data registered in alpine:init.
 * HTMX helpers added alongside existing vanilla-JS patterns.
 */

// ── Global mutable state ──
let _lookup = null;

// ── Alpine.js registration ──
document.addEventListener('alpine:init', () => {
  Alpine.store('toast', {
    message: '',
    isError: false,
    visible: false,
    _timer: null,
    show(msg, error) {
      this.message = String(msg || '');
      this.isError = Boolean(error);
      this.visible = true;
      if (this._timer) clearTimeout(this._timer);
      this._timer = setTimeout(() => { this.visible = false; }, 2600);
    },
  });

  Alpine.store('confirm', {
    open: false,
    title: '',
    body: '',
    okLabel: 'Submit',
    altLabel: '',
    onConfirm: null,
    onAlt: null,
    openDialog(title, body, onConfirm, confirmLabel, alternative) {
      this.title = String(title || '');
      this.body = String(body || '');
      this.okLabel = String(confirmLabel || 'Submit');
      this.onConfirm = typeof onConfirm === 'function' ? onConfirm : null;
      this.altLabel = (alternative && alternative.label) ? String(alternative.label) : '';
      this.onAlt = (alternative && typeof alternative.onConfirm === 'function')
        ? alternative.onConfirm
        : null;
      this.open = true;
    },
    close() {
      this.open = false;
      this.onConfirm = null;
      this.onAlt = null;
      this.altLabel = '';
    },
    confirm() {
      const callback = this.onConfirm;
      this.close();
      if (callback) callback();
    },
    alt() {
      const callback = this.onAlt;
      this.close();
      if (callback) callback();
    },
  });

  Alpine.store('edit', {
    open: false,
    mode: '',
    title: 'Edit entry',
    endpoint: '',
    date: '',
    rowId: 0,
    forceOverlap: false,
    start: '',
    end: '',
    billableHours: '',
    description: '',
    error: '',
    close() {
      this.open = false;
      this.mode = '';
      this.title = 'Edit entry';
      this.endpoint = '';
      this.date = '';
      this.rowId = 0;
      this.forceOverlap = false;
      this.start = '';
      this.end = '';
      this.billableHours = '';
      this.description = '';
      this.error = '';
    },
  });

  Alpine.store('importPreview', {
    form: null,
    options: null,
    entries: [],
    reset() {
      this.form = null;
      this.options = null;
      this.entries = [];
    },
  });

  Alpine.store('submit', {
    open: false,
    statusOnly: false,
    scope: '',
    value: '',
    title: 'Submit',
    dryRun: false,
    running: false,
    initialHtml: '',
    endpoint() {
      if (!this.scope || !this.value) return '';
      return '/partials/submit/' + encodeURIComponent(this.scope) + '/' + encodeURIComponent(this.value);
    },
    openSubmit(scope, value) {
      this.statusOnly = false;
      this.scope = String(scope || '');
      this.value = String(value || '');
      this.title = 'Submit ' + this.value;
      this.dryRun = false;
      this.running = false;
      this.initialHtml = '<div class="result-box">Choose options, then run submit.</div>';
      this.open = true;
    },
    openStatus(title, htmlContent) {
      this.statusOnly = true;
      this.scope = '';
      this.value = '';
      this.title = String(title || 'Status');
      this.dryRun = false;
      this.running = false;
      this.initialHtml = String(htmlContent || '');
      this.open = true;
    },
    close() {
      this.open = false;
      this.running = false;
      this.statusOnly = false;
      this.scope = '';
      this.value = '';
      this.title = 'Submit';
      this.dryRun = false;
      this.initialHtml = '';
    },
  });
});

function editStore() {
  return window.Alpine ? Alpine.store('edit') : null;
}

function importPreviewStore() {
  return window.Alpine ? Alpine.store('importPreview') : null;
}

function submitStore() {
  return window.Alpine ? Alpine.store('submit') : null;
}

// ── Core fetch helper ──
async function apiFetch(method, url, body, options) {
  const fetchOptions = { method: method, headers: {}, body: undefined };
  if (options && options.formData) {
    fetchOptions.body = options.formData;
  } else if (body !== undefined && body !== null) {
    fetchOptions.headers['Content-Type'] = 'application/json';
    fetchOptions.body = JSON.stringify(body);
  }
  if (options && options.headers && typeof options.headers === 'object') {
    for (const [key, value] of Object.entries(options.headers)) {
      fetchOptions.headers[key] = value;
    }
  }
  const response = await fetch(url, fetchOptions);
  const contentType = response.headers.get('content-type') || '';
  const payload = contentType.includes('application/json') ? await response.json() : await response.text();
  if (!response.ok) {
    const message = typeof payload === 'string'
      ? payload
      : (payload && payload.error ? String(payload.error) : JSON.stringify(payload));
    const error = new Error(message || ('HTTP ' + response.status));
    error.status = response.status;
    error.payload = payload;
    throw error;
  }
  return payload;
}

// ── Toast ──
function showToast(msg, isError) {
  // Prefer Alpine store when available
  if (window.Alpine) {
    try {
      Alpine.store('toast').show(msg, isError);
      return;
    } catch (e) {
      // fall through to legacy DOM approach
    }
  }
  const existing = document.getElementById('toast');
  if (existing) existing.remove();
  const toast = document.createElement('div');
  toast.id = 'toast';
  toast.className = 'toast' + (isError ? ' error' : '');
  toast.textContent = msg;
  document.body.appendChild(toast);
  setTimeout(() => {
    const current = document.getElementById('toast');
    if (current) current.remove();
  }, 2600);
}

// ── Formatting helpers ──
function fmtHours(mins) {
  return new Intl.NumberFormat(navigator.language, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(mins) / 60);
}

function fmtTime(hhmm) {
  const parts = String(hhmm || '').split(':');
  if (parts.length !== 2) return String(hhmm || '');
  const d = new Date(2000, 0, 1, Number(parts[0]), Number(parts[1]));
  return new Intl.DateTimeFormat(navigator.language, { timeStyle: 'short' }).format(d);
}

function fmtDate(iso) {
  const parts = String(iso || '').split('-');
  if (parts.length !== 3) return String(iso || '');
  const d = new Date(Number(parts[0]), Number(parts[1]) - 1, Number(parts[2]));
  return new Intl.DateTimeFormat(navigator.language, { dateStyle: 'medium' }).format(d);
}

function fmtDateTime(iso) {
  const d = new Date(String(iso || ''));
  if (Number.isNaN(d.getTime())) return String(iso || 'Not loaded yet');
  return new Intl.DateTimeFormat(navigator.language, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(d);
}

function toMins(hhmm) {
  const parts = String(hhmm || '').split(':');
  if (parts.length !== 2) return NaN;
  const h = Number(parts[0]);
  const m = Number(parts[1]);
  if (!Number.isInteger(h) || !Number.isInteger(m) || h < 0 || h > 23 || m < 0 || m > 59) {
    return NaN;
  }
  return h * 60 + m;
}

function recalcBillable(container) {
  const startInput = container.querySelector('[name=start]');
  const endInput = container.querySelector('[name=end]');
  const billableHoursInput = container.querySelector('[name=billableHours]');
  if (!startInput || !endInput || !billableHoursInput) return;

  const startMins = toMins(startInput.value);
  const endMins = toMins(endInput.value);
  const diff = endMins - startMins;
  if (Number.isFinite(diff) && diff > 0) {
    billableHoursInput.value = (diff / 60).toFixed(2);
  }
}

function applyLocaleFormatting(root) {
  const target = root || document;
  target.querySelectorAll('.js-fmt-hours').forEach((el) => {
    const mins = Number(el.dataset.mins || '0');
    if (Number.isFinite(mins)) {
      el.textContent = fmtHours(mins);
    }
  });
  target.querySelectorAll('.js-fmt-time').forEach((el) => {
    el.textContent = fmtTime(el.dataset.hhmm || el.textContent);
  });
  target.querySelectorAll('.js-fmt-date').forEach((el) => {
    el.textContent = fmtDate(el.dataset.iso || el.textContent);
  });
  target.querySelectorAll('.js-fmt-datetime').forEach((el) => {
    const raw = el.dataset.iso || el.textContent;
    el.textContent = fmtDateTime(raw);
  });
  target.querySelectorAll('.js-fmt-delta').forEach((el) => {
    const value = Number(el.dataset.hours || '0');
    const formatted = new Intl.NumberFormat(navigator.language, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
      signDisplay: 'always',
    }).format(value);
    el.textContent = formatted;
  });
}

// Re-run locale formatting after HTMX swaps
document.addEventListener('htmx:afterSettle', (evt) => {
  applyLocaleFormatting(evt.detail.elt);
  // OOB swaps are outside evt.detail.elt; reformat the document so refreshed
  // stat cards/timestamps stay locale-consistent after HTMX updates.
  applyLocaleFormatting(document);
});

// Allow month/day partial refreshes to swap server-provided 502 HTML fragments.
document.body.addEventListener('htmx:beforeSwap', (event) => {
  const detail = event.detail || {};
  const xhr = detail.xhr;
  if (!xhr || xhr.status !== 502) return;

  const target = detail.target;
  const targetID = target && target.id ? target.id : '';
  if (targetID !== 'month-rows' && targetID !== 'day-entries') {
    return;
  }

  const path = detail.requestConfig && detail.requestConfig.path
    ? String(detail.requestConfig.path)
    : '';
  if (!path.includes('/partials/month/') && !path.includes('/partials/day/')) {
    return;
  }

  detail.shouldSwap = true;
});

// ── Lookup / selects ──
async function getLookup(refresh) {
  if (!_lookup || refresh) {
    const url = refresh ? '/api/lookup?refresh=1' : '/api/lookup';
    _lookup = await apiFetch('GET', url);
  }
  return _lookup;
}

async function populateImportSelects(form) {
  const lookup = await getLookup();
  const projects = (lookup.projects || []).filter((project) => !project.archived);
  const activitiesAll = lookup.activities || [];
  const skillsAll = lookup.skills || [];

  const projectSel = form.querySelector('[name=project]');
  const activitySel = form.querySelector('[name=activity]');
  const skillSel = form.querySelector('[name=skill]');
  if (!projectSel || !activitySel || !skillSel) return;

  const selectedNumericID = (select) => {
    if (!select || !select.options.length || select.selectedIndex < 0) {
      return NaN;
    }
    const option = select.options[select.selectedIndex];
    return Number(option.dataset.id || '');
  };
  const fillImportSelect = (select, items, getID, getName) => {
    select.innerHTML = '';
    if (!items.length) {
      const option = document.createElement('option');
      option.value = '';
      option.textContent = 'No options';
      option.disabled = true;
      option.selected = true;
      select.appendChild(option);
      return;
    }
    for (const item of items) {
      const option = document.createElement('option');
      option.value = String(getName(item));
      option.textContent = getName(item);
      option.dataset.name = getName(item);
      option.dataset.id = String(getID(item));
      select.appendChild(option);
    }
    select.selectedIndex = 0;
  };

  const rebuildSkills = () => {
    const activityID = selectedNumericID(activitySel);
    fillImportSelect(
      skillSel,
      skillsAll.filter((skill) => Number(skill.activityId) === activityID),
      (skill) => skill.id,
      (skill) => skill.name,
    );
  };
  const rebuildActivities = () => {
    const projectID = selectedNumericID(projectSel);
    fillImportSelect(
      activitySel,
      activitiesAll.filter((activity) => Number(activity.projectId) === projectID && !activity.locked),
      (activity) => activity.id,
      (activity) => activity.name,
    );
    rebuildSkills();
  };

  fillImportSelect(projectSel, projects, (project) => project.id, (project) => project.name);
  rebuildActivities();
  projectSel.addEventListener('change', rebuildActivities);
  activitySel.addEventListener('change', rebuildSkills);
}

async function openImportDialog(dialogID, formID) {
  const dialog = document.getElementById(dialogID);
  const form = document.getElementById(formID);
  if (!dialog || !form) return;
  clearImportFormStatus(form);
  if (!form.dataset.lookupLoaded) {
    try {
      await populateImportSelects(form);
      form.dataset.lookupLoaded = '1';
    } catch (err) {
      delete form.dataset.lookupLoaded;
      setImportFormStatus(form, String(err.message || err), true);
      showToast(String(err.message || err), true);
      return;
    }
  }
  dialog.showModal();
}

function closeImportDialog(dialogID) {
  const dialog = document.getElementById(dialogID);
  if (dialog && dialog.open) {
    dialog.close();
  }
}

function setImportFormStatus(form, message, isError) {
  if (!form) return;
  let statusNode = form.querySelector('.import-form-status');
  if (!statusNode) {
    statusNode = document.createElement('p');
    statusNode.className = 'import-form-status muted';
    statusNode.style.marginTop = '0';
    statusNode.style.marginBottom = '0.55rem';
    const bodyNode = form.querySelector('.dialog-body');
    if (bodyNode) {
      bodyNode.prepend(statusNode);
    } else {
      form.prepend(statusNode);
    }
  }
  statusNode.textContent = String(message || '');
  statusNode.style.color = isError ? 'var(--danger)' : 'var(--muted)';
}

function clearImportFormStatus(form) {
  if (!form) return;
  const statusNode = form.querySelector('.import-form-status');
  if (statusNode) {
    statusNode.textContent = '';
  }
}

function summarizeImportOverlaps(overlaps) {
  const preview = overlaps.slice(0, 4).map((item) => {
    const existing = item.existingId ? ('#' + item.existingId) : '?';
    return item.date + ' ' + item.start + '-' + item.end + ' ' + item.project + ' (existing ' + existing + ')';
  });
  const remainder = overlaps.length > 4 ? ('; +' + (overlaps.length - 4) + ' more') : '';
  return preview.join('; ') + remainder;
}

async function submitImportForm(form, options) {
  const formData = new FormData(form);
  const preview = await apiFetch('POST', '/api/import-preview', null, { formData: formData });
  openImportPreviewDialog(preview, form, options || {});
}

function openImportPreviewDialog(previewData, form, options) {
  const dialog = document.getElementById('import-preview-dialog');
  const body = document.getElementById('preview-body');
  const summary = document.getElementById('preview-summary');
  const filenameNode = document.getElementById('preview-filename');
  if (!dialog || !body || !summary || !filenameNode) return;
  const previewState = importPreviewStore();
  if (!previewState) return;

  previewState.form = form;
  previewState.options = options || {};
  previewState.entries = Array.isArray(previewData.entries) ? previewData.entries : [];
  setImportPreviewStatus('', false);

  const fileInput = form ? form.querySelector('input[name=file]') : null;
  const fileName = fileInput && fileInput.files && fileInput.files[0] ? fileInput.files[0].name : '';
  filenameNode.textContent = fileName;

  body.innerHTML = '';
  let cleanCount = 0;
  let overlapCount = 0;
  let duplicateCount = 0;

  for (const entry of previewState.entries) {
    const status = String(entry.status || '');
    if (status === 'clean') cleanCount++;
    if (status === 'overlap') overlapCount++;
    if (status === 'duplicate') duplicateCount++;

    const row = document.createElement('tr');
    row.dataset.index = String(Number(entry.index || 0));
    row.dataset.status = status;

    let checkboxHTML = '<span class="muted">-</span>';
    if (status === 'clean') {
      checkboxHTML = '<input type="checkbox" class="preview-select" checked onchange="updatePreviewCount()">';
    } else if (status === 'overlap') {
      checkboxHTML = '<input type="checkbox" class="preview-select" onchange="updatePreviewCount()">';
    }

    let conflictText = '';
    if (status === 'overlap' && entry.conflictId) {
      conflictText = ' \u26A0 conflicts #' + String(entry.conflictId);
    } else if (status === 'duplicate' && entry.conflictId) {
      conflictText = ' already exists #' + String(entry.conflictId);
    }

    let badgeClass = 'badge-local';
    let badgeLabel = 'clean';
    if (status === 'overlap') {
      badgeClass = 'badge-conflict';
      badgeLabel = 'overlap';
    } else if (status === 'duplicate') {
      badgeClass = 'badge-synced';
      badgeLabel = 'already exists';
    }

    row.innerHTML = '' +
      '<td>' + checkboxHTML + '</td>' +
      '<td><span class="js-fmt-date" data-iso="' + escapeHtml(String(entry.date || '')) + '">' + escapeHtml(String(entry.date || '')) + '</span></td>' +
      '<td><span class="js-fmt-time" data-hhmm="' + escapeHtml(String(entry.start || '')) + '">' + escapeHtml(String(entry.start || '')) + '</span></td>' +
      '<td><span class="js-fmt-time" data-hhmm="' + escapeHtml(String(entry.end || '')) + '">' + escapeHtml(String(entry.end || '')) + '</span></td>' +
      '<td>' + escapeHtml(String(entry.project || '')) + '</td>' +
      '<td>' + escapeHtml(String(entry.activity || '')) + '</td>' +
      '<td>' + escapeHtml(String(entry.skill || '')) + '</td>' +
      '<td class="num"><span class="js-fmt-hours" data-mins="' + escapeHtml(String(Number(entry.durationMins || 0))) + '">' + escapeHtml(String(Number(entry.durationMins || 0))) + '</span></td>' +
      '<td class="num"><span class="js-fmt-hours" data-mins="' + escapeHtml(String(Number(entry.billableMins || 0))) + '">' + escapeHtml(String(Number(entry.billableMins || 0))) + '</span></td>' +
      '<td>' + escapeHtml(String(entry.description || '')) + '</td>' +
      '<td><span class="badge ' + badgeClass + '">' + badgeLabel + '</span>' + escapeHtml(conflictText) + '</td>';
    body.appendChild(row);
  }

  summary.textContent = previewState.entries.length + ' entries: ' +
    cleanCount + ' clean, ' + overlapCount + ' overlapping, ' + duplicateCount + ' duplicate';
  updatePreviewCount();
  applyLocaleFormatting(dialog);
  dialog.showModal();
  requestAnimationFrame(() => {
    const firstCheckbox = body.querySelector('input.preview-select');
    if (firstCheckbox) {
      firstCheckbox.focus();
      return;
    }
    const importButton = document.getElementById('preview-import-btn');
    if (importButton) {
      importButton.focus();
    }
  });
}

function updatePreviewCount() {
  const body = document.getElementById('preview-body');
  const button = document.getElementById('preview-import-btn');
  if (!body || !button) return;

  let count = 0;
  for (const row of Array.from(body.querySelectorAll('tr'))) {
    const status = row.dataset.status || '';
    const checkbox = row.querySelector('input.preview-select');
    if ((status === 'clean' || status === 'overlap') && checkbox && checkbox.checked) {
      count++;
    }
  }
  button.textContent = 'Import selected (' + count + ')';
}

function setImportPreviewStatus(message, isError) {
  const statusNode = document.getElementById('preview-status');
  if (!statusNode) return;
  statusNode.textContent = String(message || '');
  statusNode.style.color = isError ? 'var(--danger)' : 'var(--muted)';
}

function cancelImportPreview() {
  const previewState = importPreviewStore();
  const form = previewState ? previewState.form : null;
  if (form) {
    const fileInput = form.querySelector('input[name=file]');
    if (fileInput) {
      fileInput.value = '';
    }
  }
  const dialog = document.getElementById('import-preview-dialog');
  if (dialog && dialog.open) {
    dialog.close();
  }
  setImportPreviewStatus('', false);
  if (previewState) {
    previewState.reset();
  }
}

async function confirmImportPreview(modeFlag) {
  const previewState = importPreviewStore();
  if (!previewState) return;
  const form = previewState.form;
  if (!form) return;
  const importButton = document.getElementById('preview-import-btn');
  if (importButton) {
    importButton.disabled = true;
  }
  setImportPreviewStatus('Importing selected rows...', false);

  const body = document.getElementById('preview-body');
  const skipIndices = [];
  if (body) {
    for (const row of Array.from(body.querySelectorAll('tr'))) {
      const index = Number(row.dataset.index || '-1');
      const status = row.dataset.status || '';
      if (!Number.isInteger(index) || index < 0) continue;

      if (status === 'duplicate') {
        skipIndices.push(index);
        continue;
      }

      const checkbox = row.querySelector('input.preview-select');
      const checked = checkbox ? checkbox.checked : false;
      if ((status === 'clean' || status === 'overlap') && !checked) {
        skipIndices.push(index);
      }
    }
  }

  const formData = new FormData(form);
  formData.append('skipIndices', skipIndices.join(','));
  if (modeFlag) {
    formData.append(modeFlag, 'true');
  }

  try {
    const result = await apiFetch('POST', '/api/import', null, { formData: formData });
    const options = previewState.options || {};
    let message = 'Imported ' + result.rowsPersisted + ' row(s).';
    if (result.overlapsSkipped) {
      message += ' Skipped ' + result.overlapsSkipped + ' overlapping row(s).';
    }
    if (result.reconcileWarning) {
      message += ' Reconcile warning: ' + String(result.reconcileWarning);
    }
    setImportPreviewStatus(message, false);
    cancelImportPreview();
    openStatusDialog('Import result', '<div class="result-box">' + escapeHtml(message) + '</div>');
    if (options && options.dialogID) {
      closeImportDialog(options.dialogID);
    }
    showToast(message, false);
    if (options && options.day) {
      htmx.ajax('GET', '/partials/day/' + encodeURIComponent(options.day), {
        target: '#day-entries',
        swap: 'innerHTML',
      });
    } else if (options && options.refreshURL) {
      window.location.href = options.refreshURL;
    } else if (options && options.refreshMonth) {
      htmx.ajax('GET', '/partials/month/' + encodeURIComponent(options.refreshMonth) + '?refresh=1', {
        target: '#month-rows',
        swap: 'innerHTML',
      });
    }
  } catch (err) {
    if (
      !modeFlag &&
      err &&
      err.status === 409 &&
      err.payload &&
      Array.isArray(err.payload.overlaps) &&
      err.payload.overlaps.length > 0
    ) {
      const overlaps = err.payload.overlaps;
      openConfirmDialog(
        'Overlapping entries found',
        'Some imported entries overlap existing local entries: ' + summarizeImportOverlaps(overlaps),
        function() { confirmImportPreview('forceOverlapping'); },
        'Import anyway',
        {
          label: 'Skip overlapping',
          onConfirm: function() { confirmImportPreview('skipOverlapping'); },
        },
      );
      if (importButton) {
        importButton.disabled = false;
      }
      return;
    }
    const errMsg = String(err.message || err);
    setImportPreviewStatus(errMsg, true);
    openStatusDialog(
      'Import failed',
      '<div class="result-box">' + escapeHtml(errMsg) + '</div>'
    );
    showToast(errMsg, true);
  } finally {
    if (importButton) {
      importButton.disabled = false;
    }
  }
}

async function handleImportSubmit(event, options) {
  event.preventDefault();
  const form = event.target;
  clearImportFormStatus(form);
  try {
    await submitImportForm(form, options || {});
  } catch (err) {
    const errMsg = String(err.message || err);
    setImportFormStatus(form, errMsg, true);
    openStatusDialog(
      'Import failed',
      '<div class="result-box">' + escapeHtml(errMsg) + '</div>'
    );
    showToast(errMsg, true);
  }
}

// ── HTML escape ──
function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function normalizeName(value) {
  return String(value || '').trim().toLowerCase();
}

// ── Lookup select helpers ──
function fillSelectByName(select, items, currentName, getID, getName) {
  select.innerHTML = '';
  const currentNorm = normalizeName(currentName);
  let found = false;
  for (const item of items) {
    const name = getName(item);
    const option = document.createElement('option');
    option.value = name;
    option.textContent = name;
    option.dataset.name = name;
    option.dataset.id = String(getID(item));
    if (currentNorm && normalizeName(name) === currentNorm && !found) {
      option.selected = true;
      found = true;
    }
    select.appendChild(option);
  }
  if (!found && currentName) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = currentName + ' (unavailable)';
    option.dataset.name = currentName;
    option.disabled = true;
    option.selected = true;
    select.appendChild(option);
  }
  if (!select.options.length) {
    const option = document.createElement('option');
    option.value = '';
    option.textContent = 'No options';
    option.disabled = true;
    option.selected = true;
    select.appendChild(option);
  }
  if (!found && !currentName && select.options.length > 0) {
    select.selectedIndex = 0;
  }
}

async function buildLookupSelects(currentProject, currentActivity, currentSkill) {
  const lookup = await getLookup();
  const projects = (lookup.projects || []).filter((project) => !project.archived);
  const activitiesAll = (lookup.activities || []);
  const skillsAll = (lookup.skills || []);

  const projectSelect = document.createElement('select');
  projectSelect.name = 'project';
  const activitySelect = document.createElement('select');
  activitySelect.name = 'activity';
  const skillSelect = document.createElement('select');
  skillSelect.name = 'skill';

  fillSelectByName(projectSelect, projects, currentProject, (item) => item.id, (item) => item.name);

  const selectedOptionID = (select) => {
    if (!select || !select.options.length || select.selectedIndex < 0) return NaN;
    return Number(select.options[select.selectedIndex].dataset.id || '');
  };

  const rebuildSkills = (selectedSkill) => {
    const activityID = selectedOptionID(activitySelect);
    const skills = skillsAll.filter((skill) => Number(skill.activityId) === activityID);
    fillSelectByName(skillSelect, skills, selectedSkill || '', (item) => item.id, (item) => item.name);
  };

  const rebuildActivities = (selectedActivity, selectedSkill) => {
    const projectID = selectedOptionID(projectSelect);
    const activities = activitiesAll.filter((activity) => Number(activity.projectId) === projectID && !activity.locked);
    fillSelectByName(activitySelect, activities, selectedActivity || '', (item) => item.id, (item) => item.name);
    rebuildSkills(selectedSkill || '');
  };

  rebuildActivities(currentActivity, currentSkill);
  projectSelect.addEventListener('change', () => rebuildActivities('', ''));
  activitySelect.addEventListener('change', () => rebuildSkills(''));

  return { projectSelect, activitySelect, skillSelect };
}

// ── Day row helpers ──
function parseDayRow(row) {
  return {
    id: Number(row.dataset.id || '0'),
    date: row.dataset.date,
    source: row.dataset.source,
    start: row.dataset.start,
    end: row.dataset.end,
    durationMins: Number(row.dataset.durationMins || '0'),
    project: row.dataset.project,
    activity: row.dataset.activity,
    skill: row.dataset.skill,
    billableMins: Number(row.dataset.billableMins || '0'),
    description: row.dataset.description || ''
  };
}

function closeEditDialog() {
  const state = editStore();
  if (state) {
    state.close();
  }
  syncEditFormEndpoint();
}

function replaceDialogSelect(existingID, select) {
  const existing = document.getElementById(existingID);
  if (!existing || !existing.parentNode) return;
  select.id = existingID;
  select.required = true;
  existing.parentNode.replaceChild(select, existing);
}

function updateDialogDuration(form) {
  const startInput = form.querySelector('[name=start]');
  const endInput = form.querySelector('[name=end]');
  const durationNode = document.getElementById('edit-duration');
  if (!startInput || !endInput || !durationNode) return;

  const startMins = toMins(startInput.value);
  const endMins = toMins(endInput.value);
  const diff = endMins - startMins;
  if (!Number.isFinite(diff) || diff < 0) {
    durationNode.textContent = fmtHours(0) + ' h';
    durationNode.dataset.mins = '0';
    return;
  }
  durationNode.textContent = fmtHours(diff) + ' h';
  durationNode.dataset.mins = String(diff);
}

async function openEditDialog(options) {
  const form = document.getElementById('edit-form');
  const state = editStore();
  if (!form || !state) return;

  const values = options.values || {};
  state.mode = options.mode || 'edit';
  state.date = values.date || '';
  state.rowId = options.row ? Number(options.row.dataset.id || 0) : 0;
  state.title = (state.mode === 'create' ? 'Add entry' : 'Edit entry') + ' \u2014 ' + fmtDate(state.date);
  state.endpoint = state.mode === 'create'
    ? '/partials/day/' + encodeURIComponent(state.date) + '/worklog'
    : '/partials/day/' + encodeURIComponent(state.date) + '/worklog/' + encodeURIComponent(String(state.rowId || 0));
  state.start = values.start || '';
  state.end = values.end || '';
  state.error = '';
  state.forceOverlap = false;
  if (values.billableMins === undefined || values.billableMins === null) {
    state.billableHours = '';
  } else {
    state.billableHours = (Number(values.billableMins) / 60).toFixed(2);
  }
  state.description = values.description || '';

  let selects;
  try {
    selects = await buildLookupSelects(values.project || '', values.activity || '', values.skill || '');
  } catch (err) {
    showToast(String(err.message || err), true);
    closeEditDialog();
    return;
  }
  replaceDialogSelect('edit-project', selects.projectSelect);
  replaceDialogSelect('edit-activity', selects.activitySelect);
  replaceDialogSelect('edit-skill', selects.skillSelect);

  const startInput = form.querySelector('[name=start]');
  const endInput = form.querySelector('[name=end]');
  const billableInput = form.querySelector('[name=billableHours]');
  const descInput = form.querySelector('[name=description]');
  const dateInput = form.querySelector('[name=date]');
  if (dateInput) dateInput.value = state.date;
  if (startInput) startInput.value = state.start;
  if (endInput) endInput.value = state.end;
  if (billableInput) billableInput.value = state.billableHours;
  if (descInput) descInput.value = state.description;

  if (startInput && endInput) {
    startInput.onchange = () => { recalcBillable(form); updateDialogDuration(form); };
    endInput.onchange = () => { recalcBillable(form); updateDialogDuration(form); };
    startInput.oninput = startInput.onchange;
    endInput.oninput = endInput.onchange;
  }

  updateDialogDuration(form);
  syncEditFormEndpoint();
  state.open = true;
  requestAnimationFrame(() => {
    const startInputForFocus = form.querySelector('[name=start]');
    if (startInputForFocus) {
      startInputForFocus.focus();
    }
  });
}

function handleEditAfterRequest(event) {
  const form = document.getElementById('edit-form');
  const state = editStore();
  if (!form || !state || event.target !== form) return;
  if (!event.detail.successful) {
    return;
  }
  const message = state.mode === 'create' ? 'Entry created.' : 'Entry updated.';
  closeEditDialog();
  showToast(message, false);
}

function syncEditFormEndpoint() {
  const form = document.getElementById('edit-form');
  const state = editStore();
  if (!form || !state) return;

  if (state.endpoint) {
    form.setAttribute('hx-post', state.endpoint);
    form.setAttribute('action', state.endpoint);
    form.setAttribute('method', 'post');
  } else {
    form.removeAttribute('hx-post');
    form.removeAttribute('action');
    form.removeAttribute('method');
  }
  if (window.htmx) {
    htmx.process(form);
  }
}

function handleEditResponseError(event) {
  const form = document.getElementById('edit-form');
  const state = editStore();
  if (!form || !state || event.target !== form) return;

  const xhr = event.detail && event.detail.xhr ? event.detail.xhr : null;
  let errorText = 'Request failed';
  let payload = null;
  if (xhr) {
    errorText = String(xhr.responseText || ('HTTP ' + xhr.status));
    try {
      payload = JSON.parse(String(xhr.responseText || '{}'));
    } catch {
      payload = null;
    }
  }

  if (payload && payload.type === 'overlap' && !state.forceOverlap) {
    const existingId = payload.existingId ? String(payload.existingId) : '?';
    openConfirmDialog(
      'Overlapping entry',
      'This entry overlaps with local entry #' + existingId + '. Save anyway?',
      function() {
        state.forceOverlap = true;
        form.requestSubmit();
      },
      'Save anyway'
    );
    return;
  }

  state.forceOverlap = false;
  if (payload && payload.type === 'duplicate') {
    state.error = 'Entry already exists (duplicate).';
  } else if (payload && payload.error) {
    state.error = String(payload.error);
  } else {
    state.error = errorText;
  }
  showToast(state.error, true);
}

async function deleteRowConfirmed(row) {
  if (!row) return;
  const day = row.dataset.date;
  const id = row.dataset.id;
  if (!day || !id) return;
  try {
    await htmx.ajax('POST', '/partials/day/' + encodeURIComponent(day) + '/worklog/' + encodeURIComponent(id) + '/delete', {
      target: '#day-entries',
      swap: 'innerHTML',
    });
    showToast('Entry deleted.', false);
  } catch (err) {
    showToast(String(err.message || err), true);
  }
}

function deleteRow(button) {
  const row = button.closest('tr');
  if (!row) return;
  openConfirmDialog('Delete entry', 'Delete this local entry?', function() {
    deleteRowConfirmed(row);
  }, 'Delete');
}

async function editRow(button) {
  const row = button.closest('tr');
  if (!row || row.dataset.source === 'remote') return;
  const values = parseDayRow(row);
  await openEditDialog({
    mode: 'edit',
    row: row,
    values: values
  });
}

async function addEntryRow(day) {
  await openEditDialog({
    mode: 'create',
    values: {
      date: day,
      start: '',
      end: '',
      project: '',
      activity: '',
      skill: '',
      billableMins: null,
      description: ''
    }
  });
}

function refreshMonthPartial(month, refresh) {
  const query = refresh ? '?refresh=1' : '';
  return htmx.ajax('GET', '/partials/month/' + encodeURIComponent(month) + query, {
    target: '#month-rows',
    swap: 'innerHTML',
  });
}

// ── Month action helpers ──
async function deleteMonthEntries(month) {
  try {
    const result = await apiFetch('DELETE', '/api/month/' + encodeURIComponent(month) + '/worklogs');
    await refreshMonthPartial(month, false);
    showToast('Deleted ' + result.deleted + ' local entries.', false);
  } catch (err) {
    showToast(String(err.message || err), true);
  }
}

async function copyMonthRemote(month) {
  try {
    const result = await apiFetch('POST', '/api/month/' + encodeURIComponent(month) + '/copy-from-remote');
    await refreshMonthPartial(month, false);
    showToast('Copied ' + result.copied + ' of ' + result.total + ' remote entries.', false);
  } catch (err) {
    showToast(String(err.message || err), true);
  }
}

// Backward-compatible alias for older naming.
async function syncMonthRemote(month) {
  await copyMonthRemote(month);
}

async function deleteMonthRemoteEntries(month) {
  openStatusDialog(
    'Delete remote ' + month,
    '<div class="result-box">Deleting remote entries...</div>'
  );
  try {
    const result = await apiFetch('DELETE', '/api/month/' + encodeURIComponent(month) + '/remote-worklogs');
    let html = '<div class="result-box">Deleted: ' + result.deleted + ' | Locked days skipped: ' + result.skippedLocked + '</div>';
    if (Array.isArray(result.lockedDays) && result.lockedDays.length > 0) {
      html += '<div class="result-box">Locked days: ';
      for (let i = 0; i < result.lockedDays.length; i++) {
        const day = String(result.lockedDays[i]);
        if (i > 0) html += ', ';
        html += '<span class="js-fmt-date" data-iso="' + escapeHtml(day) + '">' + escapeHtml(day) + '</span>';
      }
      html += '</div>';
    }
    try {
      await refreshMonthPartial(month, false);
    } catch (reloadErr) {
      html += '<div class="result-box">Delete completed, but month refresh failed: ' + escapeHtml(String(reloadErr.message || reloadErr)) + '</div>';
    }
    openStatusDialog('Delete remote ' + month, html);
    let msg = 'Deleted ' + result.deleted + ' remote entries.';
    if (result.skippedLocked > 0) {
      msg += ' ' + result.skippedLocked + ' locked days skipped.';
    }
    showToast(msg, false);
  } catch (err) {
    openStatusDialog(
      'Delete remote ' + month,
      '<div class="result-box">' + escapeHtml(String(err.message || err)) + '</div>'
    );
    showToast(String(err.message || err), true);
  }
}

// ── Confirm dialog ──
function openConfirmDialog(title, body, onConfirm, confirmLabel, alternative) {
  if (!window.Alpine) return;
  Alpine.store('confirm').openDialog(title, body, onConfirm, confirmLabel, alternative);
}

function closeConfirmDialog() {
  if (!window.Alpine) return;
  Alpine.store('confirm').close();
}

// ── Submit dialog ──
function openSubmitDialog(title, htmlContent) {
  const store = submitStore();
  if (!store) return;
  store.openStatus(title, htmlContent);
  const target = document.getElementById('submit-dialog-result');
  if (target) applyLocaleFormatting(target);
}

function openStatusDialog(title, htmlContent) {
  openSubmitDialog(title, htmlContent);
}

function closeSubmitDialog() {
  const store = submitStore();
  if (!store) return;
  store.close();
  syncSubmitFormEndpoint();
}

function openSubmitAction(scope, value) {
  const store = submitStore();
  if (!store) return;
  store.openSubmit(scope, value);
  syncSubmitFormEndpoint();
  requestAnimationFrame(() => {
    const dryRunToggle = document.getElementById('submit-dry-run');
    if (dryRunToggle) {
      dryRunToggle.focus();
    }
  });
}

function syncSubmitFormEndpoint() {
  const form = document.getElementById('submit-form');
  const store = submitStore();
  if (!form || !store) return;

  const endpoint = store.endpoint();
  if (endpoint) {
    form.setAttribute('hx-post', endpoint);
    form.setAttribute('action', endpoint);
    form.setAttribute('method', 'post');
  } else {
    form.removeAttribute('hx-post');
    form.removeAttribute('action');
    form.removeAttribute('method');
  }
  if (window.htmx) {
    htmx.process(form);
  }
}

function clearHTMXIndicator(indicatorID, event) {
  const indicator = document.getElementById(indicatorID);
  if (indicator) {
    indicator.classList.remove('htmx-request');
  }

  const trigger = event && event.detail && event.detail.elt
    ? event.detail.elt
    : (event ? event.target : null);
  if (!trigger || !trigger.classList) {
    return;
  }
  trigger.classList.remove('htmx-request');
}

function handleSubmitBeforeRequest(event) {
  const form = document.getElementById('submit-form');
  const store = submitStore();
  if (!form || !store) return;

  const endpoint = store.endpoint();
  if (!endpoint) {
    event.preventDefault();
    store.running = false;
    const target = document.getElementById('submit-dialog-result');
    if (target) {
      target.innerHTML = '<div class="dialog-error">Submit target is missing. Close and reopen the submit dialog.</div>';
    }
    showToast('Submit target is missing. Reopen the submit dialog and try again.', true);
    return;
  }

  store.running = true;
}

function handleSubmitAfterRequest(event) {
  const form = document.getElementById('submit-form');
  const store = submitStore();
  if (!form || !store) return;
  store.running = false;
  const target = document.getElementById('submit-dialog-result');
  if (target) applyLocaleFormatting(target);
  if (event.detail.successful) {
    showToast(store.dryRun ? 'Dry-run finished.' : 'Submit finished.', false);
  }
}

function handleSubmitResponseError(event) {
  const form = document.getElementById('submit-form');
  const store = submitStore();
  if (!form || !store) return;
  store.running = false;
  const xhr = event.detail && event.detail.xhr ? event.detail.xhr : null;
  const errorText = xhr ? String(xhr.responseText || ('HTTP ' + xhr.status)) : 'Submit failed';
  const target = document.getElementById('submit-dialog-result');
  if (target) {
    target.innerHTML = '<div class="dialog-error">' + escapeHtml(errorText) + '</div>';
  }
  showToast(errorText, true);
}

function handleActionsMenuKeydown(event) {
  const menu = event.currentTarget;
  if (!menu) return;
  const items = Array.from(menu.querySelectorAll('[role="menuitem"]'))
    .filter((item) => !item.disabled);
  if (!items.length) return;

  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault();
    const active = document.activeElement;
    let index = items.indexOf(active);
    if (index < 0) {
      index = event.key === 'ArrowDown' ? 0 : items.length - 1;
    } else {
      index = event.key === 'ArrowDown'
        ? (index + 1) % items.length
        : (index - 1 + items.length) % items.length;
    }
    items[index].focus();
    return;
  }

  if (event.key === 'Enter' || event.key === ' ' || event.key === 'Spacebar') {
    const active = document.activeElement;
    if (items.includes(active)) {
      event.preventDefault();
      active.click();
    }
  }
}

function handleActionsMenuTriggerKeydown(event) {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') {
    return;
  }

  const trigger = event.currentTarget;
  if (!trigger) return;
  const container = trigger.closest('.actions-menu');
  const menu = container ? container.querySelector('.actions-menu-items') : null;
  if (!menu) return;

  event.preventDefault();
  const focusMenuItem = () => {
    const items = Array.from(menu.querySelectorAll('[role="menuitem"]'))
      .filter((item) => !item.disabled);
    if (!items.length) return;
    const targetIndex = event.key === 'ArrowUp' ? items.length - 1 : 0;
    items[targetIndex].focus();
  };

  const isExpanded = trigger.getAttribute('aria-expanded') === 'true';
  if (!isExpanded) {
    trigger.click();
    requestAnimationFrame(focusMenuItem);
    return;
  }

  focusMenuItem();
}

// ── Day keyboard navigation ──
document.addEventListener('keydown', (e) => {
  // Only when no dialog is open and no input is focused
  if (document.activeElement && (
    document.activeElement.tagName === 'INPUT' ||
    document.activeElement.tagName === 'SELECT' ||
    document.activeElement.tagName === 'TEXTAREA'
  )) return;
  if (document.querySelector('dialog[open]')) return;

  const prevLink = document.getElementById('day-prev-link');
  const nextLink = document.getElementById('day-next-link');

  if (e.key === 'ArrowLeft' && prevLink) {
    e.preventDefault();
    window.location.href = prevLink.href;
  }
  if (e.key === 'ArrowRight' && nextLink) {
    e.preventDefault();
    window.location.href = nextLink.href;
  }
});

document.body.addEventListener('refresh-day', (event) => {
  const detail = event.detail || {};
  const day = String(detail.day || '');
  if (!day) return;
  htmx.ajax('GET', '/partials/day/' + encodeURIComponent(day), {
    target: '#day-entries',
    swap: 'innerHTML',
  });
});

document.body.addEventListener('refresh-month', (event) => {
  const detail = event.detail || {};
  const month = String(detail.month || '');
  if (!month) return;
  htmx.ajax('GET', '/partials/month/' + encodeURIComponent(month), {
    target: '#month-rows',
    swap: 'innerHTML',
  });
});

// ── DOMContentLoaded ──
document.addEventListener('DOMContentLoaded', () => {
  applyLocaleFormatting(document);
  syncSubmitFormEndpoint();

  const importPreviewDialog = document.getElementById('import-preview-dialog');
  if (importPreviewDialog) {
    importPreviewDialog.addEventListener('cancel', (event) => {
      event.preventDefault();
      cancelImportPreview();
    });
    importPreviewDialog.addEventListener('close', () => {
      const state = importPreviewStore();
      if (state) {
        state.reset();
      }
      setImportPreviewStatus('', false);
    });
  }
});
