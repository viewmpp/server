(function () {
  'use strict';

  var gantt = null;

  var ui = {
    landing: document.getElementById('landing'),
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
    summary: document.getElementById('summary')
  };

  var model = null;
  var showCritical = true;
  var started = false;

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
    tooLarge:     function (mb) { return 'File is larger than ' + mb + ' MB'; },
    ganttFailed:  'Could not load the chart library',
    serverSaid:   function (status) { return 'server replied ' + status; },
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
    yes:          'yes',
    projStart:    'Start',
    projFinish:   'Finish',
    projDuration: 'Duration',
    weeks:        'w'
  };

  var openProjectID = ui.app.dataset.projectId;
  if (openProjectID) {
    openProject(openProjectID, ui.app.dataset.fileName);
  }

  ui.drop.addEventListener('dragover', function (e) {
    e.preventDefault();
    ui.drop.classList.add('is-over');
  });

  ui.drop.addEventListener('dragleave', function () {
    ui.drop.classList.remove('is-over');
  });

  ui.drop.addEventListener('drop', function (e) {
    e.preventDefault();
    ui.drop.classList.remove('is-over');
    send(e.dataTransfer.files[0]);
  });

  ui.file.addEventListener('change', function () {
    send(ui.file.files[0]);
  });

  document.getElementById('reset').addEventListener('click', function () {
    if (openProjectID) {
      window.location.assign('/');
      return;
    }
    ui.file.value = '';
    ui.details.classList.add('is-hidden');
    ui.app.classList.add('is-hidden');
    ui.landing.classList.remove('is-hidden');
  });

  function send(file) {
    if (!file) { return; }

    var limit = Number(ui.drop.dataset.maxUpload);
    if (limit && file.size > limit) {
      fail(TEXT.tooLarge(Math.round(limit / 1048576)));
      return;
    }

    ui.error.classList.add('is-hidden');
    ui.drop.classList.add('is-busy');

    var res = null;

    fetch('/api/v1/upload?name=' + encodeURIComponent(file.name), {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream' },
      body: file
    })
      .then(function (response) {
        res = response;
        return response.json().then(function (payload) {
          if (!response.ok) { throw new Error(payload || TEXT.serverSaid(response.status)); }
          return payload;
        });
      })
      .then(function (contract) {
        return loadGantt().then(function () {
          show(contract, file.name);
          var id = res && res.headers.get('X-Project-Id');
          if (id) { window.history.replaceState(null, '', '/p/' + id); }
        });
      })
      .catch(function (err) {
        fail(err.message);
      })
      .then(function () {
        ui.drop.classList.remove('is-busy');
      });
  }

  function openProject(id, fileName) {
    fetch('/api/v1/projects/' + encodeURIComponent(id))
      .then(function (response) {
        if (!response.ok) { throw new Error(TEXT.serverSaid(response.status)); }
        return response.json();
      })
      .then(function (contract) {
        return loadGantt().then(function () { show(contract, fileName); });
      })
      .catch(function (err) {
        ui.app.classList.add('is-hidden');
        ui.landing.classList.remove('is-hidden');
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

  function show(contract, fileName) {
    ui.landing.classList.add('is-hidden');
    ui.app.classList.remove('is-hidden');

    if (!started) {
      start();
      started = true;
    }

    model = window.MppMapper.toModel(contract);
    gantt.clearAll();
    gantt.parse({ data: model.data, links: model.links });
    setScale('day');
    describe(contract, fileName);
  }

  function start() {
    gantt.config.readonly = true;
    gantt.config.smart_rendering = true;
    gantt.config.open_tree_initially = true;
    gantt.config.row_height = 36;
    gantt.config.bar_height = 20;

    gantt.config.columns = [
      { name: 'wbs', label: 'WBS', width: 88, resize: true,
        template: function (t) { return t.$contract.wbs || t.$contract.outline_number || ''; } },
      { name: 'text', label: 'Task', tree: true, width: 320, resize: true },
      { name: 'start', label: 'Start', align: 'center', width: 104, resize: true,
        template: function (t) { return shortDate(t.start_date); } },
      { name: 'duration', label: 'Dur.', align: 'center', width: 78,
        template: function (t) { return duration(t.$contract.duration); } }
    ];

    gantt.templates.task_class = function (start, end, task) {
      var classes = [];
      if (task.$contract.is_summary) { classes.push('is-summary'); }
      if (showCritical && task.$contract.is_critical) { classes.push('is-critical'); }
      return classes.join(' ');
    };

    gantt.templates.timeline_cell_class = function (task, date) {
      return isNonWorking(date) ? 'is-nonworking' : '';
    };

    gantt.templates.grid_row_class = function (start, end, task) {
      return task.$contract.is_summary ? 'is-summary-row' : '';
    };

    gantt.init(ui.chart);

    gantt.attachEvent('onTaskClick', function (id) {
      showDetails(gantt.getTask(id).$contract);
      return true;
    });
  }

  function describe(contract, fileName) {
    ui.projectName.textContent =
      fileName || (contract.project && contract.project.name) || TEXT.noName;

    summarise(contract);

    var critical = contract.tasks.filter(function (t) { return t.is_critical; }).length;
    ui.stats.textContent =
      contract.tasks.length + ' ' + TEXT.tasks + ' · ' +
      contract.relations.length + ' ' + TEXT.links + ' · ' +
      critical + ' ' + TEXT.onCritical + ' · ' +
      TEXT.calendar + ': ' + (model.calendar.name || TEXT.defaultCal);
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
      [TEXT.wbs, escapeHtml(task.wbs || task.outline_number || '—')],
      [TEXT.start, shortDateTime(task.start)],
      [TEXT.finish, shortDateTime(task.finish)],
      [TEXT.duration, duration(task.duration)],
      [TEXT.complete, Math.round(task.percent_complete) + '%']
    ];

    if (task.is_critical) { rows.push([TEXT.critical, TEXT.yes]); }
    if (task.is_milestone) { rows.push([TEXT.milestone, TEXT.yes]); }
    if (task.baseline) {
      rows.push([TEXT.baseline, shortDateTime(task.baseline.start) + ' — ' + shortDateTime(task.baseline.finish)]);
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
    gantt.config.scale_height = 52;
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

  document.getElementById('toggle-critical').addEventListener('change', function (e) {
    showCritical = e.target.checked;
    gantt.render();
  });

  document.getElementById('toggle-grid').addEventListener('change', function (e) {
    ui.chart.classList.toggle('no-grid', !e.target.checked);
  });

  ui.search.addEventListener('input', function (e) {
    var needle = e.target.value.trim().toLowerCase();
    gantt.refreshData();
    if (!needle) {
      gantt.eachTask(function (task) { task.$open = true; });
      gantt.render();
      return;
    }
    gantt.batchUpdate(function () {
      gantt.eachTask(function (task) { task.$open = true; });
    });
    var first = null;
    gantt.eachTask(function (task) {
      if (!first && task.text.toLowerCase().indexOf(needle) >= 0) { first = task.id; }
    });
    if (first) { gantt.showTask(first); }
  });

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
    if (!value) { return '—'; }
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
