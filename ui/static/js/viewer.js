(function () {
  'use strict';

  var gantt = window.gantt;

  var ui = {
    projectName: document.getElementById('project-name'),
    stats: document.getElementById('stats'),
    chart: document.getElementById('gantt-here'),
    details: document.getElementById('details'),
    detailsBody: document.getElementById('details-body'),
    search: document.getElementById('search')
  };

  var model = null;
  var showCritical = true;

  var UNITS = {
    MINUTES: 'мин', HOURS: 'ч', DAYS: 'дн', WEEKS: 'нед',
    MONTHS: 'мес', YEARS: 'лет', PERCENT: '%',
    ELAPSED_MINUTES: 'мин (кал.)', ELAPSED_HOURS: 'ч (кал.)', ELAPSED_DAYS: 'дн (кал.)',
    ELAPSED_WEEKS: 'нед (кал.)', ELAPSED_MONTHS: 'мес (кал.)', ELAPSED_YEARS: 'лет (кал.)',
    ELAPSED_PERCENT: '% (кал.)'
  };

  var LINK_LABEL = {
    FINISH_START: 'ОН', START_START: 'НН', FINISH_FINISH: 'ОО', START_FINISH: 'НО'
  };

  gantt.config.readonly = true;
  gantt.config.smart_rendering = true;
  gantt.config.open_tree_initially = true;
  gantt.config.row_height = 30;
  gantt.config.bar_height = 18;

  gantt.config.columns = [
    { name: 'wbs', label: 'СДР', width: 78, resize: true,
      template: function (t) { return t.$contract.wbs || t.$contract.outline_number || ''; } },
    { name: 'text', label: 'Задача', tree: true, width: 280, resize: true },
    { name: 'start', label: 'Начало', align: 'center', width: 96, resize: true,
      template: function (t) { return shortDate(t.start_date); } },
    { name: 'duration', label: 'Длит.', align: 'center', width: 70,
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

  load();

  function load() {
    var query = new URLSearchParams(location.search).get('demo');
    var url = '/api/v1/schedule' + (query ? '?demo=' + encodeURIComponent(query) : '');

    fetch(url)
      .then(function (response) {
        if (!response.ok) { throw new Error('сервер ответил ' + response.status); }
        return response.json();
      })
      .then(function (contract) {
        model = window.MppMapper.toModel(contract);
        gantt.clearAll();
        gantt.parse({ data: model.data, links: model.links });
        setScale('day');
        describe(contract);
      })
      .catch(function (err) {
        ui.projectName.textContent = 'не удалось загрузить план: ' + err.message;
      });
  }

  function describe(contract) {
    var fileName = document.querySelector('.bar').dataset.fileName || '';
    ui.projectName.textContent = fileName || (contract.project && contract.project.name) || 'без названия';

    var critical = contract.tasks.filter(function (t) { return t.is_critical; }).length;
    ui.stats.textContent =
      contract.tasks.length + ' задач · ' +
      contract.relations.length + ' связей · ' +
      critical + ' на критическом пути · ' +
      'календарь: ' + (model.calendar.name || 'по умолчанию');
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
      ['Название', escapeHtml(task.name)],
      ['СДР', escapeHtml(task.wbs || task.outline_number || '—')],
      ['Начало', shortDateTime(task.start)],
      ['Окончание', shortDateTime(task.finish)],
      ['Длительность', duration(task.duration)],
      ['Выполнено', Math.round(task.percent_complete) + '%']
    ];

    if (task.is_critical) { rows.push(['Критическая', 'да']); }
    if (task.is_milestone) { rows.push(['Веха', 'да']); }
    if (task.baseline) {
      rows.push(['Baseline', shortDateTime(task.baseline.start) + ' — ' + shortDateTime(task.baseline.finish)]);
    }

    var people = (task.assignments || []).map(function (a) {
      var name = model.index.resourceName[a.resource_id] || ('#' + a.resource_id);
      return escapeHtml(name) + (a.units != null ? ' [' + a.units + '%]' : '');
    });
    if (people.length) { rows.push(['Ресурсы', people.join(', ')]); }

    var predecessors = (model.index.predecessors[task.id] || []).map(function (r) {
      return escapeHtml(model.index.taskName[r.predecessor_id] || ('#' + r.predecessor_id)) +
        ' (' + (LINK_LABEL[r.type] || r.type) + lag(r.lag) + ')';
    });
    if (predecessors.length) { rows.push(['Предшественники', predecessors.join('<br>')]); }

    var successors = (model.index.successors[task.id] || []).map(function (r) {
      return escapeHtml(model.index.taskName[r.successor_id] || ('#' + r.successor_id)) +
        ' (' + (LINK_LABEL[r.type] || r.type) + lag(r.lag) + ')';
    });
    if (successors.length) { rows.push(['Последователи', successors.join('<br>')]); }

    if (task.notes) { rows.push(['Заметки', escapeHtml(task.notes)]); }

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
        { unit: 'week', step: 1, format: 'н. %W' }
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
    return 'кв. ' + (Math.floor(date.getMonth() / 3) + 1);
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
      gantt.eachTask(function (task) {
        task.$open = true;
      });
    });
    var first = null;
    gantt.eachTask(function (task) {
      if (!first && task.text.toLowerCase().indexOf(needle) >= 0) { first = task.id; }
    });
    if (first) { gantt.showTask(first); }
  });

  function duration(value) {
    if (!value) { return ''; }
    var units = UNITS[value.units] || value.units;
    return trimNumber(value.value) + ' ' + units;
  }

  function lag(value) {
    if (!value || !value.value) { return ''; }
    var sign = value.value > 0 ? '+' : '';
    return ' ' + sign + duration(value);
  }

  function trimNumber(value) {
    return Number.isInteger(value) ? String(value) : String(value);
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
