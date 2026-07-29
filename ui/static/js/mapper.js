window.MppMapper = (function () {
  'use strict';

  var LINK_TYPE = {
    FINISH_START: '0',
    START_START: '1',
    FINISH_FINISH: '2',
    START_FINISH: '3'
  };

  var WEEKDAY = {
    SUNDAY: 0, MONDAY: 1, TUESDAY: 2, WEDNESDAY: 3,
    THURSDAY: 4, FRIDAY: 5, SATURDAY: 6
  };

  function key(id) {
    return (id === null || id === undefined) ? 0 : 't' + id;
  }

  function toDate(value) {
    return value ? new Date(value) : null;
  }

  function taskType(task) {
    if (task.is_milestone) { return 'milestone'; }
    return 'task';
  }

  function toModel(contract) {
    var resourceNames = {};
    (contract.resources || []).forEach(function (resource) {
      resourceNames[resource.id] = resource.name;
    });

    var data = (contract.tasks || []).map(function (task) {
      return {
        id: key(task.id),
        parent: key(task.parent_id),
        text: task.name || '',
        start_date: toDate(task.start),
        end_date: toDate(task.finish),
        progress: (task.percent_complete || 0) / 100,
        type: taskType(task),
        open: true,
        $contract: task
      };
    });

    var links = (contract.relations || []).map(function (relation) {
      return {
        id: 'l' + relation.id,
        source: key(relation.predecessor_id),
        target: key(relation.successor_id),
        type: LINK_TYPE[relation.type] || LINK_TYPE.FINISH_START,
        $contract: relation
      };
    });

    return {
      data: data,
      links: links,
      calendar: calendar(contract.calendar),
      index: index(contract, resourceNames),
      project: contract.project || {}
    };
  }

  function calendar(source) {
    var nonWorking = {};
    ((source && source.non_working_weekdays) || []).forEach(function (day) {
      if (WEEKDAY.hasOwnProperty(day)) {
        nonWorking[WEEKDAY[day]] = true;
      }
    });

    var exceptions = ((source && source.exceptions) || []).map(function (ex) {
      return { from: ex.from.slice(0, 10), to: ex.to.slice(0, 10), working: ex.working };
    });

    return { name: (source && source.name) || null, nonWorking: nonWorking, exceptions: exceptions };
  }

  function index(contract, resourceNames) {
    var predecessors = {};
    var successors = {};

    (contract.relations || []).forEach(function (relation) {
      (predecessors[relation.successor_id] = predecessors[relation.successor_id] || []).push(relation);
      (successors[relation.predecessor_id] = successors[relation.predecessor_id] || []).push(relation);
    });

    var names = {};
    (contract.tasks || []).forEach(function (task) {
      names[task.id] = task.name;
    });

    return {
      predecessors: predecessors,
      successors: successors,
      taskName: names,
      resourceName: resourceNames
    };
  }

  return { toModel: toModel, key: key };
})();
