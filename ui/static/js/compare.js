(function () {
  'use strict';

  var open = document.getElementById('compare-open');
  var input = document.getElementById('compare-file');
  var dialog = document.getElementById('compare');
  if (!open || !input || !dialog) { return; }

  var app = document.getElementById('app');
  var viewer = (app && app.dataset.viewer) || 'file';
  var openProject = (app && app.dataset.projectId) || '';

  var exportButton = document.getElementById('compare-export');
  var gate = document.getElementById('compare-gate');

  var menu = document.getElementById('compare-menu');
  var pickFile = document.getElementById('compare-pick-file');
  var saved = document.getElementById('compare-saved');
  var savedLoaded = false;

  var MINUTES = {
    MINUTES: 1, HOURS: 60, DAYS: 480, WEEKS: 2400, MONTHS: 9600, YEARS: 115200,
    ELAPSED_MINUTES: 1, ELAPSED_HOURS: 60, ELAPSED_DAYS: 1440, ELAPSED_WEEKS: 10080,
    ELAPSED_MONTHS: 43200, ELAPSED_YEARS: 525600
  };

  var ui = {
    files: document.getElementById('compare-files'),
    status: document.getElementById('compare-status'),
    summary: document.getElementById('compare-summary'),
    tabs: document.getElementById('compare-tabs'),
    body: document.getElementById('compare-body'),
    note: document.getElementById('compare-note'),
    swap: document.getElementById('compare-swap')
  };

  var NOTE = ui.note.textContent;
  var XER_NOTE = ' Comparing Primavera .xer versions has not been checked against real updates yet - verify anything surprising in P6.';

  var UNITS = {
    MINUTES: 'm', HOURS: 'h', DAYS: 'd', WEEKS: 'w', MONTHS: 'mo', YEARS: 'y',
    ELAPSED_MINUTES: 'em', ELAPSED_HOURS: 'eh', ELAPSED_DAYS: 'ed', ELAPSED_WEEKS: 'ew',
    ELAPSED_MONTHS: 'emo', ELAPSED_YEARS: 'ey', PERCENT: '%', ELAPSED_PERCENT: 'e%'
  };

  var LINKS = { FINISH_START: 'FS', START_START: 'SS', FINISH_FINISH: 'FF', START_FINISH: 'SF' };

  var current = null;
  var currentName = '';
  var other = null;
  var otherName = '';
  var swapped = false;
  var guessed = false;
  var report = null;
  var tab = 'edited';

  document.addEventListener('mpp:loaded', function (event) {
    current = event.detail.contract;
    currentName = event.detail.fileName || 'this plan';
  });

  open.addEventListener('click', function () {
    if (!current) { return; }
    if (menu && open.getAttribute('aria-expanded') === 'true') { return; }

    window.MppSignal('compare_open:' + viewer);

    if (menu) {
      loadSaved();
      return;
    }
    chooseFile();
  });

  if (pickFile) {
    pickFile.addEventListener('click', function () {
      closeMenu();
      chooseFile();
    });
  }

  input.addEventListener('change', function () {
    var file = input.files[0];
    if (!file) { return; }

    window.MppSignal('compare_from:file');
    begin(file.name);

    upload(file).then(compareWith).catch(fail);
  });

  ui.swap.addEventListener('click', function () {
    if (!other) { return; }
    swapped = !swapped;
    guessed = false;
    run().catch(fail);
  });

  exportButton.addEventListener('click', function () {
    if (!report || !other) { return; }
    if (!menu) {
      locked();
      return;
    }

    var pair = versions();
    exportButton.disabled = true;

    fetch('/api/v1/compare/xlsx?previous_name=' + encodeURIComponent(pair.previousName) + '&current_name=' + encodeURIComponent(pair.currentName), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'viewmpp' },
      body: JSON.stringify({ previous: pair.previous, current: pair.current })
    }).then(function (response) {
      if (response.status === 401 || response.status === 403) {
        locked();
        return null;
      }
      if (!response.ok) { return window.MppRefusal(response); }
      return response.blob();
    }).then(function (blob) {
      if (!blob) { return; }
      var a = document.createElement('a');
      a.href = URL.createObjectURL(blob);
      a.download = (String(pair.currentName).replace(/\.[^.]*$/, '') || 'plan') + ' changes.xlsx';
      a.click();
      URL.revokeObjectURL(a.href);
    }).catch(fail).then(function () {
      exportButton.disabled = false;
    });
  });

  function locked() {
    window.MppSignal('compare_export_locked:' + viewer);
    if (gate && typeof gate.showModal === 'function') { gate.showModal(); }
  }

  function versions() {
    if (swapped) {
      return { previous: current, current: other, previousName: currentName, currentName: otherName };
    }
    return { previous: other, current: current, previousName: otherName, currentName: currentName };
  }

  window.MppCompare = {
    show: function (previous, previousName) {
      if (!current) { return Promise.resolve(); }
      begin(previousName || 'previous version');
      other = previous;
      return run().catch(fail);
    }
  };

  function chooseFile() {
    input.value = '';
    input.click();
  }

  function closeMenu() {
    if (!menu) { return; }
    menu.querySelector('[data-menu-panel]').classList.add('is-hidden');
    open.setAttribute('aria-expanded', 'false');
  }

  function begin(name) {
    other = null;
    otherName = name;
    swapped = false;
    guessed = false;
    report = null;
    tab = 'edited';

    reset();
    if (typeof dialog.showModal === 'function' && !dialog.open) { dialog.showModal(); }
    status('Reading ' + name + '...');
  }

  function compareWith(contract) {
    other = contract;

    if (progress(other) > progress(current) + 0.01) {
      swapped = true;
      guessed = true;
    }

    return run();
  }

  function loadSaved() {
    if (savedLoaded) { return; }
    savedLoaded = true;

    note(saved, 'Loading your plans...');

    fetch('/api/v1/projects', { headers: { 'X-Requested-With': 'viewmpp' } })
      .then(function (response) {
        if (!response.ok) { return window.MppRefusal(response); }
        return response.json();
      })
      .then(function (plans) {
        saved.textContent = '';

        plans.filter(function (plan) { return plan.id !== openProject; }).forEach(function (plan) {
          var item = document.createElement('button');
          item.type = 'button';
          item.className = 'menu__item compare-pick__plan';

          var name = document.createElement('span');
          name.className = 'compare-pick__name';
          name.textContent = plan.file_name || 'Untitled plan';

          var when = document.createElement('span');
          when.className = 'compare-pick__date';
          when.textContent = String(plan.created_at || '').slice(0, 10);

          item.appendChild(name);
          item.appendChild(when);
          item.addEventListener('click', function () { pickSaved(plan); });
          saved.appendChild(item);
        });

        if (!saved.children.length) { note(saved, 'No other saved plans yet.'); }
      })
      .catch(function () {
        savedLoaded = false;
        note(saved, 'Your plans could not be loaded.');
      });
  }

  function pickSaved(plan) {
    closeMenu();
    window.MppSignal('compare_from:project');
    begin(plan.file_name || 'saved plan');

    fetch('/api/v1/projects/' + encodeURIComponent(plan.id))
      .then(function (response) {
        if (!response.ok) { return window.MppRefusal(response); }
        return response.json();
      })
      .then(compareWith)
      .catch(fail);
  }

  function note(box, text) {
    box.textContent = '';
    var p = document.createElement('p');
    p.className = 'compare-pick__note';
    p.textContent = text;
    box.appendChild(p);
  }

  function progress(contract) {
    var tasks = (contract && contract.tasks) || [];
    var roots = tasks.filter(function (t) { return t.parent_id === null; });

    if (roots.length === 1 && roots[0].is_summary) { return roots[0].percent_complete || 0; }

    var done = 0;
    var weight = 0;
    tasks.forEach(function (t) {
      if (t.is_summary) { return; }
      var w = t.duration ? t.duration.value * (MINUTES[t.duration.units] || 0) : 0;
      if (!(w > 0)) { w = 1; }
      done += (t.percent_complete || 0) * w;
      weight += w;
    });

    return weight ? done / weight : 0;
  }

  function upload(file) {
    return fetch('/api/v1/upload?store=no&name=' + encodeURIComponent(file.name), {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream', 'X-Requested-With': 'viewmpp' },
      body: file
    }).then(function (response) {
      if (!response.ok) { return window.MppRefusal(response); }
      return response.json();
    });
  }

  function run() {
    var pair = versions();

    ui.files.textContent = 'Older: ' + pair.previousName + '  →  Newer: ' + pair.currentName +
      (guessed ? '  ·  order guessed from progress, swap if wrong' : '');
    ui.swap.disabled = true;
    exportButton.disabled = true;
    status('Comparing...');

    return fetch('/api/v1/compare', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'viewmpp' },
      body: JSON.stringify({ previous: pair.previous, current: pair.current })
    }).then(function (response) {
      if (!response.ok) { return window.MppRefusal(response); }
      return response.json();
    }).then(function (result) {
      report = result;
      ui.swap.disabled = false;
      exportButton.disabled = false;
      render();
    });
  }

  function fail(err) {
    ui.swap.disabled = !other;
    status(err.message);
  }

  function reset() {
    ui.files.textContent = '';
    ui.summary.textContent = '';
    ui.tabs.textContent = '';
    ui.body.textContent = '';
    ui.swap.disabled = true;
    exportButton.disabled = true;
    ui.note.textContent = NOTE;
    status('');
  }

  function status(text) {
    ui.status.textContent = text;
    ui.status.hidden = !text;
  }

  function render() {
    var counts = {
      edited: report.added.length + report.removed.length + report.edited.length,
      moved: report.moved.length + report.rollups.length,
      links: report.links.length,
      critical: report.entered_critical.length + report.left_critical.length
    };
    var total = counts.edited + counts.moved + counts.links + counts.critical;

    status(total === 0 && !report.start && !report.finish ? 'These two files hold the same plan.' : '');
    ui.note.textContent = NOTE + (/\.xer$/i.test(currentName) || /\.xer$/i.test(otherName) ? XER_NOTE : '');

    renderSummary();
    renderTabs(counts);
    renderBody();
  }

  function renderSummary() {
    ui.summary.textContent = '';

    ui.summary.appendChild(fact('Project finish', report.finish ? dateChange(report.finish) : 'unchanged', report.finish));
    if (report.start) { ui.summary.appendChild(fact('Project start', dateChange(report.start), report.start)); }

    var slipped = report.moved.filter(function (m) { return m.finish && m.finish.minutes > 0; }).length;

    ui.summary.appendChild(fact('Added', String(report.added.length)));
    ui.summary.appendChild(fact('Removed', String(report.removed.length)));
    ui.summary.appendChild(fact('Edited', String(report.edited.length)));
    ui.summary.appendChild(fact('Finishing later', String(slipped)));
  }

  function fact(label, value, shift) {
    var box = document.createElement('div');
    box.className = 'compare__fact';

    var name = document.createElement('span');
    name.className = 'compare__label';
    name.textContent = label;

    var text = document.createElement('b');
    text.textContent = value;
    if (shift) { text.className = tone(shift.minutes); }

    box.appendChild(name);
    box.appendChild(text);
    return box;
  }

  function renderTabs(counts) {
    ui.tabs.textContent = '';

    [
      ['edited', 'What changed', counts.edited],
      ['moved', 'What moved', counts.moved],
      ['links', 'Links', counts.links],
      ['critical', 'Critical path', counts.critical]
    ].forEach(function (item) {
      var button = document.createElement('button');
      button.type = 'button';
      button.className = 'compare__tab' + (item[0] === tab ? ' is-active' : '');
      button.setAttribute('role', 'tab');
      button.setAttribute('aria-selected', String(item[0] === tab));
      button.textContent = item[1] + ' (' + item[2] + ')';
      button.addEventListener('click', function () {
        tab = item[0];
        renderTabs(counts);
        renderBody();
      });
      ui.tabs.appendChild(button);
    });
  }

  function renderBody() {
    ui.body.textContent = '';
    ui.body.scrollTop = 0;

    if (tab === 'edited') {
      section('Added', report.added, function (ref) { return [ref.row, label(ref), '']; });
      section('Removed - rows as they were in the previous version', report.removed, function (ref) { return [ref.row, label(ref), '']; });
      section('Edited', report.edited, function (edit) { return [edit.task.row, label(edit.task), editText(edit)]; });
    } else if (tab === 'moved') {
      table(['Row', 'Task', 'Start', 'Finish'], report.moved, moveRow);
      if (report.rollups.length) {
        var more = document.createElement('details');
        var title = document.createElement('summary');
        title.textContent = 'Summary tasks (' + report.rollups.length + ') - their dates follow the tasks inside them';
        more.appendChild(title);
        more.appendChild(buildTable(['Row', 'Task', 'Start', 'Finish'], report.rollups, moveRow));
        ui.body.appendChild(more);
      }
    } else if (tab === 'links') {
      table(['From', 'To', 'Before', 'Now'], report.links, function (change) {
        return [label(change.predecessor), label(change.successor), linkText(change.old), linkText(change.new)];
      });
    } else {
      section('Became critical', report.entered_critical, function (ref) { return [ref.row, label(ref), '']; });
      section('No longer critical', report.left_critical, function (ref) { return [ref.row, label(ref), '']; });
    }

    if (!ui.body.children.length) {
      var empty = document.createElement('p');
      empty.className = 'compare__empty';
      empty.textContent = 'Nothing here.';
      ui.body.appendChild(empty);
    }
  }

  function section(title, items, cells) {
    if (!items.length) { return; }

    var heading = document.createElement('h3');
    heading.className = 'compare__section';
    heading.textContent = title + ' (' + items.length + ')';
    ui.body.appendChild(heading);
    ui.body.appendChild(buildTable(['Row', 'Task', ''], items, cells));
  }

  function table(headers, items, cells) {
    if (!items.length) { return; }
    ui.body.appendChild(buildTable(headers, items, cells));
  }

  function buildTable(headers, items, cells) {
    var t = document.createElement('table');
    t.className = 'compare__table';

    var head = t.createTHead().insertRow();
    headers.forEach(function (h) {
      var th = document.createElement('th');
      th.textContent = h;
      head.appendChild(th);
    });

    var body = t.createTBody();
    items.forEach(function (item) {
      var row = body.insertRow();
      cells(item).forEach(function (value) {
        var td = row.insertCell();
        if (value && value.nodeType) { td.appendChild(value); } else { td.textContent = value === undefined || value === null ? '' : String(value); }
      });
    });

    return t;
  }

  function moveRow(move) {
    return [move.task.row, label(move.task), shiftCell(move.start), shiftCell(move.finish)];
  }

  function shiftCell(shift) {
    if (!shift) { return '—'; }

    var cell = document.createElement('span');
    cell.appendChild(dateSpan(shift.old));
    cell.appendChild(document.createTextNode(' → '));
    cell.appendChild(dateSpan(shift.new));
    cell.appendChild(document.createTextNode(' '));

    var delta = document.createElement('b');
    delta.className = tone(shift.minutes);
    delta.textContent = duration(shift.minutes);
    cell.appendChild(delta);

    return cell;
  }

  function dateSpan(value) {
    var span = document.createElement('span');
    span.className = 'compare__date';
    span.textContent = day(value);
    return span;
  }

  function dateChange(shift) {
    return day(shift.old) + ' → ' + day(shift.new) + ' (' + duration(shift.minutes) + ')';
  }

  function label(ref) {
    return (ref.outline ? ref.outline + '  ' : '') + (ref.name || 'Untitled task');
  }

  function editText(edit) {
    var parts = [];
    if (edit.name) { parts.push('Renamed from “' + edit.name.old + '”'); }
    if (edit.parent) { parts.push('Moved from ' + phase(edit.parent.old) + ' to ' + phase(edit.parent.new)); }
    if (edit.duration) { parts.push('Duration ' + amount(edit.duration.old) + ' → ' + amount(edit.duration.new)); }
    if (edit.percent_complete) { parts.push('Complete ' + number(edit.percent_complete.old) + '% → ' + number(edit.percent_complete.new) + '%'); }
    if (edit.milestone) { parts.push(edit.milestone.new ? 'Became a milestone' : 'No longer a milestone'); }
    return parts.join('; ');
  }

  function phase(ref) {
    return ref ? '“' + ref.name + '”' : 'the top level';
  }

  function linkText(link) {
    if (!link) { return '—'; }
    var text = LINKS[link.type] || link.type;
    if (link.lag && link.lag.value) {
      text += ' ' + (link.lag.value > 0 ? '+' : '') + number(link.lag.value) + (UNITS[link.lag.units] || link.lag.units);
    }
    return text;
  }

  function amount(a) {
    if (!a) { return '0'; }
    return number(a.value) + ' ' + (UNITS[a.units] || a.units);
  }

  function number(n) {
    return String(Math.round(n * 100) / 100);
  }

  function day(value) {
    return value ? value.slice(0, 10) : '—';
  }

  function duration(minutes) {
    var sign = minutes > 0 ? '+' : minutes < 0 ? '−' : '';
    var abs = Math.abs(minutes);
    if (abs >= 1440) { return sign + number(abs / 1440) + ' d'; }
    if (abs >= 60) { return sign + number(abs / 60) + ' h'; }
    return sign + abs + ' min';
  }

  function tone(minutes) {
    if (minutes > 0) { return 'compare__late'; }
    if (minutes < 0) { return 'compare__early'; }
    return '';
  }
})();
