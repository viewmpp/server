(function () {
  'use strict';

  var gantt = null;

  var ui = {
    landing: document.getElementById('landing'),
    foot: document.getElementById('foot'),
    app: document.getElementById('app'),
    drop: document.getElementById('drop'),
    file: document.getElementById('file'),
    error: document.getElementById('error'),
    projectName: document.getElementById('project-name'),
    stats: document.getElementById('stats'),
    chart: document.getElementById('gantt-here'),
    details: document.getElementById('details'),
    detailsBody: document.getElementById('details-body'),
    search: document.getElementById('search'),
    searchBox: document.getElementById('searchbox'),
    searchCount: document.getElementById('search-count'),
    summary: document.getElementById('summary'),
    toggleCritical: document.getElementById('toggle-critical'),
    toggleOverdue: document.getElementById('toggle-overdue'),
    legendToday: document.getElementById('legend-today'),
    legendCritical: document.getElementById('legend-critical'),
    legendOverdue: document.getElementById('legend-overdue')
  };

  var model = null;
  var loaded = null;
  var loadedName = '';
  var showCritical = ui.toggleCritical.checked;
  var showOverdue = ui.toggleOverdue.checked;
  var started = false;
  var todayLine = null;
  var todayISO = '';
  var minTextWidth = 44;
  var minFillWidth = 0;
  var matches = [];
  var matchSet = {};
  var matchIndex = -1;
  var needle = '';
  var searchTimer = null;
  var treeState = null;

  var UNITS = {
    MINUTES: 'min', HOURS: 'h', DAYS: 'd', WEEKS: 'w',
    MONTHS: 'mo', YEARS: 'y', PERCENT: '%',
    ELAPSED_MINUTES: 'min (elapsed)', ELAPSED_HOURS: 'h (elapsed)', ELAPSED_DAYS: 'd (elapsed)',
    ELAPSED_WEEKS: 'w (elapsed)', ELAPSED_MONTHS: 'mo (elapsed)', ELAPSED_YEARS: 'y (elapsed)',
    ELAPSED_PERCENT: '% (elapsed)'
  };

  var LINK_LABEL = {
    FINISH_START: 'FS', START_START: 'SS', FINISH_FINISH: 'FF', START_FINISH: 'SF'
  };

  var TEXT = {
    ganttFailed:  'Could not load the chart library',
    noName:       'untitled',
    tasks:        'tasks',
    links:        'links',
    onCritical:   'on the critical path',
    calendar:     'calendar',
    defaultCal:   'default',
    name:         'Name',
    wbs:          'WBS',
    start:        'Start',
    finish:       'Finish',
    duration:     'Duration',
    complete:     'Complete',
    critical:     'Critical',
    milestone:    'Milestone',
    baseline:     'Baseline',
    resources:    'Resources',
    predecessors: 'Predecessors',
    successors:   'Successors',
    notes:        'Notes',
    overdue:      'Overdue',
    overdueTask:  'overdue task',
    overdueTasks: 'overdue tasks',
    day:          'day',
    days:         'days',
    yes:          'yes',
    projStart:    'Start',
    projFinish:   'Finish',
    projDuration: 'Duration',
    weeks:        'w'
  };

  var openProjectID = ui.app.dataset.projectId;
  var openExampleName = ui.app.dataset.exampleName;
  var servedOpen = Boolean(openProjectID || openExampleName);

  if (openProjectID) {
    openProject(openProjectID, ui.app.dataset.fileName);
  } else if (openExampleName) {
    openExample(openExampleName, ui.app.dataset.fileName);
  }

  window.addEventListener('popstate', function () {
    if (servedOpen) { return; }

    ui.details.classList.add('is-hidden');
    ui.app.classList.add('is-hidden');
    ui.landing.classList.remove('is-hidden');
    if (ui.foot) { ui.foot.classList.remove('is-hidden'); }
  });

  window.MppUpload.bind({
    drop: ui.drop,
    file: ui.file,
    onStart: function () { ui.error.classList.add('is-hidden'); },
    onError: fail,
    onDone: function (contract, file, projectID, refused) {
      return loadGantt().then(function () {
        show(contract, file.name, refused);
        window.history.pushState({ chart: true }, '', projectID ? '/p/' + projectID : location.href);
      });
    }
  });

  var exportButton = document.getElementById('export-xlsx');
  if (exportButton) {
    exportButton.addEventListener('click', function () {
      if (!loaded) { return; }

      exportButton.disabled = true;

      window.MppExport.xlsx(loaded, loadedName)
        .catch(function (err) { fail(err.message); })
        .then(function () { exportButton.disabled = false; });
    });
  }

  document.getElementById('reset').addEventListener('click', function () {
    if (servedOpen) {
      window.location.assign('/');
      return;
    }
    ui.file.value = '';
    window.history.back();
  });


  function openProject(id, fileName) {
    openContract('/api/v1/projects/' + encodeURIComponent(id), fileName);
  }

  function openExample(name, fileName) {
    openContract('/api/v1/examples/' + encodeURIComponent(name), fileName);
  }

  function openContract(url, fileName) {
    fetch(url)
      .then(function (response) {
        if (!response.ok) { throw new Error(window.MppText.serverSaid(response.status)); }
        return response.json();
      })
      .then(function (contract) {
        return loadGantt().then(function () { show(contract, fileName); });
      })
      .catch(function (err) {
        ui.app.classList.add('is-hidden');
        ui.landing.classList.remove('is-hidden');
    if (ui.foot) { ui.foot.classList.remove('is-hidden'); }
        fail(err.message);
      });
  }

  function fail(message) {
    ui.error.textContent = message;
    ui.error.classList.remove('is-hidden');
  }

  function loadGantt() {
    if (gantt) { return Promise.resolve(); }

    var css = document.createElement('link');
    css.rel = 'stylesheet';
    css.href = '/static/vendor/dhtmlxgantt.css';
    document.head.appendChild(css);

    return new Promise(function (resolve, reject) {
      var js = document.createElement('script');
      js.src = '/static/vendor/dhtmlxgantt.js';
      js.onload = function () {
        gantt = window.gantt;
        resolve();
      };
      js.onerror = function () {
        reject(new Error(TEXT.ganttFailed));
      };
      document.head.appendChild(js);
    });
  }

  function show(contract, fileName, refused) {
    ui.landing.classList.add('is-hidden');
    if (ui.foot) { ui.foot.classList.add('is-hidden'); }
    ui.app.classList.remove('is-hidden');

    if (!started) {
      start();
      started = true;
    }

    loaded = contract;
    loadedName = fileName || '';
    todayISO = toISO(new Date());

    syncLegend();
    resetSearch();

    model = window.MppMapper.toModel(contract);
    gantt.clearAll();
    gantt.parse({ data: model.data, links: model.links });
    setScale('day');
    describe(contract, fileName);

    document.dispatchEvent(new CustomEvent('mpp:loaded', {
      detail: { contract: contract, fileName: fileName || '', refused: refused || '' }
    }));
  }

  function start() {
    gantt.config.readonly = true;
    gantt.config.smart_rendering = true;
    gantt.config.open_tree_initially = true;
    gantt.config.row_height = 41;
    gantt.config.bar_height = 25;

    minFillWidth = gantt.config.bar_height * 2;

    gantt.config.columns = [
      { name: 'wbs', label: 'WBS', width: 88, resize: true,
        template: function (t) { return t.$contract.wbs || t.$contract.outline_number || ''; } },
      { name: 'text', label: 'Task', tree: true, width: 320, resize: true },
      { name: 'start', label: 'Start', align: 'center', width: 104, resize: true,
        template: function (t) { return contractDate(t.$contract.start); } },
      { name: 'finish', label: 'Finish', align: 'center', width: 104, resize: true,
        template: function (t) { return finishCell(t.$contract); } },
      { name: 'duration', label: 'Dur.', align: 'center', width: 78,
        template: function (t) { return duration(t.$contract.duration); } }
    ];

    gantt.templates.task_class = function (start, end, task) {
      var classes = [];
      if (task.$contract.is_summary) { classes.push('is-summary'); }
      if (showCritical && task.$contract.is_critical) { classes.push('is-critical'); }
      if (showOverdue && isOverdue(task.$contract)) { classes.push('is-overdue'); }
      if (gantt.posFromDate(end) - gantt.posFromDate(start) < minFillWidth) { classes.push('is-narrow'); }
      return classes.join(' ');
    };

    gantt.templates.task_text = function (start, end, task) {
      var c = task.$contract;
      if (c.is_milestone) { return ''; }
      if (gantt.posFromDate(end) - gantt.posFromDate(start) < minTextWidth) { return ''; }

      var label = Math.round(c.percent_complete) + '%';
      var floor = (label.length * 7.6 + 9.12).toFixed(2);

      return '<span class="bar-pct" style="left:max(' + parseInt(label, 10) + '%,' + floor + 'px)">' +
        label + '</span>';
    };

    gantt.templates.timeline_cell_class = function (task, date) {
      return isNonWorking(date) ? 'is-nonworking' : '';
    };

    gantt.templates.grid_row_class = function (start, end, task) {
      var classes = [];
      if (task.$contract.is_summary) { classes.push('is-summary-row'); }
      if (showOverdue && isOverdue(task.$contract)) { classes.push('is-overdue-row'); }
      if (matchSet[task.id]) { classes.push('is-match'); }
      if (matchIndex >= 0 && matches[matchIndex] === task.id) { classes.push('is-match-current'); }
      return classes.join(' ');
    };

    gantt.init(ui.chart);

    gantt.config.link_line_width = 1;
    gantt.config.link_radius = 6;
    gantt.config.link_arrow_size = 6;

    gantt.attachEvent('onGanttRender', placeToday);

    gantt.attachEvent('onTaskClick', function (id) {
      showDetails(gantt.getTask(id).$contract);
      document.dispatchEvent(new CustomEvent('mpp:task'));
      return true;
    });
  }

  function describe(contract, fileName) {
    var name = fileName || (contract.project && contract.project.name) || TEXT.noName;

    ui.projectName.textContent = name;
    ui.projectName.title = name;

    summarise(contract);

    var critical = contract.tasks.filter(function (t) { return t.is_critical; }).length;
    var overdue = contract.tasks.filter(function (t) { return !t.is_summary && isOverdue(t); }).length;

    var parts = [
      contract.tasks.length + ' ' + TEXT.tasks,
      contract.relations.length + ' ' + TEXT.links,
      critical + ' ' + TEXT.onCritical
    ];

    if (overdue) { parts.push(overdue + ' ' + (overdue === 1 ? TEXT.overdueTask : TEXT.overdueTasks)); }

    parts.push(TEXT.calendar + ': ' + (model.calendar.name || TEXT.defaultCal));

    ui.stats.textContent = parts.join(' · ');
  }

  function summarise(contract) {
    var dates = [];
    contract.tasks.forEach(function (t) {
      if (t.start) { dates.push(t.start); }
      if (t.finish) { dates.push(t.finish); }
    });
    if (!dates.length) { ui.summary.classList.add('is-hidden'); return; }

    dates.sort();
    var from = dates[0], to = dates[dates.length - 1];
    var weeks = Math.max(1, Math.round((new Date(to) - new Date(from)) / 604800000));

    ui.summary.innerHTML =
      '<span>' + TEXT.projStart + ': <b>' + escapeHtml(shortDateTime(from).slice(0, 10)) + '</b></span>' +
      '<span>' + TEXT.projFinish + ': <b>' + escapeHtml(shortDateTime(to).slice(0, 10)) + '</b></span>' +
      '<span>' + TEXT.projDuration + ': <b>' + weeks + TEXT.weeks + '</b></span>';
    ui.summary.classList.remove('is-hidden');
  }

  function isNonWorking(date) {
    if (!model) { return false; }
    var iso = toISO(date);
    for (var i = 0; i < model.calendar.exceptions.length; i++) {
      var ex = model.calendar.exceptions[i];
      if (iso >= ex.from && iso <= ex.to) { return !ex.working; }
    }
    return !!model.calendar.nonWorking[date.getDay()];
  }

  function showDetails(task) {
    var rows = [
      [TEXT.name, escapeHtml(task.name)],
      [TEXT.wbs, escapeHtml(task.wbs || task.outline_number || '-')],
      [TEXT.start, shortDateTime(task.start)],
      [TEXT.finish, shortDateTime(task.finish)],
      [TEXT.duration, duration(task.duration)],
      [TEXT.complete, Math.round(task.percent_complete) + '%']
    ];

    if (isOverdue(task)) { rows.push([TEXT.overdue, overdueLabel(task)]); }
    if (task.is_critical) { rows.push([TEXT.critical, TEXT.yes]); }
    if (task.is_milestone) { rows.push([TEXT.milestone, TEXT.yes]); }
    if (task.baseline) {
      rows.push([TEXT.baseline, shortDateTime(task.baseline.start) + ' - ' + shortDateTime(task.baseline.finish)]);
    }

    var people = (task.assignments || []).map(function (a) {
      var name = model.index.resourceName[a.resource_id] || ('#' + a.resource_id);
      return escapeHtml(name) + (a.units != null ? ' [' + a.units + '%]' : '');
    });
    if (people.length) { rows.push([TEXT.resources, people.join(', ')]); }

    var predecessors = (model.index.predecessors[task.id] || []).map(function (r) {
      return escapeHtml(model.index.taskName[r.predecessor_id] || ('#' + r.predecessor_id)) +
        ' (' + (LINK_LABEL[r.type] || r.type) + lag(r.lag) + ')';
    });
    if (predecessors.length) { rows.push([TEXT.predecessors, predecessors.join('<br>')]); }

    var successors = (model.index.successors[task.id] || []).map(function (r) {
      return escapeHtml(model.index.taskName[r.successor_id] || ('#' + r.successor_id)) +
        ' (' + (LINK_LABEL[r.type] || r.type) + lag(r.lag) + ')';
    });
    if (successors.length) { rows.push([TEXT.successors, successors.join('<br>')]); }

    if (task.notes) { rows.push([TEXT.notes, escapeHtml(task.notes)]); }

    ui.detailsBody.innerHTML = '<h2>' + escapeHtml(task.name) + '</h2><dl>' +
      rows.map(function (row) {
        return '<dt>' + row[0] + '</dt><dd>' + row[1] + '</dd>';
      }).join('') + '</dl>';

    ui.details.classList.remove('is-hidden');
  }

  document.getElementById('details-close').addEventListener('click', function () {
    ui.details.classList.add('is-hidden');
  });

  function setScale(scale) {
    if (scale === 'day') {
      gantt.config.scales = [
        { unit: 'month', step: 1, format: '%F %Y' },
        { unit: 'day', step: 1, format: '%d' }
      ];
    } else if (scale === 'week') {
      gantt.config.scales = [
        { unit: 'month', step: 1, format: '%F %Y' },
        { unit: 'week', step: 1, format: 'wk %W' }
      ];
    } else if (scale === 'month') {
      gantt.config.scales = [
        { unit: 'year', step: 1, format: '%Y' },
        { unit: 'month', step: 1, format: '%M' }
      ];
    } else {
      gantt.config.scales = [
        { unit: 'year', step: 1, format: '%Y' },
        { unit: 'quarter', step: 1, format: quarterLabel }
      ];
    }
    gantt.config.scale_height = 59;
    gantt.render();
  }

  function quarterLabel(date) {
    return 'Q' + (Math.floor(date.getMonth() / 3) + 1);
  }

  document.querySelectorAll('[data-scale]').forEach(function (btn) {
    btn.addEventListener('click', function () {
      document.querySelectorAll('[data-scale]').forEach(function (b) { b.classList.remove('is-active'); });
      btn.classList.add('is-active');
      setScale(btn.dataset.scale);
    });
  });

  document.getElementById('collapse-all').addEventListener('click', function () {
    gantt.batchUpdate(function () {
      gantt.eachTask(function (task) { task.$open = false; });
    });
  });

  document.getElementById('expand-all').addEventListener('click', function () {
    gantt.batchUpdate(function () {
      gantt.eachTask(function (task) { task.$open = true; });
    });
  });

  ui.toggleCritical.addEventListener('change', function (e) {
    showCritical = e.target.checked;
    syncLegend();
    gantt.render();
  });

  ui.toggleOverdue.addEventListener('change', function (e) {
    showOverdue = e.target.checked;
    syncLegend();
    gantt.render();
  });

  function syncLegend() {
    ui.legendCritical.classList.toggle('is-hidden', !showCritical);
    ui.legendOverdue.classList.toggle('is-hidden', !showOverdue);
  }

  ui.search.addEventListener('input', function (e) {
    var value = e.target.value;

    clearTimeout(searchTimer);
    searchTimer = setTimeout(function () { runSearch(value); }, 120);
  });

  ui.search.addEventListener('keydown', function (e) {
    clearTimeout(searchTimer);

    if (e.key === 'Escape') {
      ui.search.value = '';
      runSearch('');
      return;
    }

    if (e.key !== 'Enter') { return; }

    e.preventDefault();

    if (!runSearch(ui.search.value)) { stepMatch(e.shiftKey ? -1 : 1); }
  });

  function matchesTask(task) {
    if ((task.name || '').toLowerCase().indexOf(needle) >= 0) { return true; }
    return (task.wbs || task.outline_number || '').toLowerCase().indexOf(needle) >= 0;
  }

  function rememberTree() {
    if (treeState) { return; }

    treeState = {};
    gantt.eachTask(function (task) { treeState[task.id] = task.$open; });
  }

  function restoreTree() {
    if (!treeState) { return; }

    var saved = treeState;
    treeState = null;

    gantt.batchUpdate(function () {
      gantt.eachTask(function (task) {
        if (saved[task.id] !== undefined) { task.$open = saved[task.id]; }
      });
    });
  }

  function updateCount() {
    var on = needle !== '';

    ui.searchBox.classList.toggle('has-count', on);
    ui.searchCount.classList.toggle('is-hidden', !on);
    ui.searchCount.textContent = on ? (matchIndex + 1) + ' / ' + matches.length : '';
  }

  function focusMatch() {
    if (matchIndex >= 0) { gantt.showTask(matches[matchIndex]); }
  }

  function runSearch(value) {
    var next = value.trim().toLowerCase();
    if (next === needle) { return false; }

    needle = next;
    matches = [];
    matchSet = {};
    matchIndex = -1;

    if (!needle) {
      restoreTree();
      updateCount();
      gantt.refreshData();
      return true;
    }

    rememberTree();

    (loaded && loaded.tasks ? loaded.tasks : []).forEach(function (task) {
      if (!matchesTask(task)) { return; }

      var id = window.MppMapper.key(task.id);
      matches.push(id);
      matchSet[id] = true;
    });

    if (matches.length) { matchIndex = 0; }

    gantt.batchUpdate(function () {
      gantt.eachTask(function (task) { task.$open = true; });
    });

    updateCount();
    gantt.refreshData();
    focusMatch();

    return true;
  }

  function stepMatch(delta) {
    if (!matches.length) { return; }

    matchIndex = (matchIndex + delta + matches.length) % matches.length;

    updateCount();
    gantt.refreshData();
    focusMatch();
  }

  function resetSearch() {
    ui.search.value = '';
    needle = '';
    matches = [];
    matchSet = {};
    matchIndex = -1;
    treeState = null;
    updateCount();
  }

  function contractDate(value) {
    return value ? escapeHtml(value.slice(0, 10)) : '';
  }

  function finishCell(task) {
    var text = contractDate(task.finish);
    if (!text || !showOverdue || !isOverdue(task)) { return text; }
    return '<span class="cell-overdue">' + text + '</span>';
  }

  function isOverdue(task) {
    if (!task || !task.finish || task.percent_complete >= 100) { return false; }
    return task.finish.slice(0, 10) < todayISO;
  }

  function overdueLabel(task) {
    var behind = Math.round((new Date(todayISO) - new Date(task.finish.slice(0, 10))) / 86400000);
    return behind + ' ' + (behind === 1 ? TEXT.day : TEXT.days);
  }

  function ensureToday() {
    if (!gantt || !gantt.$task_data) { return null; }

    if (!todayLine || todayLine.parentNode !== gantt.$task_data) {
      todayLine = document.createElement('div');
      todayLine.className = 'today is-hidden';
      todayLine.innerHTML = '<span class="today__label"></span>';
      gantt.$task_data.appendChild(todayLine);
    }

    return todayLine;
  }

  function placeToday() {
    var line = ensureToday();
    if (!line) { return; }

    var now = new Date();
    var state = gantt.getState();

    if (!state.min_date || !state.max_date || now < state.min_date || now > state.max_date) {
      line.classList.add('is-hidden');
      ui.legendToday.classList.add('is-hidden');
      return;
    }

    line.style.left = gantt.posFromDate(now) + 'px';
    line.firstChild.textContent = toISO(now);
    line.classList.remove('is-hidden');
    ui.legendToday.classList.remove('is-hidden');
  }

  function duration(value) {
    if (!value) { return ''; }
    return String(value.value) + ' ' + (UNITS[value.units] || value.units);
  }

  function lag(value) {
    if (!value || !value.value) { return ''; }
    return ' ' + (value.value > 0 ? '+' : '') + duration(value);
  }

  function shortDate(date) {
    return date ? toISO(date) : '';
  }

  function shortDateTime(value) {
    if (!value) { return '-'; }
    return value.slice(0, 10) + ' ' + value.slice(11, 16);
  }

  function toISO(date) {
    var pad = function (n) { return String(n).padStart(2, '0'); };
    return date.getFullYear() + '-' + pad(date.getMonth() + 1) + '-' + pad(date.getDate());
  }

  function escapeHtml(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

})();
