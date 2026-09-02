(function () {
  'use strict';

  var gantt = null;

  var ui = {
    landing: document.getElementById('landing'),
    foot: document.getElementById('foot'),
    app: document.getElementById('app'),
    loading: document.getElementById('loading'),
    loadingText: document.getElementById('loading-text'),
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
    scaleRange: document.getElementById('scale-range'),
    scaleLabel: document.getElementById('scale-label'),
    sizeRange: document.getElementById('size-range'),
    sizeLabel: document.getElementById('size-label'),
    filterToggle: document.getElementById('filter-toggle'),
    filterLabel: document.getElementById('filter-label'),
    filterItems: document.querySelectorAll('[data-filter]'),
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
  var filter = 'all';
  var visibleSet = null;
  var nonWorkingSeen = {};

  var UNITS = {
    MINUTES: 'min', HOURS: 'h', DAYS: 'd', WEEKS: 'w',
    MONTHS: 'mo', YEARS: 'y', PERCENT: '%',
    ELAPSED_MINUTES: 'min (elapsed)', ELAPSED_HOURS: 'h (elapsed)', ELAPSED_DAYS: 'd (elapsed)',
    ELAPSED_WEEKS: 'w (elapsed)', ELAPSED_MONTHS: 'mo (elapsed)', ELAPSED_YEARS: 'y (elapsed)',
    ELAPSED_PERCENT: '% (elapsed)'
  };

  var FILTER_LABEL = {
    all: 'Filter',
    critical: 'Critical path',
    overdue: 'Overdue',
    incomplete: 'Incomplete',
    milestones: 'Milestones'
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
    opening:      'Opening '
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
    ui.loading.classList.add('is-hidden');
    ui.landing.classList.remove('is-hidden');
    if (ui.foot) { ui.foot.classList.remove('is-hidden'); }
  });

  window.MppUpload.bind({
    drop: ui.drop,
    file: ui.file,
    onStart: function () { ui.error.classList.add('is-hidden'); },
    onError: fail,
    onDone: function (contract, file, projectID, refused) {
      if (projectID && ui.drop.hasAttribute('data-open-saved')) {
        opening(file.name);
        window.location.assign('/p/' + projectID);
        return;
      }

      return loadGantt().then(function () {
        show(contract, file.name, refused);
        window.history.pushState({ chart: true }, '', location.href);
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
        ui.loading.classList.add('is-hidden');
        ui.landing.classList.remove('is-hidden');
        if (ui.foot) { ui.foot.classList.remove('is-hidden'); }
        fail(err.message);
      });
  }

  var glideTo = null;
  var glideFrame = null;
  var glideWas = null;
  var glideStalled = 0;
  var calmMotion = window.matchMedia('(prefers-reduced-motion: reduce)');

  function scrollLimits() {
    var s = gantt.getScrollState();
    var hor = ui.chart.querySelector('.gantt_hor_scroll');
    var ver = ui.chart.querySelector('.gantt_ver_scroll');

    return {
      x: hor ? Math.max(0, hor.scrollWidth - hor.clientWidth) : Math.max(0, s.width - s.inner_width),
      y: ver ? Math.max(0, ver.scrollHeight - ver.clientHeight) : Math.max(0, s.height - s.inner_height),
      at: s
    };
  }

  function glide(dx, dy) {
    var limits = scrollLimits();
    var from = glideTo || { x: limits.at.x, y: limits.at.y };

    glideTo = {
      x: Math.min(Math.max(from.x + dx, 0), limits.x),
      y: Math.min(Math.max(from.y + dy, 0), limits.y)
    };

    if (calmMotion.matches) {
      gantt.scrollTo(glideTo.x, glideTo.y);
      glideTo = null;
      return;
    }

    if (!glideFrame) { glideFrame = requestAnimationFrame(glideStep); }
  }

  function glideStep() {
    var s = gantt.getScrollState();
    var dx = glideTo.x - s.x;
    var dy = glideTo.y - s.y;

    if (Math.abs(dx) < 1 && Math.abs(dy) < 1) {
      stopGlide();
      return;
    }

    if (glideWas && glideWas.x === s.x && glideWas.y === s.y) {
      glideStalled++;
      if (glideStalled > 2) {
        stopGlide();
        return;
      }
    } else {
      glideStalled = 0;
    }

    glideWas = { x: s.x, y: s.y };

    var nx = Math.round(Math.abs(dx) < 1 ? glideTo.x : s.x + dx * 0.28);
    var ny = Math.round(Math.abs(dy) < 1 ? glideTo.y : s.y + dy * 0.28);

    if (nx !== s.x || ny !== s.y) { gantt.scrollTo(nx, ny); }

    glideFrame = requestAnimationFrame(glideStep);
  }

  function stopGlide() {
    if (glideFrame) { cancelAnimationFrame(glideFrame); }
    glideFrame = null;
    glideTo = null;
    glideWas = null;
    glideStalled = 0;
  }

  function wheelPixels(value, mode) {
    if (mode === 1) { return value * 24; }
    if (mode === 2) { return value * 400; }
    return value;
  }

  function bindWheel() {
    ui.chart.addEventListener('wheel', function (e) {
      if (e.ctrlKey) {
        e.preventDefault();
        e.stopPropagation();
        setSize(Math.round(size * 100) - Math.sign(e.deltaY) * 3);
        return;
      }

      var dx = wheelPixels(e.deltaX, e.deltaMode);
      var dy = wheelPixels(e.deltaY, e.deltaMode);

      if (e.shiftKey && !dx) {
        dx = dy;
        dy = 0;
      }

      var limits = scrollLimits();
      if ((dx && limits.x) || (dy && limits.y)) {
        e.preventDefault();
        e.stopPropagation();
        glide(dx, dy);
      }
    }, { capture: true, passive: false });
  }

  function bindPan() {
    var from = null;

    var panTo = null;
    var panFrame = null;

    ui.chart.addEventListener('mousedown', function (e) {
      if (e.button !== 0) { return; }

      stopGlide();

      if (e.target.closest('.gantt_hor_scroll, .gantt_ver_scroll')) { return; }
      if (!e.target.closest('.gantt_task_bg, .gantt_task_row, .gantt_task_cell')) { return; }
      if (e.target.closest('.gantt_task_line, .gantt_link_arrow, .gantt_task_link')) { return; }

      var s = gantt.getScrollState();
      from = { pointerX: e.clientX, pointerY: e.clientY, x: s.x, y: s.y };
      ui.chart.classList.add('is-panning');
      e.preventDefault();
    });

    window.addEventListener('mousemove', function (e) {
      if (!from) { return; }

      var limits = scrollLimits();

      panTo = {
        x: Math.round(Math.min(Math.max(from.x - (e.clientX - from.pointerX), 0), limits.x)),
        y: Math.round(Math.min(Math.max(from.y - (e.clientY - from.pointerY), 0), limits.y))
      };

      if (!panFrame) { panFrame = requestAnimationFrame(panStep); }
    });

    window.addEventListener('mouseup', function () {
      if (!from) { return; }
      from = null;
      panTo = null;
      if (panFrame) { cancelAnimationFrame(panFrame); }
      panFrame = null;
      ui.chart.classList.remove('is-panning');
    });

    function panStep() {
      panFrame = null;
      if (!panTo) { return; }

      var s = gantt.getScrollState();
      if (panTo.x !== s.x || panTo.y !== s.y) { gantt.scrollTo(panTo.x, panTo.y); }
    }
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

  function opening(fileName) {
    ui.error.classList.add('is-hidden');
    ui.landing.classList.add('is-hidden');
    if (ui.foot) { ui.foot.classList.add('is-hidden'); }
    ui.loadingText.textContent = TEXT.opening + (fileName || '');
    ui.loading.classList.remove('is-hidden');
  }

  function show(contract, fileName, refused) {
    ui.landing.classList.add('is-hidden');
    if (ui.foot) { ui.foot.classList.add('is-hidden'); }
    ui.loading.classList.add('is-hidden');
    ui.app.classList.remove('is-hidden');

    if (!started) {
      start();
      started = true;
    }

    loaded = contract;
    loadedName = fileName || '';
    todayISO = toISO(new Date());

    syncLegend();
    resetFilter();
    resetSearch();

    nonWorkingSeen = {};
    model = window.MppMapper.toModel(contract);
    gantt.clearAll();
    gantt.parse({ data: model.data, links: model.links });
    setSize(100, true);
    ui.scaleRange.value = 0;
    zoomRendered = '';
    setZoom(0, true);
    describe(contract, fileName);

    document.dispatchEvent(new CustomEvent('mpp:loaded', {
      detail: { contract: contract, fileName: fileName || '', refused: refused || '' }
    }));
  }

  function start() {
    gantt.config.readonly = true;
    gantt.config.smart_rendering = true;
    gantt.config.open_tree_initially = true;
    gantt.config.scroll_size = 16;

    gantt.config.columns = [
      { name: 'wbs', label: 'WBS', width: 88, resize: true,
        template: function (t) { return escapeHtml(t.$contract.wbs || t.$contract.outline_number || ''); } },
      { name: 'text', label: 'Task', tree: true, width: 320, resize: true,
        template: function (t) { return escapeHtml(t.$contract.name || ''); } },
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
      var inset = 6.84;
      var floor = (label.length * 7.6 + 9.12 + inset).toFixed(2);
      var ceil = 'calc(100% - ' + inset + 'px)';

      return '<span class="bar-pct" style="left:clamp(' + floor + 'px,' +
        parseInt(label, 10) + '%,' + ceil + ')">' + label + '</span>';
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

    bindWheel();
    bindPan();

    gantt.config.link_line_width = 1;
    gantt.config.link_radius = 6;
    gantt.config.link_arrow_size = 6;

    gantt.attachEvent('onBeforeTaskDisplay', function (id) {
      return !visibleSet || visibleSet[id] === true;
    });

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
    var project = contract.project || {};
    var root = projectRow(contract);
    var parts = [];

    if (project.start) { parts.push(summaryCell(TEXT.projStart, contractDate(project.start))); }
    if (project.finish) { parts.push(summaryCell(TEXT.projFinish, contractDate(project.finish))); }
    if (root && root.duration) {
      parts.push(summaryCell(TEXT.projDuration, duration(root.duration)));
    }

    if (!parts.length) { ui.summary.classList.add('is-hidden'); return; }

    ui.summary.innerHTML = parts.join('');
    ui.summary.classList.remove('is-hidden');
  }

  function summaryCell(label, value) {
    return '<span>' + label + ': <b>' + value + '</b></span>';
  }

  function projectRow(contract) {
    var tasks = contract.tasks || [];

    for (var i = 0; i < tasks.length; i++) {
      if (tasks[i].parent_id === null && tasks[i].outline_level === 0) { return tasks[i]; }
    }

    return null;
  }

  function isNonWorking(date) {
    if (!model) { return false; }

    var iso = toISO(date);
    if (iso in nonWorkingSeen) { return nonWorkingSeen[iso]; }

    var answer = !!model.calendar.nonWorking[date.getDay()];

    for (var i = 0; i < model.calendar.exceptions.length; i++) {
      var ex = model.calendar.exceptions[i];
      if (iso >= ex.from && iso <= ex.to) {
        answer = !ex.working;
        break;
      }
    }

    nonWorkingSeen[iso] = answer;
    return answer;
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
        ' (' + escapeHtml(LINK_LABEL[r.type] || r.type) + lag(r.lag) + ')';
    });
    if (predecessors.length) { rows.push([TEXT.predecessors, predecessors.join('<br>')]); }

    var successors = (model.index.successors[task.id] || []).map(function (r) {
      return escapeHtml(model.index.taskName[r.successor_id] || ('#' + r.successor_id)) +
        ' (' + escapeHtml(LINK_LABEL[r.type] || r.type) + lag(r.lag) + ')';
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

  var SPANS = [
    { unit: 'day', days: 1, label: 'Day', least: 24 },
    { unit: 'week', days: 7, label: 'Week', least: 46 },
    { unit: 'month', days: 30.44, label: 'Month', least: 38 },
    { unit: 'quarter', days: 91.31, label: 'Quarter', least: 30 }
  ];

  var PPD_NEAR = 44;
  var PPD_FAR = 0.34;

  var zoom = 0;
  var zoomShown = 0;
  var zoomFrame = null;
  var zoomRendered = '';
  var zoomHold = null;
  var zoomSkip = false;

  var SIZE_BASE = { row: 41, bar: 25, header: 59, text: 44 };
  var SIZE_MIN = 60;
  var SIZE_MAX = 100;

  var size = 1;
  var sizeFrame = null;

  function setSize(percent, immediate) {
    size = Math.min(Math.max(percent, SIZE_MIN), SIZE_MAX) / 100;

    ui.sizeRange.value = Math.round(size * 100);
    ui.sizeLabel.textContent = Math.round(size * 100) + '%';

    if (immediate) {
      applySize();
      return;
    }

    if (!sizeFrame) {
      sizeFrame = requestAnimationFrame(function () {
        sizeFrame = null;
        applySize();
      });
    }
  }

  function applySize() {
    ui.chart.style.setProperty('--gantt-size', size.toFixed(3));
    applyZoom(spanFor(zoomShown));
  }

  function spanFor(value) {
    var ppd = PPD_NEAR * Math.pow(PPD_FAR / PPD_NEAR, value / 100) * size;

    for (var i = 0; i < SPANS.length; i++) {
      var width = ppd * SPANS[i].days;
      if (width >= SPANS[i].least || i === SPANS.length - 1) {
        return { span: SPANS[i], width: Math.max(SPANS[i].least, Math.round(width)) };
      }
    }
  }

  function setZoom(value, immediate) {
    zoom = value;

    var picked = spanFor(value);
    ui.scaleLabel.textContent = picked.span.label;
    ui.scaleRange.setAttribute('aria-valuetext', picked.span.label);

    if (!zoomHold) { zoomHold = holdCentre(); }

    if (immediate || calmMotion.matches) {
      if (zoomFrame) { cancelAnimationFrame(zoomFrame); }
      zoomFrame = null;
      zoomShown = value;
      applyZoom(picked);
      zoomHold = null;
      return;
    }

    if (!zoomFrame) { zoomFrame = requestAnimationFrame(zoomStep); }
  }

  function holdCentre() {
    var state = gantt.getScrollState();
    var middle = state.inner_width / 2;

    return { date: gantt.dateFromPos(state.x + middle), middle: middle, y: state.y };
  }

  function zoomStep() {
    var gap = zoom - zoomShown;

    if (Math.abs(gap) < 0.4) {
      zoomShown = zoom;
      zoomFrame = null;
      applyZoom(spanFor(zoomShown));
      zoomHold = null;
      return;
    }

    zoomSkip = !zoomSkip;

    if (!zoomSkip) {
      zoomShown += gap * 0.42;
      applyZoom(spanFor(zoomShown));
    }

    zoomFrame = requestAnimationFrame(zoomStep);
  }

  function applyZoom(picked) {
    var column = picked.width;
    var mark = picked.span.unit + ':' + column;
    if (mark === zoomRendered) { return; }
    zoomRendered = mark;

    var hold = zoomHold || holdCentre();

    gantt.config.row_height = SIZE_BASE.row;
    gantt.config.scale_height = SIZE_BASE.header;
    gantt.config.bar_height = Math.round(SIZE_BASE.bar * size);

    minFillWidth = gantt.config.bar_height * 2;
    minTextWidth = Math.round(SIZE_BASE.text * size);

    setScale(picked.span.unit);
    gantt.config.min_column_width = column;
    gantt.render();

    if (hold.date) {
      gantt.scrollTo(Math.max(0, gantt.posFromDate(hold.date) - hold.middle), hold.y);
    }
  }

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
  }

  function quarterLabel(date) {
    return 'Q' + (Math.floor(date.getMonth() / 3) + 1);
  }

  ui.scaleRange.addEventListener('input', function () {
    setZoom(Number(ui.scaleRange.value));
  });

  ui.sizeRange.addEventListener('input', function () {
    setSize(Number(ui.sizeRange.value));
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

  function expandAll() {
    gantt.batchUpdate(function () {
      gantt.eachTask(function (task) { task.$open = true; });
    });
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
    collectMatches();

    if (!needle) {
      if (filter === 'all') { restoreTree(); }
      updateCount();
      gantt.refreshData();
      return true;
    }

    rememberTree();
    expandAll();

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

  function collectMatches() {
    matches = [];
    matchSet = {};
    matchIndex = -1;

    if (!needle) { return; }

    (loaded && loaded.tasks ? loaded.tasks : []).forEach(function (task) {
      if (!isVisibleTask(task) || !matchesTask(task)) { return; }

      var id = window.MppMapper.key(task.id);
      matches.push(id);
      matchSet[id] = true;
    });

    if (matches.length) { matchIndex = 0; }
  }

  function passesFilter(task) {
    switch (filter) {
      case 'critical': return task.is_critical;
      case 'overdue': return isOverdue(task);
      case 'incomplete': return task.percent_complete < 100;
      case 'milestones': return task.is_milestone;
      default: return true;
    }
  }

  function isVisibleTask(task) {
    return !visibleSet || visibleSet[window.MppMapper.key(task.id)] === true;
  }

  function buildVisible() {
    if (filter === 'all' || !loaded) {
      visibleSet = null;
      return;
    }

    var parents = {};
    (loaded.tasks || []).forEach(function (task) { parents[task.id] = task.parent_id; });

    visibleSet = {};

    (loaded.tasks || []).forEach(function (task) {
      if (!passesFilter(task)) { return; }

      var id = task.id;

      while (id !== null && id !== undefined) {
        var key = window.MppMapper.key(id);
        if (visibleSet[key]) { break; }

        visibleSet[key] = true;
        id = parents[id];
      }
    });
  }

  function syncFilterUI() {
    ui.filterToggle.classList.toggle('is-active', filter !== 'all');
    ui.filterLabel.textContent = FILTER_LABEL[filter];

    ui.filterItems.forEach(function (item) {
      item.classList.toggle('is-active', item.dataset.filter === filter);
    });
  }

  function setFilter(next) {
    if (filter === next) { return; }

    filter = next;

    if (filter === 'all') {
      if (!needle) { restoreTree(); }
    } else {
      rememberTree();
      expandAll();
    }

    buildVisible();
    syncFilterUI();
    collectMatches();
    updateCount();
    gantt.refreshData();
    focusMatch();
  }

  function resetFilter() {
    filter = 'all';
    visibleSet = null;
    syncFilterUI();
  }

  ui.filterItems.forEach(function (item) {
    item.addEventListener('click', function () {
      item.closest('[data-menu-panel]').classList.add('is-hidden');
      ui.filterToggle.setAttribute('aria-expanded', 'false');

      setFilter(item.dataset.filter);
    });
  });

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
    return escapeHtml(String(value.value) + ' ' + (UNITS[value.units] || value.units));
  }

  function lag(value) {
    if (!value || !value.value) { return ''; }
    return ' ' + (value.value > 0 ? '+' : '') + duration(value);
  }

  function shortDateTime(value) {
    if (!value) { return '-'; }
    return escapeHtml(value.slice(0, 10) + ' ' + value.slice(11, 16));
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
